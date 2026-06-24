// Package schema is the dataset layer: entities, references and the runner that
// owns all cross-row state (FKs, ordering). The value layer stays pure; every
// stateful concern lives here.
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/scalecode-solutions/mvfaker/gen"
)

// Field is one column of an entity.
type Field struct {
	Name   string
	Gen    string      // builtin/registered name (empty when Ref is set)
	From   string      // coherence: derive from this sibling field's value
	Ref    string      // FK: "entity.id"
	Unique bool        // dataset-layer uniqueness (runner-enforced)
	Params data.Params // declarative attrs

	// Field-level modifiers, applied (in order) after the generator produces a
	// value: transform → maxlen → unique → when → null. They work with any generator.
	Transform string  // lower | upper | slug | title (string values)
	MaxLen    int     // truncate string values to this length (0 = no limit)
	NullProb  float64 // probability in [0,1] the value is NULL instead
	When      string  // condition on a sibling: "state == deactivated"; NULL unless it holds
	UniqueSep *string // separator for a unique string suffix; nil = default (. / +@), "" = alnum-safe

	make data.MakeFn // resolved from the registry
}

// Entity is a named record shape.
type Entity struct {
	Name   string
	IDType string // "int" (default, the row index), "uuid", or "none" (composite PK, no id column)
	// DistinctPair names two ref fields that jointly form a unique key; their
	// indices are derived from one permuted pair-index so pairs never collide.
	// If both reference the same entity, the diagonal is excluded (no self-edges).
	DistinctPair []string
	Fields       []*Field
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
			if f.Ref != "" || f.Gen == "sequence" { // sequence is filled by the runner, not a builder
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

// maxProjDepth bounds cross-entity projection so a mutual projection (A↔B) can't
// recurse forever.
const maxProjDepth = 8

// genRecord builds one record. id is the row index; count is the entity's total
// row count (the uniqueness domain). refFn resolves an "entity.id" FK. depth
// tracks cross-entity projection recursion.
func (p *Plan) genRecord(e *Entity, s gen.Source, id, count int, seed uint64, depth int) *Record {
	rec := newRecord()
	if e.IDType != "none" { // "none" → composite PK, emit no id column (#15)
		rec.Set("id", encodeID(e.IDType, seed, e.Name, id))
	}
	if depth > maxProjDepth {
		return rec
	}
	pair := p.distinctPairIdx(e, id, seed) // #17 jointly-unique ref indices
	refIdx := map[string]int{}             // ref field -> drawn target row index (for projection)
	for _, f := range e.Fields {
		if f.Ref != "" {
			target := refTarget(f.Ref)
			tc := p.Counts[target]
			var idx int
			if pi, ok := pair[f.Name]; ok { // part of a distinct pair (#17)
				idx = pi
			} else if f.Unique { // distinct 1:1 ref (#16) — requires count <= target count
				idx = permuteIndex(id, tc, hashStr(e.Name+"."+f.Name)^seed)
			} else {
				idx = RefIndex(seed, e.Name, id, f.Ref, tc)
			}
			refIdx[f.Name] = idx
			te := p.Entities[target]
			if te != nil && te.IDType == "none" {
				continue // ref to a composite-PK entity (#19): a projection source only, not a column
			}
			it := "int"
			if te != nil {
				it = te.IDType
			}
			rec.Set(f.Name, encodeID(it, seed, target, idx)) // FK encoded as target's id type
			continue
		}
		if f.Gen == "sequence" { // #20 — filled by the runner's per-parent counter pass
			rec.Set(f.Name, 0)
			continue
		}
		var dep any
		if f.From != "" {
			if local, target, ok := splitProjection(f.From); ok {
				dep = p.projectField(e, refIdx, local, target, seed, depth) // #7 cross-entity
			} else {
				dep = rec.Get(f.From) // within-entity coherence
			}
		}
		val := f.make(dep).Generate(s.Split())
		if f.Transform != "" {
			val = applyTransform(f.Transform, val)
		}
		if f.MaxLen > 0 {
			val = truncate(val, f.MaxLen)
		}
		if f.Unique {
			if f.UniqueSep != nil { // configurable separator (e.g. "" for strict ^[a-z0-9]+$ handles)
				val = UniqueValueSep(val, id, count, seed, e.Name, f.Name, *f.UniqueSep)
			} else {
				val = UniqueValue(val, id, count, seed, e.Name, f.Name)
			}
		}
		if f.When != "" && !evalWhen(f.When, rec) { // #8 conditional coherence
			val = nil
		}
		if f.NullProb > 0 && drawNull(s.Split(), f.NullProb) {
			val = nil
		}
		rec.Set(f.Name, val)
	}
	return rec
}

// splitProjection detects a cross-entity from of the form "ref_field.target".
func splitProjection(from string) (local, target string, ok bool) {
	if i := strings.IndexByte(from, '.'); i >= 0 {
		return from[:i], from[i+1:], true
	}
	return "", "", false
}

func refTarget(ref string) string {
	if k := strings.IndexByte(ref, '.'); k >= 0 {
		return ref[:k]
	}
	return ref
}

func (p *Plan) fieldRefTarget(e *Entity, name string) string {
	for _, f := range e.Fields {
		if f.Name == name && f.Ref != "" {
			return refTarget(f.Ref)
		}
	}
	return ""
}

// distinctPairIdx derives the two ref indices for a distinct_pair entity at row
// id: a permutation of the pair-index space, so pairs are unique by construction.
// Same-target pairs exclude the diagonal (no self-edges). Distinctness holds when
// the entity's row count <= the pair-space size.
func (p *Plan) distinctPairIdx(e *Entity, id int, seed uint64) map[string]int {
	if len(e.DistinctPair) != 2 {
		return nil
	}
	fa, fb := e.DistinctPair[0], e.DistinctPair[1]
	ta, tb := p.fieldRefTarget(e, fa), p.fieldRefTarget(e, fb)
	ca, cb := p.Counts[ta], p.Counts[tb]
	if ca <= 0 || cb <= 0 {
		return nil
	}
	key := hashStr(e.Name+".pair") ^ seed ^ 0x5151
	if ta == tb { // same target: exclude self-edges, space = U*(U-1)
		u := ca
		if u < 2 {
			return map[string]int{fa: 0, fb: 0}
		}
		pi := permuteIndex(id, u*(u-1), key)
		a, b := pi/(u-1), pi%(u-1)
		if b >= a {
			b++ // skip the diagonal a==b
		}
		return map[string]int{fa: a, fb: b}
	}
	pi := permuteIndex(id, ca*cb, key) // distinct (convIdx, userIdx) pairs
	return map[string]int{fa: pi / cb, fb: pi % cb}
}

// encodeID renders a row index as its emitted primary key: the int index itself,
// or a deterministic UUID for that (seed, entity, index).
func encodeID(idType string, seed uint64, entity string, index int) any {
	if idType == "uuid" {
		return deterministicUUID(seed, entity, index)
	}
	return index
}

// UUIDFor returns the deterministic UUID for a row index — exported so generated
// code (codegen) emits the same id the interpreter does.
func UUIDFor(seed uint64, entity string, index int) string {
	return deterministicUUID(seed, entity, index)
}

// deterministicUUID derives a stable RFC 4122 v4 UUID from (seed, entity, index),
// so an entity's id and any FK to it agree, and re-runs are reproducible.
func deterministicUUID(seed uint64, entity string, index int) string {
	src := gen.At(seed, hashStr(entity), uint64(index), 0x7551d) // distinct salt path
	var b [16]byte
	for i := range b {
		b[i] = byte(src.Draw(256))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// projectField re-derives the row referenced by local (a ref field already set
// in rec) at its drawn id, and returns its target field — so a field can equal a
// referenced row's value (auth.uname == users.email). Coherent by construction:
// positional determinism means the re-derived row is identical to the stored one.
func (p *Plan) projectField(e *Entity, refIdx map[string]int, local, target string, seed uint64, depth int) any {
	idx, ok := refIdx[local] // the drawn row index, not the emitted id
	if !ok {
		return nil
	}
	te := ""
	for _, lf := range e.Fields {
		if lf.Name == local && lf.Ref != "" {
			te = refTarget(lf.Ref)
			break
		}
	}
	ent := p.Entities[te]
	if ent == nil {
		return nil
	}
	tr := p.genRecord(ent, RowSource(seed, te, idx), idx, p.Counts[te], seed, depth+1)
	return tr.Get(target)
}

// evalWhen reports whether a "field op value" condition holds against rec.
// Supports ==, !=, and "in [a, b, …]".
func evalWhen(cond string, rec *Record) bool {
	cond = strings.TrimSpace(cond)
	if i := strings.Index(cond, " in "); i >= 0 {
		field := strings.TrimSpace(cond[:i])
		list := strings.Trim(strings.TrimSpace(cond[i+4:]), "[]")
		actual := fmt.Sprint(rec.Get(field))
		for _, v := range strings.Split(list, ",") {
			if strings.TrimSpace(v) == actual {
				return true
			}
		}
		return false
	}
	for _, op := range []string{"!=", "=="} {
		if i := strings.Index(cond, op); i >= 0 {
			field := strings.TrimSpace(cond[:i])
			val := strings.TrimSpace(cond[i+len(op):])
			eq := fmt.Sprint(rec.Get(field)) == val
			if op == "==" {
				return eq
			}
			return !eq
		}
	}
	return true // unparseable condition → don't gate
}

func applyTransform(t string, v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	switch t {
	case "lower":
		return strings.ToLower(s)
	case "upper":
		return strings.ToUpper(s)
	case "title":
		return strings.Title(s)
	case "slug":
		return slugify(s)
	}
	return v
}

func truncate(v any, n int) any {
	if s, ok := v.(string); ok && len(s) > n {
		return s[:n]
	}
	return v
}

func drawNull(s gen.Source, p float64) bool {
	const scale = 1_000_000
	return s.Draw(scale) < uint64(p*scale)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	return b.String()
}

func hashStr(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
