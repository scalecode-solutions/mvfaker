package schema

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/scalecode-solutions/mvfaker/data"
)

// JSON spec — a third front-end onto Plan (alongside HCL and code), so callers
// that speak JSON (e.g. the MCP server, an agent) drive the same engine.

// FieldSpec is one field in a JSON dataset spec.
type FieldSpec struct {
	Name      string         `json:"name"`
	Gen       string         `json:"gen,omitempty"`       // generator name (omit for ref)
	From      string         `json:"from,omitempty"`      // coherence: derive from this field
	Ref       string         `json:"ref,omitempty"`       // FK: "entity.id"
	Unique    bool           `json:"unique,omitempty"`    // runner-enforced uniqueness
	Transform string         `json:"transform,omitempty"` // lower|upper|slug|title
	MaxLen    int            `json:"maxlen,omitempty"`    // truncate strings to length
	NullProb  float64        `json:"null_prob,omitempty"` // probability the value is NULL
	When      string         `json:"when,omitempty"`      // NULL unless condition holds, e.g. "state == deactivated"
	Params    map[string]any `json:"params,omitempty"`    // generator params (min, max, n, locale…)
}

// EntitySpec is one entity (table/collection) in a JSON dataset spec.
type EntitySpec struct {
	Name   string      `json:"name"`
	IDType string      `json:"id_type,omitempty"` // "int" (default) or "uuid"
	Count  int         `json:"count"`
	Fields []FieldSpec `json:"fields"`
}

// Spec is a full JSON dataset description.
type Spec struct {
	Entities []EntitySpec `json:"entities"`
}

// Plan builds and resolves a *Plan from the spec.
func (s Spec) Plan() (*Plan, error) {
	if len(s.Entities) == 0 {
		return nil, fmt.Errorf("spec has no entities")
	}
	p := &Plan{Entities: map[string]*Entity{}, Counts: map[string]int{}}
	for _, es := range s.Entities {
		if es.Name == "" {
			return nil, fmt.Errorf("entity missing name")
		}
		e := &Entity{Name: es.Name, IDType: es.IDType}
		for _, fs := range es.Fields {
			f := &Field{
				Name: fs.Name, Gen: fs.Gen, From: fs.From, Ref: fs.Ref, Unique: fs.Unique,
				Transform: fs.Transform, MaxLen: fs.MaxLen, NullProb: fs.NullProb, When: fs.When,
				Params: data.Params{},
			}
			for k, v := range fs.Params {
				f.Params[k] = v
			}
			e.Fields = append(e.Fields, f)
		}
		p.Entities[es.Name] = e
		p.Order = append(p.Order, es.Name)
		cnt := es.Count
		if cnt <= 0 {
			cnt = 10
		}
		p.Counts[es.Name] = cnt
	}
	if err := p.Resolve(); err != nil {
		return nil, err
	}
	return p, nil
}

// LoadJSON parses a JSON dataset spec into a resolved Plan.
func LoadJSON(b []byte) (*Plan, error) {
	var s Spec
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return s.Plan()
}

// Render seeds the plan into a string in the given format (json | sql | copy).
// Returns the rendered data and the total row count.
func (p *Plan) Render(seed uint64, format string) (string, int, error) {
	var buf bytes.Buffer
	var sink Sink
	switch format {
	case "sql":
		sink = NewSQLSink(&buf)
	case "copy":
		sink = NewCopySink(&buf)
	default:
		sink = NewJSONSink(&buf)
	}
	if err := p.Seed(seed, sink); err != nil {
		return "", 0, err
	}
	rows := 0
	for _, c := range p.Counts {
		rows += c
	}
	return buf.String(), rows, nil
}
