package codegen_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/codegen"
	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/scalecode-solutions/mvfaker/schema"
)

func TestEmitProducesValidGo(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"customer": {Name: "customer", Fields: []*schema.Field{
				{Name: "name", Gen: "name.full"},
				{Name: "email", Gen: "internet.email", From: "name", Unique: true},
				{Name: "age", Gen: "number", Params: data.Params{"min": 18, "max": 90}},
			}},
			"order": {Name: "order", Fields: []*schema.Field{
				{Name: "customer_id", Ref: "customer.id"},
				{Name: "total", Gen: "number", Params: data.Params{"min": 1, "max": 9}},
			}},
		},
		Order:  []string{"customer", "order"},
		Counts: map[string]int{"customer": 100, "order": 500},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	// Emit runs go/format internally, so a nil error means the output parsed.
	if err := codegen.Emit(p, "fixtures", &b); err != nil {
		t.Fatalf("emit: %v", err)
	}
	out := b.String()

	for _, want := range []string{
		"package fixtures",
		"type Customer struct",
		"type Order struct",
		"func GenCustomer(seed uint64, id, count int) Customer",
		"schema.RowSource(seed, \"customer\", id)",                   // shared seam, not reimplemented
		"schema.UniqueValue(v, id, count, seed",                      // uniqueness via the seam
		"schema.RefIndex(seed, \"order\", id, \"customer.id\", 100)", // FK with target count
		"func SeedCustomers(w io.Writer, seed uint64, n int) error",
		"func SeedAll(w io.Writer, seed uint64) error",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated code missing %q", want)
		}
	}
}
