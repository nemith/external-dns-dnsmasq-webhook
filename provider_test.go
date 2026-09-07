package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testProvider(t *testing.T) *provider {
	t.Helper()
	directory := t.TempDir()
	p := &provider{
		domain:        "example.test",
		stateFile:     filepath.Join(directory, "records.json"),
		configFile:    filepath.Join(directory, "records.conf"),
		dnsmasqBinary: "dnsmasq",
		systemctl:     "systemctl",
		service:       "dnsmasq.service",
		records:       make(map[string]*endpoint),
		run:           func(context.Context, string, ...string) error { return nil },
	}
	return p
}

func TestApplyRendersRecords(t *testing.T) {
	p := testProvider(t)
	requested := &changes{Create: []*endpoint{
		{DNSName: "coder.example.test.", RecordType: "A", Targets: []string{"203.0.113.193"}},
		{DNSName: "*.apps.example.test", RecordType: "A", Targets: []string{"203.0.113.193"}},
		{DNSName: "alias.example.test", RecordType: "CNAME", Targets: []string{"coder.example.test."}},
	}}
	if err := p.apply(context.Background(), requested); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p.configFile)
	if err != nil {
		t.Fatal(err)
	}
	want := generatedHeader + strings.Join([]string{
		"address=/apps.example.test/203.0.113.193",
		"cname=alias.example.test,coder.example.test",
		"host-record=coder.example.test,203.0.113.193",
		"",
	}, "\n")
	if string(data) != want {
		t.Fatalf("unexpected config:\n%s\nwant:\n%s", data, want)
	}
	if len(p.list()) != 3 {
		t.Fatalf("got %d records, want 3", len(p.list()))
	}
}

func TestApplyRejectsRecordsOutsideDomain(t *testing.T) {
	p := testProvider(t)
	err := p.apply(context.Background(), &changes{Create: []*endpoint{{
		DNSName: "example.net", RecordType: "A", Targets: []string{"192.0.2.1"},
	}}})
	if err == nil || !strings.Contains(err.Error(), "outside managed domain") {
		t.Fatalf("got error %v", err)
	}
}

func TestApplyRollsBackFailedCheck(t *testing.T) {
	p := testProvider(t)
	if err := writeAtomic(p.configFile, []byte("old config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.run = func(context.Context, string, ...string) error { return errors.New("bad config") }
	err := p.apply(context.Background(), &changes{Create: []*endpoint{{
		DNSName: "test.example.test", RecordType: "A", Targets: []string{"192.0.2.1"},
	}}})
	if err == nil {
		t.Fatal("expected apply to fail")
	}
	data, readErr := os.ReadFile(p.configFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "old config\n" {
		t.Fatalf("config was not rolled back: %q", data)
	}
	if len(p.list()) != 0 {
		t.Fatal("failed record was committed to memory")
	}
}

func TestNormalizeRejectsPartialWildcard(t *testing.T) {
	p := testProvider(t)
	_, err := p.normalize(&endpoint{DNSName: "*-coder.example.test", RecordType: "A", Targets: []string{"192.0.2.1"}})
	if err == nil {
		t.Fatal("expected partial wildcard to be rejected")
	}
}

func TestApplyRejectsCNAMEConflict(t *testing.T) {
	p := testProvider(t)
	err := p.apply(context.Background(), &changes{Create: []*endpoint{
		{DNSName: "test.example.test", RecordType: "A", Targets: []string{"192.0.2.1"}},
		{DNSName: "test.example.test", RecordType: "CNAME", Targets: []string{"other.example.test"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("got error %v", err)
	}
}
