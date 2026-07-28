package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
	"github.com/StevenACoffman/agentic-dev-harness/internal/contextstore"
)

func TestInitScaffoldsConfigAndContext(t *testing.T) {
	t.Chdir(t.TempDir())
	// Two top-level source dirs to key starter context units on.
	for _, dir := range []string{"cmd", "internal"} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	out := mustRun(t, "init")
	if !strings.Contains(out, "written") {
		t.Errorf("first init should report the config written:\n%s", out)
	}

	// A loadable starter config.
	if _, err := os.Stat(config.RepoConfigFile); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if _, err := config.Load(func(string) string { return "" }); err != nil {
		t.Errorf("scaffolded config should load: %v", err)
	}

	// A context unit per top-level dir, keyed for routing.
	units, err := contextstore.Load(contextstore.DefaultStoreDir)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("scaffolded %d context units, want 2 (cmd, internal)", len(units))
	}
	byID := map[string]contextstore.Unit{}
	for _, u := range units {
		byID[u.ID] = u
	}
	if u, ok := byID["internal"]; !ok || len(u.Labels) == 0 || u.Labels[0] != "internal" {
		t.Errorf("internal unit missing or mislabeled: %+v", byID["internal"])
	}
}

func TestInitIsIdempotent(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("cmd", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustRun(t, "init")
	// Tailor the config, then re-run: init must not clobber it.
	custom := "autonomy = \"L0\"\n"
	if err := os.WriteFile(config.RepoConfigFile, []byte(custom), 0o600); err != nil {
		t.Fatalf("customize config: %v", err)
	}
	out := mustRun(t, "init")
	if !strings.Contains(out, "kept") {
		t.Errorf("second init should keep the existing config:\n%s", out)
	}
	data, err := os.ReadFile(config.RepoConfigFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != custom {
		t.Errorf("init clobbered a tailored config:\n%s", data)
	}
	// The store was already populated, so no new units are scaffolded.
	if !strings.Contains(out, "0 context unit(s)") {
		t.Errorf("second init should scaffold no context units:\n%s", out)
	}
}
