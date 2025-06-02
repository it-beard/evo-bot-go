CREATE TABLE IF NOT EXISTS weekly_meeting_participants (
    id SERIAL PRIMARY KEY,
    poll_id INTEGER NOT NULL REFERENCES weekly_meeting_polls(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_participating BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (poll_id, user_id)
);

COMMENT ON TABLE weekly_meeting_participants IS 'Stores user participation status for weekly meeting polls.';
COMMENT ON COLUMN weekly_meeting_participants.poll_id IS 'Foreign key referencing the specific poll in weekly_meeting_polls.';
COMMENT ON COLUMN weekly_meeting_participants.user_id IS 'Foreign key referencing the user in the users table.';
COMMENT ON COLUMN weekly_meeting_participants.is_participating IS 'True if the user is participating, False otherwise.';
COMMENT ON COLUMN weekly_meeting_participants.updated_at IS 'Timestamp of the last update to the participation status.';

-- Optional: Create a trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_weekly_meeting_participants_updated_at
    BEFORE UPDATE ON weekly_meeting_participants
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
