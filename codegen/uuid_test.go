package codegen_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/codegen"
	"github.com/scalecode-solutions/mvfaker/schema"
)

func TestEmitUUIDAndProjection(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users": {Name: "users", IDType: "uuid", Fields: []*schema.Field{
				{Name: "email", Gen: "internet.email", Unique: true},
			}},
			"auth": {Name: "auth", Fields: []*schema.Field{
				{Name: "user_id", Ref: "users.id"},
				{Name: "uname", Gen: "copy", From: "user_id.email"},
			}},
		},
		Order:  []string{"users", "auth"},
		Counts: map[string]int{"users": 100, "auth": 100},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := codegen.Emit(p, "fixtures", &b); err != nil { // Emit runs go/format → valid Go
		t.Fatalf("emit: %v", err)
	}
	out := b.String()
	for _, want := range []string{
		`schema.UUIDFor(seed, "users", id)`,         // uuid primary key
		`ix_user_id := schema.RefIndex`,             // FK index var
		`schema.UUIDFor(seed, "users", ix_user_id)`, // FK encoded as the target's uuid
		`GenUsers(seed, ix_user_id, 100).Email`,     // cross-entity projection re-derives the row
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated code missing %q", want)
		}
	}
}

func TestEmitRefusesModifiers(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"u": {Name: "u", Fields: []*schema.Field{
				{Name: "handle", Gen: "name.first", MaxLen: 30}, // modifier codegen can't replicate
			}},
		},
		Order:  []string{"u"},
		Counts: map[string]int{"u": 10},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	if err := codegen.Emit(p, "fixtures", new(bytes.Buffer)); err == nil {
		t.Fatal("expected codegen to refuse a field modifier, got nil error")
	}
}
