# mvFaker → mvServer seeding: gap analysis, round 3 — relational coherence

Rounds 1–2 got **field-level generation** (#1–#9) and **structure** (UUID PKs,
composite PKs, unique/distinct refs, #15–#17) right, plus the alnum unique
separator (handles now pass `^[a-z0-9]+$`). The full six-table recipe loads
**10,000 users / ~158k rows in ~2s, zero constraint violations.**

> **Status (updated):** both **shipped**. #19 — a `ref` to a composite-PK (id-less)
> entity is a projection source only (not emitted), so a message can `copy` both
> `conversation_id` and `from_user_id` from one `members` row → the sender **is**
> a member of the conversation, by construction (verified: 0 / N non-member
> senders). #20 — `gen = "sequence"` with `within = "<field>"` gives a dense 1..N
> per-parent ordinal (runner-level counter), satisfying `UNIQUE(conversation_id,
> seq)`. Interpreter (`--seed`) features; codegen refuses them. *(Note: `order_by`
> isn't honored yet — seq is dense per parent in generation order; for strict
> chronological seq, order `created_at` by seq, or use the SQL renumber.)*

Then a 10k load + an independent review found the data is **structurally valid
but relationally incoherent** in two ways. Rounds 1–2 made each row well-formed
and each FK point at a real row; round 3 is about the rows being *consistent with
each other across the graph*. Both are real — anything that tests member-gated
or ordering behavior (delivery, read receipts, member queries, message ordering)
behaves wrong on the current data.

**Still pending from earlier rounds:** #10–#12 (`--check` NOT-NULL / varchar-len
/ migrated-schema), #13 (plugin seam), #14 (direct PG load).

---

## The two incoherences (from the 10k load)

1. **Message senders aren't members of their conversation.** `messages.from_user_id`
   is a random `ref = "users.id"` — a draw across *all* users — so **79,946 /
   80,000** messages are "sent by" someone who isn't a member of the conversation
   they're in. Member-gated logic (delivery, read receipts, roster queries) is
   wrong on this.

2. **`seq` is global, not per-conversation.** To satisfy `UNIQUE(conversation_id,
   seq)` without per-parent logic, `seq` was made globally unique (0…79,999). The
   real app treats `seq` as a **dense 1..N run within each conversation** that
   drives ordering + `read_seq`/`recv_seq`/`clear_seq`. A single conversation's
   messages get scattered seqs (47, 1203, 8891) instead of 1,2,3.

---

## What I need

| # | Missing | Why we need it | Effort |
|---|---|---|---|
| 19 | **`ref` + project an id-less (composite-PK) entity** — `ref = "members"` to draw a membership row, then `from = "member.conversation_id"` / `from = "member.user_id"` to project its keys (including projecting a field that is *itself* a ref → returns the resolved FK). | Fixes incoherence #1 in the **recipe**: a message references a `members` row and projects BOTH `conversation_id` and `from_user_id` from it → the sender **is** a member of the conversation, by construction. Cross-entity projection (#7) already exists; the open question is ref-to-id-less + projecting a ref-field. May already partly work — worth a test. | Small–Med |
| 20 | **Per-parent sequence** — an ordinal field scoped to a ref group, dense and ordered: `seq` = 1..N **within** `conversation_id` (optionally ordered by a sibling like `created_at`). | Fixes incoherence #2: dense per-conversation `seq` that satisfies `UNIQUE(conversation_id, seq)` *and* matches how the app walks ordering / read state. The positional model has no per-parent sequence today. The clean home for it is the faker; the alternative is a careful **two-step SQL** post-pass (offset into a clear range, then renumber — needed because `UNIQUE(conv,seq)` is non-deferrable, so a naive scattered→dense remap hits a transient duplicate mid-statement). | Medium |

### Sketch of the fixed `messages` entity (with #19 + #20)

```hcl
entity "messages" {
  id_type = "uuid"

  # one membership → sender is a member of the conversation, by construction (#19)
  field "member"          { ref = "members" }
  field "conversation_id" { gen = "copy" from = "member.conversation_id" }
  field "from_user_id"    { gen = "copy" from = "member.user_id" }

  # dense 1..N within each conversation, ordered (#20)
  field "seq" { gen = "sequence" within = "conversation_id" order_by = "created_at" }

  field "content" { gen = "json" }
  field "head"    { gen = "json" }
}
```
*(`member` is a projection source only — like the auth `user_id` ref pattern — it
must not be emitted as a column, since `messages` has no such column.)*

---

## Cut line

- **#19 unblocks member-coherent senders** — and it's mostly recipe work riding
  on existing projection; possibly just needs ref-to-id-less + ref-field
  projection to be wired.
- **#20 unblocks realistic ordering / read-state** — the one genuinely new
  capability (per-parent sequence). If it's not worth building, the fallback is
  a two-step SQL renumber appended to the seed flow.
- Together they turn "the schema loads" into "the app behaves correctly against
  the seed" — the difference between load-testing and behavior-testing.

These two are the last things between the recipe and a fully app-faithful local
fleet. Everything structural (rounds 1–2) is done.
