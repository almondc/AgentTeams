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

// TestBuildCoordinationBlock_TeamLeaderEmitsOwnRoomID is a regression test for a bug
// where a Team Leader's coordination block told it to "report to Manager in Leader
// Room" but never actually gave it that room's Matrix room ID anywhere - Team Room
// and Leader DM (the Team Admin's DM) were both present, but the leader's own 1:1
// room with Manager was not. A leader triggered by worker activity in the Team Room
// (a different Matrix session from its own Leader Room) had no room ID to target and
// so never reported task completion back to Manager, even though it correctly
// understood conceptually that it should.
func TestBuildCoordinationBlock_TeamLeaderEmitsOwnRoomID(t *testing.T) {
	ctx := CoordinationContext{
		WorkerName:   "task-test-4-leader",
		Role:         "team_leader",
		MatrixDomain: "mx.woodfield.io",
		TeamName:     "task-test-4",
		LeaderRoomID: "!NBmHJRdKxi73x6TrVF:mx.woodfield.io",
	}

	block := buildCoordinationBlock(ctx)

	if !strings.Contains(block, "**Leader Room**: !NBmHJRdKxi73x6TrVF:mx.woodfield.io") {
		t.Fatalf("team_leader coordination block missing '**Leader Room**' line:\n%s", block)
	}
}

// TestBuildCoordinationBlock_TeamLeaderOmitsRoomIDWhenUnknown pins the "not known yet"
// case (the leader's own room doesn't exist until after its first reconcile) so the
// block degrades gracefully instead of emitting an empty/placeholder room ID.
func TestBuildCoordinationBlock_TeamLeaderOmitsRoomIDWhenUnknown(t *testing.T) {
	ctx := CoordinationContext{
		WorkerName:   "task-test-4-leader",
		Role:         "team_leader",
		MatrixDomain: "mx.woodfield.io",
		TeamName:     "task-test-4",
	}

	block := buildCoordinationBlock(ctx)

	if strings.Contains(block, "**Leader Room**") {
		t.Fatalf("team_leader coordination block should omit '**Leader Room**' when unknown:\n%s", block)
	}
}
