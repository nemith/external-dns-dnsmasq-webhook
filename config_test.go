package main

import (
	"strings"
	"testing"
)

func setValidEnvironment(t *testing.T) {
	t.Helper()
	values := map[string]string{
		"DNSMASQ_WEBHOOK_LISTEN_ADDRESS":   "192.0.2.1:8888",
		"DNSMASQ_WEBHOOK_ALLOWED_CIDRS":    "198.51.100.0/24",
		"DNSMASQ_WEBHOOK_DOMAIN":           "example.test",
		"DNSMASQ_WEBHOOK_STATE_FILE":       "/var/lib/external-dns-dnsmasq-webhook/records.json",
		"DNSMASQ_WEBHOOK_CONFIG_FILE":      "/var/lib/external-dns-dnsmasq-webhook/records.conf",
		"DNSMASQ_WEBHOOK_DNSMASQ_BINARY":   "/usr/sbin/dnsmasq",
		"DNSMASQ_WEBHOOK_SYSTEMCTL_BINARY": "/usr/bin/systemctl",
		"DNSMASQ_WEBHOOK_DNSMASQ_SERVICE":  "dnsmasq.service",
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
}

func TestLoadConfigRequiresSiteSettings(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("DNSMASQ_WEBHOOK_DOMAIN", "")
	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "DNSMASQ_WEBHOOK_DOMAIN is required") {
		t.Fatalf("got error %v", err)
	}
}

func TestLoadConfig(t *testing.T) {
	setValidEnvironment(t)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Domain != "example.test" || cfg.ListenAddress != "192.0.2.1:8888" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}
