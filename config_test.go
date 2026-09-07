package main

import (
	"os"
	"strings"
	"testing"
)

func setValidEnvironment(t *testing.T) {
	t.Helper()
	values := map[string]string{
		"DNSMASQ_WEBHOOK_LISTEN_ADDRESS": "192.0.2.1:8888",
		"DNSMASQ_WEBHOOK_ALLOWED_CIDRS":  "198.51.100.0/24",
		"DNSMASQ_WEBHOOK_DOMAIN":         "example.test",
	}
	for _, name := range []string{
		"DNSMASQ_WEBHOOK_STATE_FILE",
		"DNSMASQ_WEBHOOK_CONFIG_FILE",
		"DNSMASQ_WEBHOOK_DNSMASQ_BINARY",
		"DNSMASQ_WEBHOOK_SYSTEMCTL_BINARY",
		"DNSMASQ_WEBHOOK_DNSMASQ_SERVICE",
	} {
		unsetEnvironment(t, name)
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
}

func TestLoadConfigRequiresSiteSettings(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("DNSMASQ_WEBHOOK_DOMAIN", "")
	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "DNSMASQ_WEBHOOK_DOMAIN") {
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
	if cfg.StateFile != "/var/lib/external-dns-dnsmasq-webhook/records.json" ||
		cfg.DNSMasqConfigFile != "/var/lib/external-dns-dnsmasq-webhook/records.conf" ||
		cfg.DNSMasqBinary != "/usr/sbin/dnsmasq" ||
		cfg.SystemctlBinary != "/usr/bin/systemctl" ||
		cfg.DNSMasqService != "dnsmasq.service" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestLoadConfigRejectsUnprefixedAliases(t *testing.T) {
	setValidEnvironment(t)
	unsetEnvironment(t, "DNSMASQ_WEBHOOK_DOMAIN")
	t.Setenv("DOMAIN", "example.test")

	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "DNSMASQ_WEBHOOK_DOMAIN") {
		t.Fatalf("got error %v", err)
	}
}

func unsetEnvironment(t *testing.T, name string) {
	t.Helper()
	value, exists := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(name, value)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}
