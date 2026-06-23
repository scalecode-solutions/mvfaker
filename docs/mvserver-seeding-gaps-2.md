# mvFaker → mvServer seeding: gap analysis, round 2

Round 1 (`mvserver-seeding-gaps.md`) listed the value/coherence gaps from a
read of the source. This round comes from **actually drafting the recipe**
(`seed/mvserver-v90.hcl` in the mvServer repo) against the real v90 schema — so
these are the gaps that block a *full* six-table pure-HCL seed, found by writing
the thing.

> **Status (updated):** all three **shipped** — #15 `id_type = "none"` (no id
> column), #16 unique ref (`ref ... unique = true` → distinct 1:1), #17
> `distinct_pair = [a, b]` (composite uniqueness + automatic no-self-edge when
> both refs share a target). The full six-table graph (users → auth → contacts →
> conversations → members → messages) now seeds from one pure-HCL recipe. These
> are interpreter (`--seed`) features; codegen (`--gen`) refuses them for now to
> keep its byte-equivalence guarantee.

**Round 1 status:** #1–#9 shipped (generators + coherence), plus UUID primary
keys (`id_type = "uuid"`) and codegen parity for UUID — none of which were even
on the list. Remaining from round 1: #10–#12 (`--check` depth), #13 (plugin
seam), #14 (direct PG load).

---

## Where the recipe stands

mvServer's seed graph is six tables: `users → auth → contacts → conversations →
members → messages`.

| Table | uuid `id` PK? | Status |
|---|---|---|
| `users` | yes | ✅ runs now |
| `auth` | yes | ⚠ runs, but `UNIQUE(scheme, uname)` risks dup → needs #16 |
| `conversations` | yes | ✅ runs now |
| `messages` | yes | ✅ runs now (content VERIFY below) |
| `members` | **no** (PK `conversation_id, user_id`) | ⛔ blocked — needs #15 + #17 |
| `contacts` | **no** (PK `user_id, contact_id`) | ⛔ blocked — needs #15 + #17 |

So **4 of 6 run today** (a login-capable fleet with conversations + messages).
The membership/social graph (`members`, `contacts`) is blocked on the three new
gaps below.

---

## New gaps

| # | Missing | Why we need it | Effort |
|---|---|---|---|
| 15 | **`id` suppression** (e.g. `id_type = "none"`) — emit no `id` column for an entity | `members` and `contacts` have **no `id` column** (composite PKs). But mvFaker always `Set("id", …)` in `genRecord` and the sink emits every key, so these emit a spurious `id` → COPY fails (`column "id" does not exist`). Blocks 2 of 6 tables — the entire membership/social graph. | Small–Med |
| 16 | **Unique / identity `ref`** (`ref = "users.id" unique = true`, or an identity mode where row *i* → target row *i*) | `auth` has `UNIQUE(scheme, uname)`, and `uname = user.email` is unique per user — so 1:1 auth is only safe if each auth row references a **distinct** user. mvFaker's `ref` is a random draw and `unique` is **skipped for ref fields** (`genRecord` `continue`s before modifiers), so collisions → duplicate `uname` → COPY fails. Also wanted for distinct edges. | Medium |
| 17 | **Composite uniqueness / distinct-pair** (+ self-edge avoidance) | `members` PK `(conversation_id, user_id)` and `contacts` PK `(user_id, contact_id)` — random ref pairs can collide → PK violation. `unique` is per-field only (the design's deferred "unique across a composite" item). `contacts` also needs `user_id != contact_id` (no self-edges). Pairs with #15 to unblock both tables. | Medium |

## VERIFY (not a gap, confirm on first load)

- **`messages.content` is `bytea`, not `jsonb`.** The recipe writes JSON text
  into it (the EAR read path falls back to raw content on decrypt failure, so
  plaintext JSON renders). Postgres should store backslash-free JSON as its
  literal bytes via COPY, but confirm the load accepts it. If it balks: `null`
  the field, or hex-encode (`\x…`). `messages.head` is `jsonb` and takes JSON
  directly — no concern there.

---

## Cut line

- **Unblocks the full six-table graph:** #15 + #17 (id suppression + composite
  uniqueness). Without them, `members`/`contacts` can't seed at all.
- **Makes `auth` clean (no re-runs):** #16 (unique/identity ref). Without it,
  auth seeds but may hit a duplicate-`uname` COPY failure on collision.
- **Already usable without any of these:** the 4-table subset (users + auth +
  conversations + messages) — a loginable fleet (`test1234`) you can develop
  against; just keep `auth` count modest until #16 lands.

These three (#15–#17) are what stand between "4 tables seed" and "the whole
mvServer graph seeds from one pure-HCL recipe."
