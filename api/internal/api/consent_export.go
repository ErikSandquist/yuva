package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

var consentColumns = []string{
	"list_id", "list_name", "commercial", "subscription_id", "contact_id", "email", "state", "source", "consent_at",
	"unsubscribed_at", "unsubscribe_reason", "event_id", "event_kind", "event_source", "event_at", "ip", "user_agent",
	"page_url", "consent_text", "member_id", "api_key_id", "detail",
}

// consentStream writes consent records page by page, so a large list never sits in memory.
type consentStream struct {
	s      *Server
	ctx    context.Context
	format string
	name   string
	arg    store.ListConsentRecordsParams
}

func (c consentStream) VisitExportListConsentResponse(w http.ResponseWriter) error {
	return c.write(w)
}

func (c consentStream) VisitExportContactConsentResponse(w http.ResponseWriter) error {
	return c.write(w)
}

func consentRecord(r store.ListConsentRecordsRow) oas.ConsentRecord {
	out := oas.ConsentRecord{
		ListId: r.ListID, ListName: r.ListName, Commercial: r.Commercial, SubscriptionId: r.SubscriptionID, ContactId: r.ContactID,
		Email: oas.Email(r.Email), State: oas.SubscriptionState(r.State), Source: oas.SubscriptionSource(r.Source),
		ConsentAt: r.ConsentAt, UnsubscribedAt: r.UnsubscribedAt, EventId: r.EventID, EventKind: oas.ConsentEventKind(r.Kind),
		EventSource: r.EventSource, EventAt: r.EventAt, Ip: r.Ip, UserAgent: r.UserAgent, PageUrl: r.PageUrl,
		ConsentText: r.ConsentText, MemberId: r.MemberID, ApiKeyId: r.ApiKeyID, Detail: map[string]any{},
	}
	if r.UnsubscribeReason != nil && r.State == subUnsubscribed {
		v := oas.UnsubscribeReason(*r.UnsubscribeReason)
		out.UnsubscribeReason = &v
	}
	_ = json.Unmarshal(r.Detail, &out.Detail)
	return out
}

// csvCell quotes every cell and defuses spreadsheet formulas: a cell starting with =, +, -, @,
// tab or carriage return gets a leading apostrophe.
func csvCell(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		v = "'" + v
	}
	return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
}

func csvTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func csvText(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func csvRow(r oas.ConsentRecord) string {
	reason := ""
	if r.UnsubscribeReason != nil {
		reason = string(*r.UnsubscribeReason)
	}
	member, key := "", ""
	if r.MemberId != nil {
		member = r.MemberId.String()
	}
	if r.ApiKeyId != nil {
		key = r.ApiKeyId.String()
	}
	commercial := "false"
	if r.Commercial {
		commercial = "true"
	}
	cells := []string{
		r.ListId.String(), r.ListName, commercial, r.SubscriptionId.String(), r.ContactId.String(), string(r.Email),
		string(r.State), string(r.Source), csvTime(r.ConsentAt), csvTime(r.UnsubscribedAt), reason, r.EventId.String(),
		string(r.EventKind), r.EventSource, csvTime(&r.EventAt), csvText(r.Ip), csvText(r.UserAgent), csvText(r.PageUrl),
		csvText(r.ConsentText), member, key, string(mustJSON(r.Detail)),
	}
	for i, c := range cells {
		cells[i] = csvCell(c)
	}
	return strings.Join(cells, ",") + "\r\n"
}

func (c consentStream) write(w http.ResponseWriter) error {
	h := w.Header()
	if c.format == "json" {
		h.Set("Content-Type", "application/json")
		h.Set("Content-Disposition", `attachment; filename="`+c.name+`.json"`)
	} else {
		h.Set("Content-Type", "text/csv; charset=utf-8")
		h.Set("Content-Disposition", `attachment; filename="`+c.name+`.csv"`)
	}
	h.Set("Cache-Control", "no-store")
	arg := c.arg
	arg.Lim = consentExportPage
	first := true
	for page := 0; ; page++ {
		rows, err := c.s.st.ListConsentRecords(c.ctx, arg)
		if err != nil {
			if page == 0 {
				writeProblem(w, errInternal)
			}
			c.s.log.ErrorContext(c.ctx, "consent export", slog.Any("error", err))
			return nil
		}
		if page == 0 {
			w.WriteHeader(http.StatusOK)
			if c.format == "json" {
				_, _ = w.Write([]byte(`{"items":[`))
			} else {
				_, _ = w.Write([]byte(strings.Join(consentColumns, ",") + "\r\n"))
			}
		}
		for _, r := range rows {
			rec := consentRecord(r)
			if c.format == "json" {
				if !first {
					_, _ = w.Write([]byte(","))
				}
				_, _ = w.Write(mustJSON(rec))
			} else {
				_, _ = w.Write([]byte(csvRow(rec)))
			}
			first = false
		}
		if len(rows) < consentExportPage {
			break
		}
		last := rows[len(rows)-1]
		arg.AfterAt, arg.AfterID = &last.EventAt, &last.EventID
	}
	if c.format == "json" {
		_, _ = w.Write([]byte("]}\n"))
	}
	return nil
}

var iysColumns = []string{"recipient", "type", "source", "status", "consentDate", "recipientType"}

// iysZone is Türkiye's time, which İYS dates are written in; it has been UTC+3 all year since 2016.
var iysZone = func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Istanbul"); err == nil {
		return loc
	}
	return time.FixedZone("TRT", 3*60*60)
}()

func iysSource(source string) string {
	switch source {
	case string(oas.SubscriptionSourceForm), string(oas.SubscriptionSourcePreferences):
		return "HS_WEB"
	}
	return "HS_EORTAM"
}

func iysRow(r store.ListIYSRecordsRow) string {
	status, at := "ONAY", r.ConsentAt
	if r.State == subUnsubscribed {
		status, at = "RET", r.UnsubscribedAt
	}
	if at == nil {
		at = &r.CreatedAt
	}
	cells := []string{r.Email, "EPOSTA", iysSource(r.Source), status, at.In(iysZone).Format(time.DateTime), "BIREYSEL"}
	for i, c := range cells {
		cells[i] = csvCell(c)
	}
	return strings.Join(cells, ",") + "\r\n"
}

// iysStream writes a list's subscriptions in İYS's consent fields, page by page.
type iysStream struct {
	s    *Server
	ctx  context.Context
	name string
	arg  store.ListIYSRecordsParams
}

func (c iysStream) VisitExportListConsentResponse(w http.ResponseWriter) error {
	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="`+c.name+`.csv"`)
	h.Set("Cache-Control", "no-store")
	arg := c.arg
	arg.Lim = consentExportPage
	for page := 0; ; page++ {
		rows, err := c.s.st.ListIYSRecords(c.ctx, arg)
		if err != nil {
			if page == 0 {
				writeProblem(w, errInternal)
			}
			c.s.log.ErrorContext(c.ctx, "iys export", slog.Any("error", err))
			return nil
		}
		if page == 0 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strings.Join(iysColumns, ",") + "\r\n"))
		}
		for _, r := range rows {
			_, _ = w.Write([]byte(iysRow(r)))
		}
		if len(rows) < consentExportPage {
			return nil
		}
		last := rows[len(rows)-1]
		arg.AfterAt, arg.AfterID = &last.CreatedAt, &last.ID
	}
}
