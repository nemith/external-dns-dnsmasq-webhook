package main

import (
	"runtime/debug"
	"testing"
)

func TestMetadataFromBuildInfo(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef"},
			{Key: "vcs.time", Value: "2026-01-02T03:04:05Z"},
		},
	}

	got := metadataFromBuildInfo(info)
	if got.version != "1.2.3" || got.commit != "0123456789abcdef" || got.date != "2026-01-02T03:04:05Z" {
		t.Fatalf("unexpected build metadata: %#v", got)
	}
}

func TestMetadataFromDevelopmentBuild(t *testing.T) {
	got := metadataFromBuildInfo(&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}})
	if got.version != "dev" || got.commit != "unknown" || got.date != "unknown" {
		t.Fatalf("unexpected development metadata: %#v", got)
	}
}
