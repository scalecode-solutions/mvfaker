# mvfaker integration showcase — one config, four databases

mvfaker is a pure **data generator**: it never connects to your database, it emits
data in whatever format the database wants. These demos prove that with real,
running backends — the *same* `forum.hcl` (users → posts → comments, with foreign
keys and a unique email) seeded into four different databases, each verified by
that database's own rules.

| Database | Sink | What proves it | Backend app |
|---|---|---|---|
| **PostgreSQL** | `--copy` | FK + UNIQUE enforced during `COPY` load | Go API on `:8080` |
| **SQLite** | `--sql` | `.import`/INSERT round-trip, FKs ON | — |
| **MySQL / spreadsheets** | `--csv` | universal CSV (`LOAD DATA`, `.import`, Excel) | — |
| **MongoDB** | `--ndjson` | 0 orphans via `$lookup`; unique-email index | Go API on `:8081` |

The forum schema is relational, but mvfaker also produces **embedded** documents
(a post with its author and comments nested inline) for the document-store world —
so the MongoDB demo shows *both* modeling styles. One generator; the format meets
the database.

---

## 1. PostgreSQL  (`integration/`)

A real Go REST API (`pgx` + stdlib routing) over Postgres, seeded by mvfaker over
`COPY`. The seed step is belt-and-suspenders: it runs the `--check` verified stage
(config vs. schema), `pg_dump`s a backup, then loads.

```bash
docker compose -f integration/docker-compose.yml up --build -d
curl localhost:8080/stats                      # {users:5000, posts:20000, comments:80000}
curl localhost:8080/posts?limit=5              # author JOINed via author_id FK
curl localhost:8080/posts/681/comments         # comment → author, 3rd FK hop
docker compose -f integration/docker-compose.yml down -v
```

**Why it's a fair test:** Postgres enforces every foreign key *during* the `COPY`
load — a single dangling `author_id` aborts it. It doesn't. The `UNIQUE` email
constraint would reject a collision; 5000/5000 are distinct.

## 2. SQLite  (no server, just a file)

The `--sql` sink emits portable `INSERT`s that load into SQLite directly:

```bash
go build -o /tmp/mvfaker ./cmd/mvfaker
# SQLite needs named indexes; the table DDL itself is portable, so drop the
# Postgres-style unnamed CREATE INDEX lines:
grep -v '^CREATE INDEX' integration/db/schema.sql | sqlite3 forum.db
( echo "PRAGMA foreign_keys=ON; BEGIN;"; \
  /tmp/mvfaker --seed --sql integration/seed/forum.hcl; echo "COMMIT;" ) | sqlite3 forum.db
sqlite3 forum.db "SELECT count(*) FROM users;"    # 5000, FKs enforced, emails unique
```

105k rows in ~0.3s. `--check` validates the SQLite DDL too (it just parses
`CREATE TABLE`). *(Minor portability note: Postgres allows unnamed indexes,
SQLite requires names — the tables are otherwise identical.)*

## 3. MySQL / spreadsheets  (CSV)

The `--csv` sink writes one CSV per table — the universal bulk format that every
SQL database and spreadsheet reads:

```bash
mvfaker --seed --csv ./out integration/seed/forum.hcl   # ./out/{users,posts,comments}.csv
# MySQL:    LOAD DATA INFILE 'users.csv' INTO TABLE users FIELDS TERMINATED BY ',' IGNORE 1 LINES;
# Postgres: \copy users FROM 'users.csv' WITH (FORMAT csv, HEADER);
# SQLite:   .mode csv  →  .import --skip 1 users.csv users
```

## 4. MongoDB  (`integration/mongo/`)

The same config seeded into MongoDB via `--ndjson` → `mongoimport`, with a Go API
that serves **both** modeling styles. See [`mongo/README.md`](mongo/README.md).

```bash
docker compose -f integration/mongo/docker-compose.yml up --build -d
curl localhost:8081/posts?limit=5     # REFERENCED: $lookup joins post → author
curl localhost:8081/articles/0        # EMBEDDED: author + comments inline, one query
docker compose -f integration/mongo/docker-compose.yml down -v
```

Mongo doesn't enforce FKs, so the seed verifies integrity itself (0 orphan posts
via `$lookup`); a unique email index stands in for Postgres's `UNIQUE` constraint.

---

## Defining it in code instead of HCL

Every demo above is driven by `forum.hcl`. The exact same dataset can be built in
Go — see [`examples/codeseed`](../examples/codeseed), which produces **byte-identical**
output for the same seed (config and code are two faces of one engine). The
embedded-document variant lives in [`examples/embedded`](../examples/embedded).

## The shared idea

```
                       forum.hcl  (one Plan)
                            │
   ┌─────────────┬──────────┼───────────┬──────────────┐
 --copy        --sql      --csv      --ndjson       Fill()
   │             │          │           │              │
 Postgres   SQLite/MySQL   any SQL   MongoDB     embedded docs
```

mvfaker's value layer is pure (coherent fields, unique-without-a-set, valid FKs),
and the sinks are interchangeable faces on the dataset layer. Adding a database
means adding a *format*, not changing the generator.
