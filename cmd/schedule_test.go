package cmd_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScheduleHonorsRepoFlag: `--repo <dir>` puts the schedule store under that
// repo's .adh, not the working directory, so a daemon launched elsewhere finds it.
func TestScheduleHonorsRepoFlag(t *testing.T) {
	cwd := t.TempDir()
	repo := t.TempDir()
	t.Chdir(cwd)

	if _, err := run(
		t,
		"--repo",
		repo,
		"sleep",
		"schedule",
		"add",
		"nightly",
		"@daily",
		"sleep",
		"run",
	); err != nil {
		t.Fatalf("schedule add --repo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".adh", "schedule.db")); err != nil {
		t.Errorf("schedule store not under --repo dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".adh", "schedule.db")); !os.IsNotExist(err) {
		t.Errorf("schedule store leaked into the working directory: %v", err)
	}
}
