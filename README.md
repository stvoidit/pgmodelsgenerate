## usage

set PG ENV:

* PGHOST
* PGPORT
* PGDATABASE
* PGUSER
* PGPASSWORD
* PGSSLMODE

    > PGHOST=localhost PGPORT=5432 PGDATABASE=mydb PGUSER=postgres PGPASSWORD=123 PGSSLMODE=disable go run main.go


## typing

| pg_type | go_type |
|------|------|
| int* | int64 |
| nimeric | float64 |
| float* | float64 |
| varchar | string |
| text | string |
| time | time.Time |
| date | time.Time |
| timestamp* | time.Time |
| interval | time.Duration |
| bool | bool |
| bytea | []byte |
| another* | interface{} |