package schema

import (
	"fmt"

	"github.com/scalecode-solutions/mvfaker/gen"
)

// RowSource is the canonical entropy address for one row. Both the interpreter
// and generated code call it, so their values are identical by construction.
func RowSource(seed uint64, entity string, id int) gen.Source {
	return gen.At(seed, hashStr(entity), uint64(id))
}

// RefIndex resolves a foreign key: a positional draw into the target's dense row
// space [0,targetCount). No pool is materialized, so it stays parallel and
// order-independent. Exposed so generated code shares the interpreter's logic.
func RefIndex(seed uint64, entity string, id int, ref string, targetCount int) int {
	if targetCount <= 0 {
		return 0
	}
	rs := gen.At(seed, hashStr(entity), uint64(id), hashStr(ref))
	return int(rs.Draw(uint64(targetCount)))
}

// Generate yields n records for a single entity. Used by --fixt and --mock.
func (p *Plan) Generate(entity string, seed uint64, n int) ([]*Record, error) {
	e := p.Entities[entity]
	if e == nil {
		return nil, fmt.Errorf("unknown entity %q", entity)
	}
	out := make([]*Record, n)
	seq := newSeqState(e)
	for i := 0; i < n; i++ {
		src := RowSource(seed, entity, i)
		rec := p.genRecord(e, src, i, n, seed, 0)
		seq.apply(e, rec)
		out[i] = rec
	}
	return out, nil
}

// seqState holds the per-parent counters for an entity's sequence fields during
// one generation pass. Sequence is the one field type that needs sequential
// state (a dense per-parent ordinal can't be derived per-row positionally), so
// it's a runner concern over a full pass — not valid for single-row (mock) gen.
type seqState struct {
	counters map[string]map[any]int // field -> within-value -> count
	active   bool
}

func newSeqState(e *Entity) *seqState {
	s := &seqState{counters: map[string]map[any]int{}}
	for _, f := range e.Fields {
		if f.Gen == "sequence" {
			s.active = true
			s.counters[f.Name] = map[any]int{}
		}
	}
	return s
}

func (s *seqState) apply(e *Entity, rec *Record) {
	if !s.active {
		return
	}
	for _, f := range e.Fields {
		if f.Gen != "sequence" {
			continue
		}
		var key any = "" // global if no `within`
		if w, _ := f.Params["within"].(string); w != "" {
			key = rec.Get(w)
		}
		s.counters[f.Name][key]++
		rec.Set(f.Name, s.counters[f.Name][key]) // dense 1..N within the parent
	}
}

// One yields a single record at index id. Used by the mock server.
func (p *Plan) One(entity string, seed uint64, id, count int) (*Record, error) {
	e := p.Entities[entity]
	if e == nil {
		return nil, fmt.Errorf("unknown entity %q", entity)
	}
	src := RowSource(seed, entity, id)
	return p.genRecord(e, src, id, count, seed, 0), nil
}

// Seed streams every entity's dataset to the sink. Used by --seed.
func (p *Plan) Seed(seed uint64, sink Sink) error {
	for _, name := range p.Order {
		e := p.Entities[name]
		n := p.Counts[name]
		if n == 0 {
			continue
		}
		if err := sink.Begin(e); err != nil {
			return err
		}
		seq := newSeqState(e)
		for i := 0; i < n; i++ {
			src := RowSource(seed, name, i)
			rec := p.genRecord(e, src, i, n, seed, 0)
			seq.apply(e, rec)
			if err := sink.Write(rec); err != nil {
				return err
			}
		}
		if err := sink.End(e); err != nil {
			return err
		}
	}
	return sink.Close()
}
