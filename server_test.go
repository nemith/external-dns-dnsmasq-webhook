package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestWebhookProtocol(t *testing.T) {
	p := testProvider(t)
	s := &apiServer{provider: p, domain: p.domain, allowed: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", webhookMediaType)
	request.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	s.handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != webhookMediaType {
		t.Fatalf("negotiate returned status %d and content type %q", response.Code, response.Header().Get("Content-Type"))
	}

	body := []byte(`{"create":[{"dnsName":"app.example.test","recordType":"A","targets":["203.0.113.193"]}]}`)
	request = httptest.NewRequest(http.MethodPost, "/records", bytes.NewReader(body))
	request.Header.Set("Content-Type", webhookMediaType)
	request.RemoteAddr = "192.0.2.10:1234"
	response = httptest.NewRecorder()
	s.handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("apply returned status %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/records", nil)
	request.Header.Set("Accept", webhookMediaType)
	request.RemoteAddr = "192.0.2.10:1234"
	response = httptest.NewRecorder()
	s.handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte("app.example.test")) {
		t.Fatalf("records returned status %d: %s", response.Code, response.Body.String())
	}
}

func TestWebhookRejectsUnauthorizedClient(t *testing.T) {
	p := testProvider(t)
	s := &apiServer{provider: p, domain: p.domain, allowed: []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}}
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	s.handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403", response.Code)
	}
}

func TestStartupReconcilesAndRestarts(t *testing.T) {
	p := testProvider(t)
	commands := 0
	p.run = func(context.Context, string, ...string) error {
		commands++
		return nil
	}
	if err := p.reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if commands != 2 {
		t.Fatalf("ran %d commands, want dnsmasq check and restart", commands)
	}
}
