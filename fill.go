// Package mvfaker is the struct-tag front-end onto the value layer: it compiles
// a Go struct type into a generator (once, cached) and fills values by
// reflection. It is sugar over gen/data — the same registry and the same
// Source — not a second engine.
//
//	type User struct {
//	    Name  string `fake:"name.full"`
//	    Email string `fake:"internet.email,from=Name"` // coherence
//	    Age   int    `fake:"number,min=18,max=90"`
//	}
//	var u User
//	mvfaker.FillAt(&u, 1) // deterministic
package mvfaker

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/tmarq/mvfaker/data"
	"github.com/tmarq/mvfaker/gen"
)

// Fill populates the struct pointed to by ptr using a fresh random seed.
func Fill(ptr any) error { return fillPtr(ptr, gen.Positional(rand.Uint64())) }

// FillAt populates ptr deterministically from (seed, path).
func FillAt(ptr any, seed uint64, path ...uint64) error {
	return fillPtr(ptr, gen.At(seed, path...))
}

// Struct returns a composable generator that produces T by reflection, so
// struct-fill plugs into the rest of the value layer (gen.Slice(Struct[T]()), …).
// Panics on a malformed tag — that's a programming error, surfaced eagerly.
func Struct[T any]() gen.Generator[T] {
	var zero T
	if info := infoFor(reflect.TypeOf(zero)); info.err != nil {
		panic(info.err)
	}
	return gen.New(func(s gen.Source) T {
		var v T
		_ = fillStruct(reflect.ValueOf(&v).Elem(), s, 0)
		return v
	})
}

func fillPtr(ptr any, src gen.Source) error {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("mvfaker: Fill needs a pointer to a struct, got %T", ptr)
	}
	return fillStruct(rv.Elem(), src, 0)
}

// --- compiled, cached per-type metadata ------------------------------------

// maxDepth bounds recursion so self-referential types (e.g. type Node struct {
// Next *Node }) terminate: past the cap, pointers are left nil.
const maxDepth = 8

type fieldSpec struct {
	index    int
	name     string
	from     string      // sibling field this derives from
	hasGen   bool        // resolved to a registered generator
	mk       data.MakeFn // built once
	sliceLen int         // fixed slice length from `len=`; -1 = random
}

type structInfo struct {
	fields []fieldSpec
	err    error
}

var infoCache sync.Map // reflect.Type -> *structInfo

func infoFor(t reflect.Type) *structInfo {
	if v, ok := infoCache.Load(t); ok {
		return v.(*structInfo)
	}
	info := compile(t)
	actual, _ := infoCache.LoadOrStore(t, info)
	return actual.(*structInfo)
}

func compile(t reflect.Type) *structInfo {
	info := &structInfo{}
	nameField := firstNameField(t)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		tag := f.Tag.Get("fake")
		if tag == "-" {
			continue
		}
		spec := fieldSpec{index: i, name: f.Name, sliceLen: -1}
		genName, params, from := parseTag(tag)
		if from != "" {
			spec.from = from
		}
		if f.Type.Kind() == reflect.Slice {
			spec.sliceLen = params.Int("len", -1)
		}
		if genName == "" {
			genName, params, spec.from = infer(f, nameField, spec.from)
		}
		if genName != "" {
			mk, err := data.Build(genName, params)
			if err != nil {
				info.err = fmt.Errorf("mvfaker: %s.%s: %w", t.Name(), f.Name, err)
				return info
			}
			spec.mk = mk
			spec.hasGen = true
		}
		info.fields = append(info.fields, spec)
	}
	return info
}

// parseTag splits `name,key=val,from=Field` into pieces.
func parseTag(tag string) (genName string, params data.Params, from string) {
	params = data.Params{}
	if tag == "" {
		return "", params, ""
	}
	parts := strings.Split(tag, ",")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i == 0 && !strings.Contains(p, "=") {
			genName = p
			continue
		}
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "from" {
			from = v
			continue
		}
		params[k] = coerce(v)
	}
	return genName, params, from
}

func coerce(v string) any {
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return v
}

