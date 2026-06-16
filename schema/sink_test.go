package schema

import (
	"strings"
	"testing"
)

func TestCopySinkFormat(t *testing.T) {
	var b strings.Builder
	s := NewCopySink(&b)
	e := &Entity{Name: "customer"}
	s.Begin(e)

	r1 := newRecord()
	r1.Set("id", 0)
	r1.Set("name", "Ann\tLee") // embedded tab must be escaped
	r1.Set("vip", true)
	r1.Set("note", nil)
	s.Write(r1)

	r2 := newRecord()
	r2.Set("id", 1)
	r2.Set("name", "Bob")
	r2.Set("vip", false)
	r2.Set("note", "hi")
	s.Write(r2)

	s.End(e)
	s.Close()

	out := b.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	const tab = "\t" // real field separator
	if lines[0] != "COPY customer (id, name, vip, note) FROM stdin;" {
		t.Fatalf("bad header: %q", lines[0])
	}
	// embedded tab escaped to \t, bool→t, nil→\N
	want1 := "0" + tab + `Ann\tLee` + tab + "t" + tab + `\N`
	if lines[1] != want1 {
		t.Fatalf("bad row1:\n got %q\nwant %q", lines[1], want1)
	}
	want2 := "1" + tab + "Bob" + tab + "f" + tab + "hi"
	if lines[2] != want2 {
		t.Fatalf("bad row2:\n got %q\nwant %q", lines[2], want2)
	}
	if lines[3] != `\.` {
		t.Fatalf("missing terminator, got %q", lines[3])
	}
}
