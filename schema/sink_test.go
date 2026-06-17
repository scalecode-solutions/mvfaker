package schema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSVSink(t *testing.T) {
	dir := t.TempDir()
	s := NewCSVDirSink(dir)
	e := &Entity{Name: "users", Fields: []*Field{{Name: "name"}, {Name: "vip"}}}
	if err := s.Begin(e); err != nil {
		t.Fatal(err)
	}
	r := newRecord()
	r.Set("id", 0)
	r.Set("name", "Ann, Lee") // comma must get quoted by encoding/csv
	r.Set("vip", true)
	if err := s.Write(r); err != nil {
		t.Fatal(err)
	}
	if err := s.End(e); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(filepath.Join(dir, "users.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header+1 row, got %d rows", len(rows))
	}
	if strings.Join(rows[0], ",") != "id,name,vip" {
		t.Fatalf("bad header: %v", rows[0])
	}
	if rows[1][1] != "Ann, Lee" || rows[1][2] != "true" {
		t.Fatalf("bad row (quoting/bool): %v", rows[1])
	}
}

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
