// Command codeseed builds the same forum dataset as integration/seed/forum.hcl,
// but in Go instead of HCL — config and code are two faces of one engine, so
// this emits byte-identical COPY for the same seed.
package main

import (
	"os"

	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/scalecode-solutions/mvfaker/schema"
)

// BuildForumPlan constructs the forum dataset in code. It mirrors
// integration/seed/forum.hcl field-for-field.
func BuildForumPlan() *schema.Plan {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users": {Name: "users", Fields: []*schema.Field{
				{Name: "name", Gen: "name.full"},
				{Name: "email", Gen: "internet.email", From: "name", Unique: true},
				{Name: "country", Gen: "address.country"},
				{Name: "city", Gen: "address.city", From: "country"},
				{Name: "joined", Gen: "date", Params: data.Params{"min": 2019, "max": 2024}},
			}},
			"posts": {Name: "posts", Fields: []*schema.Field{
				{Name: "author_id", Ref: "users.id"},
				{Name: "title", Gen: "lorem.words", Params: data.Params{"n": 5}},
				{Name: "body", Gen: "lorem.words", Params: data.Params{"n": 30}},
				{Name: "created", Gen: "date", Params: data.Params{"min": 2022, "max": 2024}},
			}},
			"comments": {Name: "comments", Fields: []*schema.Field{
				{Name: "post_id", Ref: "posts.id"},
				{Name: "author_id", Ref: "users.id"},
				{Name: "body", Gen: "lorem.words", Params: data.Params{"n": 12}},
			}},
		},
		Order:  []string{"users", "posts", "comments"},
		Counts: map[string]int{"users": 5000, "posts": 20000, "comments": 80000},
	}
	if err := p.Resolve(); err != nil {
		panic(err)
	}
	return p
}

func main() {
	if err := BuildForumPlan().Seed(1, schema.NewCopySink(os.Stdout)); err != nil {
		panic(err)
	}
}