// infer picks a default generator when a field has no explicit gen, first by
// name heuristic then by Go kind. Returns "" to leave the field structural.
func infer(f reflect.StructField, nameField, from string) (string, data.Params, string) {
	p := data.Params{}
	lname := strings.ToLower(f.Name)
	switch {
	case lname == "email":
		return "internet.email", p, nameField
	case strings.Contains(lname, "name"):
		return "name.full", p, from
	case lname == "age":
		return "number", data.Params{"min": 0, "max": 100}, from
	case strings.Contains(lname, "id") && f.Type.Kind() == reflect.String:
		return "uuid", p, from
	}
	switch f.Type.Kind() {
	case reflect.String:
		return "lorem.word", p, from
	case reflect.Bool:
		return "bool", p, from
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "number", p, from
	case reflect.Float32, reflect.Float64:
		return "number", data.Params{"min": 0, "max": 1000}, from
	}
	return "", p, from // struct/slice/ptr → handled structurally
}

func firstNameField(t reflect.Type) string {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath == "" && f.Type.Kind() == reflect.String && strings.Contains(strings.ToLower(f.Name), "name") {
			return f.Name
		}
	}
	return ""
}

// --- runtime fill ----------------------------------------------------------

func fillStruct(rv reflect.Value, src gen.Source, depth int) error {
	info := infoFor(rv.Type())
	if info.err != nil {
		return info.err
	}
	deps := map[string]any{}
	for _, fs := range info.fields {
		fv := rv.Field(fs.index)
		if !fv.CanSet() {
			continue
		}
		if fs.hasGen {
			var dep any
			if fs.from != "" {
				dep = deps[fs.from]
			}
			v := fs.mk(dep).Generate(src.Split())
			if err := assign(fv, v); err != nil {
				return fmt.Errorf("mvfaker: field %s: %w", fs.name, err)
			}
			deps[fs.name] = fv.Interface()
			continue
		}
		if err := fillField(fv, fs, src, depth); err != nil {
			return err
		}
	}
	return nil
}

func fillField(fv reflect.Value, fs fieldSpec, src gen.Source, depth int) error {
	if depth >= maxDepth {
		return nil // cycle guard: leave zero
	}
	switch fv.Kind() {
	case reflect.Pointer:
		fv.Set(reflect.New(fv.Type().Elem()))
		return fillField(fv.Elem(), fs, src, depth+1)
	case reflect.Struct:
		return fillStruct(fv, src, depth+1)
	case reflect.Slice:
		n := fs.sliceLen
		if n < 0 {
			n = int(src.Draw(4)) // 0..3 elements
		}
		sl := reflect.MakeSlice(fv.Type(), n, n)
		for i := 0; i < n; i++ {
			if err := fillElem(sl.Index(i), src, depth+1); err != nil {
				return err
			}
		}
		fv.Set(sl)
		return nil
	default:
		return nil // untagged scalar with no inference: leave zero
	}
}

func fillElem(ev reflect.Value, src gen.Source, depth int) error {
	if depth >= maxDepth {
		return nil
	}
	switch ev.Kind() {
	case reflect.Struct:
		return fillStruct(ev, src, depth+1)
	case reflect.Pointer:
		ev.Set(reflect.New(ev.Type().Elem()))
		return fillElem(ev.Elem(), src, depth+1)
	default:
		if name, ok := scalarGen(ev.Kind()); ok {
			mk, err := data.Build(name, nil)
			if err != nil {
				return err
			}
			return assign(ev, mk(nil).Generate(src.Split()))
		}
		return nil
	}
}

func scalarGen(k reflect.Kind) (string, bool) {
	switch k {
	case reflect.String:
		return "lorem.word", true
	case reflect.Bool:
		return "bool", true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number", true
	}
	return "", false
}

func assign(fv reflect.Value, v any) error {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch {
	case rv.Type().AssignableTo(fv.Type()):
		fv.Set(rv)
	case rv.Type().ConvertibleTo(fv.Type()):
		fv.Set(rv.Convert(fv.Type()))
	default:
		return fmt.Errorf("cannot assign %T to %s", v, fv.Type())
	}
	return nil
}
