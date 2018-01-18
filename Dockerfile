FROM alpine:3.5
ADD vault_exporter /usr/bin
ENTRYPOINT ["/usr/bin/vault_exporter"]
