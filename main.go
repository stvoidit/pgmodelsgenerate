package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

// Store - ...
type Store struct {
	db *pgx.Conn
}

// NewStore - ...
func NewStore(host, port, dbname, login, password, sslmode string) (s *Store, err error) {
	conn, err := pgx.Connect(context.Background(),
		fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, login, password, dbname, sslmode))
	if err != nil {
		return nil, err
	}
	return &Store{db: conn}, conn.Ping(context.TODO())
}

// Cursor - ...
func (s *Store) Cursor() *pgx.Conn {
	return s.db
}

// Close - ...
func (s *Store) Close() {
	s.db.Close(context.TODO())
}

func getSchemaTables(ctx context.Context, db *Store) error {
	const q = `
	SELECT
		jsonb_build_object('schema', table_schema, 'table', table_name, 'cols', jsonb_agg(jsonb_build_object('position', ordinal_position, 'comment', col_comment, 'table_name', table_name, 'column_name', column_name, 'is_nullable', CASE WHEN is_nullable = 'YES' THEN TRUE ELSE FALSE END , 'type', udt_name)) )
	FROM
		(
			SELECT
				table_schema, table_name , column_name , ordinal_position , is_nullable , udt_name, COALESCE(col_description(table_name::regclass::oid, ordinal_position), '') AS col_comment
			FROM
				information_schema.COLUMNS
			WHERE
				information_schema.COLUMNS.table_schema = 'public'
			ORDER BY
				1, 3
		) AS cols
	GROUP BY
		table_schema
		, table_name`
	cur := db.Cursor()
	rows, err := cur.Query(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	var tables []table
	for rows.Next() {
		var t table
		if err := rows.Scan(&t); err != nil {
			return err
		}
		tables = append(tables, t)
	}
	var model string
	model = "package models\nimport \"time\"\n\n"
	for _, t := range tables {
		model += t.String()
	}
	outputfilename := "models_genpg.go"
	f, err := os.Create(outputfilename)
	if err != nil {
		return err
	}
	f.WriteString("// created at " + time.Now().String() + "\n")
	f.WriteString(model)
	// if _, err := fmt.Fprint(f, model); err != nil {
	// 	return err
	// }
	if err := f.Close(); err != nil {
		return err
	}
	return exec.Command("gofmt", "-s", "-w", outputfilename).Run()
}

type table struct {
	Schema string `json:"schema"`
	Name   string `json:"table"`
	Cols   []col  `json:"cols"`
}

func (t *table) String() string {
	name := generateName(t.Name)
	comment := `// ` + name + ` - [` + t.Schema + "." + t.Name + `] SQL table` + "\n"
	st := comment + "type " + name + " struct {\n"

	for _, col := range t.Cols {
		st += col.String()
	}
	return st + "}\n"
}

type col struct {
	Position   uint64 `json:"position"`
	TableName  string `json:"table_name"`
	ColumnName string `json:"column_name"`
	IsNullable bool   `json:"is_nullable"`
	Type       string `json:"type"`
	Comment    string `json:"comment"`
}

func (c *col) String() string {
	name := generateName(c.ColumnName)
	var gotype string
	switch {
	case strings.Contains(c.Type, "int") || strings.Contains(c.Type, "numeric"):
		gotype = "int64"
	case strings.Contains(c.Type, "float"):
		gotype = "float64"
	case strings.Contains(c.Type, "varchar") || strings.Contains(c.Type, "text"):
		gotype = "string"
	case strings.Contains(c.Type, "bool"):
		gotype = "bool"
	case strings.Contains(c.Type, "time") || strings.Contains(c.Type, "date"):
		gotype = "time.Time"
	case strings.Contains(c.Type, "interval"):
		gotype = "time.Duration"
	case strings.Contains(c.Type, "bytea"):
		gotype = "[]byte"
	// case strings.Contains(c.Type, "json"):
	// 	gotype = "map[string]interface{}"
	default:
		gotype = "interface{}"
	}
	if c.IsNullable && c.Type != "bytea" && !strings.Contains(gotype, "interface") && !strings.HasPrefix(c.Type, "_") {
		gotype = "*" + gotype
	}
	if strings.HasPrefix(c.Type, "_") && c.Type != "bytea" {
		gotype = "[]" + gotype
	}
	tags := "`" + strings.Join([]string{
		// (`sql:"` + c.Name + `"`),
		(`json:"` + name + `"`),
	}, ` `) + "`"
	comment := `// [` + c.TableName + "." + c.ColumnName + `]` + ` ` + c.Comment
	return name + " " + gotype + tags + comment + "\n"
}

func generateName(str string) string {
	var fmtStr string
	for _, s := range strings.Split(str, "_") {
		if len(s) <= 3 {
			fmtStr += strings.ToUpper(s)
		} else {
			fmtStr += strings.Title(s)
		}
	}
	var upperPrevStr = strings.ToUpper(fmtStr)
	for _, upConst := range commonInitialisms {
		switch {
		case strings.HasSuffix(upperPrevStr, upConst):
			start := strings.LastIndex(upperPrevStr, upConst)
			fmtStr = fmtStr[:start] + upConst
		case strings.HasPrefix(upperPrevStr, upConst):
			start := strings.LastIndex(upperPrevStr, upConst)
			fmtStr = upConst + fmtStr[start+len(upConst):]
		}
	}
	return fmtStr
}

var commonInitialisms = [...]string{
	"ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP",
	"HTTPS", "ID", "IP", "JSON", "LHS", "QPS", "RAM", "RHS", "RPC", "SLA",
	"SMTP", "SQL", "SSH", "TCP", "TLS", "TTL", "UDP", "UI", "UID", "UUID",
	"URI", "URL", "UTF8", "VM", "XML", "XMPP", "XSRF", "XSS",
}

func main() {
	db, err := NewStore(os.Getenv("PGHOST"), os.Getenv("PGPORT"), os.Getenv("PGDATABASE"), os.Getenv("PGUSER"), os.Getenv("PGPASSWORD"), os.Getenv(`PGSSLMODE`))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := getSchemaTables(ctx, db); err != nil {
		panic(err)
	}

}
