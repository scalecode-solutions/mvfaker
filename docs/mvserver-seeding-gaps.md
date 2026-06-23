# mvFaker → mvServer seeding: gap analysis

What mvFaker is missing to seed an mvServer local-docker database **exactly as we
need it** — i.e. from-blank, pure-config, reproducible, populating realistic
users (and `auth` rows) spread across the account-lifecycle bands so the
lifecycle worker and all the visibility/presence/login paths can be exercised.

**Use case:** `docker compose down -v && up` brings up a fresh v96 schema; mvFaker
then repopulates `users` (+ `auth`) with N records and controllable condition
variants — instead of a hand-seeded blob or ad-hoc fleet.

This was produced from a read-only walk of the mvFaker source (`data/`,
`schema/`, `cmd/mvfaker/`). Effort sizes are rough.

> **Status (updated):** the "minimum to seed lifecycle-testable users from pure
> HCL" cut is **shipped** — #1 `oneof`, #2 `timestamp`, #3 `json`, #4
> `password.bcrypt`, #5 `transform` (via `copy` + the `transform` modifier), #6
> `maxlen`, #9 `null_prob`. Field modifiers (`transform`/`maxlen`/`null_prob`)
> work on any field. **Remaining:** #7 cross-entity projection, #8 conditional
> coherence, #10–#12 `--check` gaps, #13 plugin seam, #14 direct PG load.

---

## A. Value generators to add (the bulk of it)

| # | Missing | Why we need it | Effort |
|---|---|---|---|
| 1 | **Weighted / `oneof` enum generator** (HCL-exposed `values=[...]` + optional `weights=[...]`) | `state` distribution (active/inactive/deactivated/deleted); also `platform`, roles. `gen.Weighted`/`OneOf` exist in Go but are **not registered as HCL builders**. | Small |
| 2 | **Relative / now-anchored timestamp** with **day/duration granularity** (e.g. `now - [0,200] days`) | `last_seen` band placement is the whole lifecycle test. `datetime` is **year-only** (min/max are years, day 1–28). Needs a fixed reference-`now` param so it stays deterministic. | Small–Med |
| 3 | **`json` / `jsonb` value generator** | `public jsonb` column. Confirmed **none** today (only a JSON *sink*, no value gen). | Small |
| 4 | **Password / bcrypt generator** (precomputed hash of a known plaintext) | `auth.secret`, so seeded users can actually log in. No crypto/hash generator exists. Must precompute (bcrypt is slow and intentionally non-deterministic). | Small |
| 5 | **String transform generator** (`lower`/`upper`/`slug`, `from=field`) | `handle_lower = lower(handle)` backs the case-insensitive unique index. `from` passes a value but there is **no transform**. | Small |
| 6 | **Length-aware string generation / truncation** | `handle varchar(30)`, `email varchar(255)`, etc. No `maxlen` anywhere — an over-length value → COPY error. | Small–Med |

## B. Coherence / structural (dataset layer — the harder ones)

| # | Missing | Why | Effort |
|---|---|---|---|
| 7 | **Cross-entity field projection** (`from = "user_id.email"`) | `auth.uname` must equal the referenced user's `email`. Today `from` is **within-entity only**; an FK is a positional id draw with no materialized row. *Feasible* given the pure positional design (re-derive the target field at the drawn id) — but it's new plumbing. | Medium |
| 8 | **Conditional coherence** (set field B *iff* field A has value X) | `deactivated_at` set **iff** `state=deactivated`; `delete_requested_at` iff delete-pending; `last_seen` band must match `state`. HCL deliberately has **no conditionals** — needs either domain generators or a `when`/`case` construct. | Medium |
| 9 | **HCL-exposed nullable / `optional` field** (emit NULL with a probability) | The lifecycle anchors are NULL for most rows. COPY *handles* `\N`, but HCL can't currently *produce* nil per-field (the `Optional` combinator isn't surfaced in HCL). | Small–Med |

