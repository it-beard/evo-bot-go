package implementations

import (
	"database/sql"
)

type AddGroupMessagesTable struct {
	BaseMigration
}

func NewAddGroupMessagesTable() *AddGroupMessagesTable {
	return &AddGroupMessagesTable{
		BaseMigration: BaseMigration{
			name:      "add_group_messages_table",
			timestamp: "20250918",
		},
	}
}

func (m *AddGroupMessagesTable) Apply(db *sql.DB) error {
	sql := `
	CREATE TABLE IF NOT EXISTS group_messages (
		id SERIAL PRIMARY KEY,
		message_id BIGINT NOT NULL,
		message_text TEXT NOT NULL,
		reply_to_message_id BIGINT,
		user_tg_id BIGINT NOT NULL,
		group_topic_id BIGINT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		
		-- Create indexes for better query performance
		CONSTRAINT unique_message_id UNIQUE (message_id)
	);
	
	-- Create indexes
	CREATE INDEX IF NOT EXISTS idx_group_messages_user_tg_id ON group_messages(user_tg_id);
	CREATE INDEX IF NOT EXISTS idx_group_messages_group_topic_id ON group_messages(group_topic_id);
	CREATE INDEX IF NOT EXISTS idx_group_messages_reply_to_message_id ON group_messages(reply_to_message_id);
	CREATE INDEX IF NOT EXISTS idx_group_messages_created_at ON group_messages(created_at);
	`
	_, err := db.Exec(sql)
	return err
}

func (m *AddGroupMessagesTable) Rollback(db *sql.DB) error {
	sql := `DROP TABLE IF EXISTS group_messages;`
	_, err := db.Exec(sql)
	return err
}
