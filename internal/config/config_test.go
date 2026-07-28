package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/agentic-dev-harness/internal/adh"
	"github.com/StevenACoffman/agentic-dev-harness/internal/authority"
	"github.com/StevenACoffman/agentic-dev-harness/internal/config"
)

func noEnv(string) string { return "" }

func TestDefaults(t *testing.T) {
	cfg := config.Defaults()
	if got := cfg.AutonomyLevel(); got != authority.L2 {
		t.Errorf("default autonomy = %s, want L2", got)
	}
	if !cfg.JudgmentRoles().Requires(adh.StageStrategy) {
		t.Errorf("default judgment set should include strategy")
	}
	if cfg.JudgmentRoles().Requires(adh.StageExecution) {
		t.Errorf("execution is not a default judgment role")
	}
}

func TestCriticDefaults(t *testing.T) {
	cfg := config.Defaults()
	if cfg.CriticUnconfirmed() != config.UnconfirmedLesson {
		t.Errorf(
			"default unconfirmed disposition = %q, want %q",
			cfg.CriticUnconfirmed(),
			config.UnconfirmedLesson,
		)
	}
	if len(cfg.Critic.GroundFrom) == 0 || len(cfg.Critic.Deny) == 0 {
		t.Errorf("default critic ground_from/deny should be populated: %+v", cfg.Critic)
	}
}

func TestProofContractDefaultsAreGeneric(t *testing.T) {
	cfg := config.Defaults()
	change := cfg.ProofContract(adh.ResolutionChange)
	if change == "" {
		t.Fatal("default change contract is empty")
	}
	if strings.Contains(change, "oracle") || strings.Contains(change, "device") {
		t.Errorf("default change contract should be generic, got %q", change)
	}
	if cfg.ProofContract(adh.Resolution("bogus")) != "" {
		t.Error("unknown resolution should have no contract")
	}
}

func TestProofContractPerKeyOverrideFallsBack(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRepoConfig(t, "[proof.contract]\nchange = \"oracle, invariant, and on-device proof\"\n")
	cfg, err := config.Load(noEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.ProofContract(
		adh.ResolutionChange,
	); got != "oracle, invariant, and on-device proof" {
		t.Errorf("change contract = %q, want the configured override", got)
	}
	// A key the config did not set still resolves to its built-in default.
	if cfg.ProofContract(adh.ResolutionDecision) == "" {
		t.Error("an unset contract key should fall back to its built-in default")
	}
}

func TestLoadRepoOverridesCriticUnconfirmed(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRepoConfig(t, "[critic]\nunconfirmed = \"drop\"\n")
	cfg, err := config.Load(noEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CriticUnconfirmed() != "drop" {
		t.Errorf("unconfirmed = %q, want drop from repo config", cfg.CriticUnconfirmed())
	}
}

func TestLoadRepoOverridesDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRepoConfig(t, "autonomy = \"L4\"\n[models.gate]\njudgment_roles = [\"strategy\"]\n")
	cfg, err := config.Load(noEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AutonomyLevel() != authority.L4 {
		t.Errorf("autonomy = %s, want L4 from repo config", cfg.AutonomyLevel())
	}
	if cfg.JudgmentRoles().Requires(adh.StageCritic) {
		t.Errorf("repo config narrowed judgment_roles to [strategy]; critic should be excluded")
	}
}

func TestLoadEnvOverridesRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRepoConfig(t, "autonomy = \"L4\"\n")
	env := func(k string) string {
		if k == "ADH_AUTONOMY" {
			return "L0"
		}
		return ""
	}
	cfg, err := config.Load(env)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AutonomyLevel() != authority.L0 {
		t.Errorf("autonomy = %s, want L0 from ADH_AUTONOMY over the file", cfg.AutonomyLevel())
	}
}

func TestLoadAutonomyFileOverridesRepoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRepoConfig(t, "autonomy = \"L4\"\n")
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".adh", "autonomy"), []byte("L1\n"), 0o600); err != nil {
		t.Fatalf("write autonomy: %v", err)
	}
	cfg, err := config.Load(noEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AutonomyLevel() != authority.L1 {
		t.Errorf("autonomy = %s, want L1 from .adh/autonomy over config.toml", cfg.AutonomyLevel())
	}
}

func TestLoadMalformedConfigErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRepoConfig(t, "autonomy = = broken\n")
	if _, err := config.Load(noEnv); err == nil {
		t.Error("Load should error on a malformed config, not silently skip it")
	}
}

func TestLoadAbsentConfigIsDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg, err := config.Load(noEnv)
	if err != nil {
		t.Fatalf("Load with no config file should succeed: %v", err)
	}
	if cfg.AutonomyLevel() != authority.L2 {
		t.Errorf("absent config = %s, want the L2 default", cfg.AutonomyLevel())
	}
}

func TestBaselineModelsFromConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRepoConfig(t, "[models]\nreasoning = \"strong\"\nfast = \"quick\"\n")
	cfg, err := config.Load(noEnv)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	models := cfg.BaselineModels()
	if models["strategy"] != "strong" {
		t.Errorf("strategy model = %q, want strong (a judgment role)", models["strategy"])
	}
	if models["execution"] != "quick" {
		t.Errorf("execution model = %q, want quick (a fast role)", models["execution"])
	}
}

func writeRepoConfig(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(".adh", 0o750); err != nil {
		t.Fatalf("mkdir .adh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".adh", "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
