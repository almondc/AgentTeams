package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestEnsurePushWorkerSkillsRegistryEntry_CreatesFile is a regression test for a bug
// where PushOnDemandSkills always failed with "Worker '<name>' not found in registry"
// in incluster mode: push-worker-skills.sh's only source of truth for "does this
// worker exist" and "what skills does it want" is a local workers-registry.json file
// that the embedded/Docker Manager process maintains continuously as workers are
// created, but nothing in the incluster controller ever writes. Confirmed live: a
// freshly created Team worker with skills: [github-operations, git-delegation] never
// received them, with "skill push failed" / "not found in registry" in the controller
// logs on every reconcile.
func TestEnsurePushWorkerSkillsRegistryEntry_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "workers-registry.json")

	if err := ensurePushWorkerSkillsRegistryEntryAt(registryPath, "labops-worker", "hermes", []string{"github-operations", "git-delegation"}); err != nil {
		t.Fatalf("ensurePushWorkerSkillsRegistryEntryAt: %v", err)
	}

	raw, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("registry file not written: %v", err)
	}

	var reg struct {
		Workers map[string]struct {
			Runtime string   `json:"runtime"`
			Skills  []string `json:"skills"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("registry file is not valid JSON: %v\ncontent: %s", err, raw)
	}

	w, ok := reg.Workers["labops-worker"]
	if !ok {
		t.Fatalf("registry missing entry for labops-worker:\n%s", raw)
	}
	if w.Runtime != "hermes" {
		t.Errorf("runtime = %q, want hermes", w.Runtime)
	}
	if len(w.Skills) != 2 || w.Skills[0] != "github-operations" || w.Skills[1] != "git-delegation" {
		t.Errorf("skills = %v, want [github-operations git-delegation]", w.Skills)
	}
}

// TestEnsurePushWorkerSkillsRegistryEntry_PreservesOtherWorkers ensures a second
// worker's upsert doesn't clobber a first worker's existing entry - the registry
// file is shared across every worker the controller ever pushes skills for within
// this pod's lifetime, so overwriting rather than merging would silently break
// on-demand skills for every worker but the most recently reconciled one.
func TestEnsurePushWorkerSkillsRegistryEntry_PreservesOtherWorkers(t *testing.T) {
	dir := t.TempDir()
	registryPath := filepath.Join(dir, "workers-registry.json")

	if err := ensurePushWorkerSkillsRegistryEntryAt(registryPath, "worker-a", "copaw", []string{"skill-a"}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := ensurePushWorkerSkillsRegistryEntryAt(registryPath, "worker-b", "hermes", []string{"skill-b"}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	raw, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("registry file not written: %v", err)
	}
	var reg struct {
		Workers map[string]struct {
			Runtime string   `json:"runtime"`
			Skills  []string `json:"skills"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("registry file is not valid JSON: %v\ncontent: %s", err, raw)
	}

	if _, ok := reg.Workers["worker-a"]; !ok {
		t.Errorf("worker-a entry was clobbered by worker-b's upsert:\n%s", raw)
	}
	if _, ok := reg.Workers["worker-b"]; !ok {
		t.Errorf("worker-b entry missing:\n%s", raw)
	}
}
