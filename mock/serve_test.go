package mock_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tmarq/mvfaker/mock"
	"github.com/tmarq/mvfaker/schema"
)

func testPlan(t *testing.T) *schema.Plan {
	p := &schema.Plan{
		Entities: map[string]*schema.Entity{
			"customer": {Name: "customer", Fields: []*schema.Field{
				{Name: "name", Gen: "name.full"},
				{Name: "email", Gen: "internet.email", From: "name"},
			}},
		},
		Order:  []string{"customer"},
		Counts: map[string]int{"customer": 100},
	}
	if err := p.Resolve(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestServeList(t *testing.T) {
	srv := httptest.NewServer(mock.Handler(testPlan(t), 1, 5))
	defer srv.Close()

	var body struct {
		Entities []string `json:"entities"`
	}
	get(t, srv.URL+"/", http.StatusOK, &body)
	if len(body.Entities) != 1 || body.Entities[0] != "customer" {
		t.Fatalf("bad entity list: %+v", body.Entities)
	}
}

func TestServeCollection(t *testing.T) {
	srv := httptest.NewServer(mock.Handler(testPlan(t), 1, 5))
	defer srv.Close()

	var recs []map[string]any
	get(t, srv.URL+"/customer?n=3", http.StatusOK, &recs)
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if recs[0]["email"] == "" {
		t.Fatal("record missing email")
	}
}

func TestServeSingleDeterministic(t *testing.T) {
	srv := httptest.NewServer(mock.Handler(testPlan(t), 1, 5))
	defer srv.Close()

	var a, b map[string]any
	get(t, srv.URL+"/customer/7", http.StatusOK, &a)
	get(t, srv.URL+"/customer/7", http.StatusOK, &b)
	if a["name"] != b["name"] || a["id"].(float64) != 7 {
		t.Fatalf("single record not stable/at id 7: %+v vs %+v", a, b)
	}
}

func TestServeErrors(t *testing.T) {
	srv := httptest.NewServer(mock.Handler(testPlan(t), 1, 5))
	defer srv.Close()

	get(t, srv.URL+"/nope", http.StatusNotFound, nil)
	get(t, srv.URL+"/customer/abc", http.StatusBadRequest, nil)
}

func get(t *testing.T, url string, wantStatus int, into any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s: status %d, want %d", url, resp.StatusCode, wantStatus)
	}
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}
