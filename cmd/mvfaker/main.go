// Command mvfaker generates fake data four ways from one set of recipes:
// --fixt (a few repeatable records), --mock (realistic JSON), --seed (a large
// consistent dataset) and --prop (property testing with shrinking).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tmarq/mvfaker/gen"
	"github.com/tmarq/mvfaker/schema"

	"flag"
)

func main() {
	var (
		fixt   = flag.Bool("fixt", false, "emit a few repeatable example records")
		mock   = flag.Bool("mock", false, "emit realistic records (random seed)")
		seed   = flag.Bool("seed", false, "emit a full dataset to a sink")
		prop   = flag.Bool("prop", false, "run the demo property test with shrinking")
		n      = flag.Int("n", 5, "record count")
		entity = flag.String("entity", "", "entity to emit (default: first in config)")
		seedV  = flag.Uint64("s", 1, "seed value (determinism)")
		sql    = flag.Bool("sql", false, "seed: emit SQL instead of JSON")
		out    = flag.String("o", "", "output file (default stdout)")
	)
	flag.Parse()

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
		runProp(w, *seedV)
	case *seed:
		runSeed(w, mustPlan(), *seedV, *sql)
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

func runSeed(w io.Writer, p *schema.Plan, seed uint64, sql bool) {
	var sink schema.Sink
	if sql {
		sink = schema.NewSQLSink(w)
	} else {
		sink = schema.NewJSONSink(w)
	}
	if err := p.Seed(seed, sink); err != nil {
		die(err)
	}
}

// runProp demonstrates the recording-source + shrinker. The property below is
// deliberately false so you can watch it shrink to the simplest failing case.
func runProp(w io.Writer, seed uint64) {
	g := gen.Slice(gen.IntRange(0, 8), gen.IntRange(0, 1000))
	prop := func(xs []int) bool {
		for _, x := range xs {
			if x >= 900 {
				return false
			}
		}
		return true
	}
	res := gen.Check(seed, 1000, g, prop)
	if !res.Failed {
		fmt.Fprintf(w, "property held over %d cases\n", res.Runs)
		return
	}
	fmt.Fprintf(w, "FAILED after %d cases, shrunk %d times\n", res.Runs, res.Shrinks)
	fmt.Fprintf(w, "simplest counterexample: %v\n", res.Value)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "mvfaker:", err)
	os.Exit(1)
}
