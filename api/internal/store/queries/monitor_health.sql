-- name: CreateMonitorService :one
INSERT INTO monitor_services (id, workspace_id, name, environment, origin, property_id, mode, health_url, interval_seconds, timeout_seconds,
    silence_seconds, error_rate_threshold, p95_threshold_ms, next_pull_at, created_by, created_at, updated_at)
VALUES (@id, @workspace_id, @name, @environment, @origin, sqlc.narg(property_id), @mode, sqlc.narg(health_url), @interval_seconds, @timeout_seconds,
    sqlc.narg(silence_seconds), @error_rate_threshold, sqlc.narg(p95_threshold_ms), sqlc.narg(next_pull_at), sqlc.narg(created_by), @now, @now)
ON CONFLICT (workspace_id, name, environment) DO NOTHING
RETURNING *;

-- name: GetMonitorService :one
SELECT * FROM monitor_services WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockMonitorService :one
SELECT * FROM monitor_services WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: GetMonitorServiceByName :one
SELECT * FROM monitor_services WHERE workspace_id = @workspace_id AND name = @name AND environment = @environment;

-- name: ListMonitorServices :many
SELECT * FROM monitor_services
WHERE workspace_id = @workspace_id AND (sqlc.narg(property_id)::uuid IS NULL OR property_id = sqlc.narg(property_id)::uuid)
ORDER BY name, environment, id;

-- name: CountMonitorServices :one
SELECT count(*) FROM monitor_services WHERE workspace_id = @workspace_id;

-- name: UpdateMonitorService :one
UPDATE monitor_services SET property_id = sqlc.narg(property_id), mode = @mode, health_url = sqlc.narg(health_url), interval_seconds = @interval_seconds,
    timeout_seconds = @timeout_seconds, silence_seconds = sqlc.narg(silence_seconds), error_rate_threshold = @error_rate_threshold,
    p95_threshold_ms = sqlc.narg(p95_threshold_ms), next_pull_at = sqlc.narg(next_pull_at), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteMonitorService :execrows
DELETE FROM monitor_services WHERE workspace_id = @workspace_id AND id = @id;

-- name: RemoveServiceFromIncidents :exec
UPDATE monitor_incidents SET service_ids = array_remove(service_ids, @service_id::uuid),
    dependency_ids = ARRAY(SELECT unnest(dependency_ids) EXCEPT SELECT unnest(@dependency_ids::uuid[]))
WHERE workspace_id = @workspace_id AND (@service_id::uuid = ANY(service_ids) OR dependency_ids && @dependency_ids::uuid[]);

-- name: SetMonitorServiceReport :exec
UPDATE monitor_services SET last_report_at = @at, last_status = @status, last_message = sqlc.narg(last_message), version = coalesce(sqlc.narg(version), version)
WHERE workspace_id = @workspace_id AND id = @id;

-- name: NoteMonitorServiceRecords :exec
UPDATE monitor_services SET version = coalesce(sqlc.narg(version), version), property_id = coalesce(property_id, sqlc.narg(property_id))
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetMonitorServiceState :one
UPDATE monitor_services SET state = @state, state_since = sqlc.narg(state_since), reason = sqlc.narg(reason), better_state = sqlc.narg(better_state),
    better_in_row = @better_in_row
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListDueMonitorPulls :many
-- The due job lists the pull services to call across workspaces (architecture.md, hosting for
-- others); everything after it is scoped by the workspace each row names.
SELECT s.workspace_id, s.id, s.next_pull_at, s.interval_seconds FROM monitor_services s
JOIN workspaces w ON w.id = s.workspace_id AND w.deleted_at IS NULL
WHERE s.mode = 'pull' AND s.next_pull_at <= @now
ORDER BY s.next_pull_at
LIMIT @lim
FOR UPDATE OF s SKIP LOCKED;

-- name: SetMonitorServiceNextPull :exec
UPDATE monitor_services SET next_pull_at = @next_pull_at WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListMonitorHealthWorkspaces :many
-- The states job finds the live workspaces with services or open incidents; everything after it is
-- scoped by workspace (architecture.md, hosting for others).
SELECT id FROM workspaces w WHERE w.deleted_at IS NULL
  AND (EXISTS (SELECT 1 FROM monitor_services s WHERE s.workspace_id = w.id)
    OR EXISTS (SELECT 1 FROM monitor_incidents i WHERE i.workspace_id = w.id AND i.resolved_at IS NULL))
ORDER BY id;

-- name: ListMonitorDependencies :many
SELECT * FROM monitor_dependencies WHERE workspace_id = @workspace_id AND service_id = ANY(@service_ids::uuid[]) ORDER BY service_id, name;

-- name: GetMonitorDependency :one
SELECT * FROM monitor_dependencies WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountMonitorDependencies :one
SELECT count(*) FROM monitor_dependencies WHERE workspace_id = @workspace_id AND service_id = @service_id;

