package schema_test

import (
	"regexp"
	"testing"

	"github.com/scalecode-solutions/mvfaker/schema"
)

var v4uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDIdsAndMixedFKEncoding(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users": {Name: "users", IDType: "uuid", Fields: []*schema.Field{
				{Name: "email", Gen: "internet.email", Unique: true},
			}},
			"auth": {Name: "auth", Fields: []*schema.Field{ // int-keyed, FK → uuid
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

	users, _ := p.Generate("users", 5, 100)
	email := map[string]string{}
	for _, u := range users {
		id, ok := u.Get("id").(string)
		if !ok || !v4uuid.MatchString(id) {
			t.Fatalf("users.id is not a v4 uuid: %v", u.Get("id"))
		}
		email[id] = u.Get("email").(string)
	}
	if len(email) != 100 {
		t.Fatalf("expected 100 distinct uuids, got %d", len(email))
	}

	auth, _ := p.Generate("auth", 5, 100)
	for _, a := range auth {
		// auth.id stays int (no id_type)
		if _, ok := a.Get("id").(int); !ok {
			t.Fatalf("auth.id should be int, got %T", a.Get("id"))
		}
		// FK to a uuid table is encoded as that uuid, and projection still resolves
		uid := a.Get("user_id").(string)
		if e, ok := email[uid]; !ok {
			t.Fatalf("auth.user_id %q is not a real user uuid", uid)
		} else if a.Get("uname").(string) != e {
			t.Fatalf("uname %q != user's email %q", a.Get("uname"), e)
		}
	}

	// deterministic: same seed → same uuids
	again, _ := p.Generate("users", 5, 100)
	if again[7].Get("id") != users[7].Get("id") {
		t.Fatal("uuid id not deterministic across runs")
	}
}
