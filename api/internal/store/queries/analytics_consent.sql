-- name: LinkAnalyticsVisitor :exec
INSERT INTO analytics_persistent_visitors (workspace_id, property_id, visitor, first_day, last_day, contact_id, linked_at)
VALUES (@workspace_id, @property_id, @visitor, @day, @day, @contact_id, @now)
ON CONFLICT (property_id, visitor) DO UPDATE SET contact_id = EXCLUDED.contact_id, linked_at = EXCLUDED.linked_at
WHERE analytics_persistent_visitors.workspace_id = EXCLUDED.workspace_id;

-- name: SetAnalyticsVisitorContact :execrows
UPDATE analytics_visits SET contact_id = sqlc.narg(contact_id)::uuid
WHERE workspace_id = @workspace_id AND property_id = @property_id AND visitor = @visitor AND persistent
    AND contact_id IS DISTINCT FROM sqlc.narg(contact_id)::uuid;

-- name: DeleteAnalyticsPersistentVisitor :execrows
DELETE FROM analytics_persistent_visitors WHERE workspace_id = @workspace_id AND property_id = @property_id AND visitor = @visitor;

-- name: DeleteAnalyticsVisitorVisits :many
DELETE FROM analytics_visits WHERE workspace_id = @workspace_id AND property_id = @property_id AND visitor = @visitor AND persistent
RETURNING property_id, day, visit;

-- name: DeleteContactAnalyticsVisits :many
DELETE FROM analytics_visits WHERE workspace_id = @workspace_id AND contact_id = @contact_id
RETURNING property_id, day, visit;

-- name: DeleteAnalyticsVisitEvents :exec
DELETE FROM analytics_events e
USING (SELECT unnest(@property_ids::uuid[]) AS property_id, unnest(@days::date[]) AS day, unnest(@visits::bigint[]) AS visit) u
WHERE e.workspace_id = @workspace_id AND e.property_id = u.property_id AND e.day = u.day AND e.visit = u.visit;

-- name: DeleteAnalyticsVisitPages :exec
DELETE FROM analytics_visit_pages p
USING (SELECT unnest(@property_ids::uuid[]) AS property_id, unnest(@days::date[]) AS day, unnest(@visits::bigint[]) AS visit) u
WHERE p.workspace_id = @workspace_id AND p.property_id = u.property_id AND p.day = u.day AND p.visit = u.visit;

-- name: MarkAnalyticsVisitDaysChanged :exec
-- Days that lost visits are rolled up again when they were, and their folds stop counting.
UPDATE analytics_rollup_days d SET dirty = d.dirty OR d.built_at IS NOT NULL, late_seq = d.late_seq + 1
FROM (SELECT DISTINCT unnest(@property_ids::uuid[]) AS property_id, unnest(@days::date[]) AS day) u
WHERE d.workspace_id = @workspace_id AND d.property_id = u.property_id AND d.day = u.day;

-- name: MoveContactAnalyticsVisits :exec
UPDATE analytics_visits SET contact_id = @to_contact WHERE workspace_id = @workspace_id AND contact_id = @from_contact;

-- name: MoveContactAnalyticsVisitors :exec
UPDATE analytics_persistent_visitors SET contact_id = @to_contact WHERE workspace_id = @workspace_id AND contact_id = @from_contact;

-- name: ListAnalyticsCohortProperties :many
SELECT p.id FROM analytics_properties p
WHERE p.workspace_id = @workspace_id AND p.deleted_at IS NULL
    AND (EXISTS (SELECT 1 FROM analytics_persistent_visitors v WHERE v.workspace_id = p.workspace_id AND v.property_id = p.id)
        OR EXISTS (SELECT 1 FROM analytics_cohorts c WHERE c.workspace_id = p.workspace_id AND c.property_id = p.id))
ORDER BY p.id;

-- name: DeleteExpiredAnalyticsVisitors :execrows
DELETE FROM analytics_persistent_visitors WHERE ctid = ANY(ARRAY(
    SELECT t.ctid FROM analytics_persistent_visitors t WHERE t.workspace_id = @workspace_id AND t.first_day < @before LIMIT 1000));

-- name: DeleteAnalyticsCohorts :exec
DELETE FROM analytics_cohorts WHERE workspace_id = @workspace_id AND property_id = @property_id;

-- name: BuildAnalyticsCohorts :execrows
-- Days reach 30 periods, weeks and months 12. Buckets are taken of the local day as a plain
-- timestamp, so the session's time zone never moves them.
INSERT INTO analytics_cohorts (workspace_id, property_id, grain, cohort, period, visitors)
SELECT @workspace_id::uuid, @property_id::uuid, g.grain, g.cohort, g.period, count(DISTINCT g.visitor)
FROM (
    SELECT 'day'::text AS grain, v.first_day AS cohort, d.day - v.first_day AS period, d.visitor
    FROM analytics_persistent_days d
    JOIN analytics_persistent_visitors v ON v.workspace_id = d.workspace_id AND v.property_id = d.property_id AND v.visitor = d.visitor
    WHERE d.workspace_id = @workspace_id::uuid AND d.property_id = @property_id::uuid AND d.day - v.first_day BETWEEN 0 AND 30
    UNION ALL
    SELECT 'week'::text, date_trunc('week', v.first_day::timestamp)::date,
        (date_trunc('week', d.day::timestamp)::date - date_trunc('week', v.first_day::timestamp)::date) / 7, d.visitor
    FROM analytics_persistent_days d
    JOIN analytics_persistent_visitors v ON v.workspace_id = d.workspace_id AND v.property_id = d.property_id AND v.visitor = d.visitor
    WHERE d.workspace_id = @workspace_id::uuid AND d.property_id = @property_id::uuid
        AND d.day >= v.first_day AND d.day < date_trunc('week', v.first_day::timestamp)::date + 91
    UNION ALL
    SELECT 'month'::text, date_trunc('month', v.first_day::timestamp)::date,
        ((extract(year FROM d.day) - extract(year FROM v.first_day)) * 12 + extract(month FROM d.day) - extract(month FROM v.first_day))::integer,
        d.visitor
    FROM analytics_persistent_days d
    JOIN analytics_persistent_visitors v ON v.workspace_id = d.workspace_id AND v.property_id = d.property_id AND v.visitor = d.visitor
    WHERE d.workspace_id = @workspace_id::uuid AND d.property_id = @property_id::uuid
        AND d.day >= v.first_day AND d.day < (date_trunc('month', v.first_day::timestamp) + interval '13 months')::date
) g
GROUP BY g.grain, g.cohort, g.period;

-- name: MarkAnalyticsCohortsBuilt :exec
UPDATE analytics_properties SET cohorts_built_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListAnalyticsCohorts :many
SELECT cohort, period, visitors FROM analytics_cohorts
WHERE workspace_id = @workspace_id AND property_id = @property_id AND grain = @grain AND cohort BETWEEN @from_day AND @to_day
ORDER BY cohort, period;

-- name: DeleteAnalyticsPersistentData :exec
WITH c AS (DELETE FROM analytics_cohorts WHERE analytics_cohorts.workspace_id = @workspace_id AND analytics_cohorts.property_id = @property_id)
DELETE FROM analytics_persistent_visitors WHERE analytics_persistent_visitors.workspace_id = @workspace_id
    AND analytics_persistent_visitors.property_id = @property_id;

-- name: DeleteAnalyticsPersistentDays :exec
DELETE FROM analytics_persistent_days WHERE workspace_id = @workspace_id AND property_id = @property_id AND day BETWEEN @from_day AND @to_day;
