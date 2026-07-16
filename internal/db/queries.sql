-- name: CreateReminder :one
INSERT INTO reminders (chat_id, text, fire_at)
VALUES ($1, $2, $3)
RETURNING *;
-- name: GetDueReminders :many
SELECT *
FROM reminders
WHERE sent = FALSE
    AND fire_at <= now()
ORDER BY fire_at
LIMIT 100;
-- name: MarkSent :exec
UPDATE reminders
SET sent = TRUE
WHERE id = $1;
-- name: ListPending :many
SELECT *
FROM reminders
WHERE chat_id = $1
    AND sent = FALSE
ORDER BY fire_at;