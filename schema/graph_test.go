package schema_test

import (
	"testing"

	"github.com/scalecode-solutions/mvfaker/schema"
)

// #15 — id_type="none" emits no id column.
func TestIDSuppression(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users": {Name: "users", Fields: []*schema.Field{{Name: "email", Gen: "internet.email"}}},
			"members": {Name: "members", IDType: "none", Fields: []*schema.Field{
				{Name: "user_id", Ref: "users.id"},
			}},
		},
		Order:  []string{"users", "members"},
		Counts: map[string]int{"users": 10, "members": 10},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("members", 1, 10)
	for _, r := range recs {
		if r.Get("id") != nil {
			t.Fatalf("id_type=none should emit no id, got %v", r.Get("id"))
		}
		if len(r.Keys) != 1 || r.Keys[0] != "user_id" {
			t.Fatalf("expected only user_id, got %v", r.Keys)
		}
	}
}

// #16 — a unique ref gives a distinct target per row (1:1).
func TestUniqueRef(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users": {Name: "users", Fields: []*schema.Field{{Name: "email", Gen: "internet.email"}}},
			"auth": {Name: "auth", Fields: []*schema.Field{
				{Name: "user_id", Ref: "users.id", Unique: true},
			}},
		},
		Order:  []string{"users", "auth"},
		Counts: map[string]int{"users": 100, "auth": 100},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("auth", 1, 100)
	seen := map[int]bool{}
	for _, r := range recs {
		uid := r.Get("user_id").(int)
		if seen[uid] {
			t.Fatalf("unique ref produced duplicate user_id %d", uid)
		}
		seen[uid] = true
	}
	if len(seen) != 100 {
		t.Fatalf("expected 100 distinct refs, got %d", len(seen))
	}
}

// #17 — distinct_pair yields unique pairs; same-target pairs avoid self-edges.
func TestDistinctPair(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users":         {Name: "users", Fields: []*schema.Field{{Name: "email", Gen: "internet.email"}}},
			"conversations": {Name: "conversations", Fields: []*schema.Field{{Name: "title", Gen: "lorem.word"}}},
			"members": {Name: "members", IDType: "none", DistinctPair: []string{"conversation_id", "user_id"},
				Fields: []*schema.Field{
					{Name: "conversation_id", Ref: "conversations.id"},
					{Name: "user_id", Ref: "users.id"},
				}},
			"contacts": {Name: "contacts", IDType: "none", DistinctPair: []string{"user_id", "contact_id"},
				Fields: []*schema.Field{
					{Name: "user_id", Ref: "users.id"},
					{Name: "contact_id", Ref: "users.id"},
				}},
		},
		Order:  []string{"users", "conversations", "members", "contacts"},
		Counts: map[string]int{"users": 30, "conversations": 15, "members": 120, "contacts": 80},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}

	members, _ := p.Generate("members", 2, 120)
	mseen := map[[2]int]bool{}
	for _, r := range members {
		k := [2]int{r.Get("conversation_id").(int), r.Get("user_id").(int)}
		if mseen[k] {
			t.Fatalf("members pair collision: %v", k)
		}
		mseen[k] = true
	}

	contacts, _ := p.Generate("contacts", 2, 80)
	cseen := map[[2]int]bool{}
	for _, r := range contacts {
		u, c := r.Get("user_id").(int), r.Get("contact_id").(int)
		if u == c {
			t.Fatalf("contacts self-edge: user_id == contact_id == %d", u)
		}
		k := [2]int{u, c}
		if cseen[k] {
			t.Fatalf("contacts pair collision: %v", k)
		}
		cseen[k] = true
	}
}
