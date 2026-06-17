#!/bin/sh
# Seed MongoDB from the SAME forum config — mvfaker emits NDJSON, mongoimport
# loads each collection. Relational `ref`s become id references, joined with
# $lookup. Mongo doesn't enforce FKs, so we verify integrity ourselves; the
# unique email index is the equivalent of Postgres's UNIQUE constraint.
set -e

echo ">> mvfaker: generating NDJSON collections..."
mvfaker --seed --ndjson /out /forum.hcl

echo ">> mongoimport: loading each collection..."
for c in users posts comments; do
  mongoimport --uri "$MONGO_URI" --collection "$c" --file "/out/$c.ndjson" --drop --numInsertionWorkers 4
done

echo ">> indexes (unique email index would REJECT the import if any email collided)..."
mongosh "$MONGO_URI" --quiet --eval '
  db.users.createIndex({ id: 1 }, { unique: true });
  db.users.createIndex({ email: 1 }, { unique: true });
  db.posts.createIndex({ author_id: 1 });
'

echo ">> counts:"
mongosh "$MONGO_URI" --quiet --eval '
  ["users","posts","comments"].forEach(c => print("   " + c + "=" + db[c].countDocuments()));
'

echo ">> FK integrity (posts whose author_id matches no user — must be 0):"
mongosh "$MONGO_URI" --quiet --eval '
  const orphans = db.posts.aggregate([
    { $lookup: { from: "users", localField: "author_id", foreignField: "id", as: "a" } },
    { $match: { a: { $size: 0 } } },
    { $count: "n" }
  ]).toArray();
  print("   orphan_posts=" + (orphans.length ? orphans[0].n : 0));
'

echo ">> a \$lookup join (Mongo resolving the post -> author reference):"
mongosh "$MONGO_URI" --quiet --eval '
  db.posts.aggregate([
    { $lookup: { from: "users", localField: "author_id", foreignField: "id", as: "author" } },
    { $limit: 3 },
    { $project: { _id: 0, id: 1, title: 1, author: { $arrayElemAt: ["$author.name", 0] }, country: { $arrayElemAt: ["$author.country", 0] } } }
  ]).forEach(d => printjson(d));
'

echo ">> seed complete."
