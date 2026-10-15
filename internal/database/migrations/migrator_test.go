package migrations

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	createMigrationsTablePattern = regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS migrations")
	selectMigrationsPattern      = regexp.QuoteMeta("SELECT name FROM migrations")
	insertMigrationPattern       = regexp.QuoteMeta("INSERT INTO migrations (name, timestamp) VALUES ($1, $2)")
	deleteMigrationPattern       = regexp.QuoteMeta("DELETE FROM migrations WHERE name = $1")
)

// expectedRegistryNames is the canonical, ordered list of migrations.
var expectedRegistryNames = []string{
	"initial_migration",
	"change_user_id_to_nullable_string",
	"rename_user_id_to_user_nickname",
	"rename_contents_to_events",
	"add_new_event_types",
	"update_events_constraints",
	"add_users_and_profiles_tables",
	"add_published_message_id_to_profiles",
	"rename_website_to_freelink",
	"remove_social_links_from_profiles",
	"add_random_coffee_poll_tables",
	"remove_chat_id_from_random_coffee_polls",
	"add_is_club_member_to_users",
	"add_random_coffee_pairs_table",
	"add_group_topics_table",
	"add_group_messages_table",
	"remove_tg_sessions_table",
}

func okResult() sql.Result {
	return sqlmock.NewResult(0, 0)
}

// appliedRows returns a "SELECT name FROM migrations" result containing the given names.
func appliedRows(names ...string) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"name"})
	for _, name := range names {
		rows.AddRow(name)
	}
	return rows
}

// expectSchemaAndApplied sets expectations for the migrations-table bootstrap and the applied-migrations lookup.
func expectSchemaAndApplied(mock sqlmock.Sqlmock, applied ...string) {
	mock.ExpectExec(createMigrationsTablePattern).WillReturnResult(okResult())
	mock.ExpectQuery(selectMigrationsPattern).WillReturnRows(appliedRows(applied...))
}

func registryNames() []string {
	var names []string
	for _, migration := range Registry() {
		names = append(names, migration.Name())
	}
	return names
}

func TestRegistry_ContainsAllMigrationsInOrder(t *testing.T) {
	assert.Equal(t, expectedRegistryNames, registryNames())
}

func TestRegistry_NamesAreUniqueAndTimestampsValid(t *testing.T) {
	timestampPattern := regexp.MustCompile(`^20\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])$`)
	seen := make(map[string]bool)

	for _, migration := range Registry() {
		name := migration.Name()
		assert.NotEmpty(t, name)
		assert.False(t, seen[name], "duplicate migration name %q", name)
		seen[name] = true
		assert.Regexp(t, timestampPattern, migration.Timestamp(), "migration %q has invalid timestamp", name)
	}
}

func TestRegistry_ReturnsFreshInstances(t *testing.T) {
	first := Registry()
	second := Registry()

	require.Len(t, second, len(first))
	for i := range first {
		assert.NotSame(t, first[i], second[i])
	}
}

func TestInitMigrationsSchema_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(regexp.QuoteMeta(
		"CREATE TABLE IF NOT EXISTS migrations ( id SERIAL PRIMARY KEY, name TEXT NOT NULL UNIQUE, timestamp TEXT NOT NULL,",
	)).WillReturnResult(okResult())

	assert.NoError(t, initMigrationsSchema(db))
}

func TestInitMigrationsSchema_Error(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(createMigrationsTablePattern).WillReturnError(errors.New("boom"))

	err := initMigrationsSchema(db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create migrations table")
	assert.Contains(t, err.Error(), "boom")
}

func TestRecordMigration(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(insertMigrationPattern).WithArgs("some_migration", "20250101").WillReturnResult(sqlmock.NewResult(1, 1))

	assert.NoError(t, recordMigration(db, "some_migration", "20250101"))
}

func TestRecordMigration_Error(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(insertMigrationPattern).WithArgs("m", "t").WillReturnError(errors.New("duplicate"))

	assert.EqualError(t, recordMigration(db, "m", "t"), "duplicate")
}

func TestRemoveMigrationRecord(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(deleteMigrationPattern).WithArgs("some_migration").WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, removeMigrationRecord(db, "some_migration"))
}

func TestRemoveMigrationRecord_Error(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(deleteMigrationPattern).WithArgs("m").WillReturnError(errors.New("locked"))

	assert.EqualError(t, removeMigrationRecord(db, "m"), "locked")
}

func TestGetAppliedMigrations_ReturnsSet(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(selectMigrationsPattern).WillReturnRows(appliedRows("a", "b"))

	applied, err := getAppliedMigrations(db)

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"a": true, "b": true}, applied)
}

func TestGetAppliedMigrations_Empty(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(selectMigrationsPattern).WillReturnRows(appliedRows())

	applied, err := getAppliedMigrations(db)

	require.NoError(t, err)
	assert.NotNil(t, applied)
	assert.Empty(t, applied)
}

func TestGetAppliedMigrations_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(selectMigrationsPattern).WillReturnError(errors.New("no table"))

	applied, err := getAppliedMigrations(db)

	require.Error(t, err)
	assert.Nil(t, applied)
	assert.Contains(t, err.Error(), "failed to query migrations")
}

