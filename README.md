# dnsmasq ExternalDNS webhook

This service lets the standard Kubernetes ExternalDNS controller manage a
dedicated dnsmasq configuration file on `dns-router` (`192.0.2.1`). It implements
version 1 of the ExternalDNS webhook API and accepts `A`, `AAAA`, and `CNAME`
records only within `example.test`.

The service never reads or changes other dnsmasq configuration. A static file in
`/etc/dnsmasq.d` includes its last-known-good generated file from `/var/lib`.
Every update is written atomically, checked with `dnsmasq --test`, and followed by
`systemctl restart dnsmasq`. If validation or restart fails, both the generated
configuration and provider state are rolled back.

## Install on dns-router

GoReleaser builds amd64 and arm64 Linux archives and Debian packages through
nFPM. For a local snapshot build:

```sh
goreleaser release --snapshot --clean
scp dist/external-dns-dnsmasq-webhook_*_amd64.deb operator@192.0.2.1:/tmp/external-dns-dnsmasq-webhook.deb
ssh operator@192.0.2.1 sudo apt install /tmp/external-dns-dnsmasq-webhook.deb
```

The package preserves the generated records across upgrades, registers the
configuration files as conffiles, and starts the systemd service. Pushing a
`v*` tag in the standalone GitHub repository runs the release workflow and
publishes both architectures, their Debian packages, and checksums.

For a manual installation without `dpkg`, build and stage the files, then run
the remaining commands in the block on `dns-router`:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o external-dns-dnsmasq-webhook .
scp external-dns-dnsmasq-webhook deploy/* operator@192.0.2.1:/tmp/
sudo install -m 0755 /tmp/external-dns-dnsmasq-webhook /usr/sbin/external-dns-dnsmasq-webhook
sudo install -m 0644 /tmp/external-dns-dnsmasq-webhook.service /etc/systemd/system/external-dns-dnsmasq-webhook.service
sudo install -m 0644 /tmp/external-dns-dnsmasq-webhook.default /etc/default/external-dns-dnsmasq-webhook
sudo install -d -m 0750 /var/lib/external-dns-dnsmasq-webhook
sudo touch /var/lib/external-dns-dnsmasq-webhook/records.conf
sudo chmod 0644 /var/lib/external-dns-dnsmasq-webhook/records.conf
sudo install -m 0644 /tmp/90-kubernetes-external-dns.conf /etc/dnsmasq.d/90-kubernetes-external-dns.conf
sudo /usr/sbin/dnsmasq --test
sudo systemctl daemon-reload
sudo systemctl enable --now external-dns-dnsmasq-webhook.service
curl http://192.0.2.1:8888/healthz
```

The active `dns-router` rules route the Kubernetes node network through `input_dns_clients`. Add the
following rule to the Ansible source of truth for that chain, then apply it. For
a temporary test before the next firewall reload:

```sh
sudo nft add rule inet filter input_dns_clients tcp dport 8888 accept
```

The application independently restricts clients to `198.51.100.0/24`. Install
and start this service, and persist the firewall rule, before reconciling the
`external-dns` Kubernetes resources.

All application settings are required. The checked-in
`deploy/external-dns-dnsmasq-webhook.default` file specifies them explicitly for
`dns-router`; the binary exits at startup if any are missing or invalid.

## Behavior and limitations

- Existing dnsmasq configuration remains unmanaged. ExternalDNS owns only
  `/var/lib/external-dns-dnsmasq-webhook/records.conf` and its JSON state file. The
  corresponding file under `/etc/dnsmasq.d` is a static include.
- `registry=noop` is intentional: ownership is the dedicated generated file,
  so TXT ownership records are neither necessary nor supported by dnsmasq.
- A DNS wildcard such as `*.apps.example.test` is rendered with dnsmasq's
  `address=/apps.example.test/IP` syntax. dnsmasq also answers the zone apex for
  that directive; an exact `host-record` elsewhere in the dnsmasq configuration
  takes precedence.
- Source-address filtering is not cryptographic authentication. Keep TCP/8888
  restricted to the Kubernetes node subnet at `dns-router`; do not expose it on WAN.
