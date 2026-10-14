package implementations

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errInjected = errors.New("injected failure")

func okResult() sql.Result {
	return sqlmock.NewResult(0, 0)
}

// collapse normalises whitespace the same way sqlmock does before matching.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// exact matches the whole statement (whitespace-insensitive).
func exact(stmt string) string {
	return "^" + regexp.QuoteMeta(collapse(stmt)) + "$"
}

// seq matches a statement containing all parts in the given order (whitespace-insensitive).
func seq(parts ...string) string {
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = regexp.QuoteMeta(collapse(p))
	}
	return strings.Join(quoted, ".*")
}

// migrationCase describes the SQL a migration issues for Apply and Rollback.
type migrationCase struct {
	migration     Migration
	name          string
	timestamp     string
	transactional bool
	apply         []string // statement patterns in execution order; nil when Apply is tested separately
	rollback      []string
}

func allMigrationCases() []migrationCase {
	return []migrationCase{
		{
			migration: NewInitialMigration(),
			name:      "initial_migration",
			timestamp: "20250401",
			// Apply is covered by the dedicated InitialMigration tests (it also runs queries)
			rollback: []string{
				exact("DROP INDEX IF EXISTS profiles_user_id_idx"),
				exact("DROP TABLE IF EXISTS profiles"),
				exact("DROP TABLE IF EXISTS topics"),
				exact("DROP TABLE IF EXISTS users"),
				exact("DROP TABLE IF EXISTS contents"),
				exact("DROP TABLE IF EXISTS prompting_templates"),
				exact("DROP TABLE IF EXISTS tg_sessions"),
			},
		},
		{
			migration: NewChangeUserIdToNullableString(),
			name:      "change_user_id_to_nullable_string",
			timestamp: "20250402",
			apply:     []string{exact("ALTER TABLE topics ALTER COLUMN user_id TYPE TEXT USING user_id::TEXT, ALTER COLUMN user_id DROP NOT NULL")},
			rollback:  []string{exact("ALTER TABLE topics ALTER COLUMN user_id TYPE INTEGER USING user_id::INTEGER, ALTER COLUMN user_id SET NOT NULL")},
		},
		{
			migration: NewRenameUserIdToUserNickname(),
			name:      "rename_user_id_to_user_nickname",
			timestamp: "20250402",
			apply:     []string{exact("ALTER TABLE topics RENAME COLUMN user_id TO user_nickname")},
			rollback:  []string{exact("ALTER TABLE topics RENAME COLUMN user_nickname TO user_id")},
		},
		{
			migration:     NewRenameContentsToEvents(),
			name:          "rename_contents_to_events",
			timestamp:     "20250403",
			transactional: true,
			apply: []string{
				exact("ALTER TABLE topics DROP CONSTRAINT IF EXISTS topics_content_id_fkey"),
				exact("ALTER TABLE contents RENAME TO events"),
				exact("ALTER TABLE topics RENAME COLUMN content_id TO event_id"),
				exact("ALTER TABLE topics ADD CONSTRAINT topics_event_id_fkey FOREIGN KEY (event_id) REFERENCES events(id)"),
			},
			rollback: []string{
				exact("ALTER TABLE topics DROP CONSTRAINT IF EXISTS topics_event_id_fkey"),
				exact("ALTER TABLE topics RENAME COLUMN event_id TO content_id"),
				exact("ALTER TABLE events RENAME TO contents"),
				exact("ALTER TABLE topics ADD CONSTRAINT topics_content_id_fkey FOREIGN KEY (content_id) REFERENCES contents(id)"),
			},
		},
		{
			migration:     NewUpdateEventsConstraints(),
			name:          "update_events_constraints",
			timestamp:     "20250404",
			transactional: true,
			apply: []string{
				exact("ALTER TABLE events DROP CONSTRAINT IF EXISTS contents_type_check"),
				exact("ALTER TABLE events DROP CONSTRAINT IF EXISTS contents_status_check"),
				exact("ALTER TABLE events ADD CONSTRAINT events_status_check CHECK (status IN ('finished', 'actual'))"),
			},
			// The CHECK values used by Rollback are intentionally not asserted (see final report).
			rollback: []string{
				exact("ALTER TABLE events DROP CONSTRAINT IF EXISTS events_status_check"),
				seq("ALTER TABLE events ADD CONSTRAINT contents_status_check CHECK (status IN ("),
				seq("ALTER TABLE events ADD CONSTRAINT contents_type_check CHECK (type IN ("),
			},
		},
		{
			migration:     NewAddNewEventTypes(),
			name:          "add_new_event_types",
			timestamp:     "20250417",
			transactional: true,
			apply: []string{
				exact("ALTER TABLE events DROP CONSTRAINT IF EXISTS events_type_check"),
				exact("ALTER TABLE events ADD CONSTRAINT events_type_check CHECK (type IN ('club-call', 'meetup', 'workshop', 'reading-club', 'conference'))"),
			},
			rollback: []string{
				exact("ALTER TABLE events DROP CONSTRAINT IF EXISTS events_type_check"),
				exact("ALTER TABLE events ADD CONSTRAINT events_type_check CHECK (type IN ('club-call', 'meetup'))"),
			},
		},
		{
			migration:     NewAddUsersAndProfilesTables(),
			name:          "add_users_and_profiles_tables",
			timestamp:     "20250519",
			transactional: true,
			apply: []string{
				seq("CREATE TABLE IF NOT EXISTS users (", "tg_id BIGINT NOT NULL UNIQUE", "has_coffee_ban BOOLEAN NOT NULL DEFAULT false"),
				seq("CREATE TABLE IF NOT EXISTS profiles (", "user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE"),
				exact("CREATE INDEX IF NOT EXISTS profiles_user_id_idx ON profiles(user_id)"),
			},
			rollback: []string{
				exact("DROP TABLE IF EXISTS profiles"),
				exact("DROP TABLE IF EXISTS users"),
			},
		},
		{
			migration:     NewAddPublishedMessageIDToProfiles(),
			name:          "add_published_message_id_to_profiles",
			timestamp:     "20250520",
			transactional: true,
			apply:         []string{exact("ALTER TABLE profiles ADD COLUMN published_message_id BIGINT DEFAULT NULL")},
			rollback:      []string{exact("ALTER TABLE profiles DROP COLUMN IF EXISTS published_message_id")},
		},
		{
			migration: NewRenameWebsiteToFreelink(),
			name:      "rename_website_to_freelink",
			timestamp: "20250520",
			apply:     []string{exact("ALTER TABLE profiles RENAME COLUMN website TO freelink")},
			rollback:  []string{exact("ALTER TABLE profiles RENAME COLUMN freelink TO website")},
		},
		{
			migration: NewRemoveSocialLinksFromProfiles(),
			name:      "remove_social_links_from_profiles",
			timestamp: "20250521",
			apply: []string{exact(
				"ALTER TABLE profiles DROP COLUMN IF EXISTS linkedin, DROP COLUMN IF EXISTS github, DROP COLUMN IF EXISTS freelink",
			)},
			rollback: []string{exact(
				"ALTER TABLE profiles ADD COLUMN IF NOT EXISTS linkedin TEXT DEFAULT '', ADD COLUMN IF NOT EXISTS github TEXT DEFAULT '', ADD COLUMN IF NOT EXISTS freelink TEXT DEFAULT ''",
			)},
		},
		{
			migration: NewAddRandomCoffeePollTables(),
			name:      "add_random_coffee_poll_tables",
			timestamp: "20250602",
			apply: []string{
				seq("CREATE TABLE IF NOT EXISTS random_coffee_polls (", "message_id BIGINT NOT NULL", "week_start_date DATE NOT NULL"),
				seq("CREATE TABLE IF NOT EXISTS random_coffee_participants (",
					"poll_id INTEGER NOT NULL REFERENCES random_coffee_polls(id) ON DELETE CASCADE",
					"user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE",
					"UNIQUE (poll_id, user_id)"),
				seq("CREATE OR REPLACE FUNCTION update_updated_at_column()",
					"NEW.updated_at = NOW();",
					"CREATE TRIGGER update_random_coffee_participants_updated_at BEFORE UPDATE ON random_coffee_participants"),
				seq("ALTER TABLE random_coffee_polls ADD COLUMN telegram_poll_id TEXT;",
					"ADD CONSTRAINT random_coffee_polls_telegram_poll_id_unique UNIQUE (telegram_poll_id);",
					"COMMENT ON COLUMN random_coffee_polls.telegram_poll_id"),
			},
			rollback: []string{
				seq("ALTER TABLE random_coffee_polls DROP CONSTRAINT IF EXISTS random_coffee_polls_telegram_poll_id_unique;",
					"ALTER TABLE random_coffee_polls DROP COLUMN IF EXISTS telegram_poll_id;"),
				seq("DROP TRIGGER IF EXISTS update_random_coffee_participants_updated_at ON random_coffee_participants;",
					"DROP FUNCTION IF EXISTS update_updated_at_column();"),
				seq("DROP TABLE IF EXISTS random_coffee_participants;", "DROP TABLE IF EXISTS random_coffee_polls;"),
			},
		},
		{
			migration: NewRemoveChatIdFromRandomCoffeePolls(),
			name:      "remove_chat_id_from_random_coffee_polls",
			timestamp: "20250603",
			apply:     []string{exact("ALTER TABLE random_coffee_polls DROP COLUMN IF EXISTS chat_id")},
			rollback:  []string{exact("ALTER TABLE random_coffee_polls ADD COLUMN IF NOT EXISTS chat_id BIGINT NOT NULL DEFAULT 0")},
		},
		{
			migration: NewAddIsClubMemberToUsers(),
			name:      "add_is_club_member_to_users",
			timestamp: "20250609",
			apply:     []string{exact("ALTER TABLE users ADD COLUMN is_club_member BOOLEAN DEFAULT TRUE")},
			rollback:  []string{exact("ALTER TABLE users DROP COLUMN IF EXISTS is_club_member")},
		},
		{
			migration: NewAddRandomCoffeePairsTable(),
			name:      "add_random_coffee_pairs_table",
			timestamp: "20250609",
			apply: []string{seq("CREATE TABLE IF NOT EXISTS random_coffee_pairs (",
				"user1_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE",
				"user2_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE",
				"UNIQUE(poll_id, user1_id, user2_id)")},
			rollback: []string{exact("DROP TABLE IF EXISTS random_coffee_pairs;")},
		},
		{
			migration: NewAddGroupTopicsTable(),
			name:      "add_group_topics_table",
			timestamp: "20250917",
			apply:     []string{seq("CREATE TABLE IF NOT EXISTS group_topics (", "topic_id BIGINT NOT NULL UNIQUE", "name TEXT NOT NULL")},
			rollback:  []string{exact("DROP TABLE IF EXISTS group_topics;")},
		},
		{
			migration: NewAddGroupMessagesTable(),
			name:      "add_group_messages_table",
			timestamp: "20250918",
			apply: []string{seq("CREATE TABLE IF NOT EXISTS group_messages (",
				"CONSTRAINT unique_message_id UNIQUE (message_id)",
				"CREATE INDEX IF NOT EXISTS idx_group_messages_user_tg_id ON group_messages(user_tg_id);",
				"CREATE INDEX IF NOT EXISTS idx_group_messages_group_topic_id ON group_messages(group_topic_id);",
				"CREATE INDEX IF NOT EXISTS idx_group_messages_reply_to_message_id ON group_messages(reply_to_message_id);",
				"CREATE INDEX IF NOT EXISTS idx_group_messages_created_at ON group_messages(created_at);")},
			rollback: []string{exact("DROP TABLE IF EXISTS group_messages;")},
		},
		{
			migration: NewRemoveTgSessionsTable(),
			name:      "remove_tg_sessions_table",
			timestamp: "20250919",
			apply:     []string{exact("DROP TABLE IF EXISTS tg_sessions")},
			rollback:  []string{seq("CREATE TABLE IF NOT EXISTS tg_sessions (")},
		},
	}
}

