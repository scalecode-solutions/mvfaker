package schema

import (
	"fmt"
	"strings"

	"github.com/tmarq/mvfaker/gen"
)

// refResolver returns a deterministic FK for row ix of the given entity. Because
// ids are dense 0..count-1, a foreign key is just a positional draw into the
// target's row space — no pool needs to be materialized, so seeding stays
// parallel and order-independent.
func (p *Plan) refResolver(seed uint64, entity string, ix int) func(ref string) any {
	return func(ref string) any {
		tname := ref
		if k := strings.IndexByte(ref, '.'); k >= 0 {
			tname = ref[:k]
		}
		tc := p.Counts[tname]
		if tc <= 0 {
			return 0
		}
		rs := gen.At(seed, hashStr(entity), uint64(ix), hashStr(ref))
		return int(rs.Draw(uint64(tc)))
	}
}

// Generate yields n records for a single entity. Used by --fixt and --mock.
func (p *Plan) Generate(entity string, seed uint64, n int) ([]*Record, error) {
	e := p.Entities[entity]
	if e == nil {
		return nil, fmt.Errorf("unknown entity %q", entity)
	}
	out := make([]*Record, n)
	for i := 0; i < n; i++ {
		src := gen.At(seed, hashStr(entity), uint64(i))
		out[i] = p.genRecord(e, src, i, n, seed, p.refResolver(seed, entity, i))
	}
	return out, nil
}

// One yields a single record at index id. Used by the mock server.
func (p *Plan) One(entity string, seed uint64, id, count int) (*Record, error) {
	e := p.Entities[entity]
	if e == nil {
		return nil, fmt.Errorf("unknown entity %q", entity)
	}
	src := gen.At(seed, hashStr(entity), uint64(id))
	return p.genRecord(e, src, id, count, seed, p.refResolver(seed, entity, id)), nil
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
		for i := 0; i < n; i++ {
			src := gen.At(seed, hashStr(name), uint64(i))
			rec := p.genRecord(e, src, i, n, seed, p.refResolver(seed, name, i))
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
