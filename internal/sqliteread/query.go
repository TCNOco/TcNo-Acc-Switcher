package sqliteread

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// queryPattern is the whole query grammar this reader accepts:
// SELECT <col> FROM <table> WHERE <col> = <literal> [LIMIT n].
var queryPattern = regexp.MustCompile(`(?is)^SELECT\s+(\w+)\s+FROM\s+(\w+)\s+WHERE\s+(\w+)\s*=\s*('(?:[^']|'')*'|-?\d+(?:\.\d+)?)\s*(?:LIMIT\s+\d+\s*)?;?$`)

type query struct {
	column  string
	table   string
	where   string
	literal string
	quoted  bool
}

func parseQuery(q string) (query, error) {
	m := queryPattern.FindStringSubmatch(strings.TrimSpace(q))
	if m == nil {
		return query{}, fmt.Errorf("unsupported query %q", q)
	}
	out := query{column: m[1], table: m[2], where: m[3], literal: m[4]}
	if strings.HasPrefix(out.literal, "'") {
		out.quoted = true
		out.literal = strings.ReplaceAll(out.literal[1:len(out.literal)-1], "''", "'")
	}
	return out, nil
}

// QueryScalar returns the first value matching a SELECT-one-column query against
// a SQLite database file, or "" when no row matches. Queries outside the grammar
// queryPattern describes, and databases this reader cannot parse, are errors.
func QueryScalar(path, q string) (string, error) {
	qy, err := parseQuery(q)
	if err != nil {
		return "", err
	}
	r, err := open(path)
	if err != nil {
		return "", err
	}
	defer r.close()

	tbl, err := r.schema(qy.table)
	if err != nil {
		return "", err
	}
	sel, where := tbl.index(qy.column), tbl.index(qy.where)
	if sel < 0 {
		return "", fmt.Errorf("table %q has no column %q", qy.table, qy.column)
	}
	if where < 0 {
		return "", fmt.Errorf("table %q has no column %q", qy.table, qy.where)
	}
	want := literalValue(qy, tbl.affinity[where])

	out := ""
	err = r.walk(tbl.root, func(rowid int64, rec []byte) (bool, error) {
		vals, err := decodeRecord(rec)
		if err != nil {
			return false, err
		}
		if !equalValues(tbl.value(vals, where, rowid), want) {
			return false, nil
		}
		out = asText(tbl.value(vals, sel, rowid))
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return out, nil
}

type tableInfo struct {
	root       int
	columns    []string
	affinity   []byte
	rowidAlias int
}

func (t tableInfo) index(name string) int {
	for i, c := range t.columns {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

// value reads column i of a row, filling in the two values that are not stored
// in the record itself: an INTEGER PRIMARY KEY (which aliases the rowid) and a
// column added by ALTER TABLE after the row was written.
func (t tableInfo) value(vals []value, i int, rowid int64) value {
	if i >= len(vals) {
		return value{kind: kindNull}
	}
	if i == t.rowidAlias && vals[i].kind == kindNull {
		return value{kind: kindInt, i: rowid}
	}
	return vals[i]
}

const schemaColumns = 5 // type, name, tbl_name, rootpage, sql

func (r *reader) schema(table string) (tableInfo, error) {
	info := tableInfo{rowidAlias: -1}
	found := false
	err := r.walk(1, func(_ int64, rec []byte) (bool, error) {
		vals, err := decodeRecord(rec)
		if err != nil {
			return false, err
		}
		if len(vals) < schemaColumns || !strings.EqualFold(asText(vals[0]), "table") || !strings.EqualFold(asText(vals[1]), table) {
			return false, nil
		}
		if vals[3].kind != kindInt || vals[3].i < 1 {
			return false, fmt.Errorf("table %q has no root page", table)
		}
		info.root = int(vals[3].i)
		if err := parseColumns(asText(vals[4]), &info); err != nil {
			return false, err
		}
		found = true
		return true, nil
	})
	if err != nil {
		return tableInfo{}, err
	}
	if !found {
		return tableInfo{}, fmt.Errorf("no table %q", table)
	}
	return info, nil
}

var columnConstraints = regexp.MustCompile(`(?i)\b(PRIMARY|NOT|NULL|UNIQUE|CHECK|DEFAULT|COLLATE|REFERENCES|GENERATED|AS)\b`)

var integerPrimaryKey = regexp.MustCompile(`(?i)PRIMARY\s+KEY`)

var tableConstraints = map[string]bool{"PRIMARY": true, "UNIQUE": true, "CHECK": true, "FOREIGN": true, "CONSTRAINT": true}

// parseColumns reads column names, declared types and any rowid alias out of a
// CREATE TABLE statement.
func parseColumns(sql string, info *tableInfo) error {
	open := strings.Index(sql, "(")
	closing := strings.LastIndex(sql, ")")
	if open < 0 || closing < open {
		return errors.New("unparsable CREATE TABLE")
	}
	if tail := strings.ToUpper(sql[closing+1:]); strings.Contains(tail, "WITHOUT ROWID") {
		return errors.New("WITHOUT ROWID tables are not supported")
	}
	for _, def := range splitDefs(sql[open+1 : closing]) {
		def = strings.TrimSpace(def)
		name, rest := splitIdent(def)
		if name == "" || tableConstraints[strings.ToUpper(name)] {
			continue
		}
		declared := rest
		if m := columnConstraints.FindStringIndex(rest); m != nil {
			declared = rest[:m[0]]
		}
		declared = strings.TrimSpace(declared)
		if strings.EqualFold(declared, "INTEGER") && integerPrimaryKey.MatchString(rest) {
			info.rowidAlias = len(info.columns)
		}
		info.columns = append(info.columns, name)
		info.affinity = append(info.affinity, affinityOf(declared))
	}
	if len(info.columns) == 0 {
		return errors.New("CREATE TABLE has no columns")
	}
	return nil
}

func splitDefs(s string) []string {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// splitIdent takes the leading (possibly quoted) identifier off a column definition.
func splitIdent(def string) (string, string) {
	if def == "" {
		return "", ""
	}
	closers := map[byte]byte{'"': '"', '`': '`', '[': ']'}
	if end, quoted := closers[def[0]]; quoted {
		if i := strings.IndexByte(def[1:], end); i >= 0 {
			return def[1 : 1+i], strings.TrimSpace(def[i+2:])
		}
		return "", ""
	}
	if i := strings.IndexAny(def, " \t\r\n"); i >= 0 {
		return def[:i], strings.TrimSpace(def[i+1:])
	}
	return def, ""
}

// affinityOf applies SQLite's declared-type-to-affinity rules, in their defined
// order of precedence.
func affinityOf(declared string) byte {
	d := strings.ToUpper(declared)
	switch {
	case strings.Contains(d, "INT"):
		return 'I'
	case strings.Contains(d, "CHAR"), strings.Contains(d, "CLOB"), strings.Contains(d, "TEXT"):
		return 'T'
	case strings.Contains(d, "BLOB"), d == "":
		return 'B'
	case strings.Contains(d, "REAL"), strings.Contains(d, "FLOA"), strings.Contains(d, "DOUB"):
		return 'R'
	default:
		return 'N'
	}
}

// literalValue applies the WHERE column's affinity to the query literal, the way
// SQLite does before comparing: a quoted literal against an INTEGER column is
// converted to a number, and a bare number against a TEXT column to text.
func literalValue(q query, affinity byte) value {
	if !q.quoted {
		if affinity == 'T' {
			return value{kind: kindText, b: []byte(q.literal)}
		}
		if i, err := strconv.ParseInt(q.literal, 10, 64); err == nil {
			return value{kind: kindInt, i: i}
		}
		f, err := strconv.ParseFloat(q.literal, 64)
		if err != nil {
			return value{kind: kindText, b: []byte(q.literal)}
		}
		return value{kind: kindReal, r: f}
	}
	if affinity == 'I' || affinity == 'R' || affinity == 'N' {
		if i, err := strconv.ParseInt(q.literal, 10, 64); err == nil {
			return value{kind: kindInt, i: i}
		}
		if f, err := strconv.ParseFloat(q.literal, 64); err == nil {
			return value{kind: kindReal, r: f}
		}
	}
	return value{kind: kindText, b: []byte(q.literal)}
}

func equalValues(a, b value) bool {
	switch {
	case a.kind == kindNull || b.kind == kindNull:
		return false
	case a.kind == kindInt && b.kind == kindInt:
		return a.i == b.i
	case (a.kind == kindInt || a.kind == kindReal) && (b.kind == kindInt || b.kind == kindReal):
		return numeric(a) == numeric(b)
	case a.kind == kindText && b.kind == kindText, a.kind == kindBlob && b.kind == kindBlob:
		return string(a.b) == string(b.b)
	default:
		return false
	}
}

func numeric(v value) float64 {
	if v.kind == kindInt {
		return float64(v.i)
	}
	return v.r
}

func asText(v value) string {
	switch v.kind {
	case kindInt:
		return strconv.FormatInt(v.i, 10)
	case kindReal:
		return strconv.FormatFloat(v.r, 'g', -1, 64)
	case kindText, kindBlob:
		return string(v.b)
	default:
		return ""
	}
}
