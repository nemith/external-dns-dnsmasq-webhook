package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("external-dns-dnsmasq-webhook %s (%s, %s)\n", version, commit, buildDate)
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: external-dns-dnsmasq-webhook [--version]")
		os.Exit(2)
	}

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	p, err := newProvider(cfg)
	if err != nil {
		slog.Error("failed to load provider", "error", err)
		os.Exit(1)
	}
	if err := p.reconcile(context.Background()); err != nil {
		slog.Error("failed to reconcile dnsmasq at startup", "error", err)
		os.Exit(1)
	}

	api := &apiServer{provider: p, domain: cfg.Domain, allowed: cfg.AllowedCIDRs}
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           api.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP shutdown failed", "error", err)
		}
	}()

	slog.Info("dnsmasq ExternalDNS webhook listening", "address", cfg.ListenAddress, "domain", cfg.Domain, "version", version)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
