package main

import (
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

type apiServer struct {
	provider *provider
	domain   string
	allowed  []netip.Prefix
}

func (s *apiServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /{$}", s.negotiate)
	mux.HandleFunc("GET /records", s.records)
	mux.HandleFunc("POST /records", s.applyChanges)
	mux.HandleFunc("POST /adjustendpoints", s.adjustEndpoints)
	return s.authorize(mux)
}

func (s *apiServer) negotiate(response http.ResponseWriter, request *http.Request) {
	if !acceptsWebhook(request.Header.Get("Accept")) {
		http.Error(response, "unsupported webhook API version", http.StatusNotAcceptable)
		return
	}
	s.writeJSON(response, domainFilter{Include: []string{s.domain}, Exclude: []string{}})
}

func (s *apiServer) records(response http.ResponseWriter, request *http.Request) {
	if !acceptsWebhook(request.Header.Get("Accept")) {
		http.Error(response, "unsupported webhook API version", http.StatusNotAcceptable)
		return
	}
	s.writeJSON(response, s.provider.list())
}

func (s *apiServer) adjustEndpoints(response http.ResponseWriter, request *http.Request) {
	if !hasWebhookContentType(request.Header.Get("Content-Type")) || !acceptsWebhook(request.Header.Get("Accept")) {
		http.Error(response, "unsupported webhook API version", http.StatusUnsupportedMediaType)
		return
	}
	var records []*endpoint
	if err := decodeJSON(response, request, &records); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(response, s.provider.adjust(records))
}

func (s *apiServer) applyChanges(response http.ResponseWriter, request *http.Request) {
	if !hasWebhookContentType(request.Header.Get("Content-Type")) {
		http.Error(response, "unsupported webhook API version", http.StatusUnsupportedMediaType)
		return
	}
	var requested changes
	if err := decodeJSON(response, request, &requested); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.provider.apply(request.Context(), &requested); err != nil {
		slog.Error("failed to apply DNS changes", "error", err)
		http.Error(response, "failed to apply DNS changes", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (s *apiServer) writeJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", webhookMediaType)
	if err := json.NewEncoder(response).Encode(value); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func (s *apiServer) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		host, _, err := net.SplitHostPort(request.RemoteAddr)
		if err != nil {
			http.Error(response, "forbidden", http.StatusForbidden)
			return
		}
		address, err := netip.ParseAddr(host)
		if err != nil {
			http.Error(response, "forbidden", http.StatusForbidden)
			return
		}
		address = address.Unmap()
		for _, prefix := range s.allowed {
			if prefix.Contains(address) {
				next.ServeHTTP(response, request)
				return
			}
		}
		slog.Warn("rejected request from disallowed address", "remote", request.RemoteAddr)
		http.Error(response, "forbidden", http.StatusForbidden)
	})
}

func decodeJSON(response http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(response, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
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
