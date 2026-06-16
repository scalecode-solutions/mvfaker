// Package schema is the dataset layer: entities, references and the runner that
// owns all cross-row state (FKs, ordering). The value layer stays pure; every
// stateful concern lives here.
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tmarq/mvfaker/data"
	"github.com/tmarq/mvfaker/gen"
)

// Field is one column of an entity.
type Field struct {
	Name   string
	Gen    string      // builtin/registered name (empty when Ref is set)
	From   string      // coherence: derive from this sibling field's value
	Ref    string      // FK: "entity.id"
	Params data.Params // declarative attrs

	make data.MakeFn // resolved from the registry
}

// Entity is a named record shape.
type Entity struct {
	Name   string
	Fields []*Field
}

// Plan is the full dataset description: shapes plus how many of each.
type Plan struct {
	Entities map[string]*Entity
	Order    []string       // stable entity order
	Counts   map[string]int // dataset cardinalities
}

// Resolve wires each non-ref field to its registered builder. Call once before
// generating.
func (p *Plan) Resolve() error {
	for _, name := range p.Order {
		e := p.Entities[name]
		for _, f := range e.Fields {
			if f.Ref != "" {
				continue
			}
			mk, err := data.Build(f.Gen, f.Params)
			if err != nil {
				return fmt.Errorf("entity %q field %q: %w", name, f.Name, err)
			}
			f.make = mk
		}
	}
	return nil
}

// Record is an ordered set of field values.
type Record struct {
	Keys []string
	Vals map[string]any
}

func newRecord() *Record { return &Record{Vals: map[string]any{}} }

func (r *Record) Set(k string, v any) {
	if _, ok := r.Vals[k]; !ok {
		r.Keys = append(r.Keys, k)
	}
	r.Vals[k] = v
}

func (r *Record) Get(k string) any { return r.Vals[k] }

// MarshalJSON preserves field order.
func (r *Record) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range r.Keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := json.Marshal(r.Vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// genRecord builds one record. refFn resolves a "entity.id" FK to a concrete id.
func (p *Plan) genRecord(e *Entity, s gen.Source, id int, refFn func(ref string) any) *Record {
	rec := newRecord()
	rec.Set("id", id)
	for _, f := range e.Fields {
		if f.Ref != "" {
			rec.Set(f.Name, refFn(f.Ref))
			continue
		}
		var dep any
		if f.From != "" {
			dep = rec.Get(f.From)
		}
		rec.Set(f.Name, f.make(dep).Generate(s.Split()))
	}
	return rec
}

func hashStr(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
