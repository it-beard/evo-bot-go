package database

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"testing"
	"time"

	"evo-bot-go/internal/database/migrations"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startFakePostgres starts a minimal in-process server speaking just enough of the
// PostgreSQL wire protocol (v3) for lib/pq to connect without auth and answer Ping
// (an empty simple query). It returns the listener address.
func startFakePostgres(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	readyForQuery := []byte{'Z', 0, 0, 0, 5, 'I'}
	authOK := []byte{'R', 0, 0, 0, 8, 0, 0, 0, 0}
	emptyQueryResponse := []byte{'I', 0, 0, 0, 4}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

				// Startup message: int32 length (incl. itself) + payload
				header := make([]byte, 4)
				if _, err := io.ReadFull(conn, header); err != nil {
					return
				}
				payload := make([]byte, binary.BigEndian.Uint32(header)-4)
				if _, err := io.ReadFull(conn, payload); err != nil {
					return
				}
				if _, err := conn.Write(append(authOK, readyForQuery...)); err != nil {
					return
				}

				// Regular messages: type byte + int32 length (incl. itself) + payload
				for {
					msgHeader := make([]byte, 5)
					if _, err := io.ReadFull(conn, msgHeader); err != nil {
						return
					}
					body := make([]byte, binary.BigEndian.Uint32(msgHeader[1:])-4)
					if _, err := io.ReadFull(conn, body); err != nil {
						return
					}
					switch msgHeader[0] {
					case 'Q':
						if _, err := conn.Write(append(emptyQueryResponse, readyForQuery...)); err != nil {
							return
						}
					case 'X':
						return
					}
				}
			}(conn)
		}
	}()

	return listener.Addr().String()
}

func TestNewDB_ReachableServer_ReturnsConnectedDB(t *testing.T) {
	addr := startFakePostgres(t)

	db, err := NewDB(fmt.Sprintf("postgres://u:p@%s/db?sslmode=disable&connect_timeout=1", addr))

	require.NoError(t, err)
	require.NotNil(t, db)
	require.NotNil(t, db.DB)
	assert.NoError(t, db.Ping())
	assert.NoError(t, db.Close())
	assert.Error(t, db.Ping(), "ping after close must fail")
}

func TestNewDB_UnreachableServer_ReturnsPingError(t *testing.T) {
	start := time.Now()

	db, err := NewDB("postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")

	require.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to ping database")
	assert.Less(t, time.Since(start), 5*time.Second, "connection failure should be fast")
}

func TestNewDB_MalformedConnectionString_ReturnsError(t *testing.T) {
	db, err := NewDB("postgres://u:p@127.0.0.1:1/db?sslmode=not-a-mode")

	require.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to ping database")
}

func TestDB_InitWithMigrations_AllMigrationsAlreadyApplied(t *testing.T) {
	sqlDB, mock := testutils.NewMockDB(t)
	db := &DB{sqlDB}

	rows := sqlmock.NewRows([]string{"name"})
	for _, migration := range migrations.Registry() {
		rows.AddRow(migration.Name())
	}
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS migrations")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT name FROM migrations")).WillReturnRows(rows)

	err := db.InitWithMigrations()

	assert.NoError(t, err)
}

func TestDB_InitWithMigrations_SchemaError_IsWrapped(t *testing.T) {
	sqlDB, mock := testutils.NewMockDB(t)
	db := &DB{sqlDB}

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS migrations")).
		WillReturnError(errors.New("permission denied"))

	err := db.InitWithMigrations()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to run migrations")
	assert.Contains(t, err.Error(), "permission denied")
}

func TestDB_Close_ClosesUnderlyingConnection(t *testing.T) {
	sqlDB, mock := testutils.NewMockDB(t)
	db := &DB{sqlDB}
	mock.ExpectClose()

	err := db.Close()

	require.NoError(t, err)
	assert.Error(t, sqlDB.Ping(), "connection must be closed")
}

func TestDB_Close_PropagatesError(t *testing.T) {
	sqlDB, mock := testutils.NewMockDB(t)
	db := &DB{sqlDB}
	mock.ExpectClose().WillReturnError(errors.New("close failed"))

	err := db.Close()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "close failed")
}
