package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Sink consumes generated records. Begin/End bracket each entity; Close flushes.
type Sink interface {
	Begin(e *Entity) error
	Write(r *Record) error
	End(e *Entity) error
	Close() error
}

// --- SQL sink: streams INSERTs, suited to scale. ---------------------------

type SQLSink struct {
	w     io.Writer
	table string
}

func NewSQLSink(w io.Writer) *SQLSink { return &SQLSink{w: w} }

func (s *SQLSink) Begin(e *Entity) error { s.table = e.Name; return nil }
func (s *SQLSink) End(*Entity) error     { return nil }
func (s *SQLSink) Close() error          { return nil }

func (s *SQLSink) Write(r *Record) error {
	vals := make([]string, len(r.Keys))
	for i, k := range r.Keys {
		vals[i] = sqlValue(r.Vals[k])
	}
	_, err := fmt.Fprintf(s.w, "INSERT INTO %s (%s) VALUES (%s);\n",
		s.table, strings.Join(r.Keys, ", "), strings.Join(vals, ", "))
	return err
}

func sqlValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	case bool:
		if x {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// --- COPY sink: Postgres text format, the fast bulk-load path. -------------
//
// Emits `COPY table (cols) FROM stdin;` + tab-separated rows + `\.`, loadable
// with `psql -f`. Far faster to ingest than per-row INSERTs.

type CopySink struct {
	w       io.Writer
	table   string
	started bool
}

func NewCopySink(w io.Writer) *CopySink { return &CopySink{w: w} }

func (s *CopySink) Begin(e *Entity) error {
	s.table = e.Name
	s.started = false
	return nil
}

func (s *CopySink) Write(r *Record) error {
	if !s.started {
		if _, err := fmt.Fprintf(s.w, "COPY %s (%s) FROM stdin;\n", s.table, strings.Join(r.Keys, ", ")); err != nil {
			return err
		}
		s.started = true
	}
	for i, k := range r.Keys {
		if i > 0 {
			if _, err := io.WriteString(s.w, "\t"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(s.w, copyValue(r.Vals[k])); err != nil {
			return err
		}
	}
	_, err := io.WriteString(s.w, "\n")
	return err
}

func (s *CopySink) End(*Entity) error {
	if s.started {
		_, err := io.WriteString(s.w, "\\.\n")
		return err
	}
	return nil
}

func (s *CopySink) Close() error { return nil }

func copyValue(v any) string {
	switch x := v.(type) {
	case nil:
		return `\N`
	case bool:
		if x {
			return "t"
		}
		return "f"
	case string:
		return escapeCopy(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func escapeCopy(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// --- JSON sink: accumulates into {entity: [...]} , flushed on Close. --------

type JSONSink struct {
	w       io.Writer
	cur     string
	byTable map[string][]*Record
	order   []string
}

func NewJSONSink(w io.Writer) *JSONSink {
	return &JSONSink{w: w, byTable: map[string][]*Record{}}
}

func (s *JSONSink) Begin(e *Entity) error {
	s.cur = e.Name
	if _, ok := s.byTable[e.Name]; !ok {
		s.order = append(s.order, e.Name)
	}
	return nil
}
func (s *JSONSink) Write(r *Record) error {
	s.byTable[s.cur] = append(s.byTable[s.cur], r)
	return nil
}
func (s *JSONSink) End(*Entity) error { return nil }

func (s *JSONSink) Close() error {
	var b strings.Builder
	b.WriteString("{\n")
	for i, name := range s.order {
		if i > 0 {
			b.WriteString(",\n")
		}
		key, _ := json.Marshal(name)
		fmt.Fprintf(&b, "  %s: ", key)
		recs, _ := json.MarshalIndent(s.byTable[name], "  ", "  ")
		b.Write(recs)
	}
	b.WriteString("\n}\n")
	_, err := io.WriteString(s.w, b.String())
	return err
}
