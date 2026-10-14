-- name: CreateMonitorCheck :one
INSERT INTO monitor_checks (id, workspace_id, property_id, name, url, host, method, headers, expected_statuses, keyword, keyword_absent,
    timeout_seconds, interval_seconds, failures_before_down, cert_warning_days, max_redirects, service, paused, next_run_at, created_by, created_at, updated_at)
VALUES (@id, @workspace_id, sqlc.narg(property_id), @name, @url, @host, @method, @headers, @expected_statuses, sqlc.narg(keyword), @keyword_absent,
    @timeout_seconds, @interval_seconds, @failures_before_down, @cert_warning_days, @max_redirects, sqlc.narg(service), @paused, @now, sqlc.narg(created_by), @now, @now)
RETURNING *;

-- name: GetMonitorCheck :one
SELECT * FROM monitor_checks WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockMonitorCheck :one
SELECT * FROM monitor_checks WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListMonitorChecks :many
SELECT * FROM monitor_checks
WHERE workspace_id = @workspace_id AND (sqlc.narg(property_id)::uuid IS NULL OR property_id = sqlc.narg(property_id)::uuid)
ORDER BY name, id;

-- name: CountMonitorChecks :one
SELECT count(*) FROM monitor_checks WHERE workspace_id = @workspace_id;

-- name: CountMonitorChecksOnHost :one
SELECT count(*) FROM monitor_checks WHERE workspace_id = @workspace_id AND host = @host AND id <> @id;

-- name: UpdateMonitorCheck :one
UPDATE monitor_checks SET property_id = sqlc.narg(property_id), name = @name, url = @url, host = @host, method = @method, headers = @headers,
    expected_statuses = @expected_statuses, keyword = sqlc.narg(keyword), keyword_absent = @keyword_absent, timeout_seconds = @timeout_seconds,
    interval_seconds = @interval_seconds, failures_before_down = @failures_before_down, cert_warning_days = @cert_warning_days,
    max_redirects = @max_redirects, service = sqlc.narg(service), paused = @paused, next_run_at = @next_run_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteMonitorCheck :execrows
DELETE FROM monitor_checks WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteMonitorCheckResults :execrows
DELETE FROM monitor_check_results WHERE workspace_id = @workspace_id AND check_id = @check_id;

-- name: DeleteWorkspaceMonitorCheckResults :execrows
DELETE FROM monitor_check_results WHERE workspace_id = @workspace_id;

-- name: ListDueMonitorChecks :many
-- The due job lists the checks to run across workspaces (architecture.md, hosting for others);
-- everything after it is scoped by the workspace each row names.
SELECT c.workspace_id, c.id, c.next_run_at, c.interval_seconds FROM monitor_checks c
JOIN workspaces w ON w.id = c.workspace_id AND w.deleted_at IS NULL
WHERE NOT c.paused AND c.next_run_at <= @now
ORDER BY c.next_run_at
LIMIT @lim
FOR UPDATE OF c SKIP LOCKED;

-- name: SetMonitorCheckNextRun :exec
UPDATE monitor_checks SET next_run_at = @next_run_at WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkMonitorCheckRun :exec
UPDATE monitor_checks SET last_run_at = greatest(last_run_at, @at), cert_expires_at = coalesce(sqlc.narg(cert_expires_at), cert_expires_at)
WHERE workspace_id = @workspace_id AND id = @id;

-- name: InsertMonitorCheckResult :exec
INSERT INTO monitor_check_results (workspace_id, check_id, at, ok, status, error, detail, retried, redirects, dns_us, connect_us, tls_us, ttfb_us,
    total_us, answered, cert_days_left, seconds)
VALUES (@workspace_id, @check_id, @at, @ok, @status, sqlc.narg(error), sqlc.narg(detail), @retried, @redirects, @dns_us, @connect_us, @tls_us, @ttfb_us,
    @total_us, @answered, sqlc.narg(cert_days_left), @seconds)
ON CONFLICT (check_id, at) DO NOTHING;

-- name: ListPendingMonitorMinutes :many
-- Minutes with runs not judged yet, across workspaces: whether most of the server's checks failed in
-- a minute is a fact about the server, not a workspace (architecture.md, hosting for others).
SELECT DISTINCT date_trunc('minute', at)::timestamptz AS minute FROM monitor_check_results
WHERE state IS NULL AND at < @before
ORDER BY 1
LIMIT 20;

-- name: CountMonitorMinute :one
SELECT count(*)::bigint AS runs, count(*) FILTER (WHERE NOT ok)::bigint AS failures FROM monitor_check_results
WHERE at >= @from_at AND at < @to_at;

-- name: ListPendingMonitorResults :many
SELECT workspace_id, check_id, at, ok, cert_days_left FROM monitor_check_results
WHERE state IS NULL AND at >= @from_at AND at < @to_at
ORDER BY check_id, at;

