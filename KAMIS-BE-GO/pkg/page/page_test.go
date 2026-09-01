package page

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewMetadata(t *testing.T) {
	first := New([]int{1, 2, 3}, 0, 3, 7)
	if first.TotalPages != 3 || !first.First || first.Last || first.NumberOfElements != 3 || first.Empty {
		t.Errorf("first page metadata wrong: %+v", first)
	}

	last := New([]int{7}, 2, 3, 7)
	if !last.Last || last.First {
		t.Errorf("last page metadata wrong: %+v", last)
	}

	// Content must marshal as [] rather than null so the frontend can iterate.
	out, err := json.Marshal(New([]int{}, 0, 3, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"content":[]`) {
		t.Errorf("empty page content = %s", out)
	}
}
