package implementations

import (
	"database/sql"
)

type AddIsClubMemberToUsers struct {
	BaseMigration
}

func NewAddIsClubMemberToUsers() *AddIsClubMemberToUsers {
	return &AddIsClubMemberToUsers{
		BaseMigration: BaseMigration{
			name:      "add_is_club_member_to_users",
			timestamp: "20250609",
		},
	}
}

func (m *AddIsClubMemberToUsers) Apply(db *sql.DB) error {
	sql := `ALTER TABLE users ADD COLUMN is_club_member BOOLEAN DEFAULT TRUE`
	_, err := db.Exec(sql)
	return err
}

func (m *AddIsClubMemberToUsers) Rollback(db *sql.DB) error {
	sql := `ALTER TABLE users DROP COLUMN IF EXISTS is_club_member`
	_, err := db.Exec(sql)
	return err
}
