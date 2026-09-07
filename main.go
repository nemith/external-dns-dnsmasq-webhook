package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

type buildMetadata struct {
	version string
	commit  string
	date    string
}

func main() {
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	build := currentBuildMetadata()
	if *showVersion {
		fmt.Printf("external-dns-dnsmasq-webhook %s (%s, %s)\n", build.version, build.commit, build.date)
		return
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

	api := newAPIServer(p, cfg.Domain, cfg.AllowedCIDRs)
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           api,
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

	slog.Info("dnsmasq ExternalDNS webhook listening", "address", cfg.ListenAddress, "domain", cfg.Domain, "version", build.version)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

func currentBuildMetadata() buildMetadata {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return buildMetadata{version: "dev", commit: "unknown", date: "unknown"}
	}
	return metadataFromBuildInfo(info)
}

func metadataFromBuildInfo(info *debug.BuildInfo) buildMetadata {
	metadata := buildMetadata{version: "dev", commit: "unknown", date: "unknown"}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		metadata.version = strings.TrimPrefix(info.Main.Version, "v")
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			metadata.commit = setting.Value
		case "vcs.time":
			metadata.date = setting.Value
		}
	}
	return metadata
}