-- name: SetMonitorResultState :exec
UPDATE monitor_check_results SET state = @state WHERE workspace_id = @workspace_id AND check_id = @check_id AND at = @at;

-- name: SetMonitorCheckState :one
UPDATE monitor_checks SET state = @state, state_since = sqlc.narg(state_since), failures_in_row = @failures_in_row
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListMonitorCheckResults :many
SELECT * FROM monitor_check_results
WHERE workspace_id = @workspace_id AND check_id = @check_id AND (sqlc.narg(before)::timestamptz IS NULL OR at < sqlc.narg(before)::timestamptz)
ORDER BY at DESC
LIMIT @lim;

-- name: ListLastMonitorCheckResults :many
SELECT DISTINCT ON (check_id) * FROM monitor_check_results
WHERE workspace_id = @workspace_id AND check_id = ANY(@check_ids::uuid[]) AND at >= @since
ORDER BY check_id, at DESC;

-- name: ListMonitorChecksToFold :many
SELECT id, created_at, folded_until FROM monitor_checks
WHERE workspace_id = @workspace_id AND (folded_until IS NULL OR folded_until < @last_day::date)
ORDER BY id;

-- name: CountPendingMonitorDay :one
SELECT count(*) FROM monitor_check_results
WHERE workspace_id = @workspace_id AND check_id = @check_id AND at >= @from_at AND at < @to_at AND state IS NULL;

-- name: ListMonitorWorkspaceIDs :many
-- Maintenance finds the live workspaces with checks, alert rules or services; everything after it
-- is scoped by workspace.
SELECT id FROM workspaces w WHERE w.deleted_at IS NULL
  AND (EXISTS (SELECT 1 FROM monitor_checks c WHERE c.workspace_id = w.id) OR EXISTS (SELECT 1 FROM monitor_alert_rules r WHERE r.workspace_id = w.id)
    OR EXISTS (SELECT 1 FROM monitor_services s WHERE s.workspace_id = w.id))
ORDER BY id;

-- name: DeleteOldMonitorCheckDays :execrows
DELETE FROM monitor_check_days WHERE workspace_id = @workspace_id AND day < @before;

-- name: CreateMonitorAlertRule :one
INSERT INTO monitor_alert_rules (id, workspace_id, name, kind, property_id, route, service, check_id, service_id, dependency_id, threshold, window_minutes,
    min_requests, recipients, enabled, created_by, created_at, updated_at)
VALUES (@id, @workspace_id, @name, @kind, sqlc.narg(property_id), sqlc.narg(route), sqlc.narg(service), sqlc.narg(check_id), sqlc.narg(service_id),
    sqlc.narg(dependency_id), sqlc.narg(threshold), @window_minutes, @min_requests, @recipients, @enabled, sqlc.narg(created_by), @now, @now)
RETURNING *;

-- name: GetMonitorAlertRule :one
SELECT * FROM monitor_alert_rules WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockMonitorAlertRule :one
SELECT * FROM monitor_alert_rules WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListMonitorAlertRules :many
SELECT * FROM monitor_alert_rules
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(property_id)::uuid IS NULL OR property_id = sqlc.narg(property_id)::uuid)
  AND (sqlc.narg(check_id)::uuid IS NULL OR check_id = sqlc.narg(check_id)::uuid)
  AND (sqlc.narg(service_id)::uuid IS NULL OR service_id = sqlc.narg(service_id)::uuid)
ORDER BY created_at, id;

-- name: CountMonitorAlertRules :one
SELECT count(*) FROM monitor_alert_rules WHERE workspace_id = @workspace_id;

-- name: UpdateMonitorAlertRule :one
UPDATE monitor_alert_rules SET name = @name, threshold = sqlc.narg(threshold), window_minutes = @window_minutes, min_requests = @min_requests,
    recipients = @recipients, enabled = @enabled, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteMonitorAlertRule :execrows
DELETE FROM monitor_alert_rules WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountOpenMonitorAlerts :many
SELECT rule_id, count(*)::bigint AS firing FROM monitor_alerts
WHERE workspace_id = @workspace_id AND resolved_at IS NULL
GROUP BY rule_id;

-- name: ListMonitorAlertWorkspaces :many
-- The alerts job finds the live workspaces with enabled rules; everything after it is scoped by
-- workspace (architecture.md, hosting for others).
SELECT DISTINCT r.workspace_id FROM monitor_alert_rules r
JOIN workspaces w ON w.id = r.workspace_id AND w.deleted_at IS NULL
WHERE r.enabled
ORDER BY r.workspace_id;

