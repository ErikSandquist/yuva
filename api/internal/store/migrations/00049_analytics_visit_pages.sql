-- +goose Up
-- Each visit's numbers per path, kept by the flusher next to the events they sum, so page
-- breakdowns of raw days read one row per visit and path instead of every event. Partitioned and
-- dropped by day like analytics_events.
CREATE TABLE analytics_visit_pages (
    workspace_id uuid NOT NULL,
    property_id  uuid NOT NULL,
    day          date NOT NULL,
    visit        bigint NOT NULL,
    path         text COLLATE "C" NOT NULL,
    views        integer NOT NULL DEFAULT 0,
    events       integer NOT NULL DEFAULT 0,
    eng_ms       bigint NOT NULL DEFAULT 0,
    eng_views    integer NOT NULL DEFAULT 0,
    scroll_sum   bigint NOT NULL DEFAULT 0,
    scroll_views integer NOT NULL DEFAULT 0,
    PRIMARY KEY (property_id, day, visit, path)
) PARTITION BY RANGE (day);

-- +goose StatementBegin
DO $$
DECLARE
    part record;
    d date;
BEGIN
    FOR part IN SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid JOIN pg_class p ON p.oid = i.inhparent
        WHERE p.relname = 'analytics_events' AND c.relname ~ '^analytics_events_\d{8}$'
    LOOP
        d := to_date(right(part.relname, 8), 'YYYYMMDD');
        EXECUTE format('CREATE TABLE %I PARTITION OF analytics_visit_pages FOR VALUES FROM (%L) TO (%L)',
            'analytics_visit_pages_' || right(part.relname, 8), d, d + 1);
    END LOOP;
END
$$;
-- +goose StatementEnd

INSERT INTO analytics_visit_pages (workspace_id, property_id, day, visit, path, views, events, eng_ms, eng_views, scroll_sum, scroll_views)
SELECT workspace_id, property_id, day, visit, path, count(*) FILTER (WHERE kind = 1), count(*) FILTER (WHERE kind = 3),
    coalesce(sum(engaged_ms) FILTER (WHERE kind = 4), 0), count(*) FILTER (WHERE kind = 4),
    coalesce(sum(scroll) FILTER (WHERE kind = 4), 0), count(scroll) FILTER (WHERE kind = 4)
FROM analytics_events GROUP BY workspace_id, property_id, day, visit, path;

-- +goose Down
DROP TABLE analytics_visit_pages;
