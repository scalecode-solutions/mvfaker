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

	mvfaker "github.com/tmarq/mvfaker"
	"github.com/tmarq/mvfaker/gen"
	"github.com/tmarq/mvfaker/schema"
)

// Demo rules so `--prop` works out of the box. Real usage registers your own
// rules via mvfaker.RegisterRule in your own binary — the registry is the seam.
func init() {
	mvfaker.RegisterRule("demo.no-big",
		gen.Slice(gen.IntRange(0, 8), gen.IntRange(0, 1000)),
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
		fixt   = flag.Bool("fixt", false, "emit a few repeatable example records")
		mock   = flag.Bool("mock", false, "emit realistic records (random seed)")
		seed   = flag.Bool("seed", false, "emit a full dataset to a sink")
		prop   = flag.Bool("prop", false, "run property tests with shrinking (optionally name a rule)")
		n      = flag.Int("n", 5, "record count")
		runs   = flag.Int("runs", 1000, "prop: cases to try per rule")
		entity = flag.String("entity", "", "entity to emit (default: first in config)")
		seedV  = flag.Uint64("s", 1, "seed value (determinism)")
		sql    = flag.Bool("sql", false, "seed: emit SQL INSERTs instead of JSON")
		copyF  = flag.Bool("copy", false, "seed: emit Postgres COPY (fast bulk load)")
		out    = flag.String("o", "", "output file (default stdout)")
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
	case *prop:
		runProp(w, flag.Args(), *seedV, *runs)
	case *seed:
		runSeed(w, mustPlan(), *seedV, format)
	case *fixt:
		runEmit(w, mustPlan(), *entity, *seedV, *n)
	case *mock:
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
