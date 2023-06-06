// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sql

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"internal/race"
	"internal/testenv"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	type dbConn struct {
		db_ *_DB
		c   *driverConn
	}
	freedFrom := make(map[dbConn]string)
	var mu sync.Mutex
	getFreedFrom := func(c dbConn) string {
		mu.Lock()
		defer mu.Unlock()
		return freedFrom[c]
	}
	setFreedFrom := func(c dbConn, s string) {
		mu.Lock()
		defer mu.Unlock()
		freedFrom[c] = s
	}
	putConnHook = func(db_ *_DB, c *driverConn) {
		idx := -1
		for i, v := range db_.freeConn {
			if v == c {
				idx = i
				break
			}
		}
		if idx >= 0 {
			// print before panic, as panic may get lost due to conflicting panic
			// (all goroutines asleep) elsewhere, since we might not unlock
			// the mutex in freeConn here.
			println("double free of conn. conflicts are:\nA) " + getFreedFrom(dbConn{db_, c}) + "\n\nand\nB) " + stack())
			panic("double free of conn.")
		}
		setFreedFrom(dbConn{db_, c}, stack())
	}
}

// pollDuration is an arbitrary interval to wait between checks when polling for
// a condition to occur.
const pollDuration = 5 * time.Millisecond

const fakeDBName = "foo"

var chrisBirthday = time.Unix(123456789, 0)

func newTestDB(t testing.TB, name string) *_DB {
	return newTestDBConnector(t, &fakeConnector{name: fakeDBName}, name)
}

func newTestDBConnector(t testing.TB, fc *fakeConnector, name string) *_DB {
	fc.name = fakeDBName
	db_ := OpenDB(fc).(*_DB)
	if _, err := db_.Exec("WIPE"); err != nil {
		t.Fatalf("exec wipe: %v", err)
	}
	if name == "people" {
		exec(t, db_, "CREATE|people|name=string,age=int32,photo=blob,dead=bool,bdate=datetime")
		exec(t, db_, "INSERT|people|name=Alice,age=?,photo=APHOTO", 1)
		exec(t, db_, "INSERT|people|name=Bob,age=?,photo=BPHOTO", 2)
		exec(t, db_, "INSERT|people|name=Chris,age=?,photo=CPHOTO,bdate=?", 3, chrisBirthday)
	}
	if name == "magicquery" {
		// Magic table name and column, known by fakedb_test.go.
		exec(t, db_, "CREATE|magicquery|op=string,millis=int32")
		exec(t, db_, "INSERT|magicquery|op=sleep,millis=10")
	}
	if name == "tx_status" {
		// Magic table name and column, known by fakedb_test.go.
		exec(t, db_, "CREATE|tx_status|tx_status=string")
		exec(t, db_, "INSERT|tx_status|tx_status=invalid")
	}
	return db_
}

func TestOpenDB(t *testing.T) {
	db_ := OpenDB(dsnConnector{dsn: fakeDBName, driver: fdriver})
	if db_.Driver() != fdriver {
		t.Fatalf("OpenDB should return the driver of the Connector")
	}
}

func TestDriverPanic(t *testing.T) {
	// Test that if driver panics, database/sql does not deadlock.
	db1, err := Open("test", fakeDBName)
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	expectPanic := func(name string, f func()) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatalf("%s did not panic", name)
			}
		}()
		f()
	}

	expectPanic("Exec Exec", func() { db_.Exec("PANIC|Exec|WIPE") })
	exec(t, db_, "WIPE") // check not deadlocked
	expectPanic("Exec NumInput", func() { db_.Exec("PANIC|NumInput|WIPE") })
	exec(t, db_, "WIPE") // check not deadlocked
	expectPanic("Exec Close", func() { db_.Exec("PANIC|Close|WIPE") })
	exec(t, db_, "WIPE")             // check not deadlocked
	exec(t, db_, "PANIC|Query|WIPE") // should run successfully: Exec does not call Query
	exec(t, db_, "WIPE")             // check not deadlocked

	exec(t, db_, "CREATE|people|name=string,age=int32,photo=blob,dead=bool,bdate=datetime")

	expectPanic("Query Query", func() { db_.Query("PANIC|Query|SELECT|people|age,name|") })
	expectPanic("Query NumInput", func() { db_.Query("PANIC|NumInput|SELECT|people|age,name|") })
	expectPanic("Query Close", func() {
		rows, err := db_.Query("PANIC|Close|SELECT|people|age,name|")
		if err != nil {
			t.Fatal(err)
		}
		rows.Close()
	})
	db_.Query("PANIC|Exec|SELECT|people|age,name|") // should run successfully: Query does not call Exec
	exec(t, db_, "WIPE")                            // check not deadlocked
}

func exec(t testing.TB, db_ *_DB, query string, args ...any) {
	t.Helper()
	_, err := db_.Exec(query, args...)
	if err != nil {
		t.Fatalf("Exec of %q: %v", query, err)
	}
}

func closeDB(t testing.TB, db_ *_DB) {
	if e := recover(); e != nil {
		fmt.Printf("Panic: %v\n", e)
		panic(e)
	}
	defer setHookpostCloseConn(nil)
	setHookpostCloseConn(func(_ *fakeConn, err error) {
		if err != nil {
			t.Errorf("Error closing fakeConn: %v", err)
		}
	})
	db_.mu.Lock()
	for i, dc := range db_.freeConn {
		if n := len(dc.openStmt); n > 0 {
			// Just a sanity check. This is legal in
			// general, but if we make the tests clean up
			// their statements first, then we can safely
			// verify this is always zero here, and any
			// other value is a leak.
			t.Errorf("while closing db_, freeConn %d/%d had %d open stmts; want 0", i, len(db_.freeConn), n)
		}
	}
	db_.mu.Unlock()

	err := db_.Close()
	if err != nil {
		t.Fatalf("error closing _DB: %v", err)
	}

	var numOpen int
	if !waitCondition(t, func() bool {
		numOpen = db_.numOpenConns()
		return numOpen == 0
	}) {
		t.Fatalf("%d connections still open after closing _DB", numOpen)
	}
}

// numPrepares assumes that _DB has exactly 1 idle conn and returns
// its count of calls to Prepare
func numPrepares(t *testing.T, db_ *_DB) int {
	if n := len(db_.freeConn); n != 1 {
		t.Fatalf("free conns = %d; want 1", n)
	}
	return db_.freeConn[0].ci.(*fakeConn).numPrepare
}

func (db_ *_DB) numDeps() int {
	db_.mu.Lock()
	defer db_.mu.Unlock()
	return len(db_.dep)
}

// Dependencies are closed via a goroutine, so this polls waiting for
// numDeps to fall to want, waiting up to nearly the test's deadline.
func (db_ *_DB) numDepsPoll(t *testing.T, want int) int {
	var n int
	waitCondition(t, func() bool {
		n = db_.numDeps()
		return n <= want
	})
	return n
}

func (db_ *_DB) numFreeConns() int {
	db_.mu.Lock()
	defer db_.mu.Unlock()
	return len(db_.freeConn)
}

func (db_ *_DB) numOpenConns() int {
	db_.mu.Lock()
	defer db_.mu.Unlock()
	return db_.numOpen
}

// clearAllConns closes all connections in db_.
func (db_ *_DB) clearAllConns(t *testing.T) {
	db_.SetMaxIdleConns(0)

	if g, w := db_.numFreeConns(), 0; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	if n := db_.numDepsPoll(t, 0); n > 0 {
		t.Errorf("number of dependencies = %d; expected 0", n)
		db_.dumpDeps(t)
	}
}

func (db_ *_DB) dumpDeps(t *testing.T) {
	for fc := range db_.dep {
		db_.dumpDep(t, 0, fc, map[finalCloser]bool{})
	}
}

func (db_ *_DB) dumpDep(t *testing.T, depth int, dep finalCloser, seen map[finalCloser]bool) {
	seen[dep] = true
	indent := strings.Repeat("  ", depth)
	ds := db_.dep[dep]
	for k := range ds {
		t.Logf("%s%T (%p) waiting for -> %T (%p)", indent, dep, dep, k, k)
		if fc, ok := k.(finalCloser); ok {
			if !seen[fc] {
				db_.dumpDep(t, depth+1, fc, seen)
			}
		}
	}
}

func TestQuery(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	prepares0 := numPrepares(t, db_)
	rows, err := db_.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	type row struct {
		age  int
		name string
	}
	got := []row{}
	for rows.Next() {
		var r row
		err = rows.Scan(&r.age, &r.name)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		got = append(got, r)
	}
	err = rows.Err()
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := []row{
		{age: 1, name: "Alice"},
		{age: 2, name: "Bob"},
		{age: 3, name: "Chris"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mismatch.\n got: %#v\nwant: %#v", got, want)
	}

	// And verify that the final rows.Next() call, which hit EOF,
	// also closed the rows connection.
	if n := db_.numFreeConns(); n != 1 {
		t.Fatalf("free conns after query hitting EOF = %d; want 1", n)
	}
	if prepares := numPrepares(t, db_) - prepares0; prepares != 1 {
		t.Errorf("executed %d Prepare statements; want 1", prepares)
	}
}

// TestQueryContext tests canceling the context while scanning the rows.
func TestQueryContext(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	prepares0 := numPrepares(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rows, err := db_.QueryContext(ctx, "SELECT|people|age,name|")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	type row struct {
		age  int
		name string
	}
	got := []row{}
	index := 0
	for rows.Next() {
		if index == 2 {
			cancel()
			waitForRowsClose(t, rows)
		}
		var r row
		err = rows.Scan(&r.age, &r.name)
		if err != nil {
			if index == 2 {
				break
			}
			t.Fatalf("Scan: %v", err)
		}
		if index == 2 && err != context.Canceled {
			t.Fatalf("Scan: %v; want context.Canceled", err)
		}
		got = append(got, r)
		index++
	}
	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != context.Canceled {
			t.Fatalf("context err = %v; want context.Canceled", err)
		}
	default:
		t.Fatalf("context err = nil; want context.Canceled")
	}
	want := []row{
		{age: 1, name: "Alice"},
		{age: 2, name: "Bob"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mismatch.\n got: %#v\nwant: %#v", got, want)
	}

	// And verify that the final rows.Next() call, which hit EOF,
	// also closed the rows connection.
	waitForRowsClose(t, rows)
	waitForFree(t, db_, 1)
	if prepares := numPrepares(t, db_) - prepares0; prepares != 1 {
		t.Errorf("executed %d Prepare statements; want 1", prepares)
	}
}

func waitCondition(t testing.TB, fn func() bool) bool {
	timeout := 5 * time.Second

	type deadliner interface {
		Deadline() (time.Time, bool)
	}
	if td, ok := t.(deadliner); ok {
		if deadline, ok := td.Deadline(); ok {
			timeout = time.Until(deadline)
			timeout = timeout * 19 / 20 // Give 5% headroom for cleanup and error-reporting.
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		if fn() {
			return true
		}
		if time.Until(deadline) < pollDuration {
			return false
		}
		time.Sleep(pollDuration)
	}
}

// waitForFree checks db_.numFreeConns until either it equals want or
// the maxWait time elapses.
func waitForFree(t *testing.T, db_ *_DB, want int) {
	var numFree int
	if !waitCondition(t, func() bool {
		numFree = db_.numFreeConns()
		return numFree == want
	}) {
		t.Fatalf("free conns after hitting EOF = %d; want %d", numFree, want)
	}
}

func waitForRowsClose(t *testing.T, rows *Rows) {
	if !waitCondition(t, func() bool {
		rows.closemu.RLock()
		defer rows.closemu.RUnlock()
		return rows.closed
	}) {
		t.Fatal("failed to close rows")
	}
}

// TestQueryContextWait ensures that rows and all internal statements are closed when
// a query context is closed during execution.
func TestQueryContextWait(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	prepares0 := numPrepares(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This will trigger the *fakeConn.Prepare method which will take time
	// performing the query. The ctxDriverPrepare func will check the context
	// after this and close the rows and return an error.
	c, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}

	c.dc.ci.(*fakeConn).waiter = func(c context.Context) {
		cancel()
		<-ctx.Done()
	}
	_, err = c.QueryContext(ctx, "SELECT|people|age,name|")
	c.Close()
	if err != context.Canceled {
		t.Fatalf("expected QueryContext to error with context deadline exceeded but returned %v", err)
	}

	// Verify closed rows connection after error condition.
	waitForFree(t, db_, 1)
	if prepares := numPrepares(t, db_) - prepares0; prepares != 1 {
		t.Fatalf("executed %d Prepare statements; want 1", prepares)
	}
}

// TestTxContextWait tests the transaction behavior when the tx context is canceled
// during execution of the query.
func TestTxContextWait(t *testing.T) {
	testContextWait(t, false)
}

// TestTxContextWaitNoDiscard is the same as TestTxContextWait, but should not discard
// the final connection.
func TestTxContextWaitNoDiscard(t *testing.T) {
	testContextWait(t, true)
}

func testContextWait(t *testing.T, keepConnOnRollback bool) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())

	tx, err := db_.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	tx.keepConnOnRollback = keepConnOnRollback

	tx.dc.ci.(*fakeConn).waiter = func(c context.Context) {
		cancel()
		<-ctx.Done()
	}
	// This will trigger the *fakeConn.Prepare method which will take time
	// performing the query. The ctxDriverPrepare func will check the context
	// after this and close the rows and return an error.
	_, err = tx.QueryContext(ctx, "SELECT|people|age,name|")
	if err != context.Canceled {
		t.Fatalf("expected QueryContext to error with context canceled but returned %v", err)
	}

	if keepConnOnRollback {
		waitForFree(t, db_, 1)
	} else {
		waitForFree(t, db_, 0)
	}
}

// TestUnsupportedOptions checks that the database fails when a driver that
// doesn't implement ConnBeginTx is used with non-default options and an
// un-cancellable context.
func TestUnsupportedOptions(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	_, err := db_.BeginTx(context.Background(), &TxOptions{
		Isolation: LevelSerializable, ReadOnly: true,
	})
	if err == nil {
		t.Fatal("expected error when using unsupported options, got nil")
	}
}

func TestMultiResultSetQuery(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	prepares0 := numPrepares(t, db_)
	rows, err := db_.Query("SELECT|people|age,name|;SELECT|people|name|")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	type row1 struct {
		age  int
		name string
	}
	type row2 struct {
		name string
	}
	got1 := []row1{}
	for rows.Next() {
		var r row1
		err = rows.Scan(&r.age, &r.name)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		got1 = append(got1, r)
	}
	err = rows.Err()
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	want1 := []row1{
		{age: 1, name: "Alice"},
		{age: 2, name: "Bob"},
		{age: 3, name: "Chris"},
	}
	if !reflect.DeepEqual(got1, want1) {
		t.Errorf("mismatch.\n got1: %#v\nwant: %#v", got1, want1)
	}

	if !rows.NextResultSet() {
		t.Errorf("expected another result set")
	}

	got2 := []row2{}
	for rows.Next() {
		var r row2
		err = rows.Scan(&r.name)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		got2 = append(got2, r)
	}
	err = rows.Err()
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	want2 := []row2{
		{name: "Alice"},
		{name: "Bob"},
		{name: "Chris"},
	}
	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("mismatch.\n got: %#v\nwant: %#v", got2, want2)
	}
	if rows.NextResultSet() {
		t.Errorf("expected no more result sets")
	}

	// And verify that the final rows.Next() call, which hit EOF,
	// also closed the rows connection.
	waitForFree(t, db_, 1)
	if prepares := numPrepares(t, db_) - prepares0; prepares != 1 {
		t.Errorf("executed %d Prepare statements; want 1", prepares)
	}
}