// operation abstracts Apply/Rollback so the same scenarios can be run against both.
type operation struct {
	label      string
	run        func(Migration, *sql.DB) error
	statements func(migrationCase) []string
}

var operations = []operation{
	{
		label:      "Apply",
		run:        func(m Migration, db *sql.DB) error { return m.Apply(db) },
		statements: func(c migrationCase) []string { return c.apply },
	},
	{
		label:      "Rollback",
		run:        func(m Migration, db *sql.DB) error { return m.Rollback(db) },
		statements: func(c migrationCase) []string { return c.rollback },
	},
}

// forEachOperation runs fn for every (migration, operation) pair that has statements defined.
func forEachOperation(t *testing.T, fn func(t *testing.T, c migrationCase, op operation, statements []string)) {
	forEachOperationWhere(t, func(migrationCase) bool { return true }, fn)
}

// forEachOperationWhere is forEachOperation restricted to the migrations accepted by filter.
func forEachOperationWhere(t *testing.T, filter func(migrationCase) bool, fn func(t *testing.T, c migrationCase, op operation, statements []string)) {
	for _, c := range allMigrationCases() {
		if !filter(c) {
			continue
		}
		for _, op := range operations {
			statements := op.statements(c)
			if len(statements) == 0 {
				continue
			}
			t.Run(c.name+"/"+op.label, func(t *testing.T) {
				fn(t, c, op, statements)
			})
		}
	}
}

