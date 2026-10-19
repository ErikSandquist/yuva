package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/csvimport"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/realtime"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	eventListImportUpdated  = string(oas.ListImportUpdated)
	eventListImportProgress = string(oas.ListImportProgressEventTypeListImportProgress)

	importValidating = string(oas.ListImportStateValidating)
	importReady      = string(oas.ListImportStateReady)
	importImporting  = string(oas.ListImportStateImporting)
	importDone       = string(oas.ListImportStateDone)
	importFailed     = string(oas.ListImportStateFailed)
	importCancelled  = string(oas.ListImportStateCancelled)

	maxImportBytes      = 20 << 20
	maxImportRows       = 100000
	importErrorsKept    = 7 * 24 * time.Hour
	importStaleAfter    = 7 * 24 * time.Hour
	importProgressEvery = time.Second
	importPreviewRows   = 20
	importPreviewCell   = 200
)

var (
	errListImportGone      = problem(http.StatusNotFound, "not_found", "no such import")
	errImportRunning       = problem(http.StatusConflict, "import_running", "another import of the workspace is being validated or imported; wait for it to end or cancel it")
	errImportNotReady      = problem(http.StatusConflict, "not_ready", "the import is not ready: it is still validating or has started or ended")
	errImportNotCSV        = problem(http.StatusUnsupportedMediaType, "not_csv", "upload a CSV file; spreadsheets must be saved as CSV first")
	errImportTooLarge      = problem(http.StatusRequestEntityTooLarge, "file_too_large", "a CSV import is at most 20 MiB")
	errAttestationRequired = problem(http.StatusUnprocessableEntity, "attestation_required", "confirm that everyone in the file agreed to receive this list (attestation) and say where and when they agreed (consent_source)")
	errEmailColumnRequired = problem(http.StatusUnprocessableEntity, "email_column_required", "map one column to email")
	errImportErrorsGone    = problem(http.StatusNotFound, "not_found", "the refused rows of this import were deleted 7 days after it ended")
)

// MXResolver looks up whether a domain takes mail; net.Resolver is one.
type MXResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// ImportSettings: MaxRows caps the data rows of a CSV import (YUVA_IMPORT_MAX_ROWS, at most and by
// default 100,000); Resolver replaces the system resolver for the MX check, for tests.
type ImportSettings struct {
	MaxRows  int
	Resolver MXResolver
}

func importSettings(in ImportSettings) ImportSettings {
	if in.MaxRows <= 0 || in.MaxRows > maxImportRows {
		in.MaxRows = maxImportRows
	}
	if in.Resolver == nil {
		in.Resolver = net.DefaultResolver
	}
	return in
}

