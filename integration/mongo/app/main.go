// forummongo is the MongoDB-backed forum API — the document-store sibling of the
// Postgres app. It serves the same mvfaker-seeded data two ways: the referenced
// collections (posts joined to users with $lookup) and the embedded collection
// (articles, where author + comments live inline — one query, no join).
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var db *mongo.Database

func main() {
	uri := os.Getenv("MONGO_URI")
	ctx := context.Background()

	var client *mongo.Client
	var err error
	for i := 0; i < 30; i++ {
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err == nil {
			if err = client.Ping(ctx, nil); err == nil {
				break
			}
		}
		log.Printf("waiting for mongo: %v", err)
		time.Sleep(time.Second)
	}
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	db = client.Database("forum")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /stats", handleStats)
	mux.HandleFunc("GET /users", handleUsers)
	mux.HandleFunc("GET /users/{id}", handleUser)
	mux.HandleFunc("GET /posts", handlePosts) // referenced: $lookup author
	mux.HandleFunc("GET /posts/{id}/comments", handlePostComments)
	mux.HandleFunc("GET /articles", handleArticles) // embedded: one query
	mux.HandleFunc("GET /articles/{id}", handleArticle)

	log.Println("forum (mongo) api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	out := map[string]int64{}
	for _, c := range []string{"users", "posts", "comments", "articles"} {
		n, err := db.Collection(c).CountDocuments(r.Context(), bson.M{})
		if err != nil {
			fail(w, err)
			return
		}
		out[c] = n
	}
	writeJSON(w, http.StatusOK, out)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "id", Value: 1}}).
		SetLimit(int64(limit(r, 20))).SetProjection(bson.M{"_id": 0})
	docs, err := findAll(r.Context(), "users", bson.M{}, opts)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func handleUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var doc bson.M
	err := db.Collection("users").FindOne(r.Context(), bson.M{"id": id},
		options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&doc)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// handlePosts is the REFERENCED read: join posts → users with $lookup.
func handlePosts(w http.ResponseWriter, r *http.Request) {
	pipeline := mongo.Pipeline{
		{{Key: "$sort", Value: bson.D{{Key: "created", Value: -1}, {Key: "id", Value: 1}}}},
		{{Key: "$limit", Value: int64(limit(r, 20))}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "users"}, {Key: "localField", Value: "author_id"},
			{Key: "foreignField", Value: "id"}, {Key: "as", Value: "author"}}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0}, {Key: "id", Value: 1}, {Key: "title", Value: 1}, {Key: "created", Value: 1},
			{Key: "author", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{"$author.name", 0}}}}}}},
	}
	docs, err := aggregate(r.Context(), "posts", pipeline)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func handlePostComments(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"post_id": id}}},
		{{Key: "$limit", Value: int64(50)}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "users"}, {Key: "localField", Value: "author_id"},
			{Key: "foreignField", Value: "id"}, {Key: "as", Value: "author"}}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0}, {Key: "id", Value: 1}, {Key: "body", Value: 1},
			{Key: "author", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{"$author.name", 0}}}}}}},
	}
	docs, err := aggregate(r.Context(), "comments", pipeline)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

// handleArticles is the EMBEDDED read: each doc already has author + comments.
func handleArticles(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "id", Value: 1}}).
		SetLimit(int64(limit(r, 10))).SetProjection(bson.M{"_id": 0})
	docs, err := findAll(r.Context(), "articles", bson.M{}, opts)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func handleArticle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var doc bson.M
	err := db.Collection("articles").FindOne(r.Context(), bson.M{"id": id},
		options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&doc)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// --- helpers ---------------------------------------------------------------

func findAll(ctx context.Context, coll string, filter any, opts *options.FindOptions) ([]bson.M, error) {
	cur, err := db.Collection(coll).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []bson.M{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func aggregate(ctx context.Context, coll string, pipeline mongo.Pipeline) ([]bson.M, error) {
	cur, err := db.Collection(coll).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	out := []bson.M{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func fail(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id must be an integer"})
		return 0, false
	}
	return id, true
}

func limit(r *http.Request, def int) int {
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 100 {
			return n
		}
	}
	return def
}
