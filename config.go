package main

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

func loadConfig() (config, error) {
	values := make(map[string]string)
	for _, name := range []string{
		"DNSMASQ_WEBHOOK_LISTEN_ADDRESS",
		"DNSMASQ_WEBHOOK_ALLOWED_CIDRS",
		"DNSMASQ_WEBHOOK_DOMAIN",
		"DNSMASQ_WEBHOOK_STATE_FILE",
		"DNSMASQ_WEBHOOK_CONFIG_FILE",
		"DNSMASQ_WEBHOOK_DNSMASQ_BINARY",
		"DNSMASQ_WEBHOOK_SYSTEMCTL_BINARY",
		"DNSMASQ_WEBHOOK_DNSMASQ_SERVICE",
	} {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			return config{}, fmt.Errorf("%s is required", name)
		}
		values[name] = value
	}

	cfg := config{
		ListenAddress:     values["DNSMASQ_WEBHOOK_LISTEN_ADDRESS"],
		Domain:            strings.ToLower(strings.TrimSuffix(values["DNSMASQ_WEBHOOK_DOMAIN"], ".")),
		StateFile:         values["DNSMASQ_WEBHOOK_STATE_FILE"],
		DNSMasqConfigFile: values["DNSMASQ_WEBHOOK_CONFIG_FILE"],
		DNSMasqBinary:     values["DNSMASQ_WEBHOOK_DNSMASQ_BINARY"],
		SystemctlBinary:   values["DNSMASQ_WEBHOOK_SYSTEMCTL_BINARY"],
		DNSMasqService:    values["DNSMASQ_WEBHOOK_DNSMASQ_SERVICE"],
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

	for _, value := range strings.Split(values["DNSMASQ_WEBHOOK_ALLOWED_CIDRS"], ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return config{}, fmt.Errorf("DNSMASQ_WEBHOOK_ALLOWED_CIDRS: %w", err)
		}
		cfg.AllowedCIDRs = append(cfg.AllowedCIDRs, prefix)
	}
	return cfg, nil
}
