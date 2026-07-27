DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'idempotency_completed_result_ref_check'
    ) THEN
        ALTER TABLE idempotency_keys
        ADD CONSTRAINT idempotency_completed_result_ref_check
        CHECK (
            (status = 'completed' AND result_ref IS NOT NULL AND btrim(result_ref) <> '')
            OR
            (status <> 'completed' AND result_ref IS NULL)
        );
    END IF;
END $$;
