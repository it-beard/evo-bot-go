package implementations

import (
	"database/sql"
)

type RenameWebsiteToFreelink struct {
	BaseMigration
}

func NewRenameWebsiteToFreelink() *RenameWebsiteToFreelink {
	return &RenameWebsiteToFreelink{
		BaseMigration: BaseMigration{
			name:      "rename_website_to_freelink",
			timestamp: "20250520",
		},
	}
}

func (m *RenameWebsiteToFreelink) Apply(db *sql.DB) error {
	sql := `ALTER TABLE profiles RENAME COLUMN website TO freelink`
	_, err := db.Exec(sql)
	return err
}

func (m *RenameWebsiteToFreelink) Rollback(db *sql.DB) error {
	sql := `ALTER TABLE profiles RENAME COLUMN freelink TO website`
	_, err := db.Exec(sql)
	return err
}
