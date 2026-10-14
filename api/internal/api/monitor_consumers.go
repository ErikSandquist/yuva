package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/productdevbook/yuva/api/internal/jobs"
	"github.com/productdevbook/yuva/api/internal/monitor"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/realtime"
	"github.com/productdevbook/yuva/api/internal/store"
)

const monitorEraseMark = ":consumer:"

// monitorEraseSweeps are when an erased consumer's rows are deleted again: after every process
// wrote what its last flush took, and after the name caches of processes that missed the notice
// expired.
var monitorEraseSweeps = []time.Duration{2 * time.Minute, 11 * time.Minute}

var errConsumerGone = problem(http.StatusNotFound, "not_found", "no such consumer")

func (s *Server) ListAnalyticsConsumers(ctx context.Context, req oas.ListAnalyticsConsumersRequestObject) (oas.ListAnalyticsConsumersResponseObject, error) {
	if _, err := s.monitorOn(); err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	size, err := pageSize(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	var after *string
	if c := req.Params.Cursor; c != nil && *c != "" {
		raw, err := base64.RawURLEncoding.DecodeString(*c)
		if err != nil || len(raw) == 0 {
			return nil, errInvalidCursor
		}
		after = new(string(raw))
	}
	var search *string
	if req.Params.Q != nil {
		v, err := trimmed(*req.Params.Q, 1, 100, "q")
		if err != nil {
			return nil, err
		}
		search = &v
	}
	prop, err := visibleProperty(ctx, s.st.Queries, p, req.PropertyId, false)
	if err != nil {
		return nil, err
	}
	if !isAPIProperty(prop) {
		return nil, errAPIOnly
	}
	rows, err := s.st.ListMonitorConsumers(ctx, store.ListMonitorConsumersParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID,
		Search: search, After: after, MaxRows: size + 1})
	if err != nil {
		return nil, err
	}
	out := oas.ListAnalyticsConsumers200JSONResponse{Items: []oas.AnalyticsConsumer{}}
	if len(rows) > int(size) {
		rows = rows[:size]
		out.NextCursor = new(base64.RawURLEncoding.EncodeToString([]byte(rows[len(rows)-1].Name)))
	}
	for _, r := range rows {
		out.Items = append(out.Items, oas.AnalyticsConsumer{Id: r.ID, Name: r.Name, FirstSeenAt: r.FirstSeenAt})
	}
	return out, nil
}

// DeleteAnalyticsConsumer erases a consumer: its name row keeps its id without a value, so no other
// consumer gets the id, its dimension rows go now and again in the sweeps, and every process drops
// what it holds of it in memory.
func (s *Server) DeleteAnalyticsConsumer(ctx context.Context, req oas.DeleteAnalyticsConsumerRequestObject) (oas.DeleteAnalyticsConsumerResponseObject, error) {
	m, err := s.monitorOn()
	if err != nil {
		return nil, err
	}
	p := principalFrom(ctx)
	var prop store.AnalyticsProperty
	err = s.st.InTxRaw(ctx, func(tx pgx.Tx, q *store.Queries) error {
		if prop, err = visibleProperty(ctx, q, p, req.PropertyId, true); err != nil {
			return err
		}
		if err := requireAnalyticsManager(p); err != nil {
			return err
		}
		if !isAPIProperty(prop) {
			return errAPIOnly
		}
		c, err := q.GetMonitorConsumerForUpdate(ctx, store.GetMonitorConsumerForUpdateParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: req.ConsumerId})
		if store.IsNotFound(err) {
			return errConsumerGone
		}
		if err != nil {
			return err
		}
		if c.Name == monitor.OtherValue {
			return errValidation("(other) holds the requests past the consumer cap, not one consumer")
		}
		if err := q.EraseMonitorConsumerName(ctx, store.EraseMonitorConsumerNameParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ID: c.ID, Now: new(s.now())}); err != nil {
			return err
		}
		if err := q.DeleteMonitorConsumerRows(ctx, store.DeleteMonitorConsumerRowsParams{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ConsumerID: c.ID}); err != nil {
			return err
		}
		if _, err := q.BumpAnalyticsRollupVersion(ctx, store.BumpAnalyticsRollupVersionParams{WorkspaceID: p.workspaceID, ID: prop.ID}); err != nil {
			return err
		}
		if err := notifyConsumerErased(ctx, q, p.workspaceID, prop.ID, c.ID); err != nil {
			return err
		}
		for i, after := range monitorEraseSweeps {
			if _, err := s.jobs.InsertTx(ctx, tx, MonitorEraseArgs{WorkspaceID: p.workspaceID, PropertyID: prop.ID, ConsumerID: c.ID, Sweep: i + 1},
				&river.InsertOpts{ScheduledAt: s.now().Add(after)}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	m.agg.EraseDim(prop.ID, monitor.DimConsumer, req.ConsumerId)
	s.analytics.forget(prop.ID)
	return oas.DeleteAnalyticsConsumer204Response{}, nil
}

// notifyConsumerErased is notifyAnalyticsChanged for a property with the consumer every process
// must stop writing.
func notifyConsumerErased(ctx context.Context, q *store.Queries, ws, propertyID uuid.UUID, consumerID int32) error {
	return q.NotifySignal(ctx, store.NotifySignalParams{Channel: realtime.InvalidateChannel,
		Payload: analyticsInvalidation + ws.String() + ":" + propertyID.String() + monitorEraseMark + strconv.Itoa(int(consumerID))})
}

// MonitorEraseArgs deletes an erased consumer's dimension rows again.
type MonitorEraseArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	PropertyID  uuid.UUID `json:"property_id"`
	ConsumerID  int32     `json:"consumer_id"`
	Sweep       int       `json:"sweep"`
}

func (MonitorEraseArgs) Kind() string { return "monitor_erase" }

func (MonitorEraseArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.AnalyticsQueue, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type monitorEraseWorker struct {
	river.WorkerDefaults[MonitorEraseArgs]
	s *Server
}

func (w *monitorEraseWorker) Work(ctx context.Context, job *river.Job[MonitorEraseArgs]) error {
	return w.s.SweepErasedConsumer(ctx, job.Args.WorkspaceID, job.Args.PropertyID, job.Args.ConsumerID)
}

// SweepErasedConsumer deletes the rows a process wrote under an erased consumer after it was erased.
func (s *Server) SweepErasedConsumer(ctx context.Context, ws, propertyID uuid.UUID, consumerID int32) error {
	return s.st.InTx(ctx, func(q *store.Queries) error {
		if err := q.DeleteMonitorConsumerRows(ctx, store.DeleteMonitorConsumerRowsParams{WorkspaceID: ws, PropertyID: propertyID, ConsumerID: consumerID}); err != nil {
			return err
		}
		if _, err := q.BumpAnalyticsRollupVersion(ctx, store.BumpAnalyticsRollupVersionParams{WorkspaceID: ws, ID: propertyID}); err != nil && !store.IsNotFound(err) {
			return err
		}
		return notifyConsumerErased(ctx, q, ws, propertyID, consumerID)
	})
}