func isTransactional(c migrationCase) bool {
	return c.transactional
}

func TestMigrations_NameAndTimestamp(t *testing.T) {
	for _, c := range allMigrationCases() {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.name, c.migration.Name())
			assert.Equal(t, c.timestamp, c.migration.Timestamp())
		})
	}
}

func TestMigrations_NamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, c := range allMigrationCases() {
		assert.False(t, seen[c.migration.Name()], "duplicate migration name %q", c.migration.Name())
		seen[c.migration.Name()] = true
	}
}

func TestBaseMigration_Accessors(t *testing.T) {
	base := &BaseMigration{name: "custom", timestamp: "20991231"}

	assert.Equal(t, "custom", base.Name())
	assert.Equal(t, "20991231", base.Timestamp())
}

func TestMigrations_Success(t *testing.T) {
	forEachOperation(t, func(t *testing.T, c migrationCase, op operation, statements []string) {
		db, mock := testutils.NewMockDB(t)
		if c.transactional {
			mock.ExpectBegin()
		}
		for _, stmt := range statements {
			mock.ExpectExec(stmt).WillReturnResult(okResult())
		}
		if c.transactional {
			mock.ExpectCommit()
		}

		assert.NoError(t, op.run(c.migration, db))
	})
}

func TestMigrations_StatementFailure_StopsAndReturnsError(t *testing.T) {
	forEachOperation(t, func(t *testing.T, c migrationCase, op operation, statements []string) {
		for failAt := range statements {
			t.Run(fmt.Sprintf("statement_%d", failAt), func(t *testing.T) {
				db, mock := testutils.NewMockDB(t)
				if c.transactional {
					mock.ExpectBegin()
				}
				for i := 0; i < failAt; i++ {
					mock.ExpectExec(statements[i]).WillReturnResult(okResult())
				}
				mock.ExpectExec(statements[failAt]).WillReturnError(errInjected)
				if c.transactional {
					// The deferred handler must roll the transaction back
					mock.ExpectRollback()
				}

				err := op.run(c.migration, db)

				require.Error(t, err)
				assert.ErrorIs(t, err, errInjected)
			})
		}
	})
}

