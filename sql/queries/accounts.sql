-- name: LockAccount :one
SELECT id, owner, is_active, created_at
FROM accounts
WHERE id = $1
FOR UPDATE;

-- name: GetAccountBalance :one
SELECT COALESCE(SUM(CASE WHEN side = 'credit' THEN amount ELSE -amount END), 0)::bigint AS balance
FROM ledger_postings
WHERE account_id = $1;
