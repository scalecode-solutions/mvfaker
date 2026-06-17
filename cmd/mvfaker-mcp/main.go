// Command mvfaker-mcp is a Model Context Protocol server that exposes mvfaker to
// AI agents over stdio — the inference-time door an agent walks through to
// discover and use mvfaker without it being in the model's weights. Tools:
//
//	list_generators   — what mvfaker can generate (the catalog)
//	list_locales      — available locales
//	generate_dataset  — turn a JSON dataset spec into data (json/sql/copy)
//
// Run it from an MCP client (e.g. Claude). See README for the config snippet.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/scalecode-solutions/mvfaker/schema"
)

const maxRows = 50000 // MCP returns data inline; large seeds belong on the CLI

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mvfaker",
		Version: "0.1.0",
		Title:   "mvfaker — coherent fake data",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_generators",
		Description: "List every field generator mvfaker offers (names like name.full, internet.email, country, creditcard, ipv6). Use these as the \"gen\" of a field in generate_dataset.",
	}, listGenerators)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_locales",
		Description: "List available locale codes (e.g. en-US, es-ES, ja-JP). Pass one as a field param {\"locale\":\"es-ES\"} to localize names/addresses.",
	}, listLocales)

	mcp.AddTool(server, &mcp.Tool{
		Name: "generate_dataset",
		Description: "Generate a coherent fake dataset from a spec and return it as data. " +
			"Each entity has a name, count, and fields. A field has a \"gen\" (generator name) " +
			"plus optional \"from\" (derive coherently from another field, e.g. email from name), " +
			"\"ref\" (foreign key like \"users.id\"), \"unique\":true, and \"params\". " +
			"Coherent + referentially valid by construction. format is json (default), sql, or copy.",
	}, generateDataset)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("mvfaker-mcp: %v", err)
	}
}

// --- list_generators -------------------------------------------------------

type listGeneratorsIn struct{}

type listGeneratorsOut struct {
	Generators []string `json:"generators"`
	Hint       string   `json:"hint"`
}

func listGenerators(_ context.Context, _ *mcp.CallToolRequest, _ listGeneratorsIn) (*mcp.CallToolResult, listGeneratorsOut, error) {
	return nil, listGeneratorsOut{
		Generators: data.Names(),
		Hint:       "Use a name as a field's \"gen\". For coherence, set \"from\" to a sibling field (e.g. internet.email from a name field; address.city/country.code/phone from a country field).",
	}, nil
}

// --- list_locales ----------------------------------------------------------

type listLocalesIn struct{}

type listLocalesOut struct {
	Locales []string `json:"locales"`
}

func listLocales(_ context.Context, _ *mcp.CallToolRequest, _ listLocalesIn) (*mcp.CallToolResult, listLocalesOut, error) {
	return nil, listLocalesOut{Locales: data.LocaleCodes()}, nil
}

// --- generate_dataset ------------------------------------------------------

type generateIn struct {
	Entities []schema.EntitySpec `json:"entities"`
	Format   string              `json:"format,omitempty"` // json (default) | sql | copy
	Seed     uint64              `json:"seed,omitempty"`   // for reproducibility
}

type generateOut struct {
	Format string `json:"format"`
	Rows   int    `json:"rows"`
	Data   string `json:"data"`
}

func generateDataset(_ context.Context, _ *mcp.CallToolRequest, in generateIn) (*mcp.CallToolResult, generateOut, error) {
	total := 0
	for _, e := range in.Entities {
		c := e.Count
		if c <= 0 {
			c = 10
		}
		total += c
	}
	if total > maxRows {
		return nil, generateOut{}, fmt.Errorf("requested %d rows exceeds the %d-row MCP limit; for large seeds use the mvfaker CLI (--seed --copy)", total, maxRows)
	}

	plan, err := schema.Spec{Entities: in.Entities}.Plan()
	if err != nil {
		return nil, generateOut{}, err
	}
	seed := in.Seed
	if seed == 0 {
		seed = 1
	}
	format := in.Format
	if format == "" {
		format = "json"
	}
	out, rows, err := plan.Render(seed, format)
	if err != nil {
		return nil, generateOut{}, err
	}
	return nil, generateOut{Format: format, Rows: rows, Data: out}, nil
}