func importColumns(raw []byte) []oas.ListImportColumn {
	out := []oas.ListImportColumn{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func importCounts(raw []byte) oas.ListImportCounts {
	var out oas.ListImportCounts
	_ = json.Unmarshal(raw, &out)
	return out
}

func listImportBody(imp store.ListImport) oas.ListImport {
	out := oas.ListImport{
		Id: imp.ID, ListId: imp.ListID, State: oas.ListImportState(imp.State), Filename: imp.Filename, Reconfirm: imp.Reconfirm,
		Attestation: imp.Attestation, ConsentSource: imp.ConsentSource, RowsTotal: imp.RowsTotal, RowsDone: imp.RowsDone,
		Counts: importCounts(imp.Counts), Columns: importColumns(imp.Columns), Error: imp.Error, MemberId: imp.MemberID,
		ApiKeyId: imp.ApiKeyID, CreatedAt: imp.CreatedAt, UpdatedAt: imp.UpdatedAt, StartedAt: imp.StartedAt, FinishedAt: imp.FinishedAt,
	}
	if imp.Encoding != nil {
		out.Encoding = new(oas.ListImportEncoding(*imp.Encoding))
	}
	if imp.Delimiter != nil {
		out.Delimiter = new(oas.ListImportDelimiter(*imp.Delimiter))
	}
	if len(imp.Preview) > 0 {
		var rows [][]string
		if json.Unmarshal(imp.Preview, &rows) == nil {
			out.Preview = &rows
		}
	}
	if imp.FinishedAt != nil {
		out.ErrorsUntil = new(imp.FinishedAt.Add(importErrorsKept))
	}
	return out
}

func (s *Server) listImportSignal(ctx context.Context, list store.MailingList, imp store.ListImport) {
	inbox := list.InboxID
	s.signal(ctx, realtime.Event{Type: eventListImportUpdated, WorkspaceID: imp.WorkspaceID, InboxID: &inbox, Data: mustJSON(listImportBody(imp))})
}

func (s *Server) listImportProgress(ctx context.Context, list store.MailingList, imp store.ListImport) {
	if !s.progressDue(imp.ID, importProgressEvery) {
		return
	}
	inbox := list.InboxID
	s.signal(ctx, realtime.Event{Type: eventListImportProgress, WorkspaceID: imp.WorkspaceID, InboxID: &inbox, Data: mustJSON(oas.ListImportProgress{
		Id: imp.ID, ListId: imp.ListID, State: oas.ListImportState(imp.State), RowsTotal: imp.RowsTotal, RowsDone: imp.RowsDone,
		Counts: importCounts(imp.Counts),
	})})
}

func visibleListImport(ctx context.Context, q *store.Queries, p principal, listID, id uuid.UUID, lock bool) (store.MailingList, store.ListImport, error) {
	list, err := visibleList(ctx, q, p, listID, false)
	if err != nil {
		return list, store.ListImport{}, err
	}
	get := q.GetListImport
	if lock {
		get = func(ctx context.Context, arg store.GetListImportParams) (store.ListImport, error) {
			return q.LockListImport(ctx, store.LockListImportParams(arg))
		}
	}
	imp, err := get(ctx, store.GetListImportParams{WorkspaceID: p.workspaceID, ID: id})
	if store.IsNotFound(err) || (err == nil && imp.ListID != list.ID) {
		return list, imp, errListImportGone
	}
	return list, imp, err
}

// checkImportMapping validates a mapping; with a header it must fit it and the result names every
// column of the file.
func checkImportMapping(m []oas.ListImportColumnMapping, header []string) ([]oas.ListImportColumn, error) {
	byIndex := map[int32]oas.ListImportColumnMapping{}
	emails := 0
	for _, c := range m {
		if !c.Field.Valid() {
			return nil, errValidation("field must be email, name, first_name, last_name, locale, external_id, attribute or skip")
		}
		if c.Index < 0 || c.Index >= csvimport.MaxColumns || (header != nil && int(c.Index) >= len(header)) {
			return nil, errValidation("column " + strconv.Itoa(int(c.Index)) + " is not in the file")
		}
		if _, dup := byIndex[c.Index]; dup {
			return nil, errValidation("column " + strconv.Itoa(int(c.Index)) + " is mapped twice")
		}
		if c.Field == oas.ListImportFieldAttribute {
			if c.Attribute == nil || !attributeName.MatchString(*c.Attribute) {
				return nil, errValidation("an attribute column needs attribute: letters, digits, _ . or -, at most 64")
			}
		} else {
			c.Attribute = nil
		}
		if c.Field == oas.ListImportFieldEmail {
			emails++
		}
		byIndex[c.Index] = c
	}
	if emails != 1 {
		return nil, errEmailColumnRequired
	}
	n := len(header)
	if header == nil {
		for i := range byIndex {
			n = max(n, int(i)+1)
		}
	}
	out := make([]oas.ListImportColumn, n)
	for i := range out {
		col := oas.ListImportColumn{Index: int32(i), Field: oas.ListImportFieldSkip}
		if header != nil {
			col.Header = header[i]
		}
		if c, ok := byIndex[int32(i)]; ok {
			col.Field, col.Attribute = c.Field, c.Attribute
		}
		out[i] = col
	}
	return out, nil
}

func (s *Server) ListListImports(ctx context.Context, req oas.ListListImportsRequestObject) (oas.ListListImportsResponseObject, error) {
	p := principalFrom(ctx)
	lim, err := pageSize(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	at, id, err := decodeCursor(req.Params.Cursor)
	if err != nil {
		return nil, err
	}
	list, err := visibleList(ctx, s.st.Queries, p, req.ListId, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.st.ListListImports(ctx, store.ListListImportsParams{WorkspaceID: p.workspaceID, ListID: list.ID, CursorAt: at, CursorID: id, Lim: lim + 1})
	if err != nil {
		return nil, err
	}
	var next *string
	if len(rows) > int(lim) {
		rows = rows[:lim]
		c := encodeCursor(rows[lim-1].CreatedAt, rows[lim-1].ID)
		next = &c
	}
	items := make([]oas.ListImport, len(rows))
	for i, r := range rows {
		items[i] = listImportBody(r)
	}
	return oas.ListListImports200JSONResponse{Items: items, NextCursor: next}, nil
}

func (s *Server) GetListImport(ctx context.Context, req oas.GetListImportRequestObject) (oas.GetListImportResponseObject, error) {
	p := principalFrom(ctx)
	_, imp, err := visibleListImport(ctx, s.st.Queries, p, req.ListId, req.ImportId, false)
	if err != nil {
		return nil, err
	}
	return oas.GetListImport200JSONResponse(listImportBody(imp)), nil
}

type importUpload struct {
	data          []byte
	filename      string
	attestation   string
	consentSource string
	reconfirm     bool
	mapping       *oas.ListImportMappingUpdate
	sawFile       bool
}

func (s *Server) CreateListImport(ctx context.Context, req oas.CreateListImportRequestObject) (oas.CreateListImportResponseObject, error) {
	p := principalFrom(ctx)
	list, err := visibleList(ctx, s.st.Queries, p, req.ListId, false)
	if err != nil {
		return nil, err
	}
	if err := requireManagerOrKey(p); err != nil {
		return nil, err
	}
	if list.Kind == listKindStatus {
		return nil, errListKind
	}
	var up importUpload
	for {
		part, err := req.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, uploadError(err)
		}
		name := part.FormName()
		if name == "file" {
			if up.sawFile {
				return nil, errValidation("send one file")
			}
			up.sawFile = true
			if up.data, err = io.ReadAll(io.LimitReader(part, maxImportBytes+1)); err != nil {
				return nil, uploadError(err)
			}
			if len(up.data) > maxImportBytes {
				return nil, errImportTooLarge
			}
			up.filename = part.FileName()
			continue
		}
		v, err := io.ReadAll(io.LimitReader(part, 64<<10))
		if err != nil {
			return nil, uploadError(err)
		}
		switch name {
		case "attestation":
			up.attestation = strings.TrimSpace(string(v))
		case "consent_source":
			up.consentSource = strings.TrimSpace(string(v))
		case "reconfirm":
			if up.reconfirm, err = strconv.ParseBool(strings.TrimSpace(string(v))); err != nil {
				return nil, errValidation("reconfirm must be true or false")
			}
		case "mapping":
			var m oas.ListImportMappingUpdate
			dec := json.NewDecoder(bytes.NewReader(v))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&m); err != nil {
				return nil, errValidation("mapping must be a ListImportMappingUpdate as JSON")
			}
			up.mapping = &m
		default:
			return nil, errValidation("unknown form field " + name)
		}
	}
	if up.attestation == "" || up.consentSource == "" {
		return nil, errAttestationRequired
	}
	if len([]rune(up.attestation)) > 2000 || len([]rune(up.consentSource)) > 1000 {
		return nil, errValidation("attestation is at most 2,000 characters and consent_source at most 1,000")
	}
	if len(up.data) == 0 {
		return nil, errValidation("file is required")
	}
	if !csvimport.LooksLikeText(up.data) {
		return nil, errImportNotCSV
	}
	columns := []oas.ListImportColumn{}
	if up.mapping != nil {
		var err error
		if columns, err = checkImportMapping(up.mapping.Columns, nil); err != nil {
			return nil, err
		}
	}
	id := newID()
	key := "list-imports/" + p.workspaceID.String() + "/" + id.String()
	if err := s.objects.Put(ctx, key, bytes.NewReader(up.data), int64(len(up.data)), "text/csv"); err != nil {
		return nil, err
	}
	var imp store.ListImport
	err = s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		list, err := visibleList(ctx, q, p, req.ListId, false)
		if err != nil {
			return err
		}
		if up.reconfirm {
			if list.ChannelID == nil {
				return errEmailChannelRequired
			}
			if err := listChannel(ctx, q, p.workspaceID, list.InboxID, *list.ChannelID); err != nil {
				return err
			}
		}
		var keyID *uuid.UUID
		if p.isKey() {
			keyID = &p.keyID
		}
		imp, err = q.CreateListImport(ctx, store.CreateListImportParams{
			ID: id, WorkspaceID: p.workspaceID, ListID: list.ID, MemberID: p.memberRef(), ApiKeyID: keyID,
			Filename: truncateRunes(cleanFilename(up.filename), 255), StorageKey: &key, Columns: mustJSON(columns),
			Attestation: up.attestation, ConsentSource: up.consentSource, Reconfirm: up.reconfirm, Now: s.now(),
		})
		if store.IsUniqueViolation(err) {
			return errImportRunning
		}
		if err != nil {
			return err
		}
		events.job(ListImportArgs{WorkspaceID: p.workspaceID, ImportID: id}, listImportOpts())
		return nil
	})
	if err != nil {
		s.deleteObjects(ctx, []string{key})
		return nil, err
	}
	return oas.CreateListImport202JSONResponse(listImportBody(imp)), nil
}

