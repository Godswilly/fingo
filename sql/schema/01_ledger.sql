CREATE TABLE accounts (
    id         text PRIMARY KEY,
    owner      text NOT NULL,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ledger_journal (
    id          text PRIMARY KEY,
    reference   text NOT NULL,
    description text NOT NULL DEFAULT '',
    metadata    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL
);

CREATE TABLE ledger_postings (
    id            bigserial PRIMARY KEY,
    journal_id    text NOT NULL REFERENCES ledger_journal (id),
    posting_index int NOT NULL,
    account_id    text NOT NULL REFERENCES accounts (id),
    amount        bigint NOT NULL CHECK (amount > 0),
    side          text NOT NULL CHECK (side IN ('debit', 'credit')),
    currency      text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    UNIQUE (journal_id, posting_index)
);

CREATE INDEX idx_ledger_postings_account_id ON ledger_postings (account_id);

CREATE TABLE idempotency_keys (
    key         text PRIMARY KEY,
    fingerprint text NOT NULL,
    status      text NOT NULL CHECK (status IN ('in_progress', 'completed', 'failed')),
    result_ref  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT idempotency_completed_result_ref_check CHECK (
        (status = 'completed' AND result_ref IS NOT NULL AND btrim(result_ref) <> '')
        OR
        (status <> 'completed' AND result_ref IS NULL)
    )
);

CREATE TABLE outbox_events (
    id           bigserial PRIMARY KEY,
    type         text NOT NULL,
    aggregate_id text NOT NULL,
    payload      jsonb NOT NULL,
    occurred_at  timestamptz NOT NULL,
    published_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_outbox_events_unpublished ON outbox_events (created_at)
    WHERE published_at IS NULL;
