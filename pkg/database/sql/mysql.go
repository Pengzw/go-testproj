package sql

import (
	"time"
	"github.com/pkg/errors"
	_ "github.com/go-sql-driver/mysql"
)

type Database struct {
	DSN          	string          // write data source name.
	ReadDSN      	[]string        // read data source name.
	Active       	int             // pool
	Idle         	int             // pool
	IdleTimeout  	time.Duration   // connect max life time.
	QueryTimeout 	time.Duration   // query sql timeout
	ExecTimeout  	time.Duration   // execute sql timeout
	TranTimeout  	time.Duration   // transaction sql timeout
}

// NewMySQL new db and retry connection when has error.
func NewMySQL(c *Database) (db *DB, err error) {
	if c.QueryTimeout == 0 || c.ExecTimeout == 0 || c.TranTimeout == 0 {
		err 		= errors.Wrap(err, "mysql must be set query/execute/transction timeout")
		return
	}
	db, err 		= Open(c)
	if err != nil {
		err 		= errors.Wrap(err, "NewMySQL open dsn fail")
	}
	return 
}