func (s *Server) UpdateListImport(ctx context.Context, req oas.UpdateListImportRequestObject) (oas.UpdateListImportResponseObject, error) {
	p := principalFrom(ctx)
	var imp store.ListImport
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		_, cur, err := visibleListImport(ctx, q, p, req.ListId, req.ImportId, true)
		if err != nil {
			return err
		}
		if err := requireManagerOrKey(p); err != nil {
			return err
		}
		if cur.State != importReady {
			return errImportNotReady
		}
		existing := importColumns(cur.Columns)
		header := make([]string, len(existing))
		for i, c := range existing {
			header[i] = c.Header
		}
		columns, err := checkImportMapping(req.Body.Columns, header)
		if err != nil {
			return err
		}
		imp, err = q.RevalidateListImport(ctx, store.RevalidateListImportParams{WorkspaceID: p.workspaceID, ID: cur.ID, Columns: mustJSON(columns), Now: s.now()})
		if store.IsUniqueViolation(err) {
			return errImportRunning
		}
		if err != nil {
			return err
		}
		events.job(ListImportArgs{WorkspaceID: p.workspaceID, ImportID: cur.ID}, listImportOpts())
		return nil
	})
	if err != nil {
		return nil, err
	}
	return oas.UpdateListImport200JSONResponse(listImportBody(imp)), nil
}

