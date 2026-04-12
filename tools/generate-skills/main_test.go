package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyUpstreamSkill_Present(t *testing.T) {
	tmp := t.TempDir()

	entryPath := filepath.Join(tmp, "library", "commerce", "yahoo-finance")
	if err := os.MkdirAll(entryPath, 0755); err != nil {
		t.Fatal(err)
	}
	upstream := []byte("---\nname: pp-yahoo-finance\ndescription: \"Upstream content with `backticks` and \\\"quotes\\\"\"\n---\n\n# Yahoo Finance\n\nNarrative content.\n")
	if err := os.WriteFile(filepath.Join(entryPath, "SKILL.md"), upstream, 0644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(tmp, "plugin", "skills", "pp-yahoo-finance")
	skillFile := filepath.Join(skillDir, "SKILL.md")

	copied, err := copyUpstreamSkill(entryPath, skillDir, skillFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !copied {
		t.Fatal("expected copied=true when upstream SKILL.md exists")
	}

	got, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("reading destination: %v", err)
	}
	if string(got) != string(upstream) {
		t.Errorf("destination content does not match upstream byte-for-byte\nwant: %q\ngot:  %q", upstream, got)
	}
}

func TestCopyUpstreamSkill_Absent(t *testing.T) {
	tmp := t.TempDir()

	entryPath := filepath.Join(tmp, "library", "commerce", "no-upstream")
	if err := os.MkdirAll(entryPath, 0755); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(tmp, "plugin", "skills", "pp-no-upstream")
	skillFile := filepath.Join(skillDir, "SKILL.md")

	copied, err := copyUpstreamSkill(entryPath, skillDir, skillFile)
	if err != nil {
		t.Fatalf("unexpected error when upstream missing: %v", err)
	}
	if copied {
		t.Error("expected copied=false when upstream SKILL.md missing")
	}
	if _, err := os.Stat(skillFile); !os.IsNotExist(err) {
		t.Errorf("expected destination not to exist, stat err=%v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("expected skill dir not to be created when no upstream, stat err=%v", err)
	}
}

func TestCopyUpstreamSkill_OverwritesExisting(t *testing.T) {
	tmp := t.TempDir()

	entryPath := filepath.Join(tmp, "library", "commerce", "yahoo-finance")
	if err := os.MkdirAll(entryPath, 0755); err != nil {
		t.Fatal(err)
	}
	upstream := []byte("UPSTREAM CONTENT")
	if err := os.WriteFile(filepath.Join(entryPath, "SKILL.md"), upstream, 0644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(tmp, "plugin", "skills", "pp-yahoo-finance")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("STALE SYNTHESIS"), 0644); err != nil {
		t.Fatal(err)
	}

	copied, err := copyUpstreamSkill(entryPath, skillDir, skillFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !copied {
		t.Fatal("expected copied=true")
	}

	got, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(upstream) {
		t.Errorf("upstream should overwrite stale synthesis\nwant: %q\ngot:  %q", upstream, got)
	}
}
