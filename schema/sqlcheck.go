package schema

import (
	"regexp"
	"strings"
)

// Pre-flight schema checking: parse a schema.sql (CREATE TABLE statements) and
// diff the columns mvfaker would emit against the real table columns — so a
// typo'd column name (the classic `town` vs `city`) is caught before generating
// anything, not at psql load time. No DB connection: mvfaker stays a pure
// generator and just reads your DDL.

// Column is a parsed table column.
type Column struct {
	Name string
	Type string
}

// Issue is a single check finding. Level is "error" (blocks seeding) or "info".
type Issue struct {
	Entity string
	Level  string
	Msg    string
}

var ifNotExists = regexp.MustCompile(`(?i)^if\s+not\s+exists\s+`)

// ParseSQLSchema extracts columns per table from CREATE TABLE statements. It is
// deliberately tolerant: it understands inline column definitions and skips
// table-level constraints.
func ParseSQLSchema(sql string) map[string][]Column {
	out := map[string][]Column{}
	lower := strings.ToLower(sql)
	pos := 0
	for {
		i := strings.Index(lower[pos:], "create table")
		if i < 0 {
			break
		}
		start := pos + i + len("create table")
		open := strings.IndexByte(sql[start:], '(')
		if open < 0 {
			break
		}
		open += start

		name := strings.TrimSpace(sql[start:open])
		name = ifNotExists.ReplaceAllString(name, "")
		name = unquote(stripSchema(strings.TrimSpace(name)))

		// balanced parens for the body
		depth, k := 0, open
		for ; k < len(sql); k++ {
			switch sql[k] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				break
			}
		}
		body := sql[open+1 : k]
		out[strings.ToLower(name)] = parseColumns(body)
		pos = k + 1
	}
	return out
}

func parseColumns(body string) []Column {
	var cols []Column
	for _, part := range splitTopLevel(body) {
		f := strings.Fields(strings.TrimSpace(part))
		if len(f) == 0 {
			continue
		}
		switch strings.ToLower(unquote(f[0])) {
		case "constraint", "primary", "foreign", "unique", "check", "exclude", "like":
			continue // table-level constraint, not a column
		}
		c := Column{Name: strings.ToLower(unquote(f[0]))}
		if len(f) > 1 {
			c.Type = strings.ToLower(f[1])
		}
		cols = append(cols, c)
	}
	return cols
}

func splitTopLevel(s string) []string {
	var parts []string
	depth, last := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[last:i])
				last = i + 1
			}
		}
	}
	return append(parts, s[last:])
}

func stripSchema(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func unquote(s string) string { return strings.Trim(s, `"`) }

// CheckSchema diffs the plan's emitted columns against the parsed schema.
func (p *Plan) CheckSchema(tables map[string][]Column) []Issue {
	var issues []Issue
	for _, name := range p.Order {
		e := p.Entities[name]
		cols, ok := tables[strings.ToLower(name)]
		if !ok {
			issues = append(issues, Issue{name, "error",
				"table not found in schema (have: " + tableNames(tables) + ")"})
			continue
		}
		byName := map[string]Column{}
		for _, c := range cols {
			byName[c.Name] = c
		}
		emitted := map[string]bool{}

		check := func(col, cat string) {
			emitted[col] = true
			c, exists := byName[col]
			if !exists {
				issues = append(issues, Issue{name, "error",
					"column \"" + col + "\" not found in table (has: " + columnNames(cols) + ")"})
				return
			}
			if typeClash(cat, sqlCategory(c.Type)) {
				issues = append(issues, Issue{name, "error",
					"column \"" + col + "\" is " + c.Type + " but config emits " + cat})
			}
		}

		check("id", "numeric")
		for _, f := range e.Fields {
			check(f.Name, genCategory(f))
		}

		// info: table columns the config doesn't populate
		for _, c := range cols {
			if !emitted[c.Name] {
				issues = append(issues, Issue{name, "info",
					"table column \"" + c.Name + "\" is not populated by the config"})
			}
		}
	}
	return issues
}

func genCategory(f *Field) string {
	if f.Ref != "" {
		return "numeric"
	}
	switch f.Gen {
	case "number":
		return "numeric"
	case "bool":
		return "bool"
	default:
		return "text"
	}
}

func sqlCategory(t string) string {
	switch {
	case strings.HasPrefix(t, "int"), strings.HasPrefix(t, "bigint"),
		strings.HasPrefix(t, "smallint"), strings.HasPrefix(t, "serial"),
		strings.HasPrefix(t, "bigserial"):
		return "int"
	case strings.HasPrefix(t, "bool"):
		return "bool"
	default:
		return "text"
	}
}

// typeClash flags only high-confidence mismatches (low false-positive).
func typeClash(emit, col string) bool {
	switch emit {
	case "text":
		return col == "int" || col == "bool"
	case "numeric":
		return col == "bool"
	case "bool":
		return col == "int"
	}
	return false
}

func columnNames(cols []Column) string {
	n := make([]string, len(cols))
	for i, c := range cols {
		n[i] = c.Name
	}
	return strings.Join(n, ", ")
}

func tableNames(t map[string][]Column) string {
	n := make([]string, 0, len(t))
	for k := range t {
		n = append(n, k)
	}
	return strings.Join(n, ", ")
}
