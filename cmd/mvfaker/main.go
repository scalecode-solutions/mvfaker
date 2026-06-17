// Command mvfaker generates fake data four ways from one set of recipes:
// --fixt (a few repeatable records), --mock (realistic JSON), --seed (a large
// consistent dataset) and --prop (property testing with shrinking).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	mvfaker "github.com/scalecode-solutions/mvfaker"
	"github.com/scalecode-solutions/mvfaker/codegen"
	"github.com/scalecode-solutions/mvfaker/gen"
	"github.com/scalecode-solutions/mvfaker/mock"
	"github.com/scalecode-solutions/mvfaker/schema"
)

// Demo rules so `--prop` works out of the box. Real usage registers your own
// rules via mvfaker.RegisterRule in your own binary — the registry is the seam.
func init() {
	mvfaker.RegisterRule("demo.no-big",
		gen.List(8, gen.IntRange(0, 1000)),
		func(xs []int) bool {
			for _, x := range xs {
				if x >= 900 {
					return false
				}
			}
			return true
		})
	mvfaker.RegisterRule("demo.abs-nonneg",
		gen.IntRange(-1000, 1000),
		func(x int) bool {
			if x < 0 {
				x = -x
			}
			return x >= 0
		})
}

func main() {
	var (
		fixt       = flag.Bool("fixt", false, "emit a few repeatable example records")
		mockF      = flag.Bool("mock", false, "emit realistic records (random seed)")
		seed       = flag.Bool("seed", false, "emit a full dataset to a sink")
		prop       = flag.Bool("prop", false, "run property tests with shrinking (optionally name a rule)")
		genGo      = flag.Bool("gen", false, "compile the config to standalone Go (scale path)")
		pkg        = flag.String("pkg", "fixtures", "gen: package name for the emitted Go")
		n          = flag.Int("n", 5, "record count")
		runs       = flag.Int("runs", 1000, "prop: cases to try per rule")
		entity     = flag.String("entity", "", "entity to emit (default: first in config)")
		seedV      = flag.Uint64("s", 1, "seed value (determinism)")
		sql        = flag.Bool("sql", false, "seed: emit SQL INSERTs instead of JSON")
		copyF      = flag.Bool("copy", false, "seed: emit Postgres COPY (fast bulk load)")
		serve      = flag.String("serve", "", "mock: serve HTTP on this address, e.g. :8080")
		check      = flag.Bool("check", false, "verify config columns against --schema and exit; emits no data")
		schemaPath = flag.String("schema", "", "check: path to a schema.sql to validate against")
		dryRun     = flag.Bool("dryrun", false, "seed: print what would be generated; emit no data")
		out        = flag.String("o", "", "output file (default stdout)")
	)
	flag.Parse()

	format := "json"
	if *sql {
		format = "sql"
	}
	if *copyF {
		format = "copy"
	}

	w := io.Writer(os.Stdout)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			die(err)
		}
		defer f.Close()
		w = f
	}

	switch {
	case *genGo:
		if err := codegen.Emit(mustPlan(), *pkg, w); err != nil {
			die(err)
		}
	case *prop:
		runProp(w, flag.Args(), *seedV, *runs)
	case *check:
		// A standalone verified stage: validate and exit, writing no data.
		runCheck(w, mustPlan(), *schemaPath)
	case *seed, *dryRun:
		p := mustPlan()
		if *dryRun {
			runDryRun(w, p)
			return
		}
		runSeed(w, p, *seedV, format)
	case *fixt:
		runEmit(w, mustPlan(), *entity, *seedV, *n)
	case *mockF:
		if *serve != "" {
			p := mustPlan()
			fmt.Fprintf(os.Stderr, "mvfaker mock server on %s — try /%s\n", *serve, p.Order[0])
			if err := mock.Serve(p, *serve, *seedV, *n); err != nil {
				die(err)
			}
			return
		}
		// mock = same recipes, a "fresh-looking" seed
		runEmit(w, mustPlan(), *entity, *seedV*2654435761+1, *n)
	default:
		fmt.Fprintln(os.Stderr, "pick a mode: --fixt | --mock | --seed | --prop  [config.hcl]")
		flag.Usage()
		os.Exit(2)
	}
}

func mustPlan() *schema.Plan {
	args := flag.Args()
	if len(args) < 1 {
		die(fmt.Errorf("need a config file, e.g. mvfaker --fixt example.hcl"))
	}
	p, err := schema.LoadHCL(args[0])
	if err != nil {
		die(err)
	}
	return p
}