-- name: UpsertMonitorDependency :one
INSERT INTO monitor_dependencies (id, workspace_id, service_id, name, kind, critical, status, latency_ms, message, reported_at, created_at)
VALUES (@id, @workspace_id, @service_id, @name, @kind, @critical, @status, sqlc.narg(latency_ms), sqlc.narg(message), @at, @at)
ON CONFLICT (service_id, name) DO UPDATE SET kind = excluded.kind, critical = excluded.critical, status = excluded.status,
    latency_ms = excluded.latency_ms, message = excluded.message, reported_at = excluded.reported_at
RETURNING *;

-- name: SetMonitorDependencyState :exec
UPDATE monitor_dependencies SET state = @state, state_since = sqlc.narg(state_since), better_state = sqlc.narg(better_state), better_in_row = @better_in_row
WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpsertMonitorInstance :exec
-- A new instance is taken while the service has fewer than 200.
INSERT INTO monitor_instances (workspace_id, service_id, instance, version, status, first_seen_at, last_seen_at)
SELECT @workspace_id, @service_id, @instance, sqlc.narg(version), sqlc.narg(status), @at, @at
WHERE (SELECT count(*) FROM monitor_instances WHERE workspace_id = @workspace_id AND service_id = @service_id) < 200
   OR EXISTS (SELECT 1 FROM monitor_instances WHERE workspace_id = @workspace_id AND service_id = @service_id AND instance = @instance)
ON CONFLICT (service_id, instance) DO UPDATE SET version = coalesce(excluded.version, monitor_instances.version),
    status = coalesce(excluded.status, monitor_instances.status), last_seen_at = greatest(monitor_instances.last_seen_at, excluded.last_seen_at);

-- name: ListMonitorInstances :many
SELECT * FROM monitor_instances WHERE workspace_id = @workspace_id AND service_id = @service_id ORDER BY last_seen_at DESC, instance;

-- name: CountMonitorInstances :many
SELECT service_id, count(*)::bigint AS n FROM monitor_instances WHERE workspace_id = @workspace_id GROUP BY service_id;

-- name: DeleteSilentMonitorInstances :execrows
DELETE FROM monitor_instances WHERE workspace_id = @workspace_id AND last_seen_at < @before;

-- name: InsertMonitorStateChange :exec
INSERT INTO monitor_state_changes (id, workspace_id, service_id, dependency_id, state, previous, reason, maintenance, at)
VALUES (@id, @workspace_id, @service_id, sqlc.narg(dependency_id), @state, @previous, sqlc.narg(reason), @maintenance, @at);

-- name: ListMonitorStateChanges :many
SELECT c.*, d.name AS dependency_name FROM monitor_state_changes c
LEFT JOIN monitor_dependencies d ON d.workspace_id = c.workspace_id AND d.id = c.dependency_id
WHERE c.workspace_id = @workspace_id AND c.service_id = @service_id
  AND (sqlc.narg(dependency_id)::uuid IS NULL OR c.dependency_id = sqlc.narg(dependency_id)::uuid)
  AND (sqlc.narg(before)::timestamptz IS NULL OR c.at < sqlc.narg(before)::timestamptz)
ORDER BY c.at DESC, c.id DESC
LIMIT @lim;

-- name: ListSubjectStateChanges :many
-- One subject's changes from the last one before from_at on: the service's own when dependency_id
-- is null.
SELECT * FROM (
    (SELECT * FROM monitor_state_changes c
     WHERE c.workspace_id = @workspace_id AND c.service_id = @service_id AND c.dependency_id IS NOT DISTINCT FROM sqlc.narg(dependency_id)::uuid
       AND c.at < @from_at
     ORDER BY c.at DESC LIMIT 1)
    UNION ALL
    (SELECT * FROM monitor_state_changes c
     WHERE c.workspace_id = @workspace_id AND c.service_id = @service_id AND c.dependency_id IS NOT DISTINCT FROM sqlc.narg(dependency_id)::uuid
       AND c.at >= @from_at)
) x ORDER BY at, id;

-- name: DeleteOldMonitorStateChanges :execrows
DELETE FROM monitor_state_changes WHERE workspace_id = @workspace_id AND at < @before;

-- name: ListMonitorHealthHoursAt :many
SELECT * FROM monitor_health_hours WHERE workspace_id = @workspace_id AND service_id = @service_id AND hour = @hour;

