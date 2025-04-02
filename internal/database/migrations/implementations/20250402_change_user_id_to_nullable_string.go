package implementations

import (
	"database/sql"
)

// ChangeUserIdToNullableString changes the user_id field in topics table
// from INTEGER NOT NULL to nullable TEXT
type ChangeUserIdToNullableString struct {
	BaseMigration
}

// NewChangeUserIdToNullableString creates a new migration instance
func NewChangeUserIdToNullableString() *ChangeUserIdToNullableString {
	return &ChangeUserIdToNullableString{
		BaseMigration: BaseMigration{
			name:      "change_user_id_to_nullable_string",
			timestamp: "20250402", // Today's date in YYYYMMDD format
		},
	}
}

// Apply executes the migration
func (m *ChangeUserIdToNullableString) Apply(db *sql.DB) error {
	sql := `ALTER TABLE topics 
			ALTER COLUMN user_id TYPE TEXT USING user_id::TEXT,
			ALTER COLUMN user_id DROP NOT NULL`
	_, err := db.Exec(sql)
	return err
}

// Rollback reverts the migration
func (m *ChangeUserIdToNullableString) Rollback(db *sql.DB) error {
	sql := `ALTER TABLE topics 
			ALTER COLUMN user_id TYPE INTEGER USING user_id::INTEGER,
			ALTER COLUMN user_id SET NOT NULL`
	_, err := db.Exec(sql)
	return err
}
