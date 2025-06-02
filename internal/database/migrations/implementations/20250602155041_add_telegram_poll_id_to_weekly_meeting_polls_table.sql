ALTER TABLE weekly_meeting_polls
ADD COLUMN telegram_poll_id TEXT;

-- It's good practice to add an index if this column will be queried frequently.
-- A UNIQUE constraint also creates an index. If polls are unique per chat/message, that's fine.
-- If telegram_poll_id should be globally unique for all polls managed by the bot:
ALTER TABLE weekly_meeting_polls
ADD CONSTRAINT weekly_meeting_polls_telegram_poll_id_unique UNIQUE (telegram_poll_id);

COMMENT ON COLUMN weekly_meeting_polls.telegram_poll_id IS 'Unique poll ID string from Telegram (Poll.Id).';