func TestQueryNamedArg(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	prepares0 := numPrepares(t, db_)
	rows, err := db_.Query(
		// Ensure the name and age parameters only match on placeholder name, not position.
		"SELECT|people|age,name|name=?name,age=?age",
		Named("age", 2),
		Named("name", "Bob"),
	)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	type row struct {
		age  int
		name string
	}
	got := []row{}
	for rows.Next() {
		var r row
		err = rows.Scan(&r.age, &r.name)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		got = append(got, r)
	}
	err = rows.Err()
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := []row{
		{age: 2, name: "Bob"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mismatch.\n got: %#v\nwant: %#v", got, want)
	}

	// And verify that the final rows.Next() call, which hit EOF,
	// also closed the rows connection.
	if n := db_.numFreeConns(); n != 1 {
		t.Fatalf("free conns after query hitting EOF = %d; want 1", n)
	}
	if prepares := numPrepares(t, db_) - prepares0; prepares != 1 {
		t.Errorf("executed %d Prepare statements; want 1", prepares)
	}
}

func TestPoolExhaustOnCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("long test")
	}

	max := 3
	var saturate, saturateDone sync.WaitGroup
	saturate.Add(max)
	saturateDone.Add(max)

	donePing := make(chan bool)
	state := 0

	// waiter will be called for all queries, including
	// initial setup queries. The state is only assigned when
	// no queries are made.
	//
	// Only allow the first batch of queries to finish once the
	// second batch of Ping queries have finished.
	waiter := func(ctx context.Context) {
		switch state {
		case 0:
			// Nothing. Initial database setup.
		case 1:
			saturate.Done()
			select {
			case <-ctx.Done():
			case <-donePing:
			}
		case 2:
		}
	}
	db_ := newTestDBConnector(t, &fakeConnector{waiter: waiter}, "people")
	defer closeDB(t, db_)

	db_.SetMaxOpenConns(max)

	// First saturate the connection pool.
	// Then start new requests for a connection that is canceled after it is requested.

	state = 1
	for i := 0; i < max; i++ {
		go func() {
			rows, err := db_.Query("SELECT|people|name,photo|")
			if err != nil {
				t.Errorf("Query: %v", err)
				return
			}
			rows.Close()
			saturateDone.Done()
		}()
	}

	saturate.Wait()
	if t.Failed() {
		t.FailNow()
	}
	state = 2

	// Now cancel the request while it is waiting.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < max; i++ {
		ctxReq, cancelReq := context.WithCancel(ctx)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancelReq()
		}()
		err := db_.PingContext(ctxReq)
		if err != context.Canceled {
			t.Fatalf("PingContext (Exhaust): %v", err)
		}
	}
	close(donePing)
	saturateDone.Wait()

	// Now try to open a normal connection.
	err := db_.PingContext(ctx)
	if err != nil {
		t.Fatalf("PingContext (Normal): %v", err)
	}
}

func TestRowsColumns(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	rows, err := db_.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("Columns: %v", err)
	}
	want := []string{"age", "name"}
	if !reflect.DeepEqual(cols, want) {
		t.Errorf("got %#v; want %#v", cols, want)
	}
	if err := rows.Close(); err != nil {
		t.Errorf("error closing rows: %s", err)
	}
}

func TestRowsColumnTypes(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	rows, err := db_.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	tt, err := rows.ColumnTypes()
	if err != nil {
		t.Fatalf("ColumnTypes: %v", err)
	}

	types := make([]reflect.Type, len(tt))
	for i, tp := range tt {
		st := tp.ScanType()
		if st == nil {
			t.Errorf("scantype is null for column %q", tp.Name())
			continue
		}
		types[i] = st
	}
	values := make([]any, len(tt))
	for i := range values {
		values[i] = reflect.New(types[i]).Interface()
	}
	ct := 0
	for rows.Next() {
		err = rows.Scan(values...)
		if err != nil {
			t.Fatalf("failed to scan values in %v", err)
		}
		if ct == 1 {
			if age := *values[0].(*int32); age != 2 {
				t.Errorf("Expected 2, got %v", age)
			}
			if name := *values[1].(*string); name != "Bob" {
				t.Errorf("Expected Bob, got %v", name)
			}
		}
		ct++
	}
	if ct != 3 {
		t.Errorf("expected 3 rows, got %d", ct)
	}

	if err := rows.Close(); err != nil {
		t.Errorf("error closing rows: %s", err)
	}
}

func TestQueryRow(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	var name string
	var age int
	var birthday time.Time

	err := db_.QueryRow("SELECT|people|age,name|age=?", 3).Scan(&age)
	if err == nil || !strings.Contains(err.Error(), "expected 2 destination arguments") {
		t.Errorf("expected error from wrong number of arguments; actually got: %v", err)
	}

	err = db_.QueryRow("SELECT|people|bdate|age=?", 3).Scan(&birthday)
	if err != nil || !birthday.Equal(chrisBirthday) {
		t.Errorf("chris birthday = %v, err = %v; want %v", birthday, err, chrisBirthday)
	}

	err = db_.QueryRow("SELECT|people|age,name|age=?", 2).Scan(&age, &name)
	if err != nil {
		t.Fatalf("age QueryRow+Scan: %v", err)
	}
	if name != "Bob" {
		t.Errorf("expected name Bob, got %q", name)
	}
	if age != 2 {
		t.Errorf("expected age 2, got %d", age)
	}

	err = db_.QueryRow("SELECT|people|age,name|name=?", "Alice").Scan(&age, &name)
	if err != nil {
		t.Fatalf("name QueryRow+Scan: %v", err)
	}
	if name != "Alice" {
		t.Errorf("expected name Alice, got %q", name)
	}
	if age != 1 {
		t.Errorf("expected age 1, got %d", age)
	}

	var photo []byte
	err = db_.QueryRow("SELECT|people|photo|name=?", "Alice").Scan(&photo)
	if err != nil {
		t.Fatalf("photo QueryRow+Scan: %v", err)
	}
	want := []byte("APHOTO")
	if !reflect.DeepEqual(photo, want) {
		t.Errorf("photo = %q; want %q", photo, want)
	}
}

func TestRowErr(t *testing.T) {
	db_ := newTestDB(t, "people")

	err := db_.QueryRowContext(context.Background(), "SELECT|people|bdate|age=?", 3).Err()
	if err != nil {
		t.Errorf("Unexpected err = %v; want %v", err, nil)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = db_.QueryRowContext(ctx, "SELECT|people|bdate|age=?", 3).Err()
	exp := "context canceled"
	if err == nil || !strings.Contains(err.Error(), exp) {
		t.Errorf("Expected err = %v; got %v", exp, err)
	}
}

func TestTxRollbackCommitErr(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	err = tx.Rollback()
	if err != nil {
		t.Errorf("expected nil error from Rollback; got %v", err)
	}
	err = tx.Commit()
	if err != ErrTxDone {
		t.Errorf("expected %q from Commit; got %q", ErrTxDone, err)
	}

	tx, err = db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		t.Errorf("expected nil error from Commit; got %v", err)
	}
	err = tx.Rollback()
	if err != ErrTxDone {
		t.Errorf("expected %q from Rollback; got %q", ErrTxDone, err)
	}
}

func TestStatementErrorAfterClose(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	stmt_, err := db_.Prepare("SELECT|people|age|name=?")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	err = stmt_.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	var name string
	err = stmt_.QueryRow("foo").Scan(&name)
	if err == nil {
		t.Errorf("expected error from QueryRow.Scan after Stmt.Close")
	}
}

func TestStatementQueryRow(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	stmt_, err := db_.Prepare("SELECT|people|age|name=?")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer stmt_.Close()
	var age int
	for n, tt := range []struct {
		name string
		want int
	}{
		{"Alice", 1},
		{"Bob", 2},
		{"Chris", 3},
	} {
		if err := stmt_.QueryRow(tt.name).Scan(&age); err != nil {
			t.Errorf("%d: on %q, QueryRow/Scan: %v", n, tt.name, err)
		} else if age != tt.want {
			t.Errorf("%d: age=%d, want %d", n, age, tt.want)
		}
	}
}

type stubDriverStmt struct {
	err error
}

func (s stubDriverStmt) Close() error {
	return s.err
}

func (s stubDriverStmt) NumInput() int {
	return -1
}

func (s stubDriverStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, nil
}

func (s stubDriverStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, nil
}

// golang.org/issue/12798
func TestStatementClose(t *testing.T) {
	want := errors.New("STMT ERROR")

	tests := []struct {
		stmt_ *stmt
		msg   string
	}{
		{&stmt{stickyErr: want}, "stickyErr not propagated"},
		{&stmt{cg: &Tx{}, cgds: &driverStmt{Locker: &sync.Mutex{}, si: stubDriverStmt{want}}}, "driverStmt.Close() error not propagated"},
	}
	for _, test := range tests {
		if err := test.stmt_.Close(); err != want {
			t.Errorf("%s. Got stmt_.Close() = %v, want = %v", test.msg, err, want)
		}
	}
}

// golang.org/issue/3734
func TestStatementQueryRowConcurrent(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	stmt_, err := db_.Prepare("SELECT|people|age|name=?")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer stmt_.Close()

	const n = 10
	ch := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			var age int
			err := stmt_.QueryRow("Alice").Scan(&age)
			if err == nil && age != 1 {
				err = fmt.Errorf("unexpected age %d", age)
			}
			ch <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-ch; err != nil {
			t.Error(err)
		}
	}
}

