package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const webhookMediaType = "application/external.dns.webhook+json;version=1"

type endpoint struct {
	DNSName          string                     `json:"dnsName,omitempty"`
	Targets          []string                   `json:"targets,omitempty"`
	RecordType       string                     `json:"recordType,omitempty"`
	SetIdentifier    string                     `json:"setIdentifier,omitempty"`
	RecordTTL        int64                      `json:"recordTTL,omitempty"`
	Labels           map[string]string          `json:"labels,omitempty"`
	ProviderSpecific []providerSpecificProperty `json:"providerSpecific,omitempty"`
}

type providerSpecificProperty struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type changes struct {
	Create    []*endpoint `json:"create,omitempty"`
	UpdateOld []*endpoint `json:"updateOld,omitempty"`
	UpdateNew []*endpoint `json:"updateNew,omitempty"`
	Delete    []*endpoint `json:"delete,omitempty"`
}

type domainFilter struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

type apiServer struct {
	provider *provider
	domain   string
	allowed  []netip.Prefix
	mux      *http.ServeMux
}

var _ http.Handler = (*apiServer)(nil)

func newAPIServer(provider *provider, domain string, allowed []netip.Prefix) *apiServer {
	s := &apiServer{provider: provider, domain: domain, allowed: allowed}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /{$}", s.negotiate)
	mux.HandleFunc("GET /records", s.records)
	mux.HandleFunc("POST /records", s.applyChanges)
	mux.HandleFunc("POST /adjustendpoints", s.adjustEndpoints)
	s.mux = mux
	return s
}

func (s *apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	address = address.Unmap()
	for _, prefix := range s.allowed {
		if prefix.Contains(address) {
			s.mux.ServeHTTP(w, r)
			return
		}
	}
	slog.Warn("rejected request from disallowed address", "remote", r.RemoteAddr)
	http.Error(w, "forbidden", http.StatusForbidden)
}

func (s *apiServer) negotiate(w http.ResponseWriter, r *http.Request) {
	if !acceptsWebhook(r.Header.Get("Accept")) {
		http.Error(w, "unsupported webhook API version", http.StatusNotAcceptable)
		return
	}
	writeJSON(w, domainFilter{Include: []string{s.domain}, Exclude: []string{}})
}

func (s *apiServer) records(w http.ResponseWriter, r *http.Request) {
	if !acceptsWebhook(r.Header.Get("Accept")) {
		http.Error(w, "unsupported webhook API version", http.StatusNotAcceptable)
		return
	}
	writeJSON(w, s.provider.list())
}

func (s *apiServer) adjustEndpoints(w http.ResponseWriter, r *http.Request) {
	if !hasWebhookContentType(r.Header.Get("Content-Type")) || !acceptsWebhook(r.Header.Get("Accept")) {
		http.Error(w, "unsupported webhook API version", http.StatusUnsupportedMediaType)
		return
	}
	var records []*endpoint
	if err := decodeJSON(w, r, &records); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, s.provider.adjust(records))
}

func (s *apiServer) applyChanges(w http.ResponseWriter, r *http.Request) {
	if !hasWebhookContentType(r.Header.Get("Content-Type")) {
		http.Error(w, "unsupported webhook API version", http.StatusUnsupportedMediaType)
		return
	}
	var requested changes
	if err := decodeJSON(w, r, &requested); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.provider.apply(r.Context(), &requested); err != nil {
		slog.Error("failed to apply DNS changes", "error", err)
		http.Error(w, "failed to apply DNS changes", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, value any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		slog.Error("failed to encode response", "error", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", webhookMediaType)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid JSON: trailing data")
	}
	return nil
}

func acceptsWebhook(value string) bool {
	for item := range strings.SplitSeq(value, ",") {
		mediaType, parameters, err := mime.ParseMediaType(strings.TrimSpace(item))
		if err == nil && strings.EqualFold(mediaType, "application/external.dns.webhook+json") && parameters["version"] == "1" {
			return true
		}
	}
	return false
}

func hasWebhookContentType(value string) bool {
	mediaType, parameters, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/external.dns.webhook+json") && parameters["version"] == "1"
}
