#!/bin/sh
# Generate the forum dataset with mvfaker and stream it straight into Postgres
# via COPY. No intermediate file — the COPY text format pipes right into psql.
set -e

echo ">> mvfaker: generating + loading forum seed data..."
mvfaker --seed --copy /forum.hcl | psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f -

echo ">> seed complete:"
psql "$DATABASE_URL" -t -c \
  "SELECT 'users='||count(*) FROM users
   UNION ALL SELECT 'posts='||count(*) FROM posts
   UNION ALL SELECT 'comments='||count(*) FROM comments;"