// just a test of fakedb itself
func TestBogusPreboundParameters(t *testing.T) {
	db_ := newTestDB(t, "foo")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")
	_, err := db_.Prepare("INSERT|t1|name=?,age=bogusconversion")
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != `fakedb: invalid conversion to int32 from "bogusconversion"` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExec(t *testing.T) {
	db_ := newTestDB(t, "foo")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")
	stmt_, err := db_.Prepare("INSERT|t1|name=?,age=?")
	if err != nil {
		t.Errorf("Stmt, err = %v, %v", stmt_, err)
	}
	defer stmt_.Close()

	type execTest struct {
		args    []any
		wantErr string
	}
	execTests := []execTest{
		// Okay:
		{[]any{"Brad", 31}, ""},
		{[]any{"Brad", int64(31)}, ""},
		{[]any{"Bob", "32"}, ""},
		{[]any{7, 9}, ""},

		// Invalid conversions:
		{[]any{"Brad", int64(0xFFFFFFFF)}, "sql: converting argument $2 type: sql/driver: value 4294967295 overflows int32"},
		{[]any{"Brad", "strconv fail"}, `sql: converting argument $2 type: sql/driver: value "strconv fail" can't be converted to int32`},

		// Wrong number of args:
		{[]any{}, "sql: expected 2 arguments, got 0"},
		{[]any{1, 2, 3}, "sql: expected 2 arguments, got 3"},
	}
	for n, et := range execTests {
		_, err := stmt_.Exec(et.args...)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		if errStr != et.wantErr {
			t.Errorf("stmt_.Execute #%d: for %v, got error %q, want error %q",
				n, et.args, errStr, et.wantErr)
		}
	}
}

func TestTxPrepare(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")
	tx, err := db_.Begin()
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	stmt_, err := tx.Prepare("INSERT|t1|name=?,age=?")
	if err != nil {
		t.Fatalf("Stmt, err = %v, %v", stmt_, err)
	}
	defer stmt_.Close()
	_, err = stmt_.Exec("Bobby", 7)
	if err != nil {
		t.Fatalf("Exec = %v", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit = %v", err)
	}
	// Commit() should have closed the statement
	if !stmt_.(*stmt).closed {
		t.Fatal("Stmt not closed after Commit")
	}
}

func TestTxStmt(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")
	stmt_, err := db_.Prepare("INSERT|t1|name=?,age=?")
	if err != nil {
		t.Fatalf("Stmt, err = %v, %v", stmt_, err)
	}
	defer stmt_.Close()
	tx, err := db_.Begin()
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	txs := tx.Stmt(stmt_).(*stmt)
	defer txs.Close()
	_, err = txs.Exec("Bobby", 7)
	if err != nil {
		t.Fatalf("Exec = %v", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit = %v", err)
	}
	// Commit() should have closed the statement
	if !txs.closed {
		t.Fatal("Stmt not closed after Commit")
	}
}

func TestTxStmtPreparedOnce(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32")

	prepares0 := numPrepares(t, db_)

	// db_.Prepare increments numPrepares.
	stmt_, err := db_.Prepare("INSERT|t1|name=?,age=?")
	if err != nil {
		t.Fatalf("Stmt, err = %v, %v", stmt_, err)
	}
	defer stmt_.Close()

	tx, err := db_.Begin()
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}

	txs1 := tx.Stmt(stmt_).(*stmt)
	txs2 := tx.Stmt(stmt_).(*stmt)

	_, err = txs1.Exec("Go", 7)
	if err != nil {
		t.Fatalf("Exec = %v", err)
	}
	txs1.Close()

	_, err = txs2.Exec("Gopher", 8)
	if err != nil {
		t.Fatalf("Exec = %v", err)
	}
	txs2.Close()

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit = %v", err)
	}

	if prepares := numPrepares(t, db_) - prepares0; prepares != 1 {
		t.Errorf("executed %d Prepare statements; want 1", prepares)
	}
}

func TestTxStmtClosedRePrepares(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32")

	prepares0 := numPrepares(t, db_)

	// db_.Prepare increments numPrepares.
	stmt_, err := db_.Prepare("INSERT|t1|name=?,age=?")
	if err != nil {
		t.Fatalf("Stmt, err = %v, %v", stmt_, err)
	}
	tx, err := db_.Begin()
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	err = stmt_.Close()
	if err != nil {
		t.Fatalf("stmt_.Close() = %v", err)
	}
	// tx.Stmt increments numPrepares because stmt_ is closed.
	txs := tx.Stmt(stmt_).(*stmt)
	if txs.stickyErr != nil {
		t.Fatal(txs.stickyErr)
	}
	if txs.parentStmt != nil {
		t.Fatal("expected nil parentStmt")
	}
	_, err = txs.Exec(`Eric`, 82)
	if err != nil {
		t.Fatalf("txs.Exec = %v", err)
	}

	err = txs.Close()
	if err != nil {
		t.Fatalf("txs.Close = %v", err)
	}

	tx.Rollback()

	if prepares := numPrepares(t, db_) - prepares0; prepares != 2 {
		t.Errorf("executed %d Prepare statements; want 2", prepares)
	}
}

func TestParentStmtOutlivesTxStmt(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32")

	// Make sure everything happens on the same connection.
	db_.SetMaxOpenConns(1)

	prepares0 := numPrepares(t, db_)

	// db_.Prepare increments numPrepares.
	stmt1, err := db_.Prepare("INSERT|t1|name=?,age=?")
	stmt_ := stmt1.(*stmt)
	if err != nil {
		t.Fatalf("Stmt, err = %v, %v", stmt_, err)
	}
	defer stmt_.Close()
	tx, err := db_.Begin()
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	txs := tx.Stmt(stmt_).(*stmt)
	if len(stmt_.css) != 1 {
		t.Fatalf("len(stmt_.css) = %v; want 1", len(stmt_.css))
	}
	err = txs.Close()
	if err != nil {
		t.Fatalf("txs.Close() = %v", err)
	}
	err = tx.Rollback()
	if err != nil {
		t.Fatalf("tx.Rollback() = %v", err)
	}
	// txs must not be valid.
	_, err = txs.Exec("Suzan", 30)
	if err == nil {
		t.Fatalf("txs.Exec(), expected err")
	}
	// Stmt must still be valid.
	_, err = stmt_.Exec("Janina", 25)
	if err != nil {
		t.Fatalf("stmt_.Exec() = %v", err)
	}

	if prepares := numPrepares(t, db_) - prepares0; prepares != 1 {
		t.Errorf("executed %d Prepare statements; want 1", prepares)
	}
}

// Test that tx.Stmt called with a statement already
// associated with tx as argument re-prepares the same
// statement again.
func TestTxStmtFromTxStmtRePrepares(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32")
	prepares0 := numPrepares(t, db_)
	// db_.Prepare increments numPrepares.
	stmt_, err := db_.Prepare("INSERT|t1|name=?,age=?")
	if err != nil {
		t.Fatalf("Stmt, err = %v, %v", stmt_, err)
	}
	defer stmt_.Close()

	tx, err := db_.Begin()
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	txs1 := tx.Stmt(stmt_).(*stmt)

	// tx.Stmt(txs1) increments numPrepares because txs1 already
	// belongs to a transaction (albeit the same transaction).
	txs2 := tx.Stmt(txs1).(*stmt)
	if txs2.stickyErr != nil {
		t.Fatal(txs2.stickyErr)
	}
	if txs2.parentStmt != nil {
		t.Fatal("expected nil parentStmt")
	}
	_, err = txs2.Exec(`Eric`, 82)
	if err != nil {
		t.Fatal(err)
	}

	err = txs1.Close()
	if err != nil {
		t.Fatalf("txs1.Close = %v", err)
	}
	err = txs2.Close()
	if err != nil {
		t.Fatalf("txs1.Close = %v", err)
	}
	err = tx.Rollback()
	if err != nil {
		t.Fatalf("tx.Rollback = %v", err)
	}

	if prepares := numPrepares(t, db_) - prepares0; prepares != 2 {
		t.Errorf("executed %d Prepare statements; want 2", prepares)
	}
}

// Issue: https://golang.org/issue/2784
// This test didn't fail before because we got lucky with the fakedb driver.
// It was failing, and now not, in github.com/bradfitz/go-sql-test
func TestTxQuery(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")
	exec(t, db_, "INSERT|t1|name=Alice")

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	r, err := tx.Query("SELECT|t1|name|")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if !r.Next() {
		if r.Err() != nil {
			t.Fatal(r.Err())
		}
		t.Fatal("expected one row")
	}

	var x string
	err = r.Scan(&x)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTxQueryInvalid(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	_, err = tx.Query("SELECT|t1|name|")
	if err == nil {
		t.Fatal("Error expected")
	}
}

// Tests fix for issue 4433, that retries in Begin happen when
// conn.Begin() returns ErrBadConn
func TestTxErrBadConn(t *testing.T) {
	db1, err := Open("test", fakeDBName+";badConn")
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db_.Exec("WIPE"); err != nil {
		t.Fatalf("exec wipe: %v", err)
	}
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")
	stmt_, err := db_.Prepare("INSERT|t1|name=?,age=?")
	if err != nil {
		t.Fatalf("Stmt, err = %v, %v", stmt_, err)
	}
	defer stmt_.Close()
	tx, err := db_.Begin()
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	txs := tx.Stmt(stmt_).(*stmt)
	defer txs.Close()
	_, err = txs.Exec("Bobby", 7)
	if err != nil {
		t.Fatalf("Exec = %v", err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit = %v", err)
	}
}

func TestConnQuery(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn.dc.ci.(*fakeConn).skipDirtySession = true
	defer conn.Close()

	var name string
	err = conn.QueryRowContext(ctx, "SELECT|people|name|age=?", 3).Scan(&name)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Chris" {
		t.Fatalf("unexpected result, got %q want Chris", name)
	}

	err = conn.PingContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnRaw(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn.dc.ci.(*fakeConn).skipDirtySession = true
	defer conn.Close()

	sawFunc := false
	err = conn.Raw(func(dc any) error {
		sawFunc = true
		if _, ok := dc.(*fakeConn); !ok {
			return fmt.Errorf("got %T want *fakeConn", dc)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawFunc {
		t.Fatal("Raw func not called")
	}

	func() {
		defer func() {
			x := recover()
			if x == nil {
				t.Fatal("expected panic")
			}
			conn.closemu.Lock()
			closed := conn.dc == nil
			conn.closemu.Unlock()
			if !closed {
				t.Fatal("expected connection to be closed after panic")
			}
		}()
		err = conn.Raw(func(dc any) error {
			panic("Conn.Raw panic should return an error")
		})
		t.Fatal("expected panic from Raw func")
	}()
}

func TestCursorFake(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	exec(t, db_, "CREATE|peoplecursor|list=table")
	exec(t, db_, "INSERT|peoplecursor|list=people!name!age")

	rows, err := db_.QueryContext(ctx, `SELECT|peoplecursor|list|`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("no rows")
	}
	var cursor = &Rows{}
	err = rows.Scan(cursor)
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close()

	const expectedRows = 3
	var currentRow int64

	var n int64
	var s string
	for cursor.Next() {
		currentRow++
		err = cursor.Scan(&s, &n)
		if err != nil {
			t.Fatal(err)
		}
		if n != currentRow {
			t.Errorf("expected number(Age)=%d, got %d", currentRow, n)
		}
	}
	if currentRow != expectedRows {
		t.Errorf("expected %d rows, got %d rows", expectedRows, currentRow)
	}
}

func TestInvalidNilValues(t *testing.T) {
	var date1 time.Time
	var date2 int

	tests := []struct {
		name          string
		input         any
		expectedError string
	}{
		{
			name:          "time.Time",
			input:         &date1,
			expectedError: `sql: Scan error on column index 0, name "bdate": unsupported Scan, storing driver.Value type <nil> into type *time.Time`,
		},
		{
			name:          "int",
			input:         &date2,
			expectedError: `sql: Scan error on column index 0, name "bdate": converting NULL to int is unsupported`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db_ := newTestDB(t, "people")
			defer closeDB(t, db_)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			conn, err := db_.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			conn.dc.ci.(*fakeConn).skipDirtySession = true
			defer conn.Close()

			err = conn.QueryRowContext(ctx, "SELECT|people|bdate|age=?", 1).Scan(tt.input)
			if err == nil {
				t.Fatal("expected error when querying nil column, but succeeded")
			}
			if err.Error() != tt.expectedError {
				t.Fatalf("Expected error: %s\nReceived: %s", tt.expectedError, err.Error())
			}

			err = conn.PingContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConnTx(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn.dc.ci.(*fakeConn).skipDirtySession = true
	defer conn.Close()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	insertName, insertAge := "Nancy", 33
	_, err = tx.ExecContext(ctx, "INSERT|people|name=?,age=?,photo=APHOTO", insertName, insertAge)
	if err != nil {
		t.Fatal(err)
	}
	err = tx.Commit()
	if err != nil {
		t.Fatal(err)
	}

	var selectName string
	err = conn.QueryRowContext(ctx, "SELECT|people|name|age=?", insertAge).Scan(&selectName)
	if err != nil {
		t.Fatal(err)
	}
	if selectName != insertName {
		t.Fatalf("got %q want %q", selectName, insertName)
	}
}

// TestConnIsValid verifies that a database connection that should be discarded,
// is actually discarded and does not re-enter the connection pool.
// If the IsValid method from *fakeConn is removed, this test will fail.
func TestConnIsValid(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	db_.SetMaxOpenConns(1)

	ctx := context.Background()

	c, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Raw(func(raw any) error {
		dc := raw.(*fakeConn)
		dc.stickyBad = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	if len(db_.freeConn) > 0 && db_.freeConn[0].ci.(*fakeConn).stickyBad {
		t.Fatal("bad connection returned to pool; expected bad connection to be discarded")
	}
}

// Tests fix for issue 2542, that we release a lock when querying on
// a closed connection.
func TestIssue2542Deadlock(t *testing.T) {
	db_ := newTestDB(t, "people")
	closeDB(t, db_)
	for i := 0; i < 2; i++ {
		_, err := db_.Query("SELECT|people|age,name|")
		if err == nil {
			t.Fatalf("expected error")
		}
	}
}

// From golang.org/issue/3865
func TestCloseStmtBeforeRows(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	s, err := db_.Prepare("SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}

	r, err := s.Query()
	if err != nil {
		s.Close()
		t.Fatal(err)
	}

	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}

	r.Close()
}

// Tests fix for issue 2788, that we bind nil to a []byte if the
// value in the column is sql null
func TestNullByteSlice(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t|id=int32,name=nullstring")
	exec(t, db_, "INSERT|t|id=10,name=?", nil)

	var name []byte

	err := db_.QueryRow("SELECT|t|name|id=?", 10).Scan(&name)
	if err != nil {
		t.Fatal(err)
	}
	if name != nil {
		t.Fatalf("name []byte should be nil for null column value, got: %#v", name)
	}

	exec(t, db_, "INSERT|t|id=11,name=?", "bob")
	err = db_.QueryRow("SELECT|t|name|id=?", 11).Scan(&name)
	if err != nil {
		t.Fatal(err)
	}
	if string(name) != "bob" {
		t.Fatalf("name []byte should be bob, got: %q", string(name))
	}
}

func TestPointerParamsAndScans(t *testing.T) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t|id=int32,name=nullstring")

	bob := "bob"
	var name *string

	name = &bob
	exec(t, db_, "INSERT|t|id=10,name=?", name)
	name = nil
	exec(t, db_, "INSERT|t|id=20,name=?", name)

	err := db_.QueryRow("SELECT|t|name|id=?", 10).Scan(&name)
	if err != nil {
		t.Fatalf("querying id 10: %v", err)
	}
	if name == nil {
		t.Errorf("id 10's name = nil; want bob")
	} else if *name != "bob" {
		t.Errorf("id 10's name = %q; want bob", *name)
	}

	err = db_.QueryRow("SELECT|t|name|id=?", 20).Scan(&name)
	if err != nil {
		t.Fatalf("querying id 20: %v", err)
	}
	if name != nil {
		t.Errorf("id 20 = %q; want nil", *name)
	}
}

func TestQueryRowClosingStmt(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	var name string
	var age int
	err := db_.QueryRow("SELECT|people|age,name|age=?", 3).Scan(&age, &name)
	if err != nil {
		t.Fatal(err)
	}
	if len(db_.freeConn) != 1 {
		t.Fatalf("expected 1 free conn")
	}
	fakeConn := db_.freeConn[0].ci.(*fakeConn)
	if made, closed := fakeConn.stmtsMade, fakeConn.stmtsClosed; made != closed {
		t.Errorf("statement close mismatch: made %d, closed %d", made, closed)
	}
}

var atomicRowsCloseHook atomic.Value // of func(*Rows, *error)

func init() {
	rowsCloseHook = func() func(*Rows, *error) {
		fn, _ := atomicRowsCloseHook.Load().(func(*Rows, *error))
		return fn
	}
}

func setRowsCloseHook(fn func(*Rows, *error)) {
	if fn == nil {
		// Can't change an atomic.Value back to nil, so set it to this
		// no-op func instead.
		fn = func(*Rows, *error) {}
	}
	atomicRowsCloseHook.Store(fn)
}

// Test issue 6651
func TestIssue6651(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	var v string

	want := "error in rows.Next"
	rowsCursorNextHook = func(dest []driver.Value) error {
		return fmt.Errorf(want)
	}
	defer func() { rowsCursorNextHook = nil }()

	err := db_.QueryRow("SELECT|people|name|").Scan(&v)
	if err == nil || err.Error() != want {
		t.Errorf("error = %q; want %q", err, want)
	}
	rowsCursorNextHook = nil

	want = "error in rows.Close"
	setRowsCloseHook(func(rows *Rows, err *error) {
		*err = fmt.Errorf(want)
	})
	defer setRowsCloseHook(nil)
	err = db_.QueryRow("SELECT|people|name|").Scan(&v)
	if err == nil || err.Error() != want {
		t.Errorf("error = %q; want %q", err, want)
	}
}

type nullTestRow struct {
	nullParam    any
	notNullParam any
	scanNullVal  any
}

type nullTestSpec struct {
	nullType    string
	notNullType string
	rows        [6]nullTestRow
}

func TestNullStringParam(t *testing.T) {
	spec := nullTestSpec{"nullstring", "string", [6]nullTestRow{
		{NullString{"aqua", true}, "", NullString{"aqua", true}},
		{NullString{"brown", false}, "", NullString{"", false}},
		{"chartreuse", "", NullString{"chartreuse", true}},
		{NullString{"darkred", true}, "", NullString{"darkred", true}},
		{NullString{"eel", false}, "", NullString{"", false}},
		{"foo", NullString{"black", false}, nil},
	}}
	nullTestRun(t, spec)
}

func TestNullInt64Param(t *testing.T) {
	spec := nullTestSpec{"nullint64", "int64", [6]nullTestRow{
		{NullInt64{31, true}, 1, NullInt64{31, true}},
		{NullInt64{-22, false}, 1, NullInt64{0, false}},
		{22, 1, NullInt64{22, true}},
		{NullInt64{33, true}, 1, NullInt64{33, true}},
		{NullInt64{222, false}, 1, NullInt64{0, false}},
		{0, NullInt64{31, false}, nil},
	}}
	nullTestRun(t, spec)
}

func TestNullInt32Param(t *testing.T) {
	spec := nullTestSpec{"nullint32", "int32", [6]nullTestRow{
		{NullInt32{31, true}, 1, NullInt32{31, true}},
		{NullInt32{-22, false}, 1, NullInt32{0, false}},
		{22, 1, NullInt32{22, true}},
		{NullInt32{33, true}, 1, NullInt32{33, true}},
		{NullInt32{222, false}, 1, NullInt32{0, false}},
		{0, NullInt32{31, false}, nil},
	}}
	nullTestRun(t, spec)
}

func TestNullInt16Param(t *testing.T) {
	spec := nullTestSpec{"nullint16", "int16", [6]nullTestRow{
		{NullInt16{31, true}, 1, NullInt16{31, true}},
		{NullInt16{-22, false}, 1, NullInt16{0, false}},
		{22, 1, NullInt16{22, true}},
		{NullInt16{33, true}, 1, NullInt16{33, true}},
		{NullInt16{222, false}, 1, NullInt16{0, false}},
		{0, NullInt16{31, false}, nil},
	}}
	nullTestRun(t, spec)
}

func TestNullByteParam(t *testing.T) {
	spec := nullTestSpec{"nullbyte", "byte", [6]nullTestRow{
		{NullByte{31, true}, 1, NullByte{31, true}},
		{NullByte{0, false}, 1, NullByte{0, false}},
		{22, 1, NullByte{22, true}},
		{NullByte{33, true}, 1, NullByte{33, true}},
		{NullByte{222, false}, 1, NullByte{0, false}},
		{0, NullByte{31, false}, nil},
	}}
	nullTestRun(t, spec)
}

func TestNullFloat64Param(t *testing.T) {
	spec := nullTestSpec{"nullfloat64", "float64", [6]nullTestRow{
		{NullFloat64{31.2, true}, 1, NullFloat64{31.2, true}},
		{NullFloat64{13.1, false}, 1, NullFloat64{0, false}},
		{-22.9, 1, NullFloat64{-22.9, true}},
		{NullFloat64{33.81, true}, 1, NullFloat64{33.81, true}},
		{NullFloat64{222, false}, 1, NullFloat64{0, false}},
		{10, NullFloat64{31.2, false}, nil},
	}}
	nullTestRun(t, spec)
}

func TestNullBoolParam(t *testing.T) {
	spec := nullTestSpec{"nullbool", "bool", [6]nullTestRow{
		{NullBool{false, true}, true, NullBool{false, true}},
		{NullBool{true, false}, false, NullBool{false, false}},
		{true, true, NullBool{true, true}},
		{NullBool{true, true}, false, NullBool{true, true}},
		{NullBool{true, false}, true, NullBool{false, false}},
		{true, NullBool{true, false}, nil},
	}}
	nullTestRun(t, spec)
}

func TestNullTimeParam(t *testing.T) {
	t0 := time.Time{}
	t1 := time.Date(2000, 1, 1, 8, 9, 10, 11, time.UTC)
	t2 := time.Date(2010, 1, 1, 8, 9, 10, 11, time.UTC)
	spec := nullTestSpec{"nulldatetime", "datetime", [6]nullTestRow{
		{NullTime{t1, true}, t2, NullTime{t1, true}},
		{NullTime{t1, false}, t2, NullTime{t0, false}},
		{t1, t2, NullTime{t1, true}},
		{NullTime{t1, true}, t2, NullTime{t1, true}},
		{NullTime{t1, false}, t2, NullTime{t0, false}},
		{t2, NullTime{t1, false}, nil},
	}}
	nullTestRun(t, spec)
}

func nullTestRun(t *testing.T, spec nullTestSpec) {
	db_ := newTestDB(t, "")
	defer closeDB(t, db_)
	exec(t, db_, fmt.Sprintf("CREATE|t|id=int32,name=string,nullf=%s,notnullf=%s", spec.nullType, spec.notNullType))

	// Inserts with db_.Exec:
	exec(t, db_, "INSERT|t|id=?,name=?,nullf=?,notnullf=?", 1, "alice", spec.rows[0].nullParam, spec.rows[0].notNullParam)
	exec(t, db_, "INSERT|t|id=?,name=?,nullf=?,notnullf=?", 2, "bob", spec.rows[1].nullParam, spec.rows[1].notNullParam)

	// Inserts with a prepared statement:
	stmt_, err := db_.Prepare("INSERT|t|id=?,name=?,nullf=?,notnullf=?")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt_.Close()
	if _, err := stmt_.Exec(3, "chris", spec.rows[2].nullParam, spec.rows[2].notNullParam); err != nil {
		t.Errorf("exec insert chris: %v", err)
	}
	if _, err := stmt_.Exec(4, "dave", spec.rows[3].nullParam, spec.rows[3].notNullParam); err != nil {
		t.Errorf("exec insert dave: %v", err)
	}
	if _, err := stmt_.Exec(5, "eleanor", spec.rows[4].nullParam, spec.rows[4].notNullParam); err != nil {
		t.Errorf("exec insert eleanor: %v", err)
	}

	// Can't put null val into non-null col
	if _, err := stmt_.Exec(6, "bob", spec.rows[5].nullParam, spec.rows[5].notNullParam); err == nil {
		t.Errorf("expected error inserting nil val with prepared statement Exec")
	}

	_, err = db_.Exec("INSERT|t|id=?,name=?,nullf=?", 999, nil, nil)
	if err == nil {
		// TODO: this test fails, but it's just because
		// fakeConn implements the optional Execer interface,
		// so arguably this is the correct behavior. But
		// maybe I should flesh out the fakeConn.Exec
		// implementation so this properly fails.
		// t.Errorf("expected error inserting nil name with Exec")
	}

	paramtype := reflect.TypeOf(spec.rows[0].nullParam)
	bindVal := reflect.New(paramtype).Interface()

	for i := 0; i < 5; i++ {
		id := i + 1
		if err := db_.QueryRow("SELECT|t|nullf|id=?", id).Scan(bindVal); err != nil {
			t.Errorf("id=%d Scan: %v", id, err)
		}
		bindValDeref := reflect.ValueOf(bindVal).Elem().Interface()
		if !reflect.DeepEqual(bindValDeref, spec.rows[i].scanNullVal) {
			t.Errorf("id=%d got %#v, want %#v", id, bindValDeref, spec.rows[i].scanNullVal)
		}
	}
}

// golang.org/issue/4859
func TestQueryRowNilScanDest(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	var name *string // nil pointer
	err := db_.QueryRow("SELECT|people|name|").Scan(name)
	want := `sql: Scan error on column index 0, name "name": destination pointer is nil`
	if err == nil || err.Error() != want {
		t.Errorf("error = %q; want %q", err.Error(), want)
	}
}

func TestIssue4902(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	driver := db_.Driver().(*fakeDriver)
	opens0 := driver.openCount

	var stmt_ *stmt

	for i := 0; i < 10; i++ {
		stmt1, err := db_.Prepare("SELECT|people|name|")
		stmt_ = stmt1.(*stmt)
		if err != nil {
			t.Fatal(err)
		}
		err = stmt_.Close()
		if err != nil {
			t.Fatal(err)
		}
	}

	opens := driver.openCount - opens0
	if opens > 1 {
		t.Errorf("opens = %d; want <= 1", opens)
		t.Logf("db_ = %#v", db_)
		t.Logf("driver = %#v", driver)
		t.Logf("stmt_ = %#v", stmt_)
	}
}

// Issue 3857
// This used to deadlock.
func TestSimultaneousQueries(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	r1, err := tx.Query("SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Close()

	r2, err := tx.Query("SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
}

func TestMaxIdleConns(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	if got := len(db_.freeConn); got != 1 {
		t.Errorf("freeConns = %d; want 1", got)
	}

	db_.SetMaxIdleConns(0)

	if got := len(db_.freeConn); got != 0 {
		t.Errorf("freeConns after set to zero = %d; want 0", got)
	}

	tx, err = db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	if got := len(db_.freeConn); got != 0 {
		t.Errorf("freeConns = %d; want 0", got)
	}
}

func TestMaxOpenConns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	defer setHookpostCloseConn(nil)
	setHookpostCloseConn(func(_ *fakeConn, err error) {
		if err != nil {
			t.Errorf("Error closing fakeConn: %v", err)
		}
	})

	db_ := newTestDB(t, "magicquery")
	defer closeDB(t, db_)

	driver := db_.Driver().(*fakeDriver)

	// Force the number of open connections to 0 so we can get an accurate
	// count for the test
	db_.clearAllConns(t)

	driver.mu.Lock()
	opens0 := driver.openCount
	closes0 := driver.closeCount
	driver.mu.Unlock()

	db_.SetMaxIdleConns(10)
	db_.SetMaxOpenConns(10)

	stmt_, err := db_.Prepare("SELECT|magicquery|op|op=?,millis=?")
	if err != nil {
		t.Fatal(err)
	}

	// Start 50 parallel slow queries.
	const (
		nquery      = 50
		sleepMillis = 25
		nbatch      = 2
	)
	var wg sync.WaitGroup
	for batch := 0; batch < nbatch; batch++ {
		for i := 0; i < nquery; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var op string
				if err := stmt_.QueryRow("sleep", sleepMillis).Scan(&op); err != nil && err != ErrNoRows {
					t.Error(err)
				}
			}()
		}
		// Wait for the batch of queries above to finish before starting the next round.
		wg.Wait()
	}

	if g, w := db_.numFreeConns(), 10; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	if n := db_.numDepsPoll(t, 20); n > 20 {
		t.Errorf("number of dependencies = %d; expected <= 20", n)
		db_.dumpDeps(t)
	}

	driver.mu.Lock()
	opens := driver.openCount - opens0
	closes := driver.closeCount - closes0
	driver.mu.Unlock()

	if opens > 10 {
		t.Logf("open calls = %d", opens)
		t.Logf("close calls = %d", closes)
		t.Errorf("_DB connections opened = %d; want <= 10", opens)
		db_.dumpDeps(t)
	}

	if err := stmt_.Close(); err != nil {
		t.Fatal(err)
	}

	if g, w := db_.numFreeConns(), 10; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	if n := db_.numDepsPoll(t, 10); n > 10 {
		t.Errorf("number of dependencies = %d; expected <= 10", n)
		db_.dumpDeps(t)
	}

	db_.SetMaxOpenConns(5)

	if g, w := db_.numFreeConns(), 5; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	if n := db_.numDepsPoll(t, 5); n > 5 {
		t.Errorf("number of dependencies = %d; expected 0", n)
		db_.dumpDeps(t)
	}

	db_.SetMaxOpenConns(0)

	if g, w := db_.numFreeConns(), 5; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	if n := db_.numDepsPoll(t, 5); n > 5 {
		t.Errorf("number of dependencies = %d; expected 0", n)
		db_.dumpDeps(t)
	}

	db_.clearAllConns(t)
}

// Issue 9453: tests that SetMaxOpenConns can be lowered at runtime
// and affects the subsequent release of connections.
func TestMaxOpenConnsOnBusy(t *testing.T) {
	defer setHookpostCloseConn(nil)
	setHookpostCloseConn(func(_ *fakeConn, err error) {
		if err != nil {
			t.Errorf("Error closing fakeConn: %v", err)
		}
	})

	db_ := newTestDB(t, "magicquery")
	defer closeDB(t, db_)

	db_.SetMaxOpenConns(3)

	ctx := context.Background()

	conn0, err := db_.conn(ctx, cachedOrNewConn)
	if err != nil {
		t.Fatalf("_DB open conn fail: %v", err)
	}

	conn1, err := db_.conn(ctx, cachedOrNewConn)
	if err != nil {
		t.Fatalf("_DB open conn fail: %v", err)
	}

	conn2, err := db_.conn(ctx, cachedOrNewConn)
	if err != nil {
		t.Fatalf("_DB open conn fail: %v", err)
	}

	if g, w := db_.numOpen, 3; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	db_.SetMaxOpenConns(2)
	if g, w := db_.numOpen, 3; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	conn0.releaseConn(nil)
	conn1.releaseConn(nil)
	if g, w := db_.numOpen, 2; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	conn2.releaseConn(nil)
	if g, w := db_.numOpen, 2; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}
}

// Issue 10886: tests that all connection attempts return when more than
// db_.maxOpen connections are in flight and the first db_.maxOpen fail.
func TestPendingConnsAfterErr(t *testing.T) {
	const (
		maxOpen = 2
		tryOpen = maxOpen*2 + 2
	)

	// No queries will be run.
	db1, err := Open("test", fakeDBName)
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeDB(t, db_)
	defer func() {
		for k, v := range db_.lastPut {
			t.Logf("%p: %v", k, v)
		}
	}()

	db_.SetMaxOpenConns(maxOpen)
	db_.SetMaxIdleConns(0)

	errOffline := errors.New("_DB offline")

	defer func() { setHookOpenErr(nil) }()

	errs := make(chan error, tryOpen)

	var opening sync.WaitGroup
	opening.Add(tryOpen)

	setHookOpenErr(func() error {
		// Wait for all connections to enqueue.
		opening.Wait()
		return errOffline
	})

	for i := 0; i < tryOpen; i++ {
		go func() {
			opening.Done() // signal one connection is in flight
			_, err := db_.Exec("will never run")
			errs <- err
		}()
	}

	opening.Wait() // wait for all workers to begin running

	const timeout = 5 * time.Second
	to := time.NewTimer(timeout)
	defer to.Stop()

	// check that all connections fail without deadlock
	for i := 0; i < tryOpen; i++ {
		select {
		case err := <-errs:
			if got, want := err, errOffline; got != want {
				t.Errorf("unexpected err: got %v, want %v", got, want)
			}
		case <-to.C:
			t.Fatalf("orphaned connection request(s), still waiting after %v", timeout)
		}
	}

	// Wait a reasonable time for the database to close all connections.
	tick := time.NewTicker(3 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			db_.mu.Lock()
			if db_.numOpen == 0 {
				db_.mu.Unlock()
				return
			}
			db_.mu.Unlock()
		case <-to.C:
			// Closing the database will check for numOpen and fail the test.
			return
		}
	}
}

func TestSingleOpenConn(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	db_.SetMaxOpenConns(1)

	rows, err := db_.Query("SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	// shouldn't deadlock
	rows, err = db_.Query("SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStats(t *testing.T) {
	db_ := newTestDB(t, "people")
	stats := db_.Stats()
	if got := stats.OpenConnections; got != 1 {
		t.Errorf("stats.OpenConnections = %d; want 1", got)
	}

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	closeDB(t, db_)
	stats = db_.Stats()
	if got := stats.OpenConnections; got != 0 {
		t.Errorf("stats.OpenConnections = %d; want 0", got)
	}
}

func TestConnMaxLifetime(t *testing.T) {
	t0 := time.Unix(1000000, 0)
	offset := time.Duration(0)

	nowFunc = func() time.Time { return t0.Add(offset) }
	defer func() { nowFunc = time.Now }()

	db_ := newTestDB(t, "magicquery")
	defer closeDB(t, db_)

	driver := db_.Driver().(*fakeDriver)

	// Force the number of open connections to 0 so we can get an accurate
	// count for the test
	db_.clearAllConns(t)

	driver.mu.Lock()
	opens0 := driver.openCount
	closes0 := driver.closeCount
	driver.mu.Unlock()

	db_.SetMaxIdleConns(10)
	db_.SetMaxOpenConns(10)

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}

	offset = time.Second
	tx2, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}

	tx.Commit()
	tx2.Commit()

	driver.mu.Lock()
	opens := driver.openCount - opens0
	closes := driver.closeCount - closes0
	driver.mu.Unlock()

	if opens != 2 {
		t.Errorf("opens = %d; want 2", opens)
	}
	if closes != 0 {
		t.Errorf("closes = %d; want 0", closes)
	}
	if g, w := db_.numFreeConns(), 2; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	// Expire first conn
	offset = 11 * time.Second
	db_.SetConnMaxLifetime(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}

	tx, err = db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx2, err = db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	tx2.Commit()

	// Give connectionCleaner chance to run.
	waitCondition(t, func() bool {
		driver.mu.Lock()
		opens = driver.openCount - opens0
		closes = driver.closeCount - closes0
		driver.mu.Unlock()

		return closes == 1
	})

	if opens != 3 {
		t.Errorf("opens = %d; want 3", opens)
	}
	if closes != 1 {
		t.Errorf("closes = %d; want 1", closes)
	}

	if s := db_.Stats(); s.MaxLifetimeClosed != 1 {
		t.Errorf("MaxLifetimeClosed = %d; want 1 %#v", s.MaxLifetimeClosed, s)
	}
}

// golang.org/issue/5323
func TestStmtCloseDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	defer setHookpostCloseConn(nil)
	setHookpostCloseConn(func(_ *fakeConn, err error) {
		if err != nil {
			t.Errorf("Error closing fakeConn: %v", err)
		}
	})

	db_ := newTestDB(t, "magicquery")
	defer closeDB(t, db_)

	driver := db_.Driver().(*fakeDriver)

	driver.mu.Lock()
	opens0 := driver.openCount
	closes0 := driver.closeCount
	driver.mu.Unlock()
	openDelta0 := opens0 - closes0

	stmt1, err := db_.Prepare("SELECT|magicquery|op|op=?,millis=?")
	stmt_ := stmt1.(*stmt)
	if err != nil {
		t.Fatal(err)
	}

	// Start 50 parallel slow queries.
	const (
		nquery      = 50
		sleepMillis = 25
		nbatch      = 2
	)
	var wg sync.WaitGroup
	for batch := 0; batch < nbatch; batch++ {
		for i := 0; i < nquery; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var op string
				if err := stmt_.QueryRow("sleep", sleepMillis).Scan(&op); err != nil && err != ErrNoRows {
					t.Error(err)
				}
			}()
		}
		// Wait for the batch of queries above to finish before starting the next round.
		wg.Wait()
	}

	if g, w := db_.numFreeConns(), 2; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	if n := db_.numDepsPoll(t, 4); n > 4 {
		t.Errorf("number of dependencies = %d; expected <= 4", n)
		db_.dumpDeps(t)
	}

	driver.mu.Lock()
	opens := driver.openCount - opens0
	closes := driver.closeCount - closes0
	openDelta := (driver.openCount - driver.closeCount) - openDelta0
	driver.mu.Unlock()

	if openDelta > 2 {
		t.Logf("open calls = %d", opens)
		t.Logf("close calls = %d", closes)
		t.Logf("open delta = %d", openDelta)
		t.Errorf("_DB connections opened = %d; want <= 2", openDelta)
		db_.dumpDeps(t)
	}

	if !waitCondition(t, func() bool {
		return len(stmt_.css) <= nquery
	}) {
		t.Errorf("len(stmt_.css) = %d; want <= %d", len(stmt_.css), nquery)
	}

	if err := stmt_.Close(); err != nil {
		t.Fatal(err)
	}

	if g, w := db_.numFreeConns(), 2; g != w {
		t.Errorf("free conns = %d; want %d", g, w)
	}

	if n := db_.numDepsPoll(t, 2); n > 2 {
		t.Errorf("number of dependencies = %d; expected <= 2", n)
		db_.dumpDeps(t)
	}

	db_.clearAllConns(t)
}

// golang.org/issue/5046
func TestCloseConnBeforeStmts(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	defer setHookpostCloseConn(nil)
	setHookpostCloseConn(func(_ *fakeConn, err error) {
		if err != nil {
			t.Errorf("Error closing fakeConn: %v; from %s", err, stack())
			db_.dumpDeps(t)
			t.Errorf("db_ = %#v", db_)
		}
	})

	stmt_, err := db_.Prepare("SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}

	if len(db_.freeConn) != 1 {
		t.Fatalf("expected 1 freeConn; got %d", len(db_.freeConn))
	}
	dc := db_.freeConn[0]
	if dc.closed {
		t.Errorf("conn shouldn't be closed")
	}

	if n := len(dc.openStmt); n != 1 {
		t.Errorf("driverConn num openStmt = %d; want 1", n)
	}
	err = db_.Close()
	if err != nil {
		t.Errorf("_DB Close = %v", err)
	}
	if !dc.closed {
		t.Errorf("after db_.Close, driverConn should be closed")
	}
	if n := len(dc.openStmt); n != 0 {
		t.Errorf("driverConn num openStmt = %d; want 0", n)
	}

	err = stmt_.Close()
	if err != nil {
		t.Errorf("Stmt close = %v", err)
	}

	if !dc.closed {
		t.Errorf("conn should be closed")
	}
	if dc.ci != nil {
		t.Errorf("after Stmt Close, driverConn's Conn interface should be nil")
	}
}

// golang.org/issue/5283: don't release the Rows' connection in Close
// before calling Stmt.Close.
func TestRowsCloseOrder(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	db_.SetMaxIdleConns(0)
	setStrictFakeConnClose(t)
	defer setStrictFakeConnClose(nil)

	rows, err := db_.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatal(err)
	}
	err = rows.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRowsImplicitClose(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	rows, err := db_.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatal(err)
	}

	want, fail := 2, errors.New("fail")
	r := rows.rowsi.(*rowsCursor)
	r.errPos, r.err = want, fail

	got := 0
	for rows.Next() {
		got++
	}
	if got != want {
		t.Errorf("got %d rows, want %d", got, want)
	}
	if err := rows.Err(); err != fail {
		t.Errorf("got error %v, want %v", err, fail)
	}
	if !r.closed {
		t.Errorf("r.closed is false, want true")
	}
}

func TestRowsCloseError(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer db_.Close()
	rows, err := db_.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	type row struct {
		age  int
		name string
	}
	got := []row{}

	rc, ok := rows.rowsi.(*rowsCursor)
	if !ok {
		t.Fatal("not using *rowsCursor")
	}
	rc.closeErr = errors.New("rowsCursor: failed to close")

	for rows.Next() {
		var r row
		err = rows.Scan(&r.age, &r.name)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		got = append(got, r)
	}
	err = rows.Err()
	if err != rc.closeErr {
		t.Fatalf("unexpected err: got %v, want %v", err, rc.closeErr)
	}
}

func TestStmtCloseOrder(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	db_.SetMaxIdleConns(0)
	setStrictFakeConnClose(t)
	defer setStrictFakeConnClose(nil)

	_, err := db_.Query("SELECT|non_existent|name|")
	if err == nil {
		t.Fatal("Querying non-existent table should fail")
	}
}

// Test cases where there's more than maxBadConnRetries bad connections in the
// pool (issue 8834)
func TestManyErrBadConn(t *testing.T) {
	manyErrBadConnSetup := func(first ...func(db_ *_DB)) *_DB {
		db_ := newTestDB(t, "people")

		for _, f := range first {
			f(db_)
		}

		nconn := maxBadConnRetries + 1
		db_.SetMaxIdleConns(nconn)
		db_.SetMaxOpenConns(nconn)
		// open enough connections
		func() {
			for i := 0; i < nconn; i++ {
				rows, err := db_.Query("SELECT|people|age,name|")
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
			}
		}()

		db_.mu.Lock()
		defer db_.mu.Unlock()
		if db_.numOpen != nconn {
			t.Fatalf("unexpected numOpen %d (was expecting %d)", db_.numOpen, nconn)
		} else if len(db_.freeConn) != nconn {
			t.Fatalf("unexpected len(db_.freeConn) %d (was expecting %d)", len(db_.freeConn), nconn)
		}
		for _, conn := range db_.freeConn {
			conn.Lock()
			conn.ci.(*fakeConn).stickyBad = true
			conn.Unlock()
		}
		return db_
	}

	// Query
	db_ := manyErrBadConnSetup()
	defer closeDB(t, db_)
	rows, err := db_.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}

	// Exec
	db_ = manyErrBadConnSetup()
	defer closeDB(t, db_)
	_, err = db_.Exec("INSERT|people|name=Julia,age=19")
	if err != nil {
		t.Fatal(err)
	}

	// Begin
	db_ = manyErrBadConnSetup()
	defer closeDB(t, db_)
	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Prepare
	db_ = manyErrBadConnSetup()
	defer closeDB(t, db_)
	stmt_, err := db_.Prepare("SELECT|people|age,name|")
	if err != nil {
		t.Fatal(err)
	}
	if err = stmt_.Close(); err != nil {
		t.Fatal(err)
	}

	// Stmt.Exec
	db_ = manyErrBadConnSetup(func(db_ *_DB) {
		stmt_, err = db_.Prepare("INSERT|people|name=Julia,age=19")
		if err != nil {
			t.Fatal(err)
		}
	})
	defer closeDB(t, db_)
	_, err = stmt_.Exec()
	if err != nil {
		t.Fatal(err)
	}
	if err = stmt_.Close(); err != nil {
		t.Fatal(err)
	}

	// Stmt.Query
	db_ = manyErrBadConnSetup(func(db_ *_DB) {
		stmt_, err = db_.Prepare("SELECT|people|age,name|")
		if err != nil {
			t.Fatal(err)
		}
	})
	defer closeDB(t, db_)
	rows, err = stmt_.Query()
	if err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err = stmt_.Close(); err != nil {
		t.Fatal(err)
	}

	// Conn
	db_ = manyErrBadConnSetup()
	defer closeDB(t, db_)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn.dc.ci.(*fakeConn).skipDirtySession = true
	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Ping
	db_ = manyErrBadConnSetup()
	defer closeDB(t, db_)
	err = db_.PingContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

// Issue 34775: Ensure that a Tx cannot commit after a rollback.
func TestTxCannotCommitAfterRollback(t *testing.T) {
	db_ := newTestDB(t, "tx_status")
	defer closeDB(t, db_)

	// First check query reporting is correct.
	var txStatus string
	err := db_.QueryRow("SELECT|tx_status|tx_status|").Scan(&txStatus)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := txStatus, "autocommit"; g != w {
		t.Fatalf("tx_status=%q, wanted %q", g, w)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx, err := db_.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Ignore dirty session for this test.
	// A failing test should trigger the dirty session flag as well,
	// but that isn't exactly what this should test for.
	tx.txi.(*fakeTx).c.skipDirtySession = true

	defer tx.Rollback()

	err = tx.QueryRow("SELECT|tx_status|tx_status|").Scan(&txStatus)
	if err != nil {
		t.Fatal(err)
	}
	if g, w := txStatus, "transaction"; g != w {
		t.Fatalf("tx_status=%q, wanted %q", g, w)
	}

	// 1. Begin a transaction.
	// 2. (A) Start a query, (B) begin Tx rollback through a ctx cancel.
	// 3. Check if 2.A has committed in Tx (pass) or outside of Tx (fail).
	sendQuery := make(chan struct{})
	// The Tx status is returned through the row results, ensure
	// that the rows results are not canceled.
	bypassRowsAwaitDone = true
	hookTxGrabConn = func() {
		cancel()
		<-sendQuery
	}
	rollbackHook = func() {
		close(sendQuery)
	}
	defer func() {
		hookTxGrabConn = nil
		rollbackHook = nil
		bypassRowsAwaitDone = false
	}()

	err = tx.QueryRow("SELECT|tx_status|tx_status|").Scan(&txStatus)
	if err != nil {
		// A failure here would be expected if skipDirtySession was not set to true above.
		t.Fatal(err)
	}
	if g, w := txStatus, "transaction"; g != w {
		t.Fatalf("tx_status=%q, wanted %q", g, w)
	}
}

// Issue 40985 transaction statement deadlock while context cancel.
func TestTxStmtDeadlock(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tx, err := db_.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	stmt_, err := tx.Prepare("SELECT|people|name,age|age=?")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	// Run number of stmt_ queries to reproduce deadlock from context cancel
	for i := 0; i < 1e3; i++ {
		// Encounter any close related errors (e.g. ErrTxDone, stmt_ is closed)
		// is expected due to context cancel.
		_, err = stmt_.Query(1)
		if err != nil {
			break
		}
	}
	_ = tx.Rollback()
}

// Issue32530 encounters an issue where a connection may
// expire right after it comes out of a used connection pool
// even when a new connection is requested.
func TestConnExpiresFreshOutOfPool(t *testing.T) {
	execCases := []struct {
		expired  bool
		badReset bool
	}{
		{false, false},
		{true, false},
		{false, true},
	}

	t0 := time.Unix(1000000, 0)
	offset := time.Duration(0)
	offsetMu := sync.RWMutex{}

	nowFunc = func() time.Time {
		offsetMu.RLock()
		defer offsetMu.RUnlock()
		return t0.Add(offset)
	}
	defer func() { nowFunc = time.Now }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db_ := newTestDB(t, "magicquery")
	defer closeDB(t, db_)

	db_.SetMaxOpenConns(1)

	for _, ec := range execCases {
		ec := ec
		name := fmt.Sprintf("expired=%t,badReset=%t", ec.expired, ec.badReset)
		t.Run(name, func(t *testing.T) {
			db_.clearAllConns(t)

			db_.SetMaxIdleConns(1)
			db_.SetConnMaxLifetime(10 * time.Second)

			conn, err := db_.conn(ctx, alwaysNewConn)
			if err != nil {
				t.Fatal(err)
			}

			afterPutConn := make(chan struct{})
			waitingForConn := make(chan struct{})

			go func() {
				defer close(afterPutConn)

				conn, err := db_.conn(ctx, alwaysNewConn)
				if err == nil {
					db_.putConn(conn, err, false)
				} else {
					t.Errorf("db_.conn: %v", err)
				}
			}()
			go func() {
				defer close(waitingForConn)

				for {
					if t.Failed() {
						return
					}
					db_.mu.Lock()
					ct := len(db_.connRequests)
					db_.mu.Unlock()
					if ct > 0 {
						return
					}
					time.Sleep(pollDuration)
				}
			}()

			<-waitingForConn

			if t.Failed() {
				return
			}

			offsetMu.Lock()
			if ec.expired {
				offset = 11 * time.Second
			} else {
				offset = time.Duration(0)
			}
			offsetMu.Unlock()

			conn.ci.(*fakeConn).stickyBad = ec.badReset

			db_.putConn(conn, err, true)

			<-afterPutConn
		})
	}
}

// TestIssue20575 ensures the Rows from query does not block
// closing a transaction. Ensure Rows is closed while closing a transaction.
func TestIssue20575(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	tx, err := db_.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = tx.QueryContext(ctx, "SELECT|people|age,name|")
	if err != nil {
		t.Fatal(err)
	}
	// Do not close Rows from QueryContext.
	err = tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	select {
	default:
	case <-ctx.Done():
		t.Fatal("timeout: failed to rollback query without closing rows:", ctx.Err())
	}
}

// TestIssue20622 tests closing the transaction before rows is closed, requires
// the race detector to fail.
func TestIssue20622(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx, err := db_.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := tx.Query("SELECT|people|age,name|")
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for rows.Next() {
		count++
		var age int
		var name string
		if err := rows.Scan(&age, &name); err != nil {
			t.Fatal("scan failed", err)
		}

		if count == 1 {
			cancel()
		}
		time.Sleep(100 * time.Millisecond)
	}
	rows.Close()
	tx.Commit()
}

// golang.org/issue/5718
func TestErrBadConnReconnect(t *testing.T) {
	db_ := newTestDB(t, "foo")
	defer closeDB(t, db_)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")

	simulateBadConn := func(name string, hook *func() bool, op func() error) {
		broken, retried := false, false
		numOpen := db_.numOpen

		// simulate a broken connection on the first try
		*hook = func() bool {
			if !broken {
				broken = true
				return true
			}
			retried = true
			return false
		}

		if err := op(); err != nil {
			t.Errorf(name+": %v", err)
			return
		}

		if !broken || !retried {
			t.Error(name + ": Failed to simulate broken connection")
		}
		*hook = nil

		if numOpen != db_.numOpen {
			t.Errorf(name+": leaked %d connection(s)!", db_.numOpen-numOpen)
			numOpen = db_.numOpen
		}
	}

	// db_.Exec
	dbExec := func() error {
		_, err := db_.Exec("INSERT|t1|name=?,age=?,dead=?", "Gordon", 3, true)
		return err
	}
	simulateBadConn("db_.Exec prepare", &hookPrepareBadConn, dbExec)
	simulateBadConn("db_.Exec exec", &hookExecBadConn, dbExec)

	// db_.Query
	dbQuery := func() error {
		rows, err := db_.Query("SELECT|t1|age,name|")
		if err == nil {
			err = rows.Close()
		}
		return err
	}
	simulateBadConn("db_.Query prepare", &hookPrepareBadConn, dbQuery)
	simulateBadConn("db_.Query query", &hookQueryBadConn, dbQuery)

	// db_.Prepare
	simulateBadConn("db_.Prepare", &hookPrepareBadConn, func() error {
		stmt_, err := db_.Prepare("INSERT|t1|name=?,age=?,dead=?")
		if err != nil {
			return err
		}
		stmt_.Close()
		return nil
	})

	// Provide a way to force a re-prepare of a statement on next execution
	forcePrepare := func(stmt_ *stmt) {
		stmt_.css = nil
	}

	// stmt_.Exec
	stmt1, err := db_.Prepare("INSERT|t1|name=?,age=?,dead=?")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt1.Close()
	// make sure we must prepare the stmt_ first
	forcePrepare(stmt1.(*stmt))

	stmtExec := func() error {
		_, err := stmt1.Exec("Gopher", 3, false)
		return err
	}
	simulateBadConn("stmt_.Exec prepare", &hookPrepareBadConn, stmtExec)
	simulateBadConn("stmt_.Exec exec", &hookExecBadConn, stmtExec)

	// stmt_.Query
	stmt2, err := db_.Prepare("SELECT|t1|age,name|")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer stmt2.Close()
	// make sure we must prepare the stmt_ first
	forcePrepare(stmt2.(*stmt))

	stmtQuery := func() error {
		rows, err := stmt2.Query()
		if err == nil {
			err = rows.Close()
		}
		return err
	}
	simulateBadConn("stmt_.Query prepare", &hookPrepareBadConn, stmtQuery)
	simulateBadConn("stmt_.Query exec", &hookQueryBadConn, stmtQuery)
}

// golang.org/issue/11264
func TestTxEndBadConn(t *testing.T) {
	db_ := newTestDB(t, "foo")
	defer closeDB(t, db_)
	db_.SetMaxIdleConns(0)
	exec(t, db_, "CREATE|t1|name=string,age=int32,dead=bool")
	db_.SetMaxIdleConns(1)

	simulateBadConn := func(name string, hook *func() bool, op func() error) {
		broken := false
		numOpen := db_.numOpen

		*hook = func() bool {
			if !broken {
				broken = true
			}
			return broken
		}

		if err := op(); !errors.Is(err, driver.ErrBadConn) {
			t.Errorf(name+": %v", err)
			return
		}

		if !broken {
			t.Error(name + ": Failed to simulate broken connection")
		}
		*hook = nil

		if numOpen != db_.numOpen {
			t.Errorf(name+": leaked %d connection(s)!", db_.numOpen-numOpen)
		}
	}

	// db_.Exec
	dbExec := func(endTx func(tx *Tx) error) func() error {
		return func() error {
			tx, err := db_.Begin()
			if err != nil {
				return err
			}
			_, err = tx.Exec("INSERT|t1|name=?,age=?,dead=?", "Gordon", 3, true)
			if err != nil {
				return err
			}
			return endTx(tx)
		}
	}
	simulateBadConn("db_.Tx.Exec commit", &hookCommitBadConn, dbExec((*Tx).Commit))
	simulateBadConn("db_.Tx.Exec rollback", &hookRollbackBadConn, dbExec((*Tx).Rollback))

	// db_.Query
	dbQuery := func(endTx func(tx *Tx) error) func() error {
		return func() error {
			tx, err := db_.Begin()
			if err != nil {
				return err
			}
			rows, err := tx.Query("SELECT|t1|age,name|")
			if err == nil {
				err = rows.Close()
			} else {
				return err
			}
			return endTx(tx)
		}
	}
	simulateBadConn("db_.Tx.Query commit", &hookCommitBadConn, dbQuery((*Tx).Commit))
	simulateBadConn("db_.Tx.Query rollback", &hookRollbackBadConn, dbQuery((*Tx).Rollback))
}

type concurrentTest interface {
	init(t testing.TB, db_ *_DB)
	finish(t testing.TB)
	test(t testing.TB) error
}

type concurrentDBQueryTest struct {
	db *_DB
}

func (c *concurrentDBQueryTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
}

func (c *concurrentDBQueryTest) finish(t testing.TB) {
	c.db = nil
}

func (c *concurrentDBQueryTest) test(t testing.TB) error {
	rows, err := c.db.Query("SELECT|people|name|")
	if err != nil {
		t.Error(err)
		return err
	}
	var name string
	for rows.Next() {
		rows.Scan(&name)
	}
	rows.Close()
	return nil
}

type concurrentDBExecTest struct {
	db *_DB
}

func (c *concurrentDBExecTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
}

func (c *concurrentDBExecTest) finish(t testing.TB) {
	c.db = nil
}

func (c *concurrentDBExecTest) test(t testing.TB) error {
	_, err := c.db.Exec("NOSERT|people|name=Chris,age=?,photo=CPHOTO,bdate=?", 3, chrisBirthday)
	if err != nil {
		t.Error(err)
		return err
	}
	return nil
}

type concurrentStmtQueryTest struct {
	db    *_DB
	stmt_ *stmt
}

func (c *concurrentStmtQueryTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
	var err error
	stmt1, err := db_.Prepare("SELECT|people|name|")
	c.stmt_ = stmt1.(*stmt)
	if err != nil {
		t.Fatal(err)
	}
}

func (c *concurrentStmtQueryTest) finish(t testing.TB) {
	if c.stmt_ != nil {
		c.stmt_.Close()
		c.stmt_ = nil
	}
	c.db = nil
}

func (c *concurrentStmtQueryTest) test(t testing.TB) error {
	rows, err := c.stmt_.Query()
	if err != nil {
		t.Errorf("error on query:  %v", err)
		return err
	}

	var name string
	for rows.Next() {
		rows.Scan(&name)
	}
	rows.Close()
	return nil
}

type concurrentStmtExecTest struct {
	db    *_DB
	stmt_ *stmt
}

func (c *concurrentStmtExecTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
	var err error
	stmt1, err := db_.Prepare("NOSERT|people|name=Chris,age=?,photo=CPHOTO,bdate=?")
	c.stmt_ = stmt1.(*stmt)
	if err != nil {
		t.Fatal(err)
	}
}

func (c *concurrentStmtExecTest) finish(t testing.TB) {
	if c.stmt_ != nil {
		c.stmt_.Close()
		c.stmt_ = nil
	}
	c.db = nil
}

func (c *concurrentStmtExecTest) test(t testing.TB) error {
	_, err := c.stmt_.Exec(3, chrisBirthday)
	if err != nil {
		t.Errorf("error on exec:  %v", err)
		return err
	}
	return nil
}

type concurrentTxQueryTest struct {
	db *_DB
	tx *Tx
}

func (c *concurrentTxQueryTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
	var err error
	c.tx, err = c.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
}

func (c *concurrentTxQueryTest) finish(t testing.TB) {
	if c.tx != nil {
		c.tx.Rollback()
		c.tx = nil
	}
	c.db = nil
}

func (c *concurrentTxQueryTest) test(t testing.TB) error {
	rows, err := c.db.Query("SELECT|people|name|")
	if err != nil {
		t.Error(err)
		return err
	}
	var name string
	for rows.Next() {
		rows.Scan(&name)
	}
	rows.Close()
	return nil
}

type concurrentTxExecTest struct {
	db *_DB
	tx *Tx
}

func (c *concurrentTxExecTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
	var err error
	c.tx, err = c.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
}

func (c *concurrentTxExecTest) finish(t testing.TB) {
	if c.tx != nil {
		c.tx.Rollback()
		c.tx = nil
	}
	c.db = nil
}

func (c *concurrentTxExecTest) test(t testing.TB) error {
	_, err := c.tx.Exec("NOSERT|people|name=Chris,age=?,photo=CPHOTO,bdate=?", 3, chrisBirthday)
	if err != nil {
		t.Error(err)
		return err
	}
	return nil
}

type concurrentTxStmtQueryTest struct {
	db    *_DB
	tx    *Tx
	stmt_ *stmt
}

func (c *concurrentTxStmtQueryTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
	var err error
	c.tx, err = c.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt_, err := c.tx.Prepare("SELECT|people|name|")
	c.stmt_ = stmt_.(*stmt)
	if err != nil {
		t.Fatal(err)
	}
}

func (c *concurrentTxStmtQueryTest) finish(t testing.TB) {
	if c.stmt_ != nil {
		c.stmt_.Close()
		c.stmt_ = nil
	}
	if c.tx != nil {
		c.tx.Rollback()
		c.tx = nil
	}
	c.db = nil
}

func (c *concurrentTxStmtQueryTest) test(t testing.TB) error {
	rows, err := c.stmt_.Query()
	if err != nil {
		t.Errorf("error on query:  %v", err)
		return err
	}

	var name string
	for rows.Next() {
		rows.Scan(&name)
	}
	rows.Close()
	return nil
}

type concurrentTxStmtExecTest struct {
	db    *_DB
	tx    *Tx
	stmt_ *stmt
}

func (c *concurrentTxStmtExecTest) init(t testing.TB, db_ *_DB) {
	c.db = db_
	var err error
	c.tx, err = c.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt_, err := c.tx.Prepare("NOSERT|people|name=Chris,age=?,photo=CPHOTO,bdate=?")
	c.stmt_ = stmt_.(*stmt)
	if err != nil {
		t.Fatal(err)
	}
}

func (c *concurrentTxStmtExecTest) finish(t testing.TB) {
	if c.stmt_ != nil {
		c.stmt_.Close()
		c.stmt_ = nil
	}
	if c.tx != nil {
		c.tx.Rollback()
		c.tx = nil
	}
	c.db = nil
}

func (c *concurrentTxStmtExecTest) test(t testing.TB) error {
	_, err := c.stmt_.Exec(3, chrisBirthday)
	if err != nil {
		t.Errorf("error on exec:  %v", err)
		return err
	}
	return nil
}

type concurrentRandomTest struct {
	tests []concurrentTest
}

func (c *concurrentRandomTest) init(t testing.TB, db_ *_DB) {
	c.tests = []concurrentTest{
		new(concurrentDBQueryTest),
		new(concurrentDBExecTest),
		new(concurrentStmtQueryTest),
		new(concurrentStmtExecTest),
		new(concurrentTxQueryTest),
		new(concurrentTxExecTest),
		new(concurrentTxStmtQueryTest),
		new(concurrentTxStmtExecTest),
	}
	for _, ct := range c.tests {
		ct.init(t, db_)
	}
}

func (c *concurrentRandomTest) finish(t testing.TB) {
	for _, ct := range c.tests {
		ct.finish(t)
	}
}

func (c *concurrentRandomTest) test(t testing.TB) error {
	ct := c.tests[rand.Intn(len(c.tests))]
	return ct.test(t)
}

func doConcurrentTest(t testing.TB, ct concurrentTest) {
	maxProcs, numReqs := 1, 500
	if testing.Short() {
		maxProcs, numReqs = 4, 50
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(maxProcs))

	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ct.init(t, db_)
	defer ct.finish(t)

	var wg sync.WaitGroup
	wg.Add(numReqs)

	reqs := make(chan bool)
	defer close(reqs)

	for i := 0; i < maxProcs*2; i++ {
		go func() {
			for range reqs {
				err := ct.test(t)
				if err != nil {
					wg.Done()
					continue
				}
				wg.Done()
			}
		}()
	}

	for i := 0; i < numReqs; i++ {
		reqs <- true
	}

	wg.Wait()
}

func TestIssue6081(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	drv := db_.Driver().(*fakeDriver)
	drv.mu.Lock()
	opens0 := drv.openCount
	closes0 := drv.closeCount
	drv.mu.Unlock()

	stmt1, err := db_.Prepare("SELECT|people|name|")
	stmt_ := stmt1.(*stmt)

	if err != nil {
		t.Fatal(err)
	}
	setRowsCloseHook(func(rows *Rows, err *error) {
		*err = driver.ErrBadConn
	})
	defer setRowsCloseHook(nil)
	for i := 0; i < 10; i++ {
		rows, err := stmt_.Query()
		if err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	if n := len(stmt_.css); n > 1 {
		t.Errorf("len(css slice) = %d; want <= 1", n)
	}
	stmt_.Close()
	if n := len(stmt_.css); n != 0 {
		t.Errorf("len(css slice) after Close = %d; want 0", n)
	}

	drv.mu.Lock()
	opens := drv.openCount - opens0
	closes := drv.closeCount - closes0
	drv.mu.Unlock()
	if opens < 9 {
		t.Errorf("opens = %d; want >= 9", opens)
	}
	if closes < 9 {
		t.Errorf("closes = %d; want >= 9", closes)
	}
}

// TestIssue18429 attempts to stress rolling back the transaction from a
// context cancel while simultaneously calling Tx.Rollback. Rolling back from a
// context happens concurrently so tx.rollback and tx.Commit must guard against
// double entry.
//
// In the test, a context is canceled while the query is in process so
// the internal rollback will run concurrently with the explicitly called
// Tx.Rollback.
//
// The addition of calling rows.Next also tests
// Issue 21117.
func TestIssue18429(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx := context.Background()
	sem := make(chan bool, 20)
	var wg sync.WaitGroup

	const milliWait = 30

	for i := 0; i < 100; i++ {
		sem <- true
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			qwait := (time.Duration(rand.Intn(milliWait)) * time.Millisecond).String()

			ctx, cancel := context.WithTimeout(ctx, time.Duration(rand.Intn(milliWait))*time.Millisecond)
			defer cancel()

			tx, err := db_.BeginTx(ctx, nil)
			if err != nil {
				return
			}
			// This is expected to give a cancel error most, but not all the time.
			// Test failure will happen with a panic or other race condition being
			// reported.
			rows, _ := tx.QueryContext(ctx, "WAIT|"+qwait+"|SELECT|people|name|")
			if rows != nil {
				var name string
				// Call Next to test Issue 21117 and check for races.
				for rows.Next() {
					// Scan the buffer so it is read and checked for races.
					rows.Scan(&name)
				}
				rows.Close()
			}
			// This call will race with the context cancel rollback to complete
			// if the rollback itself isn't guarded.
			tx.Rollback()
		}()
	}
	wg.Wait()
}

// TestIssue20160 attempts to test a short context life on a stmt_ Query.
func TestIssue20160(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx := context.Background()
	sem := make(chan bool, 20)
	var wg sync.WaitGroup

	const milliWait = 30

	stmt_, err := db_.PrepareContext(ctx, "SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt_.Close()

	for i := 0; i < 100; i++ {
		sem <- true
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			ctx, cancel := context.WithTimeout(ctx, time.Duration(rand.Intn(milliWait))*time.Millisecond)
			defer cancel()

			// This is expected to give a cancel error most, but not all the time.
			// Test failure will happen with a panic or other race condition being
			// reported.
			rows, _ := stmt_.QueryContext(ctx)
			if rows != nil {
				rows.Close()
			}
		}()
	}
	wg.Wait()
}

// TestIssue18719 closes the context right before use. The sql.driverConn
// will nil out the ci on close in a lock, but if another process uses it right after
// it will panic with on the nil ref.
//
// See https://golang.org/cl/35550 .
func TestIssue18719(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx, err := db_.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	hookTxGrabConn = func() {
		cancel()

		// Wait for the context to cancel and tx to rollback.
		for tx.isDone() == false {
			time.Sleep(pollDuration)
		}
	}
	defer func() { hookTxGrabConn = nil }()

	// This call will grab the connection and cancel the context
	// after it has done so. Code after must deal with the canceled state.
	_, err = tx.QueryContext(ctx, "SELECT|people|name|")
	if err != nil {
		t.Fatalf("expected error %v but got %v", nil, err)
	}

	// Rows may be ignored because it will be closed when the context is canceled.

	// Do not explicitly rollback. The rollback will happen from the
	// canceled context.

	cancel()
}

func TestIssue20647(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn.dc.ci.(*fakeConn).skipDirtySession = true
	defer conn.Close()

	stmt_, err := conn.PrepareContext(ctx, "SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt_.Close()

	rows1, err := stmt_.QueryContext(ctx)
	if err != nil {
		t.Fatal("rows1", err)
	}
	defer rows1.Close()

	rows2, err := stmt_.QueryContext(ctx)
	if err != nil {
		t.Fatal("rows2", err)
	}
	defer rows2.Close()

	if rows1.dc != rows2.dc {
		t.Fatal("stmt_ prepared on Conn does not use same connection")
	}
}

func TestConcurrency(t *testing.T) {
	list := []struct {
		name string
		ct   concurrentTest
	}{
		{"Query", new(concurrentDBQueryTest)},
		{"Exec", new(concurrentDBExecTest)},
		{"StmtQuery", new(concurrentStmtQueryTest)},
		{"StmtExec", new(concurrentStmtExecTest)},
		{"TxQuery", new(concurrentTxQueryTest)},
		{"TxExec", new(concurrentTxExecTest)},
		{"TxStmtQuery", new(concurrentTxStmtQueryTest)},
		{"TxStmtExec", new(concurrentTxStmtExecTest)},
		{"Random", new(concurrentRandomTest)},
	}
	for _, item := range list {
		t.Run(item.name, func(t *testing.T) {
			doConcurrentTest(t, item.ct)
		})
	}
}

func TestConnectionLeak(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	// Start by opening defaultMaxIdleConns
	rows := make([]*Rows, defaultMaxIdleConns)
	// We need to SetMaxOpenConns > MaxIdleConns, so the _DB can open
	// a new connection and we can fill the idle queue with the released
	// connections.
	db_.SetMaxOpenConns(len(rows) + 1)
	for ii := range rows {
		r, err := db_.Query("SELECT|people|name|")
		if err != nil {
			t.Fatal(err)
		}
		r.Next()
		if err := r.Err(); err != nil {
			t.Fatal(err)
		}
		rows[ii] = r
	}
	// Now we have defaultMaxIdleConns busy connections. Open
	// a new one, but wait until the busy connections are released
	// before returning control to db_.
	drv := db_.Driver().(*fakeDriver)
	drv.waitCh = make(chan struct{}, 1)
	drv.waitingCh = make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		r, err := db_.Query("SELECT|people|name|")
		if err != nil {
			t.Error(err)
			return
		}
		r.Close()
		wg.Done()
	}()
	// Wait until the goroutine we've just created has started waiting.
	<-drv.waitingCh
	// Now close the busy connections. This provides a connection for
	// the blocked goroutine and then fills up the idle queue.
	for _, v := range rows {
		v.Close()
	}
	// At this point we give the new connection to db_. This connection is
	// now useless, since the idle queue is full and there are no pending
	// requests. _DB should deal with this situation without leaking the
	// connection.
	drv.waitCh <- struct{}{}
	wg.Wait()
}

func TestStatsMaxIdleClosedZero(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	db_.SetMaxOpenConns(1)
	db_.SetMaxIdleConns(1)
	db_.SetConnMaxLifetime(0)

	preMaxIdleClosed := db_.Stats().MaxIdleClosed

	for i := 0; i < 10; i++ {
		rows, err := db_.Query("SELECT|people|name|")
		if err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}

	st := db_.Stats()
	maxIdleClosed := st.MaxIdleClosed - preMaxIdleClosed
	t.Logf("MaxIdleClosed: %d", maxIdleClosed)
	if maxIdleClosed != 0 {
		t.Fatal("expected 0 max idle closed conns, got: ", maxIdleClosed)
	}
}

func TestStatsMaxIdleClosedTen(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	db_.SetMaxOpenConns(1)
	db_.SetMaxIdleConns(0)
	db_.SetConnMaxLifetime(0)

	preMaxIdleClosed := db_.Stats().MaxIdleClosed

	for i := 0; i < 10; i++ {
		rows, err := db_.Query("SELECT|people|name|")
		if err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}

	st := db_.Stats()
	maxIdleClosed := st.MaxIdleClosed - preMaxIdleClosed
	t.Logf("MaxIdleClosed: %d", maxIdleClosed)
	if maxIdleClosed != 10 {
		t.Fatal("expected 0 max idle closed conns, got: ", maxIdleClosed)
	}
}

// testUseConns uses count concurrent connections with 1 nanosecond apart.
// Returns the returnedAt time of the final connection.
func testUseConns(t *testing.T, count int, tm time.Time, db_ *_DB) time.Time {
	conns := make([]*Conn, count)
	ctx := context.Background()
	for i := range conns {
		tm = tm.Add(time.Nanosecond)
		nowFunc = func() time.Time {
			return tm
		}
		c, err := db_.Conn(ctx)
		if err != nil {
			t.Error(err)
		}
		conns[i] = c
	}

	for i := len(conns) - 1; i >= 0; i-- {
		tm = tm.Add(time.Nanosecond)
		nowFunc = func() time.Time {
			return tm
		}
		if err := conns[i].Close(); err != nil {
			t.Error(err)
		}
	}

	return tm
}

func TestMaxIdleTime(t *testing.T) {
	usedConns := 5
	reusedConns := 2
	list := []struct {
		wantMaxIdleTime   time.Duration
		wantMaxLifetime   time.Duration
		wantNextCheck     time.Duration
		wantIdleClosed    int64
		wantMaxIdleClosed int64
		timeOffset        time.Duration
		secondTimeOffset  time.Duration
	}{
		{
			time.Millisecond,
			0,
			time.Millisecond - time.Nanosecond,
			int64(usedConns - reusedConns),
			int64(usedConns - reusedConns),
			10 * time.Millisecond,
			0,
		},
		{
			// Want to close some connections via max idle time and one by max lifetime.
			time.Millisecond,
			// nowFunc() - MaxLifetime should be 1 * time.Nanosecond in connectionCleanerRunLocked.
			// This guarantees that first opened connection is to be closed.
			// Thus it is timeOffset + secondTimeOffset + 3 (+2 for Close while reusing conns and +1 for Conn).
			10*time.Millisecond + 100*time.Nanosecond + 3*time.Nanosecond,
			time.Nanosecond,
			// Closed all not reused connections and extra one by max lifetime.
			int64(usedConns - reusedConns + 1),
			int64(usedConns - reusedConns),
			10 * time.Millisecond,
			// Add second offset because otherwise connections are expired via max lifetime in Close.
			100 * time.Nanosecond,
		},
		{
			time.Hour,
			0,
			time.Second,
			0,
			0,
			10 * time.Millisecond,
			0},
	}
	baseTime := time.Unix(0, 0)
	defer func() {
		nowFunc = time.Now
	}()
	for _, item := range list {
		nowFunc = func() time.Time {
			return baseTime
		}
		t.Run(fmt.Sprintf("%v", item.wantMaxIdleTime), func(t *testing.T) {
			db_ := newTestDB(t, "people")
			defer closeDB(t, db_)

			db_.SetMaxOpenConns(usedConns)
			db_.SetMaxIdleConns(usedConns)
			db_.SetConnMaxIdleTime(item.wantMaxIdleTime)
			db_.SetConnMaxLifetime(item.wantMaxLifetime)

			preMaxIdleClosed := db_.Stats().MaxIdleTimeClosed

			// Busy usedConns.
			testUseConns(t, usedConns, baseTime, db_)

			tm := baseTime.Add(item.timeOffset)

			// Reuse connections which should never be considered idle
			// and exercises the sorting for issue 39471.
			tm = testUseConns(t, reusedConns, tm, db_)

			tm = tm.Add(item.secondTimeOffset)
			nowFunc = func() time.Time {
				return tm
			}

			db_.mu.Lock()
			nc, closing := db_.connectionCleanerRunLocked(time.Second)
			if nc != item.wantNextCheck {
				t.Errorf("got %v; want %v next check duration", nc, item.wantNextCheck)
			}

			// Validate freeConn order.
			var last time.Time
			for _, c := range db_.freeConn {
				if last.After(c.returnedAt) {
					t.Error("freeConn is not ordered by returnedAt")
					break
				}
				last = c.returnedAt
			}

			db_.mu.Unlock()
			for _, c := range closing {
				c.Close()
			}
			if g, w := int64(len(closing)), item.wantIdleClosed; g != w {
				t.Errorf("got: %d; want %d closed conns", g, w)
			}

			st := db_.Stats()
			maxIdleClosed := st.MaxIdleTimeClosed - preMaxIdleClosed
			if g, w := maxIdleClosed, item.wantMaxIdleClosed; g != w {
				t.Errorf("got: %d; want %d max idle closed conns", g, w)
			}
		})
	}
}

type nvcDriver struct {
	fakeDriver
	skipNamedValueCheck bool
}

func (d *nvcDriver) Open(dsn string) (driver.Conn, error) {
	c, err := d.fakeDriver.Open(dsn)
	fc := c.(*fakeConn)
	fc.db.allowAny = true
	return &nvcConn{fc, d.skipNamedValueCheck}, err
}

type nvcConn struct {
	*fakeConn
	skipNamedValueCheck bool
}

type decimalInt struct {
	value int
}

type doNotInclude struct{}

var _ driver.NamedValueChecker = &nvcConn{}

func (c *nvcConn) CheckNamedValue(nv *driver.NamedValue) error {
	if c.skipNamedValueCheck {
		return driver.ErrSkip
	}
	switch v := nv.Value.(type) {
	default:
		return driver.ErrSkip
	case Out:
		switch ov := v.Dest.(type) {
		default:
			return errors.New("unknown NameValueCheck OUTPUT type")
		case *string:
			*ov = "from-server"
			nv.Value = "OUT:*string"
		}
		return nil
	case decimalInt, []int64:
		return nil
	case doNotInclude:
		return driver.ErrRemoveArgument
	}
}

func TestNamedValueChecker(t *testing.T) {
	Register("NamedValueCheck", &nvcDriver{})
	db1, err := Open("NamedValueCheck", "")
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatal(err)
	}
	defer db_.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = db_.ExecContext(ctx, "WIPE")
	if err != nil {
		t.Fatal("exec wipe", err)
	}

	_, err = db_.ExecContext(ctx, "CREATE|keys|dec1=any,str1=string,out1=string,array1=any")
	if err != nil {
		t.Fatal("exec create", err)
	}

	o1 := ""
	_, err = db_.ExecContext(ctx, "INSERT|keys|dec1=?A,str1=?,out1=?O1,array1=?", Named("A", decimalInt{123}), "hello", Named("O1", Out{Dest: &o1}), []int64{42, 128, 707}, doNotInclude{})
	if err != nil {
		t.Fatal("exec insert", err)
	}
	var (
		str1 string
		dec1 decimalInt
		arr1 []int64
	)
	err = db_.QueryRowContext(ctx, "SELECT|keys|dec1,str1,array1|").Scan(&dec1, &str1, &arr1)
	if err != nil {
		t.Fatal("select", err)
	}

	list := []struct{ got, want any }{
		{o1, "from-server"},
		{dec1, decimalInt{123}},
		{str1, "hello"},
		{arr1, []int64{42, 128, 707}},
	}

	for index, item := range list {
		if !reflect.DeepEqual(item.got, item.want) {
			t.Errorf("got %#v wanted %#v for index %d", item.got, item.want, index)
		}
	}
}

func TestNamedValueCheckerSkip(t *testing.T) {
	Register("NamedValueCheckSkip", &nvcDriver{skipNamedValueCheck: true})
	db1, err := Open("NamedValueCheckSkip", "")
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatal(err)
	}
	defer db_.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = db_.ExecContext(ctx, "WIPE")
	if err != nil {
		t.Fatal("exec wipe", err)
	}

	_, err = db_.ExecContext(ctx, "CREATE|keys|dec1=any")
	if err != nil {
		t.Fatal("exec create", err)
	}

	_, err = db_.ExecContext(ctx, "INSERT|keys|dec1=?A", Named("A", decimalInt{123}))
	if err == nil {
		t.Fatalf("expected error with bad argument, got %v", err)
	}
}

func TestOpenConnector(t *testing.T) {
	Register("testctx", &fakeDriverCtx{})
	db1, err := Open("testctx", "people")
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatal(err)
	}
	defer db_.Close()

	c, ok := db_.connector.(*fakeConnector)
	if !ok {
		t.Fatal("not using *fakeConnector")
	}

	if err := db_.Close(); err != nil {
		t.Fatal(err)
	}

	if !c.closed {
		t.Fatal("connector is not closed")
	}
}

type ctxOnlyDriver struct {
	fakeDriver
}

func (d *ctxOnlyDriver) Open(dsn string) (driver.Conn, error) {
	conn, err := d.fakeDriver.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &ctxOnlyConn{fc: conn.(*fakeConn)}, nil
}

var (
	_ driver.Conn           = &ctxOnlyConn{}
	_ driver.QueryerContext = &ctxOnlyConn{}
	_ driver.ExecerContext  = &ctxOnlyConn{}
)

type ctxOnlyConn struct {
	fc *fakeConn

	queryCtxCalled bool
	execCtxCalled  bool
}

func (c *ctxOnlyConn) Begin() (driver.Tx, error) {
	return c.fc.Begin()
}

func (c *ctxOnlyConn) Close() error {
	return c.fc.Close()
}

// Prepare is still part of the Conn interface, so while it isn't used
// must be defined for compatibility.
func (c *ctxOnlyConn) Prepare(q string) (driver.Stmt, error) {
	panic("not used")
}

func (c *ctxOnlyConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	return c.fc.PrepareContext(ctx, q)
}

func (c *ctxOnlyConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.queryCtxCalled = true
	return c.fc.QueryContext(ctx, q, args)
}

func (c *ctxOnlyConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.execCtxCalled = true
	return c.fc.ExecContext(ctx, q, args)
}

// TestQueryExecContextOnly ensures drivers only need to implement QueryContext
// and ExecContext methods.
func TestQueryExecContextOnly(t *testing.T) {
	// Ensure connection does not implement non-context interfaces.
	var connType driver.Conn = &ctxOnlyConn{}
	if _, ok := connType.(driver.Execer); ok {
		t.Fatalf("%T must not implement driver.Execer", connType)
	}
	if _, ok := connType.(driver.Queryer); ok {
		t.Fatalf("%T must not implement driver.Queryer", connType)
	}

	Register("ContextOnly", &ctxOnlyDriver{})
	db1, err := Open("ContextOnly", "")
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatal(err)
	}
	defer db_.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db_.Conn(ctx)
	if err != nil {
		t.Fatal("db_.Conn", err)
	}
	defer conn.Close()
	coc := conn.dc.ci.(*ctxOnlyConn)
	coc.fc.skipDirtySession = true

	_, err = conn.ExecContext(ctx, "WIPE")
	if err != nil {
		t.Fatal("exec wipe", err)
	}

	_, err = conn.ExecContext(ctx, "CREATE|keys|v1=string")
	if err != nil {
		t.Fatal("exec create", err)
	}
	expectedValue := "value1"
	_, err = conn.ExecContext(ctx, "INSERT|keys|v1=?", expectedValue)
	if err != nil {
		t.Fatal("exec insert", err)
	}
	rows, err := conn.QueryContext(ctx, "SELECT|keys|v1|")
	if err != nil {
		t.Fatal("query select", err)
	}
	v1 := ""
	for rows.Next() {
		err = rows.Scan(&v1)
		if err != nil {
			t.Fatal("rows scan", err)
		}
	}
	rows.Close()

	if v1 != expectedValue {
		t.Fatalf("expected %q, got %q", expectedValue, v1)
	}

	if !coc.execCtxCalled {
		t.Error("ExecContext not called")
	}
	if !coc.queryCtxCalled {
		t.Error("QueryContext not called")
	}
}

type alwaysErrScanner struct{}

var errTestScanWrap = errors.New("errTestScanWrap")

func (alwaysErrScanner) Scan(any) error {
	return errTestScanWrap
}

// Issue 38099: Ensure that Rows.Scan properly wraps underlying errors.
func TestRowsScanProperlyWrapsErrors(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	rows, err := db_.Query("SELECT|people|age|")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	var res alwaysErrScanner

	for rows.Next() {
		err = rows.Scan(&res)
		if err == nil {
			t.Fatal("expecting back an error")
		}
		if !errors.Is(err, errTestScanWrap) {
			t.Fatalf("errors.Is mismatch\n%v\nWant: %v", err, errTestScanWrap)
		}
		// Ensure that error substring matching still correctly works.
		if !strings.Contains(err.Error(), errTestScanWrap.Error()) {
			t.Fatalf("Error %v does not contain %v", err, errTestScanWrap)
		}
	}
}

// From go.dev/issue/60304
func TestContextCancelDuringRawBytesScan(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	if _, err := db_.Exec("USE_RAWBYTES"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := db_.QueryContext(ctx, "SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	numRows := 0
	var sink byte
	for r.Next() {
		numRows++
		var s RawBytes
		err = r.Scan(&s)
		if !r.closemuScanHold {
			t.Errorf("expected closemu to be held")
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("read %q", s)
		if numRows == 2 {
			cancel() // invalidate the context, which used to call close asynchronously
		}
		for _, b := range s { // some operation reading from the raw memory
			sink += b
		}
	}
	if r.closemuScanHold {
		t.Errorf("closemu held; should not be")
	}

	// There are 3 rows. We canceled after reading 2 so we expect either
	// 2 or 3 depending on how the awaitDone goroutine schedules.
	switch numRows {
	case 0, 1:
		t.Errorf("got %d rows; want 2+", numRows)
	case 2:
		if err := r.Err(); err != context.Canceled {
			t.Errorf("unexpected error: %v (%T)", err, err)
		}
	default:
		// Made it to the end. This is rare, but fine. Permit it.
	}

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestContextCancelBetweenNextAndErr(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := db_.QueryContext(ctx, "SELECT|people|name|")
	if err != nil {
		t.Fatal(err)
	}
	for r.Next() {
	}
	cancel()                          // wake up the awaitDone goroutine
	time.Sleep(10 * time.Millisecond) // increase odds of seeing failure
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
}

// badConn implements a bad driver.Conn, for TestBadDriver.
// The Exec method panics.
type badConn struct{}

func (bc badConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("badConn Prepare")
}

func (bc badConn) Close() error {
	return nil
}

func (bc badConn) Begin() (driver.Tx, error) {
	return nil, errors.New("badConn Begin")
}

func (bc badConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	panic("badConn.Exec")
}

// badDriver is a driver.Driver that uses badConn.
type badDriver struct{}

func (bd badDriver) Open(name string) (driver.Conn, error) {
	return badConn{}, nil
}

// Issue 15901.
func TestBadDriver(t *testing.T) {
	Register("bad", badDriver{})
	db1, err := Open("bad", "ignored")
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		} else {
			if want := "badConn.Exec"; r.(string) != want {
				t.Errorf("panic was %v, expected %v", r, want)
			}
		}
	}()
	defer db_.Close()
	db_.Exec("ignored")
}

type pingDriver struct {
	fails bool
}

type pingConn struct {
	badConn
	driver *pingDriver
}

var pingError = errors.New("Ping failed")

func (pc pingConn) Ping(ctx context.Context) error {
	if pc.driver.fails {
		return pingError
	}
	return nil
}

var _ driver.Pinger = pingConn{}

func (pd *pingDriver) Open(name string) (driver.Conn, error) {
	return pingConn{driver: pd}, nil
}

func TestPing(t *testing.T) {
	driver := &pingDriver{}
	Register("ping", driver)

	db1, err := Open("ping", "ignored")
	db_ := db1.(*_DB)
	if err != nil {
		t.Fatal(err)
	}

	if err := db_.Ping(); err != nil {
		t.Errorf("err was %#v, expected nil", err)
		return
	}

	driver.fails = true
	if err := db_.Ping(); err != pingError {
		t.Errorf("err was %#v, expected pingError", err)
	}
}

// Issue 18101.
func TestTypedString(t *testing.T) {
	db_ := newTestDB(t, "people")
	defer closeDB(t, db_)

	type Str string
	var scanned Str

	err := db_.QueryRow("SELECT|people|name|name=?", "Alice").Scan(&scanned)
	if err != nil {
		t.Fatal(err)
	}
	expected := Str("Alice")
	if scanned != expected {
		t.Errorf("expected %+v, got %+v", expected, scanned)
	}
}

func BenchmarkConcurrentDBExec(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentDBExecTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkConcurrentStmtQuery(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentStmtQueryTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkConcurrentStmtExec(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentStmtExecTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkConcurrentTxQuery(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentTxQueryTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkConcurrentTxExec(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentTxExecTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkConcurrentTxStmtQuery(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentTxStmtQueryTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkConcurrentTxStmtExec(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentTxStmtExecTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkConcurrentRandom(b *testing.B) {
	b.ReportAllocs()
	ct := new(concurrentRandomTest)
	for i := 0; i < b.N; i++ {
		doConcurrentTest(b, ct)
	}
}

func BenchmarkManyConcurrentQueries(b *testing.B) {
	b.ReportAllocs()
	// To see lock contention in Go 1.4, 16~ cores and 128~ goroutines are required.
	const parallelism = 16

	db_ := newTestDB(b, "magicquery")
	defer closeDB(b, db_)
	db_.SetMaxIdleConns(runtime.GOMAXPROCS(0) * parallelism)

	stmt_, err := db_.Prepare("SELECT|magicquery|op|op=?,millis=?")
	if err != nil {
		b.Fatal(err)
	}
	defer stmt_.Close()

	b.SetParallelism(parallelism)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rows, err := stmt_.Query("sleep", 1)
			if err != nil {
				b.Error(err)
				return
			}
			rows.Close()
		}
	})
}

func TestGrabConnAllocs(t *testing.T) {
	testenv.SkipIfOptimizationOff(t)
	if race.Enabled {
		t.Skip("skipping allocation test when using race detector")
	}
	c := new(Conn)
	ctx := context.Background()
	n := int(testing.AllocsPerRun(1000, func() {
		_, release, err := c.grabConn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		release(nil)
	}))
	if n > 0 {
		t.Fatalf("Conn.grabConn allocated %v objects; want 0", n)
	}
}

func BenchmarkGrabConn(b *testing.B) {
	b.ReportAllocs()
	c := new(Conn)
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_, release, err := c.grabConn(ctx)
		if err != nil {
			b.Fatal(err)
		}
		release(nil)
	}
}
