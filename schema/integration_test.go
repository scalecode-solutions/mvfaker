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
