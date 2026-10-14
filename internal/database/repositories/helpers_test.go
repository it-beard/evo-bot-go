package repositories

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// errDB is a generic driver error used to simulate database failures
var errDB = errors.New("db failure")

// Fixed timestamps used across tests for deterministic assertions
var (
	testCreatedAt = time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)
	testUpdatedAt = time.Date(2025, 9, 2, 11, 30, 0, 0, time.UTC)
	testStartedAt = time.Date(2025, 9, 10, 18, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	// Repositories log non-fatal RowsAffected failures; keep test output clean
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// exactSQL builds an anchored regexp matching the given SQL literally,
// with whitespace collapsed the same way sqlmock collapses the actual query.
func exactSQL(query string) string {
	return "^" + regexp.QuoteMeta(strings.Join(strings.Fields(query), " ")) + "$"
}

// sqlRecorder records every SQL statement sqlmock sees (after whitespace collapsing).
type sqlRecorder struct {
	mu      sync.Mutex
	queries []string
}

func (r *sqlRecorder) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.queries) == 0 {
		return ""
	}
	return r.queries[len(r.queries)-1]
}

// newRecordingMockDB is like testutils.NewMockDB (regexp matching, unmet expectations fail the test)
// but additionally records the actual SQL text, which is needed for queries built from Go maps.
func newRecordingMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *sqlRecorder) {
	t.Helper()

	rec := &sqlRecorder{}
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if err := sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL); err != nil {
			return err
		}
		rec.mu.Lock()
		rec.queries = append(rec.queries, strings.Join(strings.Fields(actualSQL), " "))
		rec.mu.Unlock()
		return nil
	})

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
		db.Close()
	})

	return db, mock, rec
}

// captureArg is a sqlmock.Argument that accepts any value and stores it
type captureArg struct {
	value *driver.Value
}

func (c captureArg) Match(v driver.Value) bool {
	*c.value = v
	return true
}

// captureArgs returns n capturing matchers and the slice their values land in
func captureArgs(n int) ([]driver.Value, []driver.Value) {
	values := make([]driver.Value, n)
	matchers := make([]driver.Value, n)
	for i := range values {
		matchers[i] = captureArg{value: &values[i]}
	}
	return values, matchers
}

// timeBetween is a sqlmock.Argument matching a time.Time in [from, to]
type timeBetween struct {
	from, to time.Time
}

func (tb timeBetween) Match(v driver.Value) bool {
	ts, ok := v.(time.Time)
	return ok && !ts.Before(tb.from) && !ts.After(tb.to)
}

var setClauseRe = regexp.MustCompile(`(\w+) = \$(\d+)`)

// assertDynamicUpdate verifies a dynamically built "UPDATE <table> SET updated_at = NOW(), col = $n ... WHERE id = $k"
// statement: every column in fields is bound to its own value, and the WHERE placeholder is bound to id.
func assertDynamicUpdate(t *testing.T, table string, query string, args []driver.Value, fields map[string]interface{}, id int) {
	t.Helper()

	prefix := "UPDATE " + table + " SET updated_at = NOW()"
	require.True(t, strings.HasPrefix(query, prefix), "unexpected query prefix: %s", query)

	whereIdx := strings.LastIndex(query, " WHERE ")
	require.NotEqual(t, -1, whereIdx, "missing WHERE clause: %s", query)

	setPart := query[len(prefix):whereIdx]
	matches := setClauseRe.FindAllStringSubmatch(setPart, -1)
	require.Len(t, matches, len(fields), "unexpected SET clause: %s", setPart)
	require.Len(t, args, len(fields)+1)

	seen := map[string]bool{}
	for _, m := range matches {
		col := m[1]
		pos, err := strconv.Atoi(m[2])
		require.NoError(t, err)
		expected, ok := fields[col]
		require.True(t, ok, "unexpected column %q in SET clause", col)
		require.False(t, seen[col], "column %q set twice", col)
		seen[col] = true
		converted, err := driver.DefaultParameterConverter.ConvertValue(expected)
		require.NoError(t, err)
		require.Equal(t, converted, args[pos-1], "column %q bound to wrong value", col)
	}

	require.Equal(t, " WHERE id = $"+strconv.Itoa(len(fields)+1), query[whereIdx:])
	require.Equal(t, int64(id), args[len(fields)])
}
