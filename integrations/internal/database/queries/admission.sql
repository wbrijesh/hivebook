-- Admission-bucket CRUD (ADR-0036). The adaptive AIMD acquire/release logic is
-- Phase 3; these are the basic rows it operates on. One bucket per limiter scope:
-- 'credential' (a connection's adaptive API budget L) or 'tenant' (the hard
-- in-flight cap).

-- name: UpsertAdmissionBucket :exec
-- Seed a bucket (the connector's RateBudget seed) or refresh its connector tag,
-- leaving the learned limit and live in_flight untouched on conflict.
INSERT INTO admission_bucket (scope_type, scope_key, connector_id, concurrency_limit)
VALUES (@scope_type, @scope_key, @connector_id, @concurrency_limit)
ON CONFLICT (scope_type, scope_key) DO UPDATE SET
    connector_id = EXCLUDED.connector_id,
    updated_at = now();

-- name: GetAdmissionBucket :one
SELECT * FROM admission_bucket
WHERE scope_type = @scope_type AND scope_key = @scope_key;

-- name: SetAdmissionLimit :exec
-- Persist the AIMD-adjusted limit and (optionally) a Retry-After cooldown window.
UPDATE admission_bucket
SET concurrency_limit = @concurrency_limit, cooldown_until = @cooldown_until, updated_at = now()
WHERE scope_type = @scope_type AND scope_key = @scope_key;

-- name: SetAdmissionInFlight :exec
-- Set the live in-flight count (the acquire/release accounting lands here in Phase 3).
UPDATE admission_bucket
SET in_flight = @in_flight, updated_at = now()
WHERE scope_type = @scope_type AND scope_key = @scope_key;

-- name: DeleteAdmissionBucket :exec
DELETE FROM admission_bucket
WHERE scope_type = @scope_type AND scope_key = @scope_key;

-- name: SeedAdmissionBucket :exec
-- Lazily create a bucket at the connector's seed concurrency on first use, leaving an
-- existing bucket's learned limit + live in_flight untouched (ADR-0036). Distinct from
-- UpsertAdmissionBucket only in that it never bumps updated_at on conflict — a no-op
-- seed must not look like activity.
INSERT INTO admission_bucket (scope_type, scope_key, connector_id, concurrency_limit)
VALUES (@scope_type, @scope_key, @connector_id, @concurrency_limit)
ON CONFLICT (scope_type, scope_key) DO NOTHING;

-- name: AcquireAdmission :one
-- Atomic AIMD acquire: take a slot iff one is free and no cooldown is in effect. A
-- single UPDATE is the compare-and-swap — the WHERE is the gate, RETURNING proves we
-- won the row. No row updated → caller backs off (the workflow re-schedules the unit).
UPDATE admission_bucket
SET in_flight = in_flight + 1, updated_at = now()
WHERE scope_type = @scope_type AND scope_key = @scope_key
  AND in_flight < concurrency_limit
  AND (cooldown_until IS NULL OR cooldown_until <= now())
RETURNING in_flight;

-- name: ReleaseAdmission :exec
-- Free a slot, clamped at zero (a double-release or a release after a reset must never
-- drive in_flight negative).
UPDATE admission_bucket
SET in_flight = GREATEST(in_flight - 1, 0), updated_at = now()
WHERE scope_type = @scope_type AND scope_key = @scope_key;

-- name: AdmissionAdditiveIncrease :exec
-- Additive increase on sustained success: grow L by one toward the connector's hard
-- cap, only while we're near the ceiling (in_flight close to the limit) so an idle
-- bucket doesn't drift its limit upward without real load.
UPDATE admission_bucket
SET concurrency_limit = LEAST(concurrency_limit + 1, @max_limit), updated_at = now()
WHERE scope_type = @scope_type AND scope_key = @scope_key
  AND concurrency_limit < @max_limit
  AND in_flight >= concurrency_limit - 1;

-- name: AdmissionMultiplicativeDecrease :exec
-- Multiplicative decrease + cooldown on a throttle (429 / secondary-403 / Retry-After):
-- halve L (floor 1) and park the bucket until the Retry-After window passes.
UPDATE admission_bucket
SET concurrency_limit = GREATEST(concurrency_limit / 2, 1),
    cooldown_until = @cooldown_until,
    updated_at = now()
WHERE scope_type = @scope_type AND scope_key = @scope_key;

-- name: GetAdmissionLimit :one
-- The current learned limit L — the workflow's Capacity macro reads this per scope.
SELECT concurrency_limit FROM admission_bucket
WHERE scope_type = @scope_type AND scope_key = @scope_key;
