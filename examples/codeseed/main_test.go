package main

import (
	"strings"
	"testing"
)

// Invariants checked in Go — the same things the DB's constraints enforce, but
// without needing a database: unique+coherent emails and in-range foreign keys.
func TestForumInvariants(t *testing.T) {
	p := BuildForumPlan()

	users, err := p.Generate("users", 1, 5000)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, u := range users {
		email := u.Get("email").(string)
		if seen[email] {
			t.Fatalf("duplicate email: %s", email)
		}
		seen[email] = true
		name := u.Get("name").(string)
		slug := strings.ReplaceAll(strings.ToLower(name), " ", ".")
		if !strings.HasPrefix(email, slug) {
			t.Fatalf("email %q not coherent with name %q", email, name)
		}
	}
	if len(seen) != 5000 {
		t.Fatalf("expected 5000 unique emails, got %d", len(seen))
	}

	posts, _ := p.Generate("posts", 1, 20000)
	for _, po := range posts {
		if aid := po.Get("author_id").(int); aid < 0 || aid >= 5000 {
			t.Fatalf("post author_id %d out of [0,5000)", aid)
		}
	}

	comments, _ := p.Generate("comments", 1, 80000)
	for _, c := range comments {
		if pid := c.Get("post_id").(int); pid < 0 || pid >= 20000 {
			t.Fatalf("comment post_id %d out of [0,20000)", pid)
		}
		if aid := c.Get("author_id").(int); aid < 0 || aid >= 5000 {
			t.Fatalf("comment author_id %d out of [0,5000)", aid)
		}
	}
}
