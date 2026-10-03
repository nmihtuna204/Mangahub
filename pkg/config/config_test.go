package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFileAndEnvOverrides(t *testing.T) {
	viper.Reset()
	path := writeConfig(t, "server:\n  port: 1234\ntcp:\n  host: file-host\n")
	t.Setenv("TCP_HOST", "tcp-server") // docker-compose style override
	t.Setenv("MANGAHUB_CONFIG", "")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 1234 {
		t.Errorf("server.port = %d, want 1234 from the file", cfg.Server.Port)
	}
	if cfg.TCP.Host != "tcp-server" {
		t.Errorf("tcp.host = %q, want the TCP_HOST override (env overrides used to be ignored)", cfg.TCP.Host)
	}
	if cfg.GRPC.Port != 9092 {
		t.Errorf("grpc.port = %d, want default 9092", cfg.GRPC.Port)
	}
}

func TestLoadHonorsMangahubConfig(t *testing.T) {
	viper.Reset()
	path := writeConfig(t, "server:\n  port: 4321\n")
	t.Setenv("MANGAHUB_CONFIG", path)

	cfg, err := Load("./does/not/exist.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 4321 {
		t.Errorf("server.port = %d, want 4321 from MANGAHUB_CONFIG", cfg.Server.Port)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	viper.Reset()
	t.Setenv("MANGAHUB_CONFIG", "")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("missing config should fall back to defaults: %v", err)
	}
	if cfg.Server.Port != 8080 || cfg.UDP.Port != 9091 {
		t.Errorf("defaults not applied: %+v", cfg.Server)
	}
}
