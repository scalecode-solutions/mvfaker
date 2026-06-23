package schema_test

import (
	"testing"

	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/scalecode-solutions/mvfaker/schema"
)

// #7 — auth.uname must equal the referenced user's email, re-derived at the FK id.
func TestCrossEntityProjection(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users": {Name: "users", Fields: []*schema.Field{
				{Name: "email", Gen: "internet.email", Unique: true},
			}},
			"auth": {Name: "auth", Fields: []*schema.Field{
				{Name: "user_id", Ref: "users.id"},
				{Name: "uname", Gen: "copy", From: "user_id.email"},
			}},
		},
		Order:  []string{"users", "auth"},
		Counts: map[string]int{"users": 150, "auth": 150},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	users, _ := p.Generate("users", 7, 150)
	email := map[int]string{}
	for _, u := range users {
		email[u.Get("id").(int)] = u.Get("email").(string)
	}
	auth, _ := p.Generate("auth", 7, 150)
	for _, a := range auth {
		uid := a.Get("user_id").(int)
		if a.Get("uname").(string) != email[uid] {
			t.Fatalf("uname %q != users[%d].email %q", a.Get("uname"), uid, email[uid])
		}
	}
}

// #8 — deactivated_at is set iff state == deactivated.
func TestConditionalWhen(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"u": {Name: "u", Fields: []*schema.Field{
				{Name: "state", Gen: "oneof", Params: data.Params{
					"values": []any{"active", "deactivated", "deleted"}, "weights": []any{5, 3, 2}}},
				{Name: "deactivated_at", Gen: "timestamp", When: "state == deactivated"},
				{Name: "any_but_active", Gen: "timestamp", When: "state != active"},
			}},
		},
		Order:  []string{"u"},
		Counts: map[string]int{"u": 200},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("u", 3, 200)
	seenDeact := false
	for _, r := range recs {
		st := r.Get("state").(string)
		hasDeact := r.Get("deactivated_at") != nil
		if hasDeact != (st == "deactivated") {
			t.Fatalf("state=%s but deactivated_at set=%v", st, hasDeact)
		}
		if (r.Get("any_but_active") != nil) != (st != "active") {
			t.Fatalf("state=%s but any_but_active mismatch", st)
		}
		seenDeact = seenDeact || st == "deactivated"
	}
	if !seenDeact {
		t.Fatal("no deactivated rows generated — test is vacuous")
	}
}
