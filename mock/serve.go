// Package mock serves coherent fake records over HTTP — the --mock --serve
// stand-in API. Routes:
//
//	GET /                 → list of entities
//	GET /<entity>?n=N     → N records (default defN)
//	GET /<entity>/<id>    → a single record at that index
//
// Output is deterministic from the seed, so a given URL always returns the same
// record — handy for front-ends developing against a stable fixture.
package mock

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/scalecode-solutions/mvfaker/schema"
)

// Handler builds the mock HTTP handler for a plan.
func Handler(p *schema.Plan, seed uint64, defN int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(r.URL.Path, "/")
		if path == "" {
			writeJSON(w, http.StatusOK, map[string]any{"entities": p.Order})
			return
		}
		parts := strings.SplitN(path, "/", 2)
		entity := parts[0]
		if _, ok := p.Entities[entity]; !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown entity: " + entity})
			return
		}
		// /<entity>/<id>
		if len(parts) == 2 && parts[1] != "" {
			id, err := strconv.Atoi(parts[1])
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id must be an integer"})
				return
			}
			rec, err := p.One(entity, seed, id, id+1)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, rec)
			return
		}
		// /<entity>?n=N
		n := defN
		if q := r.URL.Query().Get("n"); q != "" {
			if v, err := strconv.Atoi(q); err == nil && v >= 0 {
				n = v
			}
		}
		recs, err := p.Generate(entity, seed, n)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, recs)
	})
	return mux
}

// Serve runs the mock server until the process exits.
func Serve(p *schema.Plan, addr string, seed uint64, defN int) error {
	return http.ListenAndServe(addr, Handler(p, seed, defN))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
