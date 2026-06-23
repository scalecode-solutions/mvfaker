package schema

import (
	"errors"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/scalecode-solutions/mvfaker/data"
	"github.com/zclconf/go-cty/cty"
)

// Restricted-subset HCL: entity/field blocks plus dataset counts. Field bodies
// are decoded as free-form attributes so any registered generator's params flow
// through without the parser knowing them. No conditionals/loops are surfaced.

type configFile struct {
	Entities []entityBlock  `hcl:"entity,block"`
	Datasets []datasetBlock `hcl:"dataset,block"`
}

type entityBlock struct {
	Name   string       `hcl:"name,label"`
	Fields []fieldBlock `hcl:"field,block"`
}

type fieldBlock struct {
	Name string   `hcl:"name,label"`
	Body hcl.Body `hcl:",remain"`
}

type datasetBlock struct {
	Name   string         `hcl:"name,label"`
	Counts map[string]int `hcl:"counts"`
}

// LoadHCL parses a config file into a resolved Plan.
func LoadHCL(path string) (*Plan, error) {
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errors.New(diags.Error())
	}
	var cfg configFile
	if d := gohcl.DecodeBody(f.Body, nil, &cfg); d.HasErrors() {
		return nil, errors.New(d.Error())
	}

	p := &Plan{Entities: map[string]*Entity{}, Counts: map[string]int{}}
	for _, eb := range cfg.Entities {
		e := &Entity{Name: eb.Name}
		for _, fb := range eb.Fields {
			f, err := decodeField(fb)
			if err != nil {
				return nil, err
			}
			e.Fields = append(e.Fields, f)
		}
		p.Entities[e.Name] = e
		p.Order = append(p.Order, e.Name)
	}
	for _, ds := range cfg.Datasets {
		for name, n := range ds.Counts {
			p.Counts[name] = n
		}
	}
	if err := p.Resolve(); err != nil {
		return nil, err
	}
	return p, nil
}

func decodeField(fb fieldBlock) (*Field, error) {
	attrs, diags := fb.Body.JustAttributes()
	if diags.HasErrors() {
		return nil, errors.New(diags.Error())
	}
	f := &Field{Name: fb.Name, Params: data.Params{}}
	for name, attr := range attrs {
		v, d := attr.Expr.Value(nil)
		if d.HasErrors() {
			return nil, errors.New(d.Error())
		}
		gv := ctyToGo(v)
		switch name {
		case "gen":
			f.Gen, _ = gv.(string)
		case "from":
			f.From, _ = gv.(string)
		case "ref":
			f.Ref, _ = gv.(string)
		case "unique":
			f.Unique, _ = gv.(bool)
		case "transform":
			f.Transform, _ = gv.(string)
		case "maxlen":
			if i, ok := gv.(int); ok {
				f.MaxLen = i
			}
		case "null_prob":
			switch x := gv.(type) {
			case float64:
				f.NullProb = x
			case int:
				f.NullProb = float64(x)
			}
		case "when":
			f.When, _ = gv.(string)
		default:
			f.Params[name] = gv
		}
	}
	return f, nil
}

func ctyToGo(v cty.Value) any {
	if v.IsNull() {
		return nil
	}
	t := v.Type()
	if t.IsTupleType() || t.IsListType() || t.IsSetType() {
		var out []any
		for _, el := range v.AsValueSlice() {
			out = append(out, ctyToGo(el))
		}
		return out
	}
	switch t {
	case cty.String:
		return v.AsString()
	case cty.Bool:
		return v.True()
	case cty.Number:
		bf := v.AsBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return int(i)
		}
		fl, _ := bf.Float64()
		return fl
	}
	return nil
}
