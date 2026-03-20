package main

import "testing"

func TestBootstrap_Smoke(t *testing.T) {
	cfg, logger, err := bootstrap()
	if err != nil {
		t.Fatalf("expected bootstrap success, got error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	_ = logger.Sync()
}

func TestBootstrap_InvalidConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "0")

	_, _, err := bootstrap()
	if err == nil {
		t.Fatal("expected validation error for invalid SERVER_PORT")
	}
}
