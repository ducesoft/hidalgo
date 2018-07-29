package sqltuple

import (
	"fmt"
	"strings"

	"github.com/hidal-go/hidalgo/values"
)

type ErrorFunc func(err error) error

type Dialect struct {
	Errors              ErrorFunc
	BytesType           string
	StringType          string
	BytesKeyType        string
	StringKeyType       string
	TimeType            string
	StringTypeCollation string
	QuoteIdentifierFunc func(s string) string
	Placeholder         func(i int) string
	// DefaultSchema will be used to query table metadata.
	// If not set, defaults to the database name.
	DefaultSchema string
	// ListColumns is a query that will be executed to get columns info.
	// Two parameters will be passed to the query: current schema and the table name.
	ListColumns string
	// Unsigned indicates that a database supports UNSIGNED modifier for integer types.
	Unsigned bool
	// NoIteratorsWhenMutating mark indicates that backend cannot run iterators and
	// mutations in the same transaction (example: SELECT and DELETE while iterating).
	NoIteratorsWhenMutating bool
	// ReplaceStmt indicates that backend supports REPLACE statement.
	ReplaceStmt bool
	// OnConflict indicates that backend supports ON CONFLICT in INSERT statement.
	OnConflict          bool
	ColumnCommentInline func(s string) string
	ColumnCommentSet    func(b *Builder, tbl, col, s string)
}

func (d *Dialect) SetDefaults() {
	if d.StringType == "" {
		d.StringType = "TEXT"
	}
	if d.StringKeyType == "" {
		d.StringKeyType = d.StringType
	}
	if d.BytesType == "" {
		d.BytesType = "BLOB"
	}
	if d.BytesKeyType == "" {
		d.BytesKeyType = d.BytesType
	}
	if d.TimeType == "" {
		d.TimeType = "TIMESTAMP"
	}
	if d.Placeholder == nil {
		d.Placeholder = func(_ int) string {
			return "?"
		}
	}
}

func (d *Dialect) QuoteIdentifier(s string) string {
	if q := d.QuoteIdentifierFunc; q != nil {
		return q(s)
	}
	return "`" + strings.Replace(s, "`", "", -1) + "`"
}
func (d *Dialect) QuoteString(s string) string {
	// only used when setting comments, so it's pretty naive
	return "'" + strings.Replace(s, "'", "''", -1) + "'"
}

func needQuotes(s string) bool {
	for i, r := range s {
		if (r < 'a' || r > 'z') && r != '_' && (i == 0 || r < '0' || r > '9') {
			return true
		}
	}
	return false
}

func (d *Dialect) sqlType(t values.Type, key bool) string {
	var tp string
	switch t.(type) {
	case values.StringType:
		tp = d.StringType
		if key {
			tp = d.StringKeyType
		}
		if d.StringTypeCollation != "" {
			// TODO: set it on the table/database
			tp += " " + d.StringTypeCollation
		}
	case values.BytesType:
		tp = d.BytesType
		if key {
			tp = d.BytesKeyType
		}
	case values.IntType:
		tp = "BIGINT"
	case values.UIntType:
		tp = "BIGINT"
		if d.Unsigned {
			tp += " UNSIGNED"
		}
	case values.FloatType:
		tp = "DOUBLE PRECISION"
	case values.BoolType:
		tp = "BOOLEAN"
	case values.TimeType:
		tp = d.TimeType
	default:
		panic(fmt.Errorf("unsupported type: %T", t))
	}
	if key {
		tp += " NOT NULL"
	} else {
		tp += " NULL"
	}
	return tp
}
func (d *Dialect) sqlColumnComment(t values.Type) string {
	var c string
	switch t.(type) {
	case values.BoolType:
		c = "Bool"
	case values.UIntType:
		if !d.Unsigned {
			c = "UInt"
		}
	}
	return c
}
func (d *Dialect) sqlColumnCommentInline(t values.Type) string {
	if d.ColumnCommentInline == nil {
		return ""
	}
	c := d.sqlColumnComment(t)
	if c == "" {
		return ""
	}
	return " " + d.ColumnCommentInline(d.QuoteString(c))
}

func (d *Dialect) nativeType(typ, comment string) (values.Type, error) {
	typ = strings.ToLower(typ)
	var opt string
	if i := strings.Index(typ, "("); i > 0 {
		typ, opt = typ[:i], typ[i:]
	}
	if i := strings.Index(typ, " "); i > 0 {
		typ, opt = typ[:i], typ[i:]+opt
	}
	switch typ {
	case "text", "varchar", "char":
		return values.StringType{}, nil
	case "blob", "bytea", "varbinary", "binary":
		return values.BytesType{}, nil
	case "double", "double precision":
		return values.FloatType{}, nil
	case "boolean":
		return values.BoolType{}, nil
	case "tinyint":
		if opt == "(1)" && comment == "Bool" { // TODO: or rather if it's MySQL
			return values.BoolType{}, nil
		}
		fallthrough
	case "bigint", "int", "integer", "mediumint", "smallint":
		if strings.HasSuffix(opt, "unsigned") || comment == "UInt" {
			return values.UIntType{}, nil
		}
		return values.IntType{}, nil
	case "timestamp", "datetime", "date", "time":
		return values.TimeType{}, nil
	}
	return nil, fmt.Errorf("unsupported column type: %q", typ)
}

func escapeNullByte(s string) string {
	return strings.Replace(s, "\u0000", `\x00`, -1)
}
func unescapeNullByte(s string) string {
	return strings.Replace(s, `\x00`, "\u0000", -1)
}
