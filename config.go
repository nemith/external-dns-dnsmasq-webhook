package main

import (
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	ListenAddress     string         `envconfig:"DNSMASQ_WEBHOOK_LISTEN_ADDRESS" required:"true"`
	AllowedCIDRs      []netip.Prefix `envconfig:"DNSMASQ_WEBHOOK_ALLOWED_CIDRS" required:"true"`
	Domain            string         `envconfig:"DNSMASQ_WEBHOOK_DOMAIN" required:"true"`
	StateFile         string         `envconfig:"DNSMASQ_WEBHOOK_STATE_FILE" default:"/var/lib/external-dns-dnsmasq-webhook/records.json"`
	DNSMasqConfigFile string         `envconfig:"DNSMASQ_WEBHOOK_CONFIG_FILE" default:"/var/lib/external-dns-dnsmasq-webhook/records.conf"`
	DNSMasqBinary     string         `envconfig:"DNSMASQ_WEBHOOK_DNSMASQ_BINARY" default:"/usr/sbin/dnsmasq"`
	SystemctlBinary   string         `envconfig:"DNSMASQ_WEBHOOK_SYSTEMCTL_BINARY" default:"/usr/bin/systemctl"`
	DNSMasqService    string         `envconfig:"DNSMASQ_WEBHOOK_DNSMASQ_SERVICE" default:"dnsmasq.service"`
}

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]*$`)

func loadConfig() (config, error) {
	var cfg config
	if err := envconfig.Process("", &cfg); err != nil {
		return config{}, err
	}

	cfg.ListenAddress = strings.TrimSpace(cfg.ListenAddress)
	cfg.Domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(cfg.Domain), "."))
	cfg.StateFile = strings.TrimSpace(cfg.StateFile)
	cfg.DNSMasqConfigFile = strings.TrimSpace(cfg.DNSMasqConfigFile)
	cfg.DNSMasqBinary = strings.TrimSpace(cfg.DNSMasqBinary)
	cfg.SystemctlBinary = strings.TrimSpace(cfg.SystemctlBinary)
	cfg.DNSMasqService = strings.TrimSpace(cfg.DNSMasqService)

	host, portValue, err := net.SplitHostPort(cfg.ListenAddress)
	if err != nil {
		return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_LISTEN_ADDRESS: %w", err)
	}
	if _, err := netip.ParseAddr(host); err != nil {
		return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_LISTEN_ADDRESS must use an IP address: %w", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_LISTEN_ADDRESS has an invalid port")
	}
	if err := validateDNSName(cfg.Domain, false); err != nil {
		return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_DOMAIN: %w", err)
	}
	if len(cfg.AllowedCIDRs) == 0 {
		return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_ALLOWED_CIDRS must contain at least one network")
	}
	for name, path := range map[string]string{
		"DNSMASQ_WEBHOOK_STATE_FILE":       cfg.StateFile,
		"DNSMASQ_WEBHOOK_CONFIG_FILE":      cfg.DNSMasqConfigFile,
		"DNSMASQ_WEBHOOK_DNSMASQ_BINARY":   cfg.DNSMasqBinary,
		"DNSMASQ_WEBHOOK_SYSTEMCTL_BINARY": cfg.SystemctlBinary,
	} {
		if !filepath.IsAbs(path) {
			return config{}, fmt.Errorf("%s must be an absolute path", name)
		}
	}
	if !serviceNamePattern.MatchString(cfg.DNSMasqService) {
		return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_DNSMASQ_SERVICE is invalid")
	}

	return cfg, nil
}
