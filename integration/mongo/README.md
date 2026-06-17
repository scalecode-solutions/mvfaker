# mvfaker + MongoDB

The same forum dataset as the Postgres demo, but seeded into **MongoDB** — proof
that mvfaker is genuinely DB-agnostic. One config (`forum.hcl`, identical to the
Postgres one), a different sink.

```
mvfaker --seed --ndjson /out forum.hcl   →   /out/{users,posts,comments}.ndjson
                                              (one JSON document per line)
mongoimport --collection users --file /out/users.ndjson   →   MongoDB
```

## Run it

```bash
docker compose -f integration/mongo/docker-compose.yml up --build -d
docker compose -f integration/mongo/docker-compose.yml logs seed
docker compose -f integration/mongo/docker-compose.yml down -v   # tear down
```

The seed step generates NDJSON with mvfaker, `mongoimport`s each collection, then
verifies with `mongosh`.

## Relational → document

The config still uses `ref = "users.id"`. In a document store there are no foreign
keys, so:

- The integer `id` / `author_id` become plain **id references** between
  collections, joined at query time with `$lookup` (Mongo's join).
- Mongo doesn't *enforce* references, so the seed verifies integrity itself
  (count posts whose `author_id` matches no user — it's 0).
- A **unique index on `email`** plays the role Postgres's `UNIQUE` constraint
  played: building it would fail if mvfaker emitted a duplicate. It doesn't.

(If you wanted *embedded* documents instead — a post with its comments nested —
mvfaker's struct-tag front-end (`mvfaker.Fill` / `Struct[T]`) produces nested
JSON directly, which is more idiomatic Mongo than the flat relational shape.)

## Same config, three databases

| Database | Sink | Demo |
|---|---|---|
| Postgres | `--copy` (COPY) | [`integration/`](../) |
| SQLite / MySQL | `--sql` (INSERT) | seed a `.db` with `--sql \| sqlite3` |
| MongoDB | `--ndjson` → `mongoimport` | this directory |

One `Plan`, many sinks — mvfaker emits the data; the format meets the database.
