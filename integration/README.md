# mvfaker integration: a real Go backend + Postgres, seeded by mvfaker

A self-contained demo that takes mvfaker for a real spin. Three containers:

- **db** — Postgres 16 with a real forum schema (`users` → `posts` → `comments`,
  foreign keys, a `UNIQUE` email constraint).
- **seed** — builds the mvfaker binary, generates the dataset, and streams it into
  Postgres via `COPY` (`mvfaker --seed --copy forum.hcl | psql`).
- **api** — a normal idiomatic Go REST service (`pgx` + stdlib 1.22 routing) that
  serves the seeded data.

The point: the database's own constraints are the test oracle. If mvfaker produced
a dangling foreign key or a duplicate email, the `COPY` load would **fail**. It
doesn't.

## Run it

```bash
docker compose -f integration/docker-compose.yml up --build -d
curl localhost:8080/stats
```

```
{ "users": 500, "posts": 2000, "comments": 8000 }
```

### Try the API

```bash
curl localhost:8080/users?limit=5          # coherent: city matches country, unique emails
curl localhost:8080/users/0
curl localhost:8080/posts?limit=5          # author JOINed via author_id FK
curl localhost:8080/posts/681
curl localhost:8080/posts/681/comments     # comment → author, a 3rd FK hop
```

### What it proves

```bash
# Zero orphans — Postgres enforced every FK mvfaker generated:
docker compose -f integration/docker-compose.yml exec db psql -U forum -d forum -c \
  "SELECT count(*) FROM posts p LEFT JOIN users u ON u.id=p.author_id WHERE u.id IS NULL;"

# 500/500 distinct emails — the UNIQUE constraint would reject a collision:
docker compose -f integration/docker-compose.yml exec db psql -U forum -d forum -c \
  "SELECT count(*), count(DISTINCT email) FROM users;"
```

### Tear down

```bash
docker compose -f integration/docker-compose.yml down -v
```

## How it fits together

`db/schema.sql` defines the tables; `seed/forum.hcl` is the mvfaker config whose
field order matches each table's columns, so the generated `COPY` stream loads
directly. Change the `counts` in `forum.hcl` and re-run (`down -v` first) to seed
more or less. To seed *millions* of rows, swap the seed step for the codegen path
(`mvfaker --gen`) and compile the generated `SeedAll` into the seeder.

```
integration/
  docker-compose.yml
  db/schema.sql           # forum schema (FKs, UNIQUE email)
  seed/forum.hcl          # mvfaker config matching the schema
  seed/Dockerfile         # builds mvfaker, bakes it into a psql image
  seed/load.sh            # mvfaker --seed --copy | psql
  app/                    # the Go REST backend (pgx + net/http)
```
