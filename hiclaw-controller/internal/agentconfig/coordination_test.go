package agentconfig

import (
	"strings"
	"testing"
)

// TestBuildCoordinationBlock_WorkerEmitsTeamName is a regression test for a bug where
// a team worker's AGENTS.md coordination block never included a "**Team**: <name>" line
// (only team_leader did). Downstream consumers that key off that exact marker to detect
// team membership from AGENTS.md (e.g. the hermes-worker file-sync skill and its Python
// FileSync._get_team_id()) silently fell back to treating every team worker as if it
// were a standalone worker, syncing shared/ to the global bucket prefix instead of
// teams/<team>/shared/.
func TestBuildCoordinationBlock_WorkerEmitsTeamName(t *testing.T) {
	ctx := CoordinationContext{
		WorkerName:     "task-test-3-worker",
		Role:           "worker",
		MatrixDomain:   "mx.woodfield.io",
		TeamName:       "task-test-3",
		TeamLeaderName: "task-test-3-leader",
	}

	block := buildCoordinationBlock(ctx)

	if !strings.Contains(block, "**Team**: task-test-3\n") {
		t.Fatalf("worker coordination block missing '**Team**: %s' line:\n%s", ctx.TeamName, block)
	}
}

// TestBuildCoordinationBlock_TeamLeaderEmitsTeamName pins the existing team_leader
// behavior so the two roles can't silently drift apart again.
func TestBuildCoordinationBlock_TeamLeaderEmitsTeamName(t *testing.T) {
	ctx := CoordinationContext{
		WorkerName:   "task-test-3-leader",
		Role:         "team_leader",
		MatrixDomain: "mx.woodfield.io",
		TeamName:     "task-test-3",
	}

	block := buildCoordinationBlock(ctx)

	if !strings.Contains(block, "**Team**: task-test-3\n") {
		t.Fatalf("team_leader coordination block missing '**Team**: %s' line:\n%s", ctx.TeamName, block)
	}
}
