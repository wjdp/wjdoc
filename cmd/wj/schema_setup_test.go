package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaIsJSONWithThreeTypes(t *testing.T) {
	got := runOK(t, "schema")

	var parsed struct {
		Types []struct {
			Name string `json:"name"`
		} `json:"types"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, got)
	}
	if len(parsed.Types) != 3 {
		t.Fatalf("got %d types, want 3: %v", len(parsed.Types), parsed.Types)
	}
}

func TestSkillSourceCopyMatchesCanonical(t *testing.T) {
	canonical, err := os.ReadFile("../../skill/SKILL.md")
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	embedded, err := os.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("read embedded skill: %v", err)
	}
	if string(canonical) != string(embedded) {
		t.Error("cmd/wj/skill/SKILL.md is out of date with skill/SKILL.md")
	}
}

func TestSetupInstallsThenReportsExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	got := runOK(t, "setup")
	target := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "wj-docs", "SKILL.md")
	if got != "installed "+target+"\n" {
		t.Fatalf("first run: got %q", got)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(content) != skillContent {
		t.Error("installed content does not match the embedded skill")
	}

	got = runOK(t, "setup")
	if got != target+" exists\n" {
		t.Fatalf("second run: got %q", got)
	}
}

func TestSetupUpdatesStaleContent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	skillDir := filepath.Join(os.Getenv("HOME"), ".claude", "skills", "wj-docs")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(target, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}

	got := runOK(t, "setup")
	if got != "updated "+target+"\n" {
		t.Fatalf("got %q", got)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read updated skill: %v", err)
	}
	if string(content) != skillContent {
		t.Error("updated content does not match the embedded skill")
	}
}
