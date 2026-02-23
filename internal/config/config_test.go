package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultOutputDirSuffix(t *testing.T) {
	got := defaultOutputDir()
	if !strings.Contains(got, "wiro-outputs") {
		t.Fatalf("default output dir should contain wiro-outputs, got: %s", got)
	}
}

func TestLoadMigratesLegacyOutputDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	cfgDir := filepath.Join(tmp, ".config", "wiro")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `{"defaultProject":"","projects":[],"preferences":{"watchDefault":true,"outputDirDefault":"./wiro-outputs"}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Preferences.OutputDirDefault == legacyOutputDir {
		t.Fatalf("legacy output dir should be migrated")
	}
	if !strings.Contains(cfg.Preferences.OutputDirDefault, "wiro-outputs") {
		t.Fatalf("migrated output dir invalid: %s", cfg.Preferences.OutputDirDefault)
	}
}
