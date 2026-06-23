package schema_test

import (
	"strings"
	"testing"

	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/scalecode-solutions/mvfaker/schema"
)

func TestFieldModifiers(t *testing.T) {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"u": {Name: "u", Fields: []*schema.Field{
				{Name: "handle", Gen: "name.first", Unique: true},
				{Name: "handle_lower", Gen: "copy", From: "handle", Transform: "lower"},
				{Name: "code", Gen: "lorem.words", Params: data.Params{"n": 12}, MaxLen: 5},
				{Name: "always_null", Gen: "name.first", NullProb: 1.0},
				{Name: "never_null", Gen: "name.first", NullProb: 0.0},
			}},
		},
		Order:  []string{"u"},
		Counts: map[string]int{"u": 50},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("u", 1, 50)
	for _, r := range recs {
		h := r.Get("handle").(string)
		hl := r.Get("handle_lower").(string)
		if hl != strings.ToLower(h) {
			t.Fatalf("transform: handle_lower %q != lower(%q)", hl, h)
		}
		if c := r.Get("code").(string); len(c) > 5 {
			t.Fatalf("maxlen: %q exceeds 5", c)
		}
		if r.Get("always_null") != nil {
			t.Fatal("null_prob=1.0 should always be NULL")
		}
		if r.Get("never_null") == nil {
			t.Fatal("null_prob=0.0 should never be NULL")
		}
	}
}

func TestModifiersViaJSON(t *testing.T) {
	spec := `{"entities":[{"name":"u","count":30,"fields":[
	  {"name":"raw","gen":"name.full"},
	  {"name":"slug","gen":"copy","from":"raw","transform":"slug"},
	  {"name":"state","gen":"oneof","params":{"values":["on","off"],"weights":[9,1]}}
	]}]}`
	p, err := schema.LoadJSON([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	recs, _ := p.Generate("u", 1, 30)
	for _, r := range recs {
		s := r.Get("slug").(string)
		if strings.ContainsAny(s, " ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			t.Fatalf("slug %q not normalized", s)
		}
		if v := r.Get("state").(string); v != "on" && v != "off" {
			t.Fatalf("oneof produced %q", v)
		}
	}
}
