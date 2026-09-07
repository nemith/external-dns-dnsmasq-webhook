# external-dns-dnsmasq-webhook

An [ExternalDNS webhook provider](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/webhook-provider.md) for dnsmasq. It supports `A`, `AAAA`, and `CNAME` records within one configured domain.

Updates are written to a dedicated dnsmasq include, validated with `dnsmasq --test`, and applied by restarting dnsmasq. Failed updates restore the previous configuration and provider state.

## Install

Download the appropriate `.deb` or `.rpm` from a GitHub release, then configure:

- Debian: `/etc/default/external-dns-dnsmasq-webhook`
- RPM: `/etc/sysconfig/external-dns-dnsmasq-webhook`

All environment variables in that file are required. At minimum, set the listen address, allowed client CIDRs, and managed domain before starting the service:

```sh
sudo systemctl enable --now external-dns-dnsmasq-webhook.service
```

## ExternalDNS

Configure ExternalDNS with:

```text
--provider=webhook
--webhook-provider-url=http://dns-router.example.test:8888
--registry=noop
--domain-filter=example.test
```

The webhook has no application-layer authentication. Bind it to a trusted interface, restrict it with a firewall, and limit `DNSMASQ_WEBHOOK_ALLOWED_CIDRS` to the Kubernetes node network.

## Build

```sh
goreleaser release --snapshot --clean
```

Pushing a `v*` tag publishes Linux amd64 and arm64 archives, Debian packages, RPMs, and checksums.
