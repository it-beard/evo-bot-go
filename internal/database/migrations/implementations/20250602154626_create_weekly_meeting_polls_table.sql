CREATE TABLE IF NOT EXISTS weekly_meeting_polls (
    id SERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    week_start_date DATE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMENT ON TABLE weekly_meeting_polls IS 'Stores information about weekly meeting polls sent to chats.';
COMMENT ON COLUMN weekly_meeting_polls.message_id IS 'Telegram message ID of the poll.';
COMMENT ON COLUMN weekly_meeting_polls.chat_id IS 'Telegram chat ID where the poll was sent.';
COMMENT ON COLUMN weekly_meeting_polls.week_start_date IS 'Start date of the week for which the poll is relevant (e.g., the Monday of that week).';
