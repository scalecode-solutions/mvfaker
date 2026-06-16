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
