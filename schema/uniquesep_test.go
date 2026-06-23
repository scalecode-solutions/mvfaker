package schema_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/schema"
)

// unique_sep="" → alphanumeric handles (^[a-z0-9]+$), still unique by construction.
func TestUniqueSepAlnum(t *testing.T) {
	empty := ""
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"u": {Name: "u", Fields: []*schema.Field{
				{Name: "handle", Gen: "name.first", Transform: "lower", Unique: true, UniqueSep: &empty},
			}},
		},
		Order:  []string{"u"},
		Counts: map[string]int{"u": 2000},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("u", 1, 2000)
	re := regexp.MustCompile(`^[a-z0-9]+$`)
	seen := map[string]bool{}
	for _, r := range recs {
		h := r.Get("handle").(string)
		if !re.MatchString(h) {
			t.Fatalf("handle %q violates ^[a-z0-9]+$", h)
		}
		if seen[h] {
			t.Fatalf("duplicate alnum handle %q", h)
		}
		seen[h] = true
	}
	if len(seen) != 2000 {
		t.Fatalf("expected 2000 distinct handles, got %d", len(seen))
	}
}

// Default (nil UniqueSep) keeps the "." separator for non-email strings.
func TestUniqueSepDefault(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"u": {Name: "u", Fields: []*schema.Field{
				{Name: "handle", Gen: "name.first", Unique: true}, // no UniqueSep
			}},
		},
		Order:  []string{"u"},
		Counts: map[string]int{"u": 50},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("u", 1, 50)
	for _, r := range recs {
		if !strings.Contains(r.Get("handle").(string), ".") {
			t.Fatalf("default unique should use '.', got %q", r.Get("handle"))
		}
	}
}