## C. `--check` validator gaps (the drift guard we want to rely on)

| # | Missing | Why | Effort |
|---|---|---|---|
| 10 | **NOT NULL validation** | A non-nullable column with no field *and* no DB default → COPY fails at load. `--check` should catch it before the load, not psql. | Medium |
| 11 | **`varchar` length validation** | Pairs with #6 — catch `varchar(30)` overflow at check time. | Small |
| 12 | **Run `--check` against the migrated (v96) schema** | On the layered model the lifecycle columns live in **migration 092**, not `schema.sql` (v90 baseline). So `--check --schema store/schema.sql` won't see `deactivated_at` etc. Needs either a "dump v96 first" doc step or a tool flag to apply migrations before checking. | Small (doc) / Med (tool) |

## D. Plumbing / ergonomics

| # | Missing | Why | Effort |
|---|---|---|---|
| 13 | **External generator registration visible to the stock `mvfaker` CLI** | `data.Register` runs in mvFaker's own `init()`s. To add `mvserver.*` generators usable by the **prebuilt CLI**, you must fork mvFaker (add an mvserver gen-pack) or use the **library path** (a small Go `main` importing mvfaker). No plugin seam today. | Med–Large (or: accept the library path) |
| 14 | **Direct Postgres load** (`--seed --pg <dsn>`, in a txn) | Today it's `--copy` → stdout → pipe to `psql`. A direct loader is a convenience, not a blocker. | Small |

---

## What is NOT missing (already works)

So the picture is complete and we don't reinvent these:

- `-s` deterministic seed value ✓
- `-n` record-count override ✓
- `-o` output file ✓
- `-dryrun` ✓
- COPY emits `\N` for NULL ✓
- **partial-column COPY** — `COPY users (only-the-cols-you-define) FROM stdin`, so omitted NOT-NULL-with-default columns just take their DB defaults (the feature that makes seeding a wide table practical) ✓
- dataset-layer `unique = true` (Feistel/index-derived, no mutable set) ✓
- `from` within-entity coherence (e.g. email derived from name) ✓
- `--check` type-clash detection (`typeClash` / `genCategory` / `sqlCategory`) ✓

---

## Suggested cut lines

- **Minimum to seed lifecycle-testable users from pure HCL:** #1, #2, #3, #5, #6, #9
  (the value generators + nullable). Yields realistic users spread across
  `last_seen` bands with a coherent `state`/anchor distribution — enough to
  exercise the worker and the visibility/presence paths.
- **Add for login-capable users:** #4, #7 (bcrypt + cross-entity `uname=email`).
  These let seeded accounts actually authenticate.
- **Add to trust the rig long-term:** #10–#12 (`--check` actually guaranteeing a
  clean load).
- **Only two are architecturally meaningful:** #8 (conditional coherence) and #13
  (CLI-visible custom generators). Everything else is a small, self-contained
  generator.

## The hybrid alternative (no mvFaker changes)

If we don't want to touch mvFaker yet, items #1 / #2 / #5 / #8 / #9 collapse into
"do it in a short SQL post-pass, and let the worker band the users":

```bash
docker compose down -v && docker compose up -d --build          # fresh v90 → 091..096 → v96
mvfaker --seed --copy mvserver.hcl | docker exec -i mvserver-db psql -U mvserver -d mvserver
docker exec -i mvserver-db psql -U mvserver -d mvserver <<'SQL'
  UPDATE users SET handle_lower = lower(handle);                              -- #5
  UPDATE users SET last_seen = now() - (random()*200 || ' days')::interval;  -- #2: spread all bands
  UPDATE users SET deactivated_at = now() - interval '95 days'               -- a manual-deactivation cohort
    WHERE random() < 0.1;
SQL
# then run the lifecycle worker and watch it transition the bands
```

mvFaker does what it's best at (realistic, unique, coherent identity at scale);
a few lines of SQL carve the lifecycle conditions; the worker does the real work.
The gaps above are specifically the price of going **pure mvFaker** instead.
