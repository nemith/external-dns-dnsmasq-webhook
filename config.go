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
	ListenAddress     string
	Domain            string
	AllowedCIDRs      []netip.Prefix
	StateFile         string
	DNSMasqConfigFile string
	DNSMasqBinary     string
	SystemctlBinary   string
	DNSMasqService    string
}

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]*$`)

type environmentConfig struct {
	ListenAddress   string `envconfig:"LISTEN_ADDRESS" required:"true"`
	AllowedCIDRs    string `envconfig:"ALLOWED_CIDRS" required:"true"`
	Domain          string `envconfig:"DOMAIN" required:"true"`
	StateFile       string `envconfig:"STATE_FILE" required:"true"`
	ConfigFile      string `envconfig:"CONFIG_FILE" required:"true"`
	DNSMasqBinary   string `envconfig:"DNSMASQ_BINARY" required:"true"`
	SystemctlBinary string `envconfig:"SYSTEMCTL_BINARY" required:"true"`
	DNSMasqService  string `envconfig:"DNSMASQ_SERVICE" required:"true"`
}

func loadConfig() (config, error) {
	var environment environmentConfig
	if err := envconfig.Process("DNSMASQ_WEBHOOK", &environment); err != nil {
		return config{}, err
	}

	cfg := config{
		ListenAddress:     strings.TrimSpace(environment.ListenAddress),
		Domain:            strings.ToLower(strings.TrimSuffix(strings.TrimSpace(environment.Domain), ".")),
		StateFile:         strings.TrimSpace(environment.StateFile),
		DNSMasqConfigFile: strings.TrimSpace(environment.ConfigFile),
		DNSMasqBinary:     strings.TrimSpace(environment.DNSMasqBinary),
		SystemctlBinary:   strings.TrimSpace(environment.SystemctlBinary),
		DNSMasqService:    strings.TrimSpace(environment.DNSMasqService),
	}

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

	for _, value := range strings.Split(environment.AllowedCIDRs, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_ALLOWED_CIDRS: %w", err)
		}
		cfg.AllowedCIDRs = append(cfg.AllowedCIDRs, prefix)
	}
	return cfg, nil
}
