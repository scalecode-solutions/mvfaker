package schema_test

import (
	"testing"

	"github.com/scalecode-solutions/mvfaker/schema"
)

// The validator must know what the seed path knows about id_type: a "none"
// entity emits no id column (composite PK), and uuid ids/FKs are text.
func TestCheckRespectsIDType(t *testing.T) {
	ddl := `
CREATE TABLE users (id uuid PRIMARY KEY, email text NOT NULL);
CREATE TABLE members (
    user_id uuid NOT NULL,
    peer_id uuid NOT NULL,
    PRIMARY KEY (user_id, peer_id)
);`
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users": {Name: "users", IDType: "uuid", Fields: []*schema.Field{
				{Name: "email", Gen: "internet.email"},
			}},
			"members": {Name: "members", IDType: "none", Fields: []*schema.Field{
				{Name: "user_id", Ref: "users.id"}, // uuid FK → must check as text
				{Name: "peer_id", Ref: "users.id"},
			}},
		},
		Order:  []string{"users", "members"},
		Counts: map[string]int{"users": 10, "members": 10},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	// No errors: members has no id column (and the table has none), uuid id and
	// uuid FKs match their text-ish columns.
	if e := errorsOnly(p.CheckSchema(schema.ParseSQLSchema(ddl))); len(e) != 0 {
		t.Fatalf("composite-PK / uuid plan reported errors: %v", e)
	}
}

// Regression: a "none" entity whose table genuinely lacks a wanted column still errors.
func TestCheckNoneStillCatchesWrongColumn(t *testing.T) {
	ddl := `CREATE TABLE members (user_id uuid NOT NULL, peer_id uuid NOT NULL, PRIMARY KEY (user_id, peer_id));`
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"members": {Name: "members", IDType: "none", Fields: []*schema.Field{
				{Name: "user_id", Gen: "uuid"},
				{Name: "wrong_col", Gen: "uuid"}, // not in the table
			}},
		},
		Order:  []string{"members"},
		Counts: map[string]int{"members": 10},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	e := errorsOnly(p.CheckSchema(schema.ParseSQLSchema(ddl)))
	if len(e) != 1 {
		t.Fatalf("expected 1 error for wrong_col, got %v", e)
	}
}
