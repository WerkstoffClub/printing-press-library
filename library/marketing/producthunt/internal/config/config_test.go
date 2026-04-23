package config

import (
	"path/filepath"
	"testing"
)

func TestSaveGraphQLTokenUnlocksGraphQLFeatures(t *testing.T) {
	cfg := &Config{Path: filepath.Join(t.TempDir(), "config.toml")}
	if err := cfg.SaveGraphQLToken("dev_tok"); err != nil {
		t.Fatalf("SaveGraphQLToken: %v", err)
	}
	reloaded, err := Load(cfg.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.HasGraphQLToken() {
		t.Fatalf("developer token should unlock GraphQL features")
	}
	if reloaded.GraphQLAuthMode() != "developer_token" {
		t.Fatalf("GraphQLAuthMode = %q, want developer_token", reloaded.GraphQLAuthMode())
	}
}

func TestLoadDeveloperTokenFromEnv(t *testing.T) {
	t.Setenv("PRODUCTHUNT_DEVELOPER_TOKEN", "env_tok")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessToken != "env_tok" {
		t.Fatalf("AccessToken = %q, want env_tok", cfg.AccessToken)
	}
	if !cfg.HasGraphQLToken() {
		t.Fatalf("env token should unlock GraphQL features")
	}
	if cfg.AuthSource != "env" {
		t.Fatalf("AuthSource = %q, want env", cfg.AuthSource)
	}
}
