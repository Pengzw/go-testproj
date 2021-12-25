package sql

import (
	"context"
	"database/sql"
	"strings"
	"sync/atomic"
	"time"
	"github.com/pkg/errors"
)

var (
	// ErrStmtNil prepared stmt error
	ErrStmtNil 	= errors.New("sql: prepare failed and stmt nil")
	// ErrNoMaster is returned by Master when call master multiple times.
	ErrNoMaster = errors.New("sql: no master instance")
	// ErrNoRows is returned by Scan when QueryRow doesn't return a row.
	// In such a case, QueryRow returns a placeholder *Row value that defers
	// this error until a Scan.
	ErrNoRows 	= sql.ErrNoRows
	// ErrTxDone transaction done.
	ErrTxDone 	= sql.ErrTxDone
)

// DB database.
type DB struct {
	write  *conn
	read   []*conn
	idx    int64
	master *DB
}

// conn database connection
type conn struct {
	*sql.DB
	conf    *Database
}

// Tx transaction.
type Tx struct {
	db     *conn
	tx     *sql.Tx
	c      context.Context
}

// Row row.
type Row struct {
	err 	error
	*sql.Row
	db     *conn
	query  string
	args   []interface{}
}

// Scan copies the columns from the matched row into the values pointed at by dest.
func (r *Row) Scan(dest ...interface{}) (err error) {
	if r.err != nil {
		err = r.err
	} else if r.Row == nil {
		err = ErrStmtNil
	}
	if err != nil {
		return
	}
	err = r.Row.Scan(dest...)
	if err != ErrNoRows {
		err = errors.Wrapf(err, "query %s args %+v", r.query, r.args)
	}
	return
}

// Rows rows.
type Rows struct {
	*sql.Rows
}

// Close closes the Rows, preventing further enumeration. If Next is called
// and returns false and there are no further result sets,
// the Rows are closed automatically and it will suffice to check the
// result of Err. Close is idempotent and does not affect the result of Err.
func (rs *Rows) Close() (err error) {
	err = errors.WithStack(rs.Rows.Close())
	return
}

// Stmt prepared stmt.
type Stmt struct {
	db    *conn
	tx    bool
	query string
	stmt  atomic.Value
	// t     trace.Trace
}

// Open opens a database specified by its database driver name and a
// driver-specific data source name, usually consisting of at least a database
// name and connection information.
/**
 * 主从读写句柄
 * @param {[type]} c *Database) (*DB, error [description]
 */
func Open(c *Database) (*DB, error) {
	db 			:= new(DB)
	d, err 		:= connect(c, c.DSN)
	if err != nil {
		return nil, err
	}
	w 			:= &conn{DB: d, conf: c}

	rs 			:= make([]*conn, 0, len(c.ReadDSN))
	for _, rd := range c.ReadDSN {
		d, err 	:= connect(c, rd)
		if err != nil {
			return nil, err
		}
		r 		:= &conn{DB: d, conf: c}
		rs 		= append(rs, r)
	}
	
	db.write 	= w
	db.read 	= rs
	db.master 	= &DB{write: db.write}
	return db, nil
}

/**
 * 建立连接
 */
func connect(c *Database, dataSourceName string) (*sql.DB, error) {
	d, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(c.Active) 	// 同时打开的连接数
	d.SetMaxIdleConns(c.Idle) 		// 最多允许N个空闲连接保留在连接池中
	// maxLifetime
	// d.SetConnMaxLifetime(0) 		// 设置连接可重用的最大时间长度的方法
	d.SetConnMaxLifetime(time.Duration(c.IdleTimeout))
	return d, nil
}

// Begin starts a transaction. The isolation level is dependent on the driver.
func (db *DB) Begin(c context.Context) (tx *Tx, err error) {
	return db.write.begin(c)
}

// Exec executes a query without returning any rows.
// The args are for any placeholder parameters in the query.
func (db *DB) Exec(c context.Context, query string, args ...interface{}) (res sql.Result, err error) {
	return db.write.exec(c, query, args...)
}

// Prepare creates a prepared statement for later queries or executions.
// Multiple queries or executions may be run concurrently from the returned
// statement. The caller must call the statement's Close method when the
// statement is no longer needed.
func (db *DB) Prepare(query string) (*Stmt, error) {
	return db.write.prepare(query)
}

// Prepared creates a prepared statement for later queries or executions.
// Multiple queries or executions may be run concurrently from the returned
// statement. The caller must call the statement's Close method when the
// statement is no longer needed.
func (db *DB) Prepared(query string) (stmt *Stmt) {
	return db.write.prepared(query)
}

