package schema_test

import (
	"strings"
	"testing"

	"github.com/tmarq/mvfaker/data"
	"github.com/tmarq/mvfaker/schema"
)

func buildPlan() *schema.Plan {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"customer": {Name: "customer", Fields: []*schema.Field{
				{Name: "name", Gen: "name.full"},
				{Name: "email", Gen: "internet.email", From: "name"},
			}},
			"order": {Name: "order", Fields: []*schema.Field{
				{Name: "customer_id", Ref: "customer.id"},
				{Name: "total", Gen: "number", Params: data.Params{"min": 1, "max": 500}},
			}},
		},
		Order:  []string{"customer", "order"},
		Counts: map[string]int{"customer": 200, "order": 1000},
	}
	if err := p.Resolve(); err != nil {
		panic(err)
	}
	return p
}

func TestDeterministic(t *testing.T) {
	p := buildPlan()
	a, _ := p.Generate("customer", 99, 50)
	b, _ := p.Generate("customer", 99, 50)
	for i := range a {
		if a[i].Get("email") != b[i].Get("email") {
			t.Fatalf("row %d not deterministic", i)
		}
	}
}

func TestEmailCoherence(t *testing.T) {
	p := buildPlan()
	recs, _ := p.Generate("customer", 7, 100)
	for _, r := range recs {
		name := r.Get("name").(string)
		email := r.Get("email").(string)
		want := strings.ReplaceAll(strings.ToLower(name), " ", ".") + "@"
		if !strings.HasPrefix(email, want) {
			t.Fatalf("email %q not coherent with name %q (want prefix %q)", email, name, want)
		}
	}
}

func TestUniqueness(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"u": {Name: "u", Fields: []*schema.Field{
				{Name: "name", Gen: "name.full"},
				{Name: "email", Gen: "internet.email", From: "name", Unique: true},
				// base range 0..5, but unique over 500 rows: permutation widens it
				{Name: "code", Gen: "number", Params: data.Params{"min": 0, "max": 5}, Unique: true},
			}},
		},
		Order:  []string{"u"},
		Counts: map[string]int{"u": 500},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("u", 9, 500)

	emails := map[string]bool{}
	codes := map[int]bool{}
	for _, r := range recs {
		e := r.Get("email").(string)
		if emails[e] {
			t.Fatalf("duplicate email: %s", e)
		}
		emails[e] = true
		c := r.Get("code").(int)
		if codes[c] {
			t.Fatalf("duplicate code: %d", c)
		}
		codes[c] = true
	}
	if len(emails) != 500 || len(codes) != 500 {
		t.Fatalf("expected 500 unique of each, got emails=%d codes=%d", len(emails), len(codes))
	}
	// the Feistel permutation must cover [0,500) exactly
	for c := range codes {
		if c < 0 || c >= 500 {
			t.Fatalf("permuted code %d out of [0,500)", c)
		}
	}
}

func TestForeignKeyIntegrity(t *testing.T) {
	p := buildPlan()
	recs, _ := p.Generate("order", 3, 1000)
	for _, r := range recs {
		cid := r.Get("customer_id").(int)
		if cid < 0 || cid >= 200 {
			t.Fatalf("customer_id %d out of range [0,200)", cid)
		}
	}
}