func (s *Server) StartListImport(ctx context.Context, req oas.StartListImportRequestObject) (oas.StartListImportResponseObject, error) {
	p := principalFrom(ctx)
	var (
		imp  store.ListImport
		list store.MailingList
	)
	err := s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		l, cur, err := visibleListImport(ctx, q, p, req.ListId, req.ImportId, true)
		if err != nil {
			return err
		}
		list = l
		if err := requireManagerOrKey(p); err != nil {
			return err
		}
		if cur.State != importReady {
			return errImportNotReady
		}
		if importColumn(importColumns(cur.Columns), oas.ListImportFieldEmail) < 0 {
			return errEmailColumnRequired
		}
		counts := oas.ListImportCounts{Valid: importCounts(cur.Counts).Valid}
		imp, err = q.StartListImport(ctx, store.StartListImportParams{WorkspaceID: p.workspaceID, ID: cur.ID, Counts: mustJSON(counts), Now: new(s.now())})
		if store.IsUniqueViolation(err) {
			return errImportRunning
		}
		if err != nil {
			return err
		}
		events.job(ListImportArgs{WorkspaceID: p.workspaceID, ImportID: cur.ID}, listImportOpts())
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.listImportSignal(ctx, list, imp)
	return oas.StartListImport202JSONResponse(listImportBody(imp)), nil
}