func TestMigrations_Transactional_BeginFailure(t *testing.T) {
	forEachOperationWhere(t, isTransactional, func(t *testing.T, c migrationCase, op operation, _ []string) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectBegin().WillReturnError(errInjected)

		err := op.run(c.migration, db)

		require.Error(t, err)
		assert.ErrorIs(t, err, errInjected)
		assert.Contains(t, err.Error(), "failed to begin transaction")
	})
}

func TestMigrations_Transactional_CommitFailure(t *testing.T) {
	forEachOperationWhere(t, isTransactional, func(t *testing.T, c migrationCase, op operation, statements []string) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectBegin()
		for _, stmt := range statements {
			mock.ExpectExec(stmt).WillReturnResult(okResult())
		}
		mock.ExpectCommit().WillReturnError(errInjected)

		err := op.run(c.migration, db)

		require.Error(t, err)
		assert.ErrorIs(t, err, errInjected)
		assert.Contains(t, err.Error(), "failed to commit")
	})
}

func TestMigrations_Transactional_ErrorMessages(t *testing.T) {
	tests := []struct {
		migration Migration
		op        operation
		failAt    int
		expected  string
	}{
		{NewRenameContentsToEvents(), operations[0], 0, "failed to drop foreign key constraint"},
		{NewRenameContentsToEvents(), operations[0], 1, "failed to rename contents table to events"},
		{NewRenameContentsToEvents(), operations[0], 2, "failed to rename content_id column"},
		{NewRenameContentsToEvents(), operations[0], 3, "failed to add new foreign key constraint"},
		{NewRenameContentsToEvents(), operations[1], 0, "failed to drop foreign key constraint"},
		{NewRenameContentsToEvents(), operations[1], 1, "failed to rename event_id column"},
		{NewRenameContentsToEvents(), operations[1], 2, "failed to rename events table back to contents"},
		{NewRenameContentsToEvents(), operations[1], 3, "failed to add original foreign key constraint"},
		{NewUpdateEventsConstraints(), operations[0], 0, "failed to drop contents_type_check constraint"},
		{NewUpdateEventsConstraints(), operations[0], 1, "failed to drop contents_status_check constraint"},
		{NewUpdateEventsConstraints(), operations[0], 2, "failed to add events_status_check constraint"},
		{NewUpdateEventsConstraints(), operations[1], 0, "failed to drop events_status_check constraint"},
		{NewUpdateEventsConstraints(), operations[1], 1, "failed to add contents_status_check constraint"},
		{NewUpdateEventsConstraints(), operations[1], 2, "failed to add contents_type_check constraint"},
		{NewAddNewEventTypes(), operations[0], 0, "failed to drop type check constraint"},
		{NewAddNewEventTypes(), operations[0], 1, "failed to add new type check constraint"},
		{NewAddNewEventTypes(), operations[1], 0, "failed to drop type check constraint"},
		{NewAddNewEventTypes(), operations[1], 1, "failed to restore original type check constraint"},
		{NewAddUsersAndProfilesTables(), operations[0], 0, "failed to create users table"},
		{NewAddUsersAndProfilesTables(), operations[0], 1, "failed to create profiles table"},
		{NewAddUsersAndProfilesTables(), operations[0], 2, "failed to create index on profiles.user_id"},
		{NewAddUsersAndProfilesTables(), operations[1], 0, "failed to drop profiles table"},
		{NewAddUsersAndProfilesTables(), operations[1], 1, "failed to drop users table"},
		{NewAddPublishedMessageIDToProfiles(), operations[0], 0, "failed to add published_message_id column to profiles table"},
		{NewAddPublishedMessageIDToProfiles(), operations[1], 0, "failed to drop published_message_id column from profiles table"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s/%d", tt.migration.Name(), tt.op.label, tt.failAt), func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectBegin()
			for i := 0; i < tt.failAt; i++ {
				mock.ExpectExec(".+").WillReturnResult(okResult())
			}
			mock.ExpectExec(".+").WillReturnError(errInjected)
			mock.ExpectRollback()

			err := tt.op.run(tt.migration, db)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// --- InitialMigration.Apply ---

type defaultPrompt struct {
	key      string
	text     string
	errorMsg string
}

func defaultPrompts() []defaultPrompt {
	return []defaultPrompt{
		{key: "get_content_prompt", errorMsg: "failed to insert content prompt"},
		{key: "daily_summarization_prompt", errorMsg: "failed to insert summarization prompt"},
		{key: "get_tool_prompt", errorMsg: "failed to insert tool prompt"},
		{key: "get_intro_prompt", errorMsg: "failed to insert intro prompt"},
	}
}

var (
	checkPromptPattern  = exact("SELECT EXISTS(SELECT 1 FROM prompting_templates WHERE template_key = $1)")
	insertPromptPattern = exact("INSERT INTO prompting_templates (template_key, template_text) VALUES ($1, $2)")
)

// initialApplyStep is one database interaction of InitialMigration.Apply.
type initialApplyStep struct {
	label    string
	errorMsg string
	expect   func(mock sqlmock.Sqlmock, fail bool)
}

// initialApplySteps returns every interaction of InitialMigration.Apply on an empty database
// (no prompts exist yet, so each prompt is checked and then inserted).
func initialApplySteps() []initialApplyStep {
	createStep := func(label, stmt, errorMsg string) initialApplyStep {
		return initialApplyStep{label: label, errorMsg: errorMsg, expect: func(mock sqlmock.Sqlmock, fail bool) {
			e := mock.ExpectExec(exact(stmt))
			if fail {
				e.WillReturnError(errInjected)
			} else {
				e.WillReturnResult(okResult())
			}
		}}
	}

	steps := []initialApplyStep{
		createStep("create tg_sessions", createSessionsTableSQL, "failed to create tg_sessions table"),
		createStep("create prompting_templates", createPromptingTemplatesTableSQL, "failed to create prompting_templates table"),
	}
	for _, p := range defaultPrompts() {
		p := p
		steps = append(steps,
			initialApplyStep{label: "check " + p.key, errorMsg: p.errorMsg + ": failed to check if prompt exists", expect: func(mock sqlmock.Sqlmock, fail bool) {
				e := mock.ExpectQuery(checkPromptPattern).WithArgs(p.key)
				if fail {
					e.WillReturnError(errInjected)
				} else {
					e.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				}
			}},
			initialApplyStep{label: "insert " + p.key, errorMsg: p.errorMsg + ": failed to insert prompt", expect: func(mock sqlmock.Sqlmock, fail bool) {
				e := mock.ExpectExec(insertPromptPattern).WithArgs(p.key, sqlmock.AnyArg())
				if fail {
					e.WillReturnError(errInjected)
				} else {
					e.WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}},
		)
	}
	steps = append(steps,
		createStep("create contents", createContentsTableSQL, "failed to create contents table"),
		createStep("create topics", createTopicsTableSQL, "failed to create topics table"),
		createStep("create users", createUsersTableSQL, "failed to create users table"),
		createStep("create profiles", createProfilesTableSQL, "failed to create profiles table"),
		createStep("create profiles index", createProfilesIndexSQL, "failed to create profiles index"),
	)
	return steps
}

func TestInitialMigration_Apply_FreshDatabase_CreatesSchemaAndInsertsPrompts(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	for _, step := range initialApplySteps() {
		step.expect(mock, false)
	}

	assert.NoError(t, NewInitialMigration().Apply(db))
}

func TestInitialMigration_Apply_InsertsDefaultPromptTexts(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(exact(createSessionsTableSQL)).WillReturnResult(okResult())
	mock.ExpectExec(exact(createPromptingTemplatesTableSQL)).WillReturnResult(okResult())

	var inserted []string
	for _, p := range defaultPrompts() {
		mock.ExpectQuery(checkPromptPattern).WithArgs(p.key).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectExec(insertPromptPattern).
			WithArgs(p.key, nonEmptyStringArg{collect: &inserted}).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	for _, stmt := range []string{createContentsTableSQL, createTopicsTableSQL, createUsersTableSQL, createProfilesTableSQL, createProfilesIndexSQL} {
		mock.ExpectExec(exact(stmt)).WillReturnResult(okResult())
	}

	require.NoError(t, NewInitialMigration().Apply(db))
	assert.Len(t, inserted, 4)
}

func TestInitialMigration_Apply_ExistingPromptsAreNotReinserted(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectExec(exact(createSessionsTableSQL)).WillReturnResult(okResult())
	mock.ExpectExec(exact(createPromptingTemplatesTableSQL)).WillReturnResult(okResult())
	for _, p := range defaultPrompts() {
		mock.ExpectQuery(checkPromptPattern).WithArgs(p.key).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	}
	for _, stmt := range []string{createContentsTableSQL, createTopicsTableSQL, createUsersTableSQL, createProfilesTableSQL, createProfilesIndexSQL} {
		mock.ExpectExec(exact(stmt)).WillReturnResult(okResult())
	}

	assert.NoError(t, NewInitialMigration().Apply(db))
}

func TestInitialMigration_Apply_FailureAtEachStep(t *testing.T) {
	steps := initialApplySteps()
	for failAt, failing := range steps {
		t.Run(failing.label, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			for i := 0; i < failAt; i++ {
				steps[i].expect(mock, false)
			}
			failing.expect(mock, true)

			err := NewInitialMigration().Apply(db)

			require.Error(t, err)
			assert.ErrorIs(t, err, errInjected)
			assert.Contains(t, err.Error(), failing.errorMsg)
		})
	}
}

func TestInitialMigration_SchemaDefinitions(t *testing.T) {
	// Key properties of the original schema that later migrations rely on
	assert.Contains(t, collapse(createContentsTableSQL), "type TEXT NOT NULL CHECK (type IN ('club-call', 'meetup'))")
	assert.Contains(t, collapse(createContentsTableSQL), "status TEXT NOT NULL DEFAULT 'actual' CHECK (status IN ('finished', 'actual'))")
	assert.Contains(t, collapse(createTopicsTableSQL), "content_id INTEGER REFERENCES contents(id)")
	assert.Contains(t, collapse(createTopicsTableSQL), "user_id INTEGER NOT NULL")
	assert.Contains(t, collapse(createUsersTableSQL), "tg_id BIGINT NOT NULL UNIQUE")
	assert.Contains(t, collapse(createProfilesTableSQL), "user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE")
	assert.Contains(t, collapse(createProfilesTableSQL), "website TEXT")
	assert.Contains(t, collapse(createPromptingTemplatesTableSQL), "template_key TEXT PRIMARY KEY")
	assert.Contains(t, collapse(createSessionsTableSQL), "CREATE TABLE IF NOT EXISTS tg_sessions")
	assert.Equal(t, "CREATE INDEX IF NOT EXISTS profiles_user_id_idx ON profiles(user_id)", collapse(createProfilesIndexSQL))
}

func TestInsertDefaultPromptIfNotExists_InsertsWhenMissing(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(checkPromptPattern).WithArgs("k").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(insertPromptPattern).WithArgs("k", "text").WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, insertDefaultPromptIfNotExists(db, "k", "text"))
}

func TestInsertDefaultPromptIfNotExists_SkipsWhenPresent(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(checkPromptPattern).WithArgs("k").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	assert.NoError(t, insertDefaultPromptIfNotExists(db, "k", "text"))
}

func TestInsertDefaultPromptIfNotExists_CheckError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(checkPromptPattern).WithArgs("k").WillReturnError(errInjected)

	err := insertDefaultPromptIfNotExists(db, "k", "text")

	require.Error(t, err)
	assert.ErrorIs(t, err, errInjected)
	assert.Contains(t, err.Error(), "failed to check if prompt exists")
}

func TestInsertDefaultPromptIfNotExists_InsertError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	mock.ExpectQuery(checkPromptPattern).WithArgs("k").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(insertPromptPattern).WithArgs("k", "text").WillReturnError(errInjected)

	err := insertDefaultPromptIfNotExists(db, "k", "text")

	require.Error(t, err)
	assert.ErrorIs(t, err, errInjected)
	assert.Contains(t, err.Error(), "failed to insert prompt")
}

func TestInsertDefaultPrompts_InsertsAllFourInOrder(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	for _, p := range defaultPrompts() {
		mock.ExpectQuery(checkPromptPattern).WithArgs(p.key).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectExec(insertPromptPattern).WithArgs(p.key, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	}

	assert.NoError(t, insertDefaultPrompts(db))
}

// nonEmptyStringArg is a sqlmock.Argument that accepts non-empty strings and records them.
type nonEmptyStringArg struct {
	collect *[]string
}

func (a nonEmptyStringArg) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return false
	}
	*a.collect = append(*a.collect, s)
	return true
}