-- name: ListEnabledMonitorAlertRules :many
SELECT r.* FROM monitor_alert_rules r
LEFT JOIN analytics_properties p ON p.workspace_id = r.workspace_id AND p.id = r.property_id
WHERE r.workspace_id = @workspace_id AND r.enabled AND (r.property_id IS NULL OR p.deleted_at IS NULL)
ORDER BY r.created_at, r.id;

-- name: OpenMonitorAlert :one
INSERT INTO monitor_alerts (id, workspace_id, rule_id, key, kind, property_id, check_id, service_id, dependency_id, value, threshold, baseline, window_minutes,
    text, fired_at, created_at)
VALUES (@id, @workspace_id, @rule_id, @key, @kind, sqlc.narg(property_id), sqlc.narg(check_id), sqlc.narg(service_id), sqlc.narg(dependency_id), @value,
    sqlc.narg(threshold), sqlc.narg(baseline), @window_minutes, @text, @fired_at, @now)
ON CONFLICT (rule_id, key) WHERE resolved_at IS NULL DO NOTHING
RETURNING *;

-- name: ResolveMonitorAlert :one
UPDATE monitor_alerts SET resolved_at = @resolved_at, resolved_value = sqlc.narg(resolved_value)
WHERE workspace_id = @workspace_id AND id = @id AND resolved_at IS NULL
RETURNING *;

-- name: ResolveRuleAlertsQuietly :execrows
UPDATE monitor_alerts SET resolved_at = @resolved_at
WHERE workspace_id = @workspace_id AND rule_id = @rule_id AND resolved_at IS NULL;

-- name: ListOpenMonitorAlerts :many
SELECT * FROM monitor_alerts WHERE workspace_id = @workspace_id AND rule_id = @rule_id AND resolved_at IS NULL ORDER BY fired_at, id;

-- name: SetMonitorAlertAnnotation :one
UPDATE monitor_alerts SET annotation_id = @annotation_id WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListMonitorAlerts :many
SELECT a.*, r.name AS rule_name, r.route AS rule_route, r.service AS rule_service FROM monitor_alerts a
JOIN monitor_alert_rules r ON r.workspace_id = a.workspace_id AND r.id = a.rule_id
WHERE a.workspace_id = @workspace_id
  AND (@all_properties::boolean OR a.property_id IS NULL OR a.property_id = ANY(@property_ids::uuid[]))
  AND (sqlc.narg(state)::text IS NULL OR (sqlc.narg(state)::text = 'firing') = (a.resolved_at IS NULL))
  AND (sqlc.narg(property_id)::uuid IS NULL OR a.property_id = sqlc.narg(property_id)::uuid)
  AND (sqlc.narg(check_id)::uuid IS NULL OR a.check_id = sqlc.narg(check_id)::uuid)
  AND (sqlc.narg(rule_id)::uuid IS NULL OR a.rule_id = sqlc.narg(rule_id)::uuid)
  AND (sqlc.narg(service_id)::uuid IS NULL OR a.service_id = sqlc.narg(service_id)::uuid)
  AND (sqlc.narg(before)::timestamptz IS NULL OR a.fired_at < sqlc.narg(before)::timestamptz)
ORDER BY a.fired_at DESC, a.id DESC
LIMIT @lim;

-- name: ListMonitorAlertsFired :many
SELECT a.*, r.name AS rule_name FROM monitor_alerts a
JOIN monitor_alert_rules r ON r.workspace_id = a.workspace_id AND r.id = a.rule_id
WHERE a.workspace_id = @workspace_id AND a.property_id = @property_id AND a.fired_at >= @from_at AND a.fired_at < @to_at
ORDER BY a.fired_at, a.id;

-- name: DeleteOldMonitorAlerts :execrows
DELETE FROM monitor_alerts WHERE workspace_id = @workspace_id AND resolved_at < @before;

-- name: CreateMonitorAlertAnnotation :one
INSERT INTO analytics_annotations (id, workspace_id, property_id, kind, starts_on, ends_on, text, alert_id, at, service, created_at, updated_at)
VALUES (@id, @workspace_id, @property_id, 'alert', @day, @day, @text, @alert_id, @at, sqlc.narg(service), @now, @now)
RETURNING *;

-- name: ListMonitorRecipients :many
-- The rule's recipients who are still members and see what it is about: with a property, owners
-- and admins, and agents of its linked inbox; without one, every member.
SELECT m.id AS member_id, m.person_id, pe.email, pe.locale FROM members m
JOIN people pe ON pe.id = m.person_id
WHERE m.workspace_id = @workspace_id AND m.id = ANY(@member_ids::uuid[])
  AND (NOT @scoped::boolean OR m.role IN ('owner', 'admin') OR EXISTS (
      SELECT 1 FROM inbox_members im WHERE im.workspace_id = m.workspace_id AND im.inbox_id = sqlc.narg(inbox_id)::uuid AND im.member_id = m.id))
ORDER BY m.id;
