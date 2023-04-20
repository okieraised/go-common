package sqlcommon

import (
	"database/sql"
	"fmt"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mssqldialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/schema"
	"time"
)

const (
	PostgreSQL = "postgresql"
	MySQL      = "mysql"
	MSSQL      = "mssql"
	SQLite     = "sqlite"
)

type Database struct {
	sql *bun.DB
}

var db = new(Database)
var sqldb *sql.DB
var err error

var DriverMapper = map[string]bool{
	PostgreSQL: true,
	MySQL:      true,
	MSSQL:      true,
	SQLite:     true,
}

var DriverDialectMapper = map[string]schema.Dialect{
	PostgreSQL: pgdialect.New(),
	MySQL:      mysqldialect.New(),
	MSSQL:      mssqldialect.New(),
	SQLite:     sqlitedialect.New(),
}

func DB() *Database {
	return db
}

func (d *Database) SQL() *bun.DB {
	return d.sql
}

func New(driver, url string, retry time.Duration, opts ...pgdriver.Option) error {
	if !DriverMapper[driver] {
		return ErrUnsupportedDialect
	}
	if driver == PostgreSQL {
		if len(opts) == 0 {
			sqldb = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(url)))
		} else {
			sqldb = sql.OpenDB(pgdriver.NewConnector(opts...))
		}
	} else if driver == SQLite {
		sqldb, err = sql.Open(sqliteshim.ShimName, url)
	} else {
		sqldb, err = sql.Open(driver, url)
		if err != nil {
			return err
		}
	}

	newDB := bun.NewDB(sqldb, DriverDialectMapper[driver])

	// Init retry = 5 seconds
	for {
		err = newDB.Ping()
		if err == nil {
			break
		} else {
			fmt.Println(err)
			if retry <= 0 {
				return err
			}
			time.Sleep(retry * time.Second)
		}
	}

	db.sql = newDB
	return nil
}