-- name: UpsertMonitorHealthHour :exec
INSERT INTO monitor_health_hours (workspace_id, service_id, dependency, hour, reports, healthy, degraded, down, latency_sum, latency_n, latencies)
VALUES (@workspace_id, @service_id, @dependency, @hour, 1, @healthy, @degraded, @down, @latency_sum, @latency_n, sqlc.narg(latencies))
ON CONFLICT (service_id, dependency, hour) DO UPDATE SET reports = monitor_health_hours.reports + 1,
    healthy = monitor_health_hours.healthy + excluded.healthy, degraded = monitor_health_hours.degraded + excluded.degraded,
    down = monitor_health_hours.down + excluded.down, latency_sum = monitor_health_hours.latency_sum + excluded.latency_sum,
    latency_n = monitor_health_hours.latency_n + excluded.latency_n, latencies = coalesce(excluded.latencies, monitor_health_hours.latencies);

-- name: ListMonitorHealthHours :many
SELECT * FROM monitor_health_hours WHERE workspace_id = @workspace_id AND service_id = @service_id AND hour >= @from_at ORDER BY hour, dependency;

-- name: DeleteOldMonitorHealthHours :execrows
DELETE FROM monitor_health_hours WHERE workspace_id = @workspace_id AND hour < @before;

-- name: ListMonitorChecksForServices :many
SELECT * FROM monitor_checks WHERE workspace_id = @workspace_id AND service = ANY(@names::text[]) ORDER BY name, id;

-- name: CreateMonitorIncident :one
INSERT INTO monitor_incidents (id, workspace_id, origin, kind, title, state, impact, service_ids, dependency_ids, check_ids, auto_service_id, started_at,
    resolved_at, scheduled_start, scheduled_end, published, created_by, created_at, updated_at)
VALUES (@id, @workspace_id, @origin, @kind, @title, @state, @impact, @service_ids, @dependency_ids, @check_ids, sqlc.narg(auto_service_id), @started_at,
    sqlc.narg(resolved_at), sqlc.narg(scheduled_start), sqlc.narg(scheduled_end), @published, sqlc.narg(created_by), @now, @now)
ON CONFLICT (auto_service_id) WHERE resolved_at IS NULL DO NOTHING
RETURNING *;

-- name: GetMonitorIncident :one
SELECT * FROM monitor_incidents WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockMonitorIncident :one
SELECT * FROM monitor_incidents WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListMonitorIncidents :many
SELECT * FROM monitor_incidents
WHERE workspace_id = @workspace_id
  AND (sqlc.narg(open)::boolean IS NULL OR (resolved_at IS NULL) = sqlc.narg(open)::boolean)
  AND (sqlc.narg(kind)::text IS NULL OR kind = sqlc.narg(kind)::text)
  AND (sqlc.narg(service_id)::uuid IS NULL OR sqlc.narg(service_id)::uuid = ANY(service_ids))
  AND (sqlc.narg(before)::timestamptz IS NULL OR started_at < sqlc.narg(before)::timestamptz)
ORDER BY started_at DESC, id DESC
LIMIT @lim;

-- name: ListOpenMonitorIncidents :many
SELECT * FROM monitor_incidents WHERE workspace_id = @workspace_id AND resolved_at IS NULL ORDER BY started_at, id;

-- name: UpdateMonitorIncident :one
UPDATE monitor_incidents SET title = @title, state = @state, impact = @impact, service_ids = @service_ids, dependency_ids = @dependency_ids,
    check_ids = @check_ids, started_at = @started_at, resolved_at = sqlc.narg(resolved_at), scheduled_start = sqlc.narg(scheduled_start),
    scheduled_end = sqlc.narg(scheduled_end), published = @published, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteMonitorIncident :execrows
DELETE FROM monitor_incidents WHERE workspace_id = @workspace_id AND id = @id;

-- name: InsertMonitorIncidentUpdate :one
INSERT INTO monitor_incident_updates (id, workspace_id, incident_id, state, text, author_id, notify, created_at)
VALUES (@id, @workspace_id, @incident_id, @state, @text, sqlc.narg(author_id), @notify, @now)
RETURNING *;

-- name: ListMonitorIncidentUpdates :many
SELECT * FROM monitor_incident_updates WHERE workspace_id = @workspace_id AND incident_id = ANY(@incident_ids::uuid[]) ORDER BY incident_id, created_at DESC, id DESC;

-- name: LockOpenAutoIncident :one
SELECT * FROM monitor_incidents WHERE workspace_id = @workspace_id AND auto_service_id = @service_id AND resolved_at IS NULL FOR UPDATE;

-- name: CountMonitorServicesIn :one
SELECT count(*) FROM monitor_services WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: CountMonitorDependenciesIn :one
SELECT count(*) FROM monitor_dependencies WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: CountMonitorChecksIn :one
SELECT count(*) FROM monitor_checks WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: ListMonitorMaintenanceBetween :many
SELECT * FROM monitor_incidents
WHERE workspace_id = @workspace_id AND kind = 'maintenance' AND scheduled_start < @to_at AND coalesce(least(resolved_at, scheduled_end), scheduled_end) > @from_at
ORDER BY scheduled_start, id;
