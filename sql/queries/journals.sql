-- name: InsertJournal :exec
INSERT INTO ledger_journal (id, reference, description, metadata, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertPosting :exec
INSERT INTO ledger_postings (journal_id, posting_index, account_id, amount, side, currency)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetJournal :one
SELECT id, reference, description, metadata, created_at
FROM ledger_journal
WHERE id = $1;

-- name: ListJournalPostings :many
SELECT account_id, amount, side, currency
FROM ledger_postings
WHERE journal_id = $1
ORDER BY posting_index ASC;
