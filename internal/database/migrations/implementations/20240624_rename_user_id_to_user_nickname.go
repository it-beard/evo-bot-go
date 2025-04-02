package implementations

import (
	"database/sql"
)

// RenameUserIdToUserNickname renames the user_id column to user_nickname in topics table
type RenameUserIdToUserNickname struct {
	BaseMigration
}

// NewRenameUserIdToUserNickname creates a new migration instance
func NewRenameUserIdToUserNickname() *RenameUserIdToUserNickname {
	return &RenameUserIdToUserNickname{
		BaseMigration: BaseMigration{
			name:      "rename_user_id_to_user_nickname",
			timestamp: "20240624",
		},
	}
}

// Apply executes the migration
func (m *RenameUserIdToUserNickname) Apply(db *sql.DB) error {
	sql := `ALTER TABLE topics RENAME COLUMN user_id TO user_nickname`
	_, err := db.Exec(sql)
	return err
}

// Rollback reverts the migration
func (m *RenameUserIdToUserNickname) Rollback(db *sql.DB) error {
	sql := `ALTER TABLE topics RENAME COLUMN user_nickname TO user_id`
	_, err := db.Exec(sql)
	return err
}
