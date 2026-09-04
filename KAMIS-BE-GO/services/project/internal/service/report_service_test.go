package service

import (
	"slices"
	"testing"

	"github.com/karina/kamis-be-go/services/project/internal/model"
)

// TestStatusScopeFor pins how the chart's three filters read, including the
// misleading-but-deliberate "ALL".
func TestStatusScopeFor(t *testing.T) {
	cancelled := statusScopeFor("CANCELLED")
	if cancelled.Exclude || !slices.Equal(cancelled.Statuses, []int{model.StatusBatal}) {
		t.Errorf("CANCELLED scope = %+v, want the cancelled status, included", cancelled)
	}

	done := statusScopeFor("done")
	if done.Exclude || !slices.Equal(done.Statuses, []int{model.StatusSelesai}) {
		t.Errorf("DONE scope = %+v, want the finished status, included", done)
	}

	// "ALL" names the cancelled status but excludes it: every project that was
	// not cancelled.
	all := statusScopeFor("ALL")
	if !all.Exclude || !slices.Equal(all.Statuses, []int{model.StatusBatal}) {
		t.Errorf("ALL scope = %+v, want the cancelled status, excluded", all)
	}

	// Java fell through to ALL for anything unrecognised rather than rejecting
	// it, and the frontend only ever sends the three.
	if fallback := statusScopeFor("SOMETHING_ELSE"); !fallback.Exclude {
		t.Errorf("unrecognised filter = %+v, want it to behave as ALL", fallback)
	}
}
