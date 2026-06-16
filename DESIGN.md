# mvfaker — Design

*Status: v0, pre-freeze. Internal, with eventual OSS in mind.*

## 1. Purpose

One tool for believable fake data, serving four jobs from one shared set of
definitions:

- `--fixt` — a few repeatable example records for tests.
- `--mock` — realistic, well-formed records for stand-in APIs/demos.
- `--seed` — huge, internally-consistent datasets for databases.
- `--prop` — property testing: invent edge cases, find breaks, shrink them.

You describe your data once; each mode is a different front door onto it.

### Non-goals

- Not an ORM; sinks are thin adapters.
- No giant baked-in locale bundles; locale tables are opt-in data.
- No seeded-output stability guarantee until explicitly frozen (`v0` may change).

## 2. Core principles

**The invariant (the rule everything depends on):**
> A generator is a **pure** function from entropy to a value, holding **no
> cross-value state**. All statefulness — uniqueness, FKs, pools, ordering —
> lives in a layer *above* generators, never inside them.

Violating this is what makes other fakers hacky (e.g. faker's `.unique()` puts
state in a generator and thereby breaks shrinking, replay, parallel seeding, and
codegen all at once).

**The recurring shape (why the design coheres):**
> Every layer is *one canonical thing in the middle, many interchangeable faces
> on the outside.* State never crosses a layer boundary — only the canonical
> object does.

| Layer | Canonical thing | Faces |
|---|---|---|
| Entropy | `Source` | positional · recording · (replay) |
| Value | `Generator[T]` | combinators · registry/HCL |
| Dataset | `Plan` | code · HCL |
| Tool | one recipe set | `--fixt` · `--mock` · `--seed` · `--prop` |

## 3. Architecture

```
┌─ Dataset layer (schema/) ───────────────────────────────────┐
│  Plan: entities, cardinality, refs(FK). Runner owns ALL      │  ← all state here
│  cross-row state and drives sinks (JSON/SQL).                │
├─ Value layer (gen/, data/) ─────────────────────────────────┤
│  Generator[T]: PURE entropy → T. Map/Bind/OneOf/Weighted…    │  ← the core
│  Coherence within a record = Bind. Registry names builders.  │
├─ Entropy layer (gen/source.go) ─────────────────────────────┤
│  Source: the only randomness. Draw + Split (splittable).     │  ← one primitive
└──────────────────────────────────────────────────────────────┘
```

## 4. Entropy — `Source`

`Draw(n) uint64` + `Split() Source`. Splittable, not merely drawable: every
structural step takes a fresh child. **Invariant:** the all-zero draw decodes to
each generator's simplest value, giving the shrinker a direction.

- **Positional** (`At(seed, path…)`): value = pure function of (seed, path) via
  splitmix64. Reproducible, order-independent, and trivially parallel — row
  #1,000,000 is generable without touching rows 0..999,999. Default for
  fixt/mock/seed.
- **Recording**: linear; records each draw so the choice-sequence *is* the seed.
  The shrinker minimizes that sequence and replays. Used by `--prop`.
  *(Rough edge: shrinking is over a flat sequence; structured/tree shrinking is
  future. The deletion pass recovers most structural minimization in practice —
  lists collapse to a single element.)*

## 5. Value — `Generator[T]`

Pure `Generate(Source) T`. Combinators: `Map`, `Bind` (the coherence spine),
`OneOf`, `Weighted`, `Slice`, `Optional`; primitives `IntRange`, `Float64Range`,
`NormalInt`, `Bool`, `Pick`. Two front-ends emit into this one layer:

1. **Combinators** — direct Go, full power.
2. **Registry + HCL** — named builders (`data.Register`) that config references.

Coherence within a record is `Bind`: the email builder receives the already-
generated name and derives its local-part from it.

## 6. Dataset — `Plan` + runner

Where all stateful concerns live. Entities have ordered fields; a field is either
a registered generator (optionally `from` another field) or a `ref` FK. Because
ids are dense `0..count-1`, an FK is just a positional draw into the target's row
space — no pool is materialized, so seeding stays parallel. Sinks: `JSONSink`
(accumulates `{entity: [...]}`), `SQLSink` (streams `INSERT`s, suited to scale).

## 7. Config — HCL (restricted subset)

Chosen because the domain is a reference graph and HCL is built for declarative
named blocks with references. Surface restricted to blocks, attributes,
references and (future) registered-function calls; no conditionals/loops. Field
bodies decode as free-form attributes so any builder's params flow through
without the parser knowing them.

```hcl
entity "customer" {
  field "name"  { gen = "name.full" }
  field "email" {
    gen  = "internet.email"
    from = "name"            # coherence
  }
}
entity "order" {
  field "customer_id" { ref = "customer.id" }   # FK
}
dataset "demo" {
  counts = { customer = 1000, order = 5000 }
}
```

*(Rough edge: HCL forbids comma-separated attrs on one line — field bodies are
multi-line.)*

## 8. The registry — the seam between config and code

Config stays declarative; anything custom is written in code, named via
`data.Register`, and referenced by name. `--prop`'s rule rides the same seam
(future: registered rules), so all four modes share one recipe set.

## 9. Open questions / deferred

- Tree-structured (not flat) shrinking for the recording source.
- Uniqueness as a dataset-layer strategy; weighted/Zipf realism for names.
- Counter-based RNG choice; codegen path for extreme-scale seeding.
- More sinks (CSV, Postgres `COPY`), `--mock --serve` HTTP mode.
- `--prop` from user code (registered rules) vs. the built-in demo.

## 10. Build order (done)

1. Entropy + Value core ✓  2. Recording source + shrinker ✓
3. Registry + built-ins ✓  4. Dataset + sinks ✓  5. HCL front-end ✓
6. CLI ✓ — *next:* reflection/struct-tag front-end, more generators.
