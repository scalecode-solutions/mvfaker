// Command embedded generates embedded (denormalized) forum documents and writes
// NDJSON — each article carries its author and its comments inline. This is the
// document-store counterpart to the relational referenced model: mvfaker's
// struct-tag front-end fills nested structs, and a nested struct IS an embedded
// document. Run: go run ./examples/embedded 1000 > articles.ndjson
package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"

	mvfaker "github.com/scalecode-solutions/mvfaker"
)

// Author is embedded as a denormalized snapshot (the document-store way: you
// copy what you need to read alongside the parent).
type Author struct {
	Name    string `fake:"name.full"      json:"name"`
	Country string `fake:"address.country" json:"country"`
	City    string `fake:"address.city,from=Country" json:"city"` // coherent with Country
}

type Comment struct {
	Author string `fake:"name.full"        json:"author"`
	Body   string `fake:"lorem.words,n=12" json:"body"`
}

type Article struct {
	ID       int       `fake:"-"                json:"id"` // we set this
	Title    string    `fake:"lorem.words,n=5"  json:"title"`
	Created  string    `fake:"date,min=2022,max=2024" json:"created"`
	Author   Author    `json:"author"`                // embedded sub-document
	Comments []Comment `fake:"len=4" json:"comments"` // embedded array
}

func main() {
	n := 1000
	if len(os.Args) > 1 {
		n, _ = strconv.Atoi(os.Args[1])
	}
	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()
	enc := json.NewEncoder(bw) // Encode writes one JSON value + newline → NDJSON
	for i := 0; i < n; i++ {
		var a Article
		if err := mvfaker.FillAt(&a, 1, uint64(i)); err != nil {
			panic(err)
		}
		a.ID = i
		if err := enc.Encode(&a); err != nil {
			panic(err)
		}
	}
}
