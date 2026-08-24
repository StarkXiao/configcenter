package config

import "testing"

func TestLoadUsesRunnableLocalDefaults(t *testing.T) {
	t.Setenv("CONFIG_CENTER_ADDR", "")
	t.Setenv("CONFIG_CENTER_DB", "")
	t.Setenv("CONFIG_CENTER_ADMIN_TOKEN", "")
	t.Setenv("CONFIG_CENTER_HEARTBEAT", "")
	t.Setenv("CONFIG_CENTER_SHUTDOWN_TIMEOUT", "")
	t.Setenv("CONFIG_CENTER_EVENT_HISTORY", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:8081" {
		t.Fatalf("unexpected default address: %s", cfg.Address)
	}
	if cfg.AdminToken != "local-admin-token" {
		t.Fatalf("unexpected development token: %s", cfg.AdminToken)
	}
}

func TestLoadRejectsInvalidRuntimeValues(t *testing.T) {
	t.Setenv("CONFIG_CENTER_HEARTBEAT", "100ms")
	if _, err := Load(); err == nil {
		t.Fatal("expected heartbeat validation error")
	}
}