func TestGetAppliedMigrations_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(selectMigrationsPattern).WillReturnRows(appliedRows("ok").AddRow(nil))

	applied, err := getAppliedMigrations(db)

	require.Error(t, err)
	assert.Nil(t, applied)
	assert.Contains(t, err.Error(), "failed to scan migration name")
}

func TestGetAppliedMigrations_RowsError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	rows := appliedRows("a", "b").RowError(1, errors.New("connection reset"))
	mock.ExpectQuery(selectMigrationsPattern).WillReturnRows(rows)

	applied, err := getAppliedMigrations(db)

	require.Error(t, err)
	assert.Nil(t, applied)
	assert.Contains(t, err.Error(), "error iterating migration rows")
	assert.Contains(t, err.Error(), "connection reset")
}

func TestRunMigrations_AllApplied_RunsNothing(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	expectSchemaAndApplied(mock, registryNames()...)

	err := RunMigrations(db)

	assert.NoError(t, err)
}

func TestRunMigrations_AppliesOnlyPendingMigrationsInOrder(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	names := registryNames()
	// Everything but the last two migrations is already applied; extra unknown names are ignored.
	applied := append([]string{"some_legacy_migration"}, names[:len(names)-2]...)
	expectSchemaAndApplied(mock, applied...)

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS group_messages")).WillReturnResult(okResult())
	mock.ExpectExec(insertMigrationPattern).WithArgs("add_group_messages_table", "20250918").WillReturnResult(okResult())
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE IF EXISTS tg_sessions")).WillReturnResult(okResult())
	mock.ExpectExec(insertMigrationPattern).WithArgs("remove_tg_sessions_table", "20250919").WillReturnResult(okResult())

	err := RunMigrations(db)

	assert.NoError(t, err)
}

func TestRunMigrations_SkipsAppliedMigrationsInTheMiddle(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	names := registryNames()
	// Only "add_group_topics_table" (third from the end) and the last one are pending.
	var applied []string
	for _, name := range names {
		if name != "add_group_topics_table" && name != "remove_tg_sessions_table" {
			applied = append(applied, name)
		}
	}
	expectSchemaAndApplied(mock, applied...)

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS group_topics")).WillReturnResult(okResult())
	mock.ExpectExec(insertMigrationPattern).WithArgs("add_group_topics_table", "20250917").WillReturnResult(okResult())
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE IF EXISTS tg_sessions")).WillReturnResult(okResult())
	mock.ExpectExec(insertMigrationPattern).WithArgs("remove_tg_sessions_table", "20250919").WillReturnResult(okResult())

	assert.NoError(t, RunMigrations(db))
}

func TestRunMigrations_FreshDatabase_AppliesAllMigrations(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	expectSchemaAndApplied(mock)

	expectations := freshDatabaseExpectations()
	for _, migration := range Registry() {
		expect, ok := expectations[migration.Name()]
		require.True(t, ok, "no fresh-database expectations defined for migration %q", migration.Name())
		expect(mock)
		mock.ExpectExec(insertMigrationPattern).
			WithArgs(migration.Name(), migration.Timestamp()).
			WillReturnResult(okResult())
	}

	assert.NoError(t, RunMigrations(db))
}

func TestRunMigrations_SchemaInitError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(createMigrationsTablePattern).WillReturnError(errors.New("read-only"))

	err := RunMigrations(db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize migrations schema")
	assert.Contains(t, err.Error(), "read-only")
}

func TestRunMigrations_GetAppliedError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(createMigrationsTablePattern).WillReturnResult(okResult())
	mock.ExpectQuery(selectMigrationsPattern).WillReturnError(errors.New("timeout"))

	err := RunMigrations(db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get applied migrations")
	assert.Contains(t, err.Error(), "timeout")
}

func TestRunMigrations_RecordMigrationError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	names := registryNames()
	expectSchemaAndApplied(mock, names[:len(names)-2]...)

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS group_messages")).WillReturnResult(okResult())
	mock.ExpectExec(insertMigrationPattern).WithArgs("add_group_messages_table", "20250918").
		WillReturnError(errors.New("insert failed"))
	// remove_tg_sessions_table must not run after the record failure

	err := RunMigrations(db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to record migration add_group_messages_table")
	assert.Contains(t, err.Error(), "insert failed")
}

// The tests below cover the failure/rollback path. They intentionally do not assert the
// returned error: RunMigrations currently swallows a failed migration and returns nil
// (see final report), and these tests must not lock that behaviour in.

func TestRunMigrations_ApplyFails_RollsBackRemovesRecordAndStops(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	names := registryNames()
	expectSchemaAndApplied(mock, names[:len(names)-2]...)

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS group_messages")).
		WillReturnError(errors.New("syntax error"))
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE IF EXISTS group_messages")).WillReturnResult(okResult())
	mock.ExpectExec(deleteMigrationPattern).WithArgs("add_group_messages_table").WillReturnResult(okResult())
	// No INSERT for the failed migration and no further migrations (remove_tg_sessions_table) are executed.

	_ = RunMigrations(db)
}

