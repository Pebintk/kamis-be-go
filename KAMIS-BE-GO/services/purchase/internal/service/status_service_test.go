package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/karina/kamis-be-go/pkg/blob"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
)

// TestAdvanceMachine pins where a purchase can go and who may take it there.
// The rules are status-dependent, which is why they cannot be route guards.
func TestAdvanceMachine(t *testing.T) {
	cases := []struct {
		from    string
		to      string
		allowed []string
	}{
		{model.StatusDiajukan, model.StatusDisetujui, []string{"Direksi", "Finance"}},
		{model.StatusDisetujui, model.StatusDiproses, []string{"Operasional", "Admin"}},
		{model.StatusDiproses, model.StatusSelesai, []string{"Operasional", "Admin"}},
	}

	for _, tc := range cases {
		t.Run(tc.from, func(t *testing.T) {
			step, ok := advance[tc.from]
			if !ok {
				t.Fatalf("no transition out of %s", tc.from)
			}
			if step.to != tc.to {
				t.Errorf("%s advances to %s, want %s", tc.from, step.to, tc.to)
			}
			if !slices.Equal(step.allowed, tc.allowed) {
				t.Errorf("%s allows %v, want %v", tc.from, step.allowed, tc.allowed)
			}
			if step.refusal == "" {
				t.Errorf("%s has no refusal message; the frontend shows it verbatim", tc.from)
			}
		})
	}

	// Terminal statuses are handled by the Terminal check before the table is
	// consulted, so they must not appear in it.
	for _, s := range []string{model.StatusSelesai, model.StatusDibatalkan, model.StatusDitolak} {
		if _, ok := advance[s]; ok {
			t.Errorf("%s is terminal but has an advance transition", s)
		}
	}
}

// TestAbandonMachine pins the cancel path, including the one asymmetry with
// advance: Admin may complete a purchase at Diproses but may not cancel it
// there. Inherited verbatim from the legacy service.
func TestAbandonMachine(t *testing.T) {
	cases := []struct {
		from    string
		to      string
		allowed []string
	}{
		{model.StatusDiajukan, model.StatusDitolak, []string{"Direksi", "Finance"}},
		{model.StatusDisetujui, model.StatusDibatalkan, []string{"Operasional", "Admin"}},
		{model.StatusDiproses, model.StatusDibatalkan, []string{"Operasional"}},
	}

	for _, tc := range cases {
		t.Run(tc.from, func(t *testing.T) {
			step, ok := abandon[tc.from]
			if !ok {
				t.Fatalf("no transition out of %s", tc.from)
			}
			if step.to != tc.to {
				t.Errorf("%s cancels to %s, want %s", tc.from, step.to, tc.to)
			}
			if !slices.Equal(step.allowed, tc.allowed) {
				t.Errorf("%s allows %v, want %v", tc.from, step.allowed, tc.allowed)
			}
		})
	}

	if slices.Contains(abandon[model.StatusDiproses].allowed, "Admin") {
		t.Error("Admin may not cancel at Diproses — the asymmetry with advance is deliberate")
	}
	if !slices.Contains(advance[model.StatusDiproses].allowed, "Admin") {
		t.Error("Admin may complete at Diproses; the asymmetry is only on cancel")
	}
}

// TestEveryNonTerminalStatusCanMove guards against a status that no machine
// covers, which would strand a purchase with no way forward or back.
func TestEveryNonTerminalStatusCanMove(t *testing.T) {
	for _, s := range []string{model.StatusDiajukan, model.StatusDisetujui, model.StatusDiproses} {
		if _, ok := advance[s]; !ok {
			t.Errorf("%s cannot be advanced", s)
		}
		if _, ok := abandon[s]; !ok {
			t.Errorf("%s cannot be cancelled", s)
		}
	}
}

// TestStagedPhotoKey covers the key generation: valid for both backends, unique
// per call, and carrying the extension the disk backend reads the type from.
func TestStagedPhotoKey(t *testing.T) {
	first, err := stagedPhotoKey("image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if !blob.ValidKey(first) {
		t.Errorf("key %q is rejected by blob.ValidKey", first)
	}
	if !strings.HasPrefix(first, "asset_") {
		t.Errorf("key = %q, want the legacy asset_ prefix", first)
	}
	if !strings.HasSuffix(first, ".jpg") {
		t.Errorf("key = %q, want a .jpg extension", first)
	}

	second, err := stagedPhotoKey("image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("keys are not unique; two staged assets would share one object")
	}
}
