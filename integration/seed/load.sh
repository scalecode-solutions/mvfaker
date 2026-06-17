#!/bin/sh
# Belt-and-suspenders seeding:
#   1. pre-flight — mvfaker validates the config against the real schema (--check)
#      and shows the plan (--dryrun). Bad column/type => non-zero exit, no load.
#   2. backup     — pg_dump the current DB before we touch it (the workflow owns
#      the database; mvfaker never connects to it).
#   3. load       — generate + stream COPY into Postgres.
set -e

echo ">> pre-flight: validating config against schema..."
mvfaker --seed --check --schema /schema.sql --dryrun /forum.hcl

echo ">> backup: pg_dump current database before loading..."
pg_dump "$DATABASE_URL" > /backup.sql
echo "   backup written to /backup.sql ($(wc -l < /backup.sql) lines)"

echo ">> generating + loading forum seed data..."
mvfaker --seed --copy /forum.hcl | psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f -

echo ">> seed complete:"
psql "$DATABASE_URL" -t -c \
  "SELECT 'users='||count(*) FROM users
   UNION ALL SELECT 'posts='||count(*) FROM posts
   UNION ALL SELECT 'comments='||count(*) FROM comments;"