func (s *Server) CancelListImport(ctx context.Context, req oas.CancelListImportRequestObject) (oas.CancelListImportResponseObject, error) {
	p := principalFrom(ctx)
	var (
		imp       store.ListImport
		list      store.MailingList
		key       *string
		cancelled bool
	)
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		l, cur, err := visibleListImport(ctx, q, p, req.ListId, req.ImportId, true)
		if err != nil {
			return err
		}
		list = l
		if err := requireManagerOrKey(p); err != nil {
			return err
		}
		if cur.State != importValidating && cur.State != importReady && cur.State != importImporting {
			return nil
		}
		key, cancelled = cur.StorageKey, true
		imp, err = q.FinishListImport(ctx, store.FinishListImportParams{WorkspaceID: p.workspaceID, ID: cur.ID, State: importCancelled, Now: new(s.now())})
		return err
	})
	if err != nil {
		return nil, err
	}
	if cancelled {
		if key != nil {
			s.deleteObjects(ctx, []string{*key})
		}
		s.listImportSignal(ctx, list, imp)
	}
	return oas.CancelListImport204Response{}, nil
}

func (s *Server) GetListImportErrors(ctx context.Context, req oas.GetListImportErrorsRequestObject) (oas.GetListImportErrorsResponseObject, error) {
	p := principalFrom(ctx)
	_, imp, err := visibleListImport(ctx, s.st.Queries, p, req.ListId, req.ImportId, false)
	if err != nil {
		return nil, err
	}
	if imp.FinishedAt != nil && s.now().After(imp.FinishedAt.Add(importErrorsKept)) {
		return nil, errImportErrorsGone
	}
	return importErrorsStream{s: s, ctx: ctx, ws: p.workspaceID, id: imp.ID}, nil
}

// importErrorsStream writes the refused rows page by page.
type importErrorsStream struct {
	s   *Server
	ctx context.Context
	ws  uuid.UUID
	id  uuid.UUID
}

func (e importErrorsStream) VisitGetListImportErrorsResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="import-errors-`+e.id.String()+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	var buf strings.Builder
	buf.WriteString(csvCell("row") + "," + csvCell("email") + "," + csvCell("reason") + "\r\n")
	after := int32(0)
	for {
		rows, err := e.s.st.ListListImportErrors(e.ctx, store.ListListImportErrorsParams{WorkspaceID: e.ws, ImportID: e.id, AfterRow: after, Lim: consentExportPage})
		if err != nil {
			return err
		}
		for _, r := range rows {
			buf.WriteString(csvCell(strconv.Itoa(int(r.Row))) + "," + csvCell(r.Email) + "," + csvCell(r.Reason) + "\r\n")
			after = r.Row
		}
		if _, err := io.WriteString(w, buf.String()); err != nil {
			return err
		}
		buf.Reset()
		if len(rows) < consentExportPage {
			return nil
		}
	}
}

// CleanListImports cancels imports left validating or ready for 7 days, deleting their uploads,
// and deletes refused rows 7 days after their import ended.
func (s *Server) CleanListImports(ctx context.Context) error {
	ids, err := s.st.ListWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	for _, ws := range ids {
		keys, err := s.st.CancelStaleListImports(ctx, store.CancelStaleListImportsParams{WorkspaceID: ws, Before: now.Add(-importStaleAfter), Now: &now})
		if err != nil {
			return err
		}
		s.deleteObjects(ctx, keys)
		if _, err := s.st.DeleteExpiredListImportErrors(ctx, store.DeleteExpiredListImportErrorsParams{WorkspaceID: ws, Before: now.Add(-importErrorsKept)}); err != nil {
			return err
		}
	}
	return nil
}
