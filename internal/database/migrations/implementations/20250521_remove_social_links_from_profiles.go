package implementations

import (
	"database/sql"
)

type RemoveSocialLinksFromProfiles struct {
	BaseMigration
}

func NewRemoveSocialLinksFromProfiles() *RemoveSocialLinksFromProfiles {
	return &RemoveSocialLinksFromProfiles{
		BaseMigration: BaseMigration{
			name:      "remove_social_links_from_profiles",
			timestamp: "20250521",
		},
	}
}

func (m *RemoveSocialLinksFromProfiles) Apply(db *sql.DB) error {
	sql := `ALTER TABLE profiles 
			DROP COLUMN IF EXISTS linkedin,
			DROP COLUMN IF EXISTS github,
			DROP COLUMN IF EXISTS freelink`
	_, err := db.Exec(sql)
	return err
}

func (m *RemoveSocialLinksFromProfiles) Rollback(db *sql.DB) error {
	sql := `ALTER TABLE profiles 
			ADD COLUMN IF NOT EXISTS linkedin TEXT DEFAULT '',
			ADD COLUMN IF NOT EXISTS github TEXT DEFAULT '',
			ADD COLUMN IF NOT EXISTS freelink TEXT DEFAULT ''`
	_, err := db.Exec(sql)
	return err
}
