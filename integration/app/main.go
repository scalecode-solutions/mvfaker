// forumapi is a small but real REST backend: users, posts and comments over
// Postgres. It's deliberately a normal idiomatic Go service — the kind of thing
// you'd actually deploy — so that seeding it with mvfaker is a fair test.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

func main() {
	url := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	var err error
	for i := 0; i < 30; i++ {
		if db, err = pgxpool.New(ctx, url); err == nil {
			if err = db.Ping(ctx); err == nil {
				break
			}
		}
		log.Printf("waiting for db: %v", err)
		time.Sleep(time.Second)
	}
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /stats", handleStats)
	mux.HandleFunc("GET /users", handleUsers)
	mux.HandleFunc("GET /users/{id}", handleUser)
	mux.HandleFunc("GET /posts", handlePosts)
	mux.HandleFunc("GET /posts/{id}", handlePost)
	mux.HandleFunc("GET /posts/{id}/comments", handleComments)

	log.Println("forum api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

type Author struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type User struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Country string `json:"country"`
	City    string `json:"city"`
	Joined  string `json:"joined"`
}

type Post struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body,omitempty"`
	Created string `json:"created"`
	Author  Author `json:"author"`
}

type Comment struct {
	ID     int    `json:"id"`
	Body   string `json:"body"`
	Author Author `json:"author"`
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	out := map[string]int{}
	for _, t := range []string{"users", "posts", "comments"} {
		var n int
		if err := db.QueryRow(r.Context(), "SELECT count(*) FROM "+t).Scan(&n); err != nil {
			fail(w, err)
			return
		}
		out[t] = n
	}
	writeJSON(w, http.StatusOK, out)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(r.Context(),
		`SELECT id, name, email, country, city, joined::text FROM users ORDER BY id LIMIT $1`,
		limit(r, 20))
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Country, &u.City, &u.Joined); err != nil {
			fail(w, err)
			return
		}
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, users)
}

func handleUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var u User
	err := db.QueryRow(r.Context(),
		`SELECT id, name, email, country, city, joined::text FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Country, &u.City, &u.Joined)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func handlePosts(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(r.Context(),
		`SELECT p.id, p.title, p.created::text, u.id, u.name
		   FROM posts p JOIN users u ON u.id = p.author_id
		  ORDER BY p.created DESC, p.id LIMIT $1`, limit(r, 20))
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	posts := []Post{}
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Created, &p.Author.ID, &p.Author.Name); err != nil {
			fail(w, err)
			return
		}
		posts = append(posts, p)
	}
	writeJSON(w, http.StatusOK, posts)
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var p Post
	err := db.QueryRow(r.Context(),
		`SELECT p.id, p.title, p.body, p.created::text, u.id, u.name
		   FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id=$1`, id).
		Scan(&p.ID, &p.Title, &p.Body, &p.Created, &p.Author.ID, &p.Author.Name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func handleComments(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rows, err := db.Query(r.Context(),
		`SELECT c.id, c.body, u.id, u.name
		   FROM comments c JOIN users u ON u.id = c.author_id
		  WHERE c.post_id=$1 ORDER BY c.id LIMIT 50`, id)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	comments := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.Body, &c.Author.ID, &c.Author.Name); err != nil {
			fail(w, err)
			return
		}
		comments = append(comments, c)
	}
	writeJSON(w, http.StatusOK, comments)
}

// --- helpers ---------------------------------------------------------------

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
