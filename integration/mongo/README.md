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

## Both modeling styles (because real Mongo apps use both)

Mongo schemas mix two strategies, and mvfaker produces both:

| Style | When | mvfaker path | This demo |
|---|---|---|---|
| **Referenced** | shared / unbounded data (the author, joined with `$lookup`) | relational `Plan` + `--ndjson` | `users` / `posts` / `comments` |
| **Embedded** | read-together / bounded data (a post's comments, inline) | struct-tag `Fill` (nested struct = embedded doc) | `articles` |

The embedded variant lives in [`examples/embedded`](../../examples/embedded):

```bash
go run ./examples/embedded 1000 > articles.ndjson   # author + comments nested inline
mongoimport --collection articles --file articles.ndjson
```

The payoff is the read. Embedded — one query, no join:

```js
db.articles.findOne({ id: 0 })   // → post + author + comments, all in one document
```

Referenced — `$lookup` across collections for the same shape:

```js
db.posts.aggregate([
  { $match: { id: 0 } },
  { $lookup: { from: "users",    localField: "author_id", foreignField: "id",      as: "author" } },
  { $lookup: { from: "comments", localField: "id",        foreignField: "post_id", as: "comments" } },
])
```

Same generator, both shapes — embed what you read together, reference what you share.

## Same config, three databases

| Database | Sink | Demo |
|---|---|---|
| Postgres | `--copy` (COPY) | [`integration/`](../) |
| SQLite / MySQL | `--sql` (INSERT) | seed a `.db` with `--sql \| sqlite3` |
| MongoDB | `--ndjson` → `mongoimport` | this directory |

One `Plan`, many sinks — mvfaker emits the data; the format meets the database.