func runEmit(w io.Writer, p *schema.Plan, entity string, seed uint64, n int) {
	if entity == "" {
		if len(p.Order) == 0 {
			die(fmt.Errorf("config has no entities"))
		}
		entity = p.Order[0]
	}
	recs, err := p.Generate(entity, seed, n)
	if err != nil {
		die(err)
	}
	b, _ := json.MarshalIndent(recs, "", "  ")
	fmt.Fprintln(w, string(b))
}

// runCheck is a standalone verified stage: it validates the config's emitted
// columns against a schema.sql and exits, writing no data ever. Exit 0 when
// everything matches, exit 1 on any mismatch — so it drops into CI or a deploy
// gate as "is this config safe to seed?" without side effects.
func runCheck(w io.Writer, p *schema.Plan, schemaPath string) {
	if schemaPath == "" {
		die(fmt.Errorf("--check requires --schema <file.sql>"))
	}
	ddl, err := os.ReadFile(schemaPath)
	if err != nil {
		die(err)
	}
	tables := schema.ParseSQLSchema(string(ddl))
	issues := p.CheckSchema(tables)

	byEntity := map[string][]schema.Issue{}
	errCount := 0
	for _, is := range issues {
		byEntity[is.Entity] = append(byEntity[is.Entity], is)
		if is.Level == "error" {
			errCount++
		}
	}
	total := 0
	for _, name := range p.Order {
		total += p.Counts[name]
		es := byEntity[name]
		hasErr := false
		for _, is := range es {
			if is.Level == "error" {
				hasErr = true
			}
		}
		mark := "✓"
		if hasErr {
			mark = "✗"
		}
		fmt.Fprintf(w, "  %s %s\n", mark, name)
		for _, is := range es {
			bullet := "•"
			if is.Level == "error" {
				bullet = "✗"
			}
			fmt.Fprintf(w, "      %s %s\n", bullet, is.Msg)
		}
	}
	if errCount > 0 {
		fmt.Fprintf(w, "\nschema check FAILED: %d mismatch(es). Nothing written.\n", errCount)
		os.Exit(1)
	}
	fmt.Fprintf(w, "\nschema check passed: %d entities, ~%d rows would seed. Nothing written.\n", len(p.Order), total)
}

// runDryRun prints what would be generated, without emitting any data.
func runDryRun(w io.Writer, p *schema.Plan) {
	fmt.Fprintln(w, "dry run — would generate (no data written):")
	total := 0
	for _, name := range p.Order {
		e := p.Entities[name]
		n := p.Counts[name]
		total += n
		cols := []string{"id"}
		for _, f := range e.Fields {
			cols = append(cols, f.Name)
		}
		fmt.Fprintf(w, "  %-10s %8d rows   COPY %s (%s)\n", name, n, name, strings.Join(cols, ", "))
	}
	fmt.Fprintf(w, "  ~%d rows total.\n", total)
}

func runSeed(w io.Writer, p *schema.Plan, seed uint64, format string) {
	var sink schema.Sink
	switch format {
	case "copy":
		sink = schema.NewCopySink(w)
	case "sql":
		sink = schema.NewSQLSink(w)
	default:
		sink = schema.NewJSONSink(w)
	}
	if err := p.Seed(seed, sink); err != nil {
		die(err)
	}
}

// runProp runs registered rules through the recording-source + shrinker. With a
// rule name it runs just that one; otherwise it runs them all.
func runProp(w io.Writer, args []string, seed uint64, runs int) {
	if len(args) >= 1 {
		res, err := mvfaker.RunRule(args[0], seed, runs)
		if err != nil {
			die(err)
		}
		printRule(w, res)
		return
	}
	results := mvfaker.RunAllRules(seed, runs)
	if len(results) == 0 {
		fmt.Fprintln(w, "no rules registered")
		return
	}
	for _, r := range results {
		printRule(w, r)
	}
}

func printRule(w io.Writer, r mvfaker.RuleResult) {
	if r.Failed {
		fmt.Fprintf(w, "✗ %s — FAILED after %d cases, shrunk %d× → %s\n",
			r.Name, r.Runs, r.Shrinks, r.Example)
		return
	}
	fmt.Fprintf(w, "✓ %s — held over %d cases\n", r.Name, r.Runs)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "mvfaker:", err)
	os.Exit(1)
}
