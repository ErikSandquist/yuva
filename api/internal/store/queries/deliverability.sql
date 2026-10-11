-- name: StartDeliverabilityCheck :one
INSERT INTO deliverability_checks (workspace_id, channel_id, probe_token, state, result, created_at, finished_at)
VALUES (@workspace_id, @channel_id, @probe_token, 'queued', @result, @now, NULL)
ON CONFLICT (workspace_id, channel_id) DO UPDATE SET
    probe_token = excluded.probe_token, state = 'queued', result = excluded.result,
    created_at = excluded.created_at, finished_at = NULL
RETURNING *;

-- name: GetDeliverabilityCheck :one
SELECT * FROM deliverability_checks WHERE workspace_id = $1 AND channel_id = $2;

-- name: GetDeliverabilityProbe :one
SELECT * FROM deliverability_checks WHERE workspace_id = $1 AND probe_token = $2;

-- name: SaveDeliverabilitySent :one
UPDATE deliverability_checks SET result = result || @result,
    state = CASE WHEN state = 'queued' THEN 'sent' ELSE state END
WHERE workspace_id = @workspace_id AND channel_id = @channel_id AND probe_token = @probe_token
RETURNING *;

-- name: FailDeliverabilityCheck :one
UPDATE deliverability_checks SET result = result || @result, state = 'failed', finished_at = @now
WHERE workspace_id = @workspace_id AND channel_id = @channel_id AND probe_token = @probe_token AND state = 'queued'
RETURNING *;

-- name: FinishDeliverabilityProbe :one
UPDATE deliverability_checks SET result = result || @result, state = @state, finished_at = @now
WHERE workspace_id = @workspace_id AND probe_token = @probe_token AND state IN ('queued', 'sent')
RETURNING *;