// Query executes a query that returns rows, typically a SELECT. The args are
// for any placeholder parameters in the query.
func (db *DB) Query(c context.Context, query string, args ...interface{}) (rows *Rows, err error) {
	return db.write.query(c, query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
// QueryRow always returns a non-nil value. Errors are deferred until Row's
// Scan method is called.
func (db *DB) QueryRow(c context.Context, query string, args ...interface{}) *Row {
	idx := db.readIndex()
	for i := range db.read {
		row := db.read[(idx+i)%len(db.read)].queryRow(c, query, args...)	
		if row.err == nil {
			return row
		}
	}
	row := db.write.queryRow(c, query, args...)
	return row
}

func (db *DB) readIndex() int {
	if len(db.read) == 0 {
		return 0
	}
	v := atomic.AddInt64(&db.idx, 1)
	return int(v) % len(db.read)
}

// Close closes the write and read database, releasing any open resources.
func (db *DB) Close() (err error) {
	if e := db.write.Close(); e != nil {
		err = errors.WithStack(e)
	}
	for _, rd := range db.read {
		if e := rd.Close(); e != nil {
			err = errors.WithStack(e)
		}
	}
	return
}

// Ping verifies a connection to the database is still alive, establishing a connection if necessary.
func (db *DB) Ping(c context.Context) (err error) {
	if err = db.write.ping(c); err != nil {
		return
	}
	for _, rd := range db.read {
		if err = rd.ping(c); err != nil {
			return
		}
	}
	return
}

// Master return *DB instance direct use master conn
// use this *DB instance only when you have some reason need to get result without any delay.
func (db *DB) Master() *DB {
	if db.master == nil {
		panic(ErrNoMaster)
	}
	return db.master
}

func (db *conn) begin(c context.Context) (tx *Tx, err error) {
	rtx, err := db.BeginTx(c, nil)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	tx = &Tx{tx: rtx, db: db, c: c}
	return
}

func (db *conn) exec(c context.Context, query string, args ...interface{}) (res sql.Result, err error) {
	res, err = db.ExecContext(c, query, args...)
	if err != nil {
		err = errors.Wrapf(err, "exec:%s, args:%+v", query, args)
	}
	return
}

func (db *conn) ping(c context.Context) (err error) {
	err = db.PingContext(c)
	if err != nil {
		err = errors.WithStack(err)
	}
	return
}

func (db *conn) prepare(query string) (*Stmt, error) {
	stmt, err := db.Prepare(query)
	if err != nil {
		err = errors.Wrapf(err, "prepare %s", query)
		return nil, err
	}
	st := &Stmt{query: query, db: db}
	st.stmt.Store(stmt)
	return st, nil
}

func (db *conn) prepared(query string) (stmt *Stmt) {
	stmt = &Stmt{query: query, db: db}
	s, err := db.Prepare(query)
	if err == nil {
		stmt.stmt.Store(s)
		return
	}
	go func() {
		for {
			s, err := db.Prepare(query)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			stmt.stmt.Store(s)
			return
		}
	}()
	return
}

func (db *conn) query(c context.Context, query string, args ...interface{}) (rows *Rows, err error) {
	rs, err := db.DB.QueryContext(c, query, args...)
	
	rows = &Rows{Rows: rs}
	return
}

func (db *conn) queryRow(c context.Context, query string, args ...interface{}) *Row {
	r := db.DB.QueryRowContext(c, query, args...)
	return &Row{db: db, Row: r, query: query, args: args}
}

// Close closes the statement.
func (s *Stmt) Close() (err error) {
	if s == nil {
		err = ErrStmtNil
		return
	}
	stmt, ok := s.stmt.Load().(*sql.Stmt)
	if ok {
		err = errors.WithStack(stmt.Close())
	}
	return
}


// Commit commits the transaction.
func (tx *Tx) Commit() (err error) {
	err = tx.tx.Commit()
	if err != nil {
		err = errors.WithStack(err)
	}
	return
}

// Rollback aborts the transaction.
func (tx *Tx) Rollback() (err error) {
	err = tx.tx.Rollback()
	
	if err != nil {
		err = errors.WithStack(err)
	}
	return
}

// Exec executes a query that doesn't return rows. For example: an INSERT and
// UPDATE.
func (tx *Tx) Exec(query string, args ...interface{}) (res sql.Result, err error) {
	res, err = tx.tx.ExecContext(tx.c, query, args...)
	if err != nil {
		err = errors.Wrapf(err, "exec:%s, args:%+v", query, args)
	}
	return
}

// Query executes a query that returns rows, typically a SELECT.
func (tx *Tx) Query(query string, args ...interface{}) (rows *Rows, err error) {
	rs, err := tx.tx.QueryContext(tx.c, query, args...)
	if err == nil {
		rows = &Rows{Rows: rs}
	} else {
		err = errors.Wrapf(err, "query:%s, args:%+v", query, args)
	}
	return
}

// QueryRow executes a query that is expected to return at most one row.
// QueryRow always returns a non-nil value. Errors are deferred until Row's
// Scan method is called.
func (tx *Tx) QueryRow(query string, args ...interface{}) *Row {
	r := tx.tx.QueryRowContext(tx.c, query, args...)
	return &Row{Row: r, db: tx.db, query: query, args: args}
}

// parseDSNAddr parse dsn name and return addr.
func parseDSNAddr(dsn string) (addr string) {
	if dsn == "" {
		return
	}
	part0 := strings.Split(dsn, "@")
	if len(part0) > 1 {
		part1 := strings.Split(part0[1], "?")
		if len(part1) > 0 {
			addr = part1[0]
		}
	}
	return
}
