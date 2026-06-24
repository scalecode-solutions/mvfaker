package schema_test

import (
	"sort"
	"testing"

	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/scalecode-solutions/mvfaker/schema"
)

func messageGraph() *schema.Plan {
	return &schema.Plan{
		Entities: map[string]*schema.Entity{
			"users":         {Name: "users", Fields: []*schema.Field{{Name: "email", Gen: "internet.email"}}},
			"conversations": {Name: "conversations", Fields: []*schema.Field{{Name: "title", Gen: "lorem.word"}}},
			"members": {Name: "members", IDType: "none", DistinctPair: []string{"conversation_id", "user_id"},
				Fields: []*schema.Field{
					{Name: "conversation_id", Ref: "conversations.id"},
					{Name: "user_id", Ref: "users.id"},
				}},
			"messages": {Name: "messages", Fields: []*schema.Field{
				{Name: "member", Ref: "members"}, // composite-PK ref: projection source only
				{Name: "conversation_id", Gen: "copy", From: "member.conversation_id"},
				{Name: "from_user_id", Gen: "copy", From: "member.user_id"},
				{Name: "seq", Gen: "sequence", Params: data.Params{"within": "conversation_id"}},
			}},
		},
		Order:  []string{"users", "conversations", "members", "messages"},
		Counts: map[string]int{"users": 30, "conversations": 10, "members": 80, "messages": 1500},
	}
}

// #19 — a message's sender is a member of its conversation, by construction.
func TestMemberCoherentSenders(t *testing.T) {
	p := messageGraph()
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	members, _ := p.Generate("members", 1, 80)
	mem := map[[2]int]bool{}
	for _, m := range members {
		mem[[2]int{m.Get("conversation_id").(int), m.Get("user_id").(int)}] = true
	}
	msgs, _ := p.Generate("messages", 1, 1500)
	for _, msg := range msgs {
		for _, k := range msg.Keys {
			if k == "member" {
				t.Fatal("'member' (composite-PK ref) must not be emitted as a column")
			}
		}
		pair := [2]int{msg.Get("conversation_id").(int), msg.Get("from_user_id").(int)}
		if !mem[pair] {
			t.Fatalf("sender %v is not a member of its conversation", pair)
		}
	}
}

// #20 — seq is dense 1..N within each conversation (UNIQUE(conversation_id, seq)).
func TestPerParentSequence(t *testing.T) {
	p := messageGraph()
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	msgs, _ := p.Generate("messages", 1, 1500)
	byConv := map[int][]int{}
	for _, msg := range msgs {
		c := msg.Get("conversation_id").(int)
		byConv[c] = append(byConv[c], msg.Get("seq").(int))
	}
	for conv, seqs := range byConv {
		sort.Ints(seqs)
		for i, s := range seqs {
			if s != i+1 {
				t.Fatalf("conversation %d has non-dense seq: %v", conv, seqs)
			}
		}
	}
}
