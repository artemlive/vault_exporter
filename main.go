package main

import (
	"net/http"
	_ "net/http/pprof"

	vault_api "github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "vault"
)

var (
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last query of Vault successful.",
		nil, nil,
	)
	initialized = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "initialized"),
		"Is the Vault initialised (according to this node).",
		nil, nil,
	)
	sealed = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "sealed"),
		"Is the Vault node sealed.",
		nil, nil,
	)
	standby = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "standby"),
		"Is this Vault node in standby.",
		nil, nil,
	)
	info = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "info"),
		"Version of this Vault node.",
		[]string{"version", "cluster_name", "cluster_id"}, nil,
	)
)

// Exporter collects Vault health from the given server and exports them using
// the Prometheus metrics package.
type Exporter struct {
	client *vault_api.Client
}

// NewExporter returns an initialized Exporter.
func NewExporter(config vault_api.Config) (*Exporter, error) {
	client, err := vault_api.NewClient(&config)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		client: client,
	}, nil
}

// Describe describes all the metrics ever exported by the Vault exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- initialized
	ch <- sealed
	ch <- standby
	ch <- info
}

func bool2float(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Collect fetches the stats from configured Vault and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	health, err := e.client.Sys().Health()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		log.Errorf("Failted to collect health from Vault server: %v", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,
	)
	ch <- prometheus.MustNewConstMetric(
		initialized, prometheus.GaugeValue, bool2float(health.Initialized),
	)
	ch <- prometheus.MustNewConstMetric(
		sealed, prometheus.GaugeValue, bool2float(health.Sealed),
	)
	ch <- prometheus.MustNewConstMetric(
		standby, prometheus.GaugeValue, bool2float(health.Standby),
	)
	ch <- prometheus.MustNewConstMetric(
		info, prometheus.GaugeValue, 1, health.Version, health.ClusterName, health.ClusterID,
	)
}

func init() {
	prometheus.MustRegister(version.NewCollector("vault_exporter"))
}

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address",
			"Address to listen on for web interface and telemetry.").
			Default(":9410").String()
		metricsPath = kingpin.Flag("web.telemetry-path",
			"Path under which to expose metrics.").
			Default("/metrics").String()
		vaultAddress = kingpin.Flag("cfg.vault-server",
			"Vault server address").
			Default("http://127.0.0.1:8200/").String()
	)

	kingpin.Version(version.Print("vault_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting vault_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infoln("Vault address", *vaultAddress)

	exporter, err := NewExporter(vault_api.Config{ Address: *vaultAddress})
	if err != nil {
		log.Fatalln(err)
	}
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
             <head><title>Vault Exporter</title></head>
             <body>
             <h1>Vault Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             <h2>Build</h2>
             <pre>` + version.Info() + ` ` + version.BuildContext() + `</pre>
             </body>
             </html>`))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
