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
	s := newAPIServer(p, p.domain, []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", webhookMediaType)
	r.RemoteAddr = "192.0.2.10:1234"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != webhookMediaType {
		t.Fatalf("negotiate returned status %d and content type %q", w.Code, w.Header().Get("Content-Type"))
	}

	body := []byte(`{"create":[{"dnsName":"app.example.test","recordType":"A","targets":["203.0.113.193"]}]}`)
	r = httptest.NewRequest(http.MethodPost, "/records", bytes.NewReader(body))
	r.Header.Set("Content-Type", webhookMediaType)
	r.RemoteAddr = "192.0.2.10:1234"
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("apply returned status %d: %s", w.Code, w.Body.String())
	}

	r = httptest.NewRequest(http.MethodGet, "/records", nil)
	r.Header.Set("Accept", webhookMediaType)
	r.RemoteAddr = "192.0.2.10:1234"
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("app.example.test")) {
		t.Fatalf("records returned status %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhookRejectsUnauthorizedClient(t *testing.T) {
	p := testProvider(t)
	s := newAPIServer(p, p.domain, []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")})
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403", w.Code)
	}
}

func TestWriteJSONReturnsInternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, make(chan struct{}))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500", w.Code)
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
