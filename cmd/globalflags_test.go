package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuietSuppressesStdout(t *testing.T) {
	loud, err := run(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if strings.TrimSpace(loud) == "" {
		t.Fatal("version printed nothing without --quiet; test cannot detect suppression")
	}
	quiet, err := run(t, "--quiet", "version")
	if err != nil {
		t.Fatalf("version --quiet: %v", err)
	}
	if quiet != "" {
		t.Errorf("--quiet still wrote to stdout: %q", quiet)
	}
}

func TestConfigFlagSelectsFile(t *testing.T) {
	t.Chdir(t.TempDir())
	path := filepath.Join(t.TempDir(), "custom.toml")
	if err := os.WriteFile(path, []byte("autonomy = \"L4\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	out, err := run(t, "--config", path, "autonomy", "show")
	if err != nil {
		t.Fatalf("autonomy show --config: %v", err)
	}
	if !strings.Contains(out, "L4") {
		t.Errorf("--config file not honored: autonomy show = %q, want L4", out)
	}
}
