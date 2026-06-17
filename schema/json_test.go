package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/schema"
)

const jsonSpec = `{
  "entities": [
    {"name": "users", "count": 50, "fields": [
      {"name": "name", "gen": "name.full"},
      {"name": "email", "gen": "internet.email", "from": "name", "unique": true}
    ]},
    {"name": "posts", "count": 120, "fields": [
      {"name": "author_id", "ref": "users.id"},
      {"name": "total", "gen": "number", "params": {"min": 1, "max": 9}}
    ]}
  ]
}`

func TestLoadJSONRenderAndIntegrity(t *testing.T) {
	p, err := schema.LoadJSON([]byte(jsonSpec))
	if err != nil {
		t.Fatal(err)
	}
	out, rows, err := p.Render(1, "json")
	if err != nil {
		t.Fatal(err)
	}
	if rows != 170 {
		t.Fatalf("rows = %d, want 170", rows)
	}

	var data struct {
		Users []map[string]any `json:"users"`
		Posts []map[string]any `json:"posts"`
	}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("rendered JSON invalid: %v", err)
	}
	if len(data.Users) != 50 || len(data.Posts) != 120 {
		t.Fatalf("counts off: users=%d posts=%d", len(data.Users), len(data.Posts))
	}

	seen := map[string]bool{}
	for _, u := range data.Users {
		e := u["email"].(string)
		if seen[e] {
			t.Fatalf("duplicate email %q", e)
		}
		seen[e] = true
	}
	for _, po := range data.Posts {
		if aid := po["author_id"].(float64); aid < 0 || aid >= 50 {
			t.Fatalf("post author_id %v out of [0,50)", aid)
		}
	}
}

func TestRenderFormats(t *testing.T) {
	p, _ := schema.LoadJSON([]byte(jsonSpec))
	sql, _, _ := p.Render(1, "sql")
	if !strings.Contains(sql, "INSERT INTO users") {
		t.Fatal("sql render missing INSERT")
	}
	cp, _, _ := p.Render(1, "copy")
	if !strings.Contains(cp, "COPY users (") {
		t.Fatal("copy render missing COPY header")
	}
}