func TestRunMigrations_ApplyFails_RollbackFails_DoesNotRemoveRecord(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	names := registryNames()
	expectSchemaAndApplied(mock, names[:len(names)-2]...)

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS group_messages")).
		WillReturnError(errors.New("syntax error"))
	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE IF EXISTS group_messages")).
		WillReturnError(errors.New("rollback failed"))
	// No DELETE FROM migrations and no further migrations.

	_ = RunMigrations(db)
}

func TestRunMigrations_ApplyFails_RemoveRecordFails_StillStops(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	names := registryNames()
	expectSchemaAndApplied(mock, names[:len(names)-1]...)

	mock.ExpectExec(regexp.QuoteMeta("DROP TABLE IF EXISTS tg_sessions")).
		WillReturnError(errors.New("lock timeout"))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS tg_sessions")).WillReturnResult(okResult())
	mock.ExpectExec(deleteMigrationPattern).WithArgs("remove_tg_sessions_table").
		WillReturnError(errors.New("delete failed"))

	_ = RunMigrations(db)
}

// freshDatabaseExpectations maps every registered migration to the SQL it issues on Apply.
// Patterns are deliberately loose: per-statement details are covered by the implementations tests.
func freshDatabaseExpectations() map[string]func(sqlmock.Sqlmock) {
	execs := func(patterns ...string) func(sqlmock.Sqlmock) {
		return func(mock sqlmock.Sqlmock) {
			for _, p := range patterns {
				mock.ExpectExec(regexp.QuoteMeta(p)).WillReturnResult(okResult())
			}
		}
	}
	txExecs := func(patterns ...string) func(sqlmock.Sqlmock) {
		return func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			execs(patterns...)(mock)
			mock.ExpectCommit()
		}
	}

	return map[string]func(sqlmock.Sqlmock){
		"initial_migration": func(mock sqlmock.Sqlmock) {
			execs("CREATE TABLE IF NOT EXISTS tg_sessions", "CREATE TABLE IF NOT EXISTS prompting_templates")(mock)
			for i := 0; i < 4; i++ {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM prompting_templates")).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO prompting_templates")).WillReturnResult(okResult())
			}
			execs(
				"CREATE TABLE IF NOT EXISTS contents",
				"CREATE TABLE IF NOT EXISTS topics",
				"CREATE TABLE IF NOT EXISTS users",
				"CREATE TABLE IF NOT EXISTS profiles",
				"CREATE INDEX IF NOT EXISTS profiles_user_id_idx",
			)(mock)
		},
		"change_user_id_to_nullable_string": execs("ALTER COLUMN user_id TYPE TEXT"),
		"rename_user_id_to_user_nickname":   execs("RENAME COLUMN user_id TO user_nickname"),
		"rename_contents_to_events": txExecs(
			"DROP CONSTRAINT IF EXISTS topics_content_id_fkey",
			"ALTER TABLE contents RENAME TO events",
			"RENAME COLUMN content_id TO event_id",
			"ADD CONSTRAINT topics_event_id_fkey",
		),
		"add_new_event_types": txExecs(
			"DROP CONSTRAINT IF EXISTS events_type_check",
			"ADD CONSTRAINT events_type_check",
		),
		"update_events_constraints": txExecs(
			"DROP CONSTRAINT IF EXISTS contents_type_check",
			"DROP CONSTRAINT IF EXISTS contents_status_check",
			"ADD CONSTRAINT events_status_check",
		),
		"add_users_and_profiles_tables": txExecs(
			"CREATE TABLE IF NOT EXISTS users",
			"CREATE TABLE IF NOT EXISTS profiles",
			"CREATE INDEX IF NOT EXISTS profiles_user_id_idx",
		),
		"add_published_message_id_to_profiles":    txExecs("ADD COLUMN published_message_id"),
		"rename_website_to_freelink":              execs("RENAME COLUMN website TO freelink"),
		"remove_social_links_from_profiles":       execs("DROP COLUMN IF EXISTS linkedin"),
		"add_random_coffee_poll_tables":           execs("CREATE TABLE IF NOT EXISTS random_coffee_polls", "CREATE TABLE IF NOT EXISTS random_coffee_participants", "CREATE OR REPLACE FUNCTION update_updated_at_column", "ADD COLUMN telegram_poll_id"),
		"remove_chat_id_from_random_coffee_polls": execs("DROP COLUMN IF EXISTS chat_id"),
		"add_is_club_member_to_users":             execs("ADD COLUMN is_club_member"),
		"add_random_coffee_pairs_table":           execs("CREATE TABLE IF NOT EXISTS random_coffee_pairs"),
		"add_group_topics_table":                  execs("CREATE TABLE IF NOT EXISTS group_topics"),
		"add_group_messages_table":                execs("CREATE TABLE IF NOT EXISTS group_messages"),
		"remove_tg_sessions_table":                execs("DROP TABLE IF EXISTS tg_sessions"),
	}
}
