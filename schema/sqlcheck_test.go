package schema_test

import (
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/schema"
)

const ddl = `
CREATE TABLE users (
    id      integer PRIMARY KEY,
    name    text NOT NULL,
    email   text NOT NULL UNIQUE,
    city    text NOT NULL,
    age     integer
);
CREATE INDEX ON users (city);
`

func usersPlan(fields ...*schema.Field) *schema.Plan {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{"users": {Name: "users", Fields: fields}},
		Order:    []string{"users"},
		Counts:   map[string]int{"users": 10},
	}
	if err := p.Resolve(); err != nil {
		panic(err)
	}
	return p
}

func errorsOnly(iss []schema.Issue) []schema.Issue {
	var out []schema.Issue
	for _, i := range iss {
		if i.Level == "error" {
			out = append(out, i)
		}
	}
	return out
}

func TestParseSQLSchema(t *testing.T) {
	tables := schema.ParseSQLSchema(ddl)
	cols := tables["users"]
	got := make([]string, len(cols))
	for i, c := range cols {
		got[i] = c.Name
	}
	want := "id, name, email, city, age"
	if strings.Join(got, ", ") != want {
		t.Fatalf("parsed columns = %q, want %q", strings.Join(got, ", "), want)
	}
}

func TestCheckGoodPlan(t *testing.T) {
	p := usersPlan(
		&schema.Field{Name: "name", Gen: "name.full"},
		&schema.Field{Name: "email", Gen: "internet.email", From: "name"},
		&schema.Field{Name: "city", Gen: "address.city"},
	)
	if e := errorsOnly(p.CheckSchema(schema.ParseSQLSchema(ddl))); len(e) != 0 {
		t.Fatalf("good plan reported errors: %v", e)
	}
}

func TestCheckWrongColumn(t *testing.T) {
	p := usersPlan(
		&schema.Field{Name: "name", Gen: "name.full"},
		&schema.Field{Name: "town", Gen: "address.city"}, // table has "city"
	)
	e := errorsOnly(p.CheckSchema(schema.ParseSQLSchema(ddl)))
	if len(e) != 1 || !strings.Contains(e[0].Msg, `"town" not found`) {
		t.Fatalf("expected a 'town not found' error, got %v", e)
	}
}

func TestCheckTypeClash(t *testing.T) {
	p := usersPlan(
		&schema.Field{Name: "name", Gen: "name.full"},
		&schema.Field{Name: "age", Gen: "name.full"}, // text into integer column
	)
	e := errorsOnly(p.CheckSchema(schema.ParseSQLSchema(ddl)))
	if len(e) != 1 || !strings.Contains(e[0].Msg, "emits text") {
		t.Fatalf("expected a type-clash error on age, got %v", e)
	}
}
