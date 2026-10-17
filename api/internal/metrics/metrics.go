package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	Requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yuva_http_requests_total", Help: "HTTP requests by method and status.",
	}, []string{"method", "status"})
	RequestSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "yuva_http_request_seconds", Help: "HTTP request latency.", Buckets: prometheus.DefBuckets,
	}, []string{"method"})
	BroadcastMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yuva_broadcast_messages_total", Help: "Broadcast mails by result: sent, bounced, rejected, deferred.",
	}, []string{"result"})
	BroadcastComplaints = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yuva_broadcast_complaints_total", Help: "Spam complaints about broadcast mails.",
	})
	AnalyticsEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yuva_analytics_events_total", Help: "Analytics events by result: accepted (buffered), stored, dropped.",
	}, []string{"result"})
	AnalyticsDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yuva_analytics_dropped_total", Help: "Analytics events not stored, by reason.",
	}, []string{"reason"})
	AnalyticsFlushSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "yuva_analytics_flush_seconds", Help: "Time to write one batch of analytics events.", Buckets: prometheus.DefBuckets,
	})
	AnalyticsBufferEvents = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "yuva_analytics_buffer_events", Help: "Analytics events waiting to be written.",
	})
	AnalyticsLateEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yuva_analytics_late_events_total", Help: "Analytics events written to a day that was already rolled up.",
	})
	SESTenants = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "yuva_ses_tenants", Help: "Amazon SES tenants the server manages, by sending status.",
	}, []string{"sending_status"})
	SESAPIErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "yuva_ses_api_errors_total", Help: "Refused or failed Amazon SES and STS calls, by operation and error code.",
	}, []string{"operation", "code"})
)

func Serve(ctx context.Context, addr string, log *slog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("metrics listener", slog.Any("error", err))
	}
}
