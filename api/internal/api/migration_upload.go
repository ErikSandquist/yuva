package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/email"
	"github.com/productdevbook/yuva/api/internal/migrate"
	"github.com/productdevbook/yuva/api/internal/migrate/fileread"
	"github.com/productdevbook/yuva/api/internal/migrate/imapread"
	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/storage"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	fileSource = string(oas.MigrationSourceFile)

	uploadPartSize        = 8 << 20
	defaultUploadMaxBytes = 20 << 30
	zipMaxEntries         = 1000000
	zipMaxRatio           = 100
	missingPartsShown     = 100
	uploadStreams         = 2
	uploadSkipForward     = 256 << 10
)

var (
	errPartTooLarge     = problem(http.StatusRequestEntityTooLarge, "part_too_large", "a part is at most 8 MiB")
	errChecksumMismatch = problem(http.StatusUnprocessableEntity, "checksum_mismatch", "the part's bytes do not match its sha256; send it again")
	errNotFileMigration = errMigrationState("this migration reads a mailbox; only a file migration takes an upload")
	errUploadFirst      = errMigrationState("upload the export file and complete the upload first")
	errScanPaused       = errors.New("the scan's time is up")
)

func errUploadTooLarge(max int64) *apiError {
	return errHostedLimit(oas.LimitMigrationUploadMaxBytes, "this server takes export files of at most "+strconv.FormatInt(max, 10)+" bytes (YUVA_MIGRATION_UPLOAD_MAX_BYTES)")
}

// uploadDoc is migrations.upload: the export file of a file migration and how far its scan read.
type uploadDoc struct {
	Filename    string          `json:"filename,omitempty"`
	Size        int64           `json:"size,omitempty"`
	Parts       int32           `json:"parts,omitempty"`
	MboxFormat  string          `json:"mbox_format,omitempty"`
	Layout      fileread.Layout `json:"layout"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
	Scan        fileread.Cursor `json:"scan"`
}

func readUpload(raw []byte) uploadDoc {
	var d uploadDoc
	_ = json.Unmarshal(raw, &d)
	return d
}

func fileMigration(m store.Migration) bool { return m.Source == fileSource }

// uploadReady tells a file migration whose export file is complete and still there.
func uploadReady(m store.Migration) bool {
	d := readUpload(m.Upload)
	return d.CompletedAt != nil && d.DeletedAt == nil
}

// migrationConnectedSource refuses an action that reads the source before it is connected: a
// mailbox's credentials, or a file migration's complete upload.
func migrationConnectedSource(ctx context.Context, q *store.Queries, m store.Migration) error {
	if fileMigration(m) {
		if !uploadReady(m) {
			return errUploadFirst
		}
		return nil
	}
	return hasMigrationCredentials(ctx, q, m)
}

// folderPreset is the preset whose folders a migration reads unless the team chooses: a Takeout
// file reads All Mail as Gmail does, other files their inbox, sent, archive and user folders.
func folderPreset(m store.Migration) migrate.Preset {
	id := m.Preset
	if fileMigration(m) {
		id = migrate.PresetFile
		if readUpload(m.Upload).Layout.Takeout {
			id = migrate.PresetGmail
		}
	}
	p, _ := migrate.LookupPreset(id)
	return p
}

func fileBody(m store.Migration, parts int32, bytes int64) *oas.MigrationFile {
	d := readUpload(m.Upload)
	out := &oas.MigrationFile{SizeBytes: bytes, Parts: parts, PartSize: uploadPartSize, CompletedAt: d.CompletedAt, DeletedAt: d.DeletedAt}
	if d.Filename != "" {
		out.Filename = &d.Filename
	}
	if d.CompletedAt != nil {
		out.SizeBytes, out.Parts = d.Size, d.Parts
		out.Format = new(oas.MigrationFileFormat(d.Layout.Format))
		if d.MboxFormat != "" {
			out.MboxFormat = new(oas.MigrationMboxFormat(d.MboxFormat))
		}
	}
	return out
}

func (s *Server) GetMigrationUpload(ctx context.Context, req oas.GetMigrationUploadRequestObject) (oas.GetMigrationUploadResponseObject, error) {
	p := principalFrom(ctx)
	m, err := visibleMigration(ctx, s.st.Queries, p, req.MigrationId, false)
	if err != nil {
		return nil, err
	}
	if !fileMigration(m) {
		return nil, errNotFileMigration
	}
	rows, err := s.st.ListMigrationUploadParts(ctx, store.ListMigrationUploadPartsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
	if err != nil {
		return nil, err
	}
	d := readUpload(m.Upload)
	out := oas.GetMigrationUpload200JSONResponse{
		PartSize: uploadPartSize, MaxBytes: s.migrations.UploadMaxBytes, Parts: make([]oas.MigrationUploadPart, len(rows)),
		CompletedAt: d.CompletedAt, DeletedAt: d.DeletedAt,
	}
	for i, r := range rows {
		out.Parts[i] = oas.MigrationUploadPart{Part: r.Part, SizeBytes: r.SizeBytes, Sha256: r.Sha256}
		out.ReceivedBytes += r.SizeBytes
	}
	return out, nil
}

func randomSuffix() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) UploadMigrationPart(ctx context.Context, req oas.UploadMigrationPartRequestObject) (oas.UploadMigrationPartResponseObject, error) {
	p := principalFrom(ctx)
	m, err := visibleMigration(ctx, s.st.Queries, p, req.MigrationId, false)
	if err != nil {
		return nil, err
	}
	if err := uploadOpen(m); err != nil {
		return nil, err
	}
	max := s.migrations.UploadMaxBytes
	if req.Part < 0 {
		return nil, errValidation("part is a number from 0")
	}
	start := int64(req.Part) * uploadPartSize
	if start >= max {
		return nil, errUploadTooLarge(max)
	}
	var data []byte
	sum, sawFile := "", false
	for {
		part, err := req.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return nil, errPartTooLarge
		}
		if err != nil {
			return nil, errValidation("the multipart body is not valid")
		}
		switch part.FormName() {
		case "file":
			if sawFile {
				return nil, errValidation("send one file")
			}
			sawFile = true
			if data, err = io.ReadAll(io.LimitReader(part, uploadPartSize+1)); err != nil {
				if errors.As(err, &tooBig) {
					return nil, errPartTooLarge
				}
				return nil, errValidation("the multipart body is not valid")
			}
			if len(data) > uploadPartSize {
				return nil, errPartTooLarge
			}
		case "sha256":
			v, err := io.ReadAll(io.LimitReader(part, 200))
			if err != nil {
				return nil, errValidation("the multipart body is not valid")
			}
			sum = strings.ToLower(strings.TrimSpace(string(v)))
		default:
			return nil, errValidation("unknown form field " + part.FormName())
		}
	}
	if !sawFile || len(data) == 0 {
		return nil, errValidation("file is required and not empty")
	}
	if b, err := hex.DecodeString(sum); err != nil || len(b) != sha256.Size {
		return nil, errValidation("sha256 must be the part's SHA-256 in hexadecimal")
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != sum {
		return nil, errChecksumMismatch
	}
	if start+int64(len(data)) > max {
		return nil, errUploadTooLarge(max)
	}
	key := m.WorkspaceID.String() + "/migrations/" + m.ID.String() + "/upload/" + strconv.Itoa(int(req.Part)) + "-" + randomSuffix()
	if err := s.objects.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "application/octet-stream"); err != nil {
		return nil, err
	}
	var old string
	err = s.st.InTx(ctx, func(q *store.Queries) error {
		cur, err := visibleMigration(ctx, q, p, req.MigrationId, true)
		if err != nil {
			return err
		}
		if err := uploadOpen(cur); err != nil {
			return err
		}
		if prev, err := q.GetMigrationUploadPart(ctx, store.GetMigrationUploadPartParams{WorkspaceID: cur.WorkspaceID, MigrationID: cur.ID, Part: req.Part}); err == nil {
			old = prev.StorageKey
		} else if !store.IsNotFound(err) {
			return err
		}
		return q.UpsertMigrationUploadPart(ctx, store.UpsertMigrationUploadPartParams{
			WorkspaceID: cur.WorkspaceID, MigrationID: cur.ID, Part: req.Part, SizeBytes: int64(len(data)), Sha256: sum, StorageKey: key, Now: s.now(),
		})
	})
	if err != nil {
		s.deleteObjects(ctx, []string{key})
		return nil, err
	}
	if old != "" {
		s.deleteObjects(ctx, []string{old})
	}
	return oas.UploadMigrationPart200JSONResponse{Part: req.Part, SizeBytes: int64(len(data)), Sha256: sum}, nil
}

// uploadOpen refuses a part for a migration that does not take one now.
func uploadOpen(m store.Migration) error {
	switch {
	case !fileMigration(m):
		return errNotFileMigration
	case m.State != migrationDraft || readUpload(m.Upload).CompletedAt != nil:
		return errMigrationState("parts are uploaded while the migration is a draft, before the upload is completed; start a new migration for another file")
	}
	return nil
}

func uploadIncomplete(detail string, missing []int32) *apiError {
	e := problem(http.StatusUnprocessableEntity, "upload_incomplete", detail)
	if len(missing) > 0 {
		e.Extra = map[string]any{"missing_parts": missing}
	}
	return e
}

func fileRefused(r *fileread.Refusal) *apiError {
	e := problem(http.StatusUnprocessableEntity, "file_refused", r.Detail)
	e.Extra = map[string]any{"reason": r.Reason}
	if r.Entry != "" {
		e.Extra["entry"] = r.Entry
	}
	return e
}

// checkParts compares the stored parts with the file the person says they make.
func checkParts(rows []store.ListMigrationUploadPartsRow, parts int32, size int64) error {
	have := map[int32]int64{}
	var total int64
	for _, r := range rows {
		if r.Part >= parts {
			return uploadIncomplete("part "+strconv.Itoa(int(r.Part))+" is beyond the "+strconv.Itoa(int(parts))+" parts named", nil)
		}
		have[r.Part] = r.SizeBytes
		total += r.SizeBytes
	}
	var missing []int32
	for i := int32(0); i < parts; i++ {
		if _, ok := have[i]; !ok && len(missing) < missingPartsShown {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		return uploadIncomplete("some parts are missing; upload them and complete again", missing)
	}
	for i := int32(0); i < parts-1; i++ {
		if have[i] != uploadPartSize {
			return uploadIncomplete("part "+strconv.Itoa(int(i))+" is not 8 MiB; every part but the last is", []int32{i})
		}
	}
	if total != size {
		return uploadIncomplete("the parts hold "+strconv.FormatInt(total, 10)+" bytes, not size_bytes", nil)
	}
	return nil
}

func (s *Server) CompleteMigrationUpload(ctx context.Context, req oas.CompleteMigrationUploadRequestObject) (oas.CompleteMigrationUploadResponseObject, error) {
	p := principalFrom(ctx)
	b := req.Body
	m, err := visibleMigration(ctx, s.st.Queries, p, req.MigrationId, false)
	if err != nil {
		return nil, err
	}
	if !fileMigration(m) {
		return nil, errNotFileMigration
	}
	if m.State == migrationConnected && readUpload(m.Upload).CompletedAt != nil {
		out, err := s.oneMigration(ctx, s.st.Queries, m)
		if err != nil {
			return nil, err
		}
		return oas.CompleteMigrationUpload200JSONResponse(out), nil
	}
	if err := uploadOpen(m); err != nil {
		return nil, err
	}
	if b.Parts < 1 || b.SizeBytes < 1 {
		return nil, errValidation("parts and size_bytes are at least 1")
	}
	filename := ""
	if b.Filename != nil {
		filename = truncateRunes(strings.TrimSpace(*b.Filename), 255)
	}
	variant := fileread.MboxRD
	if b.MboxFormat != nil {
		if !b.MboxFormat.Valid() {
			return nil, errValidation("mbox_format is mboxrd or mboxo")
		}
		variant = string(*b.MboxFormat)
	}
	if b.SizeBytes > s.migrations.UploadMaxBytes {
		return nil, errUploadTooLarge(s.migrations.UploadMaxBytes)
	}
	rows, err := s.st.ListMigrationUploadParts(ctx, store.ListMigrationUploadPartsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
	if err != nil {
		return nil, err
	}
	if err := checkParts(rows, b.Parts, b.SizeBytes); err != nil {
		return nil, err
	}
	f := s.uploadReader(ctx, rows)
	layout, folders, err := fileread.Inspect(f, f.size, filename, fileread.Limits{MaxEntries: zipMaxEntries, MaxRatio: zipMaxRatio, MaxEntry: s.migrations.UploadMaxBytes})
	f.Close()
	var refused *fileread.Refusal
	if errors.As(err, &refused) {
		return nil, s.refuseUpload(ctx, p, m, rows, refused)
	}
	if err != nil {
		return nil, err
	}
	var out oas.Migration
	err = s.inTx(ctx, p.workspaceID, func(q *store.Queries, events *eventBatch) error {
		cur, err := visibleMigration(ctx, q, p, req.MigrationId, true)
		if err != nil {
			return err
		}
		if err := uploadOpen(cur); err != nil {
			return err
		}
		current, err := q.ListMigrationUploadParts(ctx, store.ListMigrationUploadPartsParams{WorkspaceID: cur.WorkspaceID, MigrationID: cur.ID})
		if err != nil {
			return err
		}
		if !slices.Equal(current, rows) {
			return errMigrationState("a part changed while the file was read; complete the upload again")
		}
		now := s.now()
		doc := uploadDoc{Filename: filename, Size: b.SizeBytes, Parts: b.Parts, MboxFormat: variant, Layout: layout, CompletedAt: &now}
		m, err := q.SetMigrationUpload(ctx, store.SetMigrationUploadParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Upload: mustJSON(doc), State: migrationConnected, Now: now})
		if err != nil {
			return err
		}
		listed := make([]listedFolder, len(folders))
		for i, f := range folders {
			listed[i] = listedFolder{Name: f.Name, Delim: f.Delim, Kind: f.Kind, Messages: f.Messages}
		}
		if err := s.recordFolders(ctx, q, m, listed); err != nil {
			return err
		}
		if _, err := s.migrationAudit(ctx, q, p, m.ID, "upload_completed", map[string]any{"format": layout.Format, "size": b.SizeBytes, "parts": b.Parts}); err != nil {
			return err
		}
		if out, err = s.oneMigration(ctx, q, m); err != nil {
			return err
		}
		return s.migrationChanged(ctx, q, events, m, &cur.State)
	})
	if err != nil {
		return nil, err
	}
	return oas.CompleteMigrationUpload200JSONResponse(out), nil
}

// refuseUpload records why a file was refused and deletes its parts, which no scan can use.
func (s *Server) refuseUpload(ctx context.Context, p principal, m store.Migration, rows []store.ListMigrationUploadPartsRow, r *fileread.Refusal) error {
	err := s.st.InTx(ctx, func(q *store.Queries) error {
		if _, err := visibleMigration(ctx, q, p, m.ID, true); err != nil {
			return err
		}
		if err := q.DeleteMigrationUploadParts(ctx, store.DeleteMigrationUploadPartsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID}); err != nil {
			return err
		}
		_, err := s.migrationAudit(ctx, q, p, m.ID, "upload_refused", map[string]any{"reason": r.Reason, "entry": r.Entry})
		return err
	})
	if err != nil {
		return err
	}
	keys := make([]string, len(rows))
	for i, row := range rows {
		keys[i] = row.StorageKey
	}
	s.deleteObjects(ctx, keys)
	return fileRefused(r)
}

// dropUpload deletes a file migration's parts, and so its file, once the migration ended.
func (s *Server) dropUpload(ctx context.Context, workspaceID, migrationID uuid.UUID) error {
	ctx = context.WithoutCancel(ctx)
	rows, err := s.st.ListMigrationUploadParts(ctx, store.ListMigrationUploadPartsParams{WorkspaceID: workspaceID, MigrationID: migrationID})
	if err != nil || len(rows) == 0 {
		return err
	}
	for _, r := range rows {
		if err := s.objects.Delete(ctx, r.StorageKey); err != nil {
			return err
		}
	}
	return s.st.InTx(ctx, func(q *store.Queries) error {
		if err := q.DeleteMigrationUploadParts(ctx, store.DeleteMigrationUploadPartsParams{WorkspaceID: workspaceID, MigrationID: migrationID}); err != nil {
			return err
		}
		if err := q.SetMigrationUploadDeleted(ctx, store.SetMigrationUploadDeletedParams{WorkspaceID: workspaceID, ID: migrationID, Now: s.now()}); err != nil {
			return err
		}
		_, err := q.AddMigrationAudit(ctx, store.AddMigrationAuditParams{
			WorkspaceID: workspaceID, MigrationID: migrationID, Action: "file_deleted", Detail: mustJSON(map[string]any{"parts": len(rows)}), Now: s.now(),
		})
		return err
	})
}

func (s *Server) dropUploadLogged(ctx context.Context, workspaceID, migrationID uuid.UUID) {
	if err := s.dropUpload(ctx, workspaceID, migrationID); err != nil {
		s.log.WarnContext(ctx, "migration file not deleted yet", slog.String("migration_id", migrationID.String()), slog.Any("error", err))
	}
}

// dropEndedUploads deletes the parts of migrations that ended while a deletion failed.
func (s *Server) dropEndedUploads(ctx context.Context) error {
	rows, err := s.st.ListEndedMigrationUploads(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := s.dropUpload(ctx, r.WorkspaceID, r.ID); err != nil {
			return err
		}
	}
	return nil
}

// uploadFile reads an export file from its parts. Reads that follow each other continue the
// part streams already open (two of them, as a zip is read in two places), so reading the file
// in order costs one request per part.
type uploadFile struct {
	ctx     context.Context
	objects storage.Storage
	parts   []store.ListMigrationUploadPartsRow
	starts  []int64
	size    int64

	mu      sync.Mutex
	streams []*partStream
	tick    int
}

type partStream struct {
	r    io.ReadCloser
	part int
	pos  int64
	used int
}

func (s *Server) uploadReader(ctx context.Context, rows []store.ListMigrationUploadPartsRow) *uploadFile {
	u := &uploadFile{ctx: ctx, objects: s.objects, parts: rows, starts: make([]int64, len(rows))}
	for i, r := range rows {
		u.starts[i] = u.size
		u.size += r.SizeBytes
	}
	return u
}

func (s *Server) openUpload(ctx context.Context, m store.Migration) (*uploadFile, error) {
	rows, err := s.st.ListMigrationUploadParts(ctx, store.ListMigrationUploadPartsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
	if err != nil {
		return nil, err
	}
	if d := readUpload(m.Upload); int32(len(rows)) != d.Parts {
		return nil, errors.New("the export file's parts are gone")
	}
	return s.uploadReader(ctx, rows), nil
}

func (u *uploadFile) Close() {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, st := range u.streams {
		_ = st.r.Close()
	}
	u.streams = nil
}

func (u *uploadFile) ReadAt(p []byte, off int64) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	n := 0
	for n < len(p) {
		at := off + int64(n)
		if at >= u.size {
			return n, io.EOF
		}
		i := sort.Search(len(u.starts), func(i int) bool { return u.starts[i] > at }) - 1
		st, err := u.stream(i, at)
		if err != nil {
			return n, err
		}
		want := min(int64(len(p)-n), u.starts[i]+u.parts[i].SizeBytes-at)
		got, err := io.ReadFull(st.r, p[n:n+int(want)])
		n += got
		st.pos += int64(got)
		if err != nil {
			u.drop(st)
			return n, err
		}
	}
	return n, nil
}

// stream is an open read of part i at offset at: one already there, one a little behind it, or
// a new one in place of the least used.
func (u *uploadFile) stream(i int, at int64) (*partStream, error) {
	u.tick++
	for _, st := range u.streams {
		if st.part != i || st.pos > at || at-st.pos > uploadSkipForward {
			continue
		}
		if _, err := io.CopyN(io.Discard, st.r, at-st.pos); err != nil {
			u.drop(st)
			break
		}
		st.pos, st.used = at, u.tick
		return st, nil
	}
	in := at - u.starts[i]
	r, err := u.objects.OpenRange(u.ctx, u.parts[i].StorageKey, in, u.parts[i].SizeBytes-in)
	if err != nil {
		return nil, err
	}
	st := &partStream{r: r, part: i, pos: at, used: u.tick}
	if len(u.streams) >= uploadStreams {
		oldest := slices.MinFunc(u.streams, func(a, b *partStream) int { return a.used - b.used })
		u.drop(oldest)
	}
	u.streams = append(u.streams, st)
	return st, nil
}

func (u *uploadFile) drop(st *partStream) {
	_ = st.r.Close()
	u.streams = slices.DeleteFunc(u.streams, func(x *partStream) bool { return x == st })
}

// listFileFolders records a file's folders again, as known, with the current options.
func (r *migrationRun) listFileFolders(ctx context.Context) error {
	return r.s.st.InTx(ctx, func(q *store.Queries) error {
		rows, err := q.ListMigrationFolders(ctx, store.ListMigrationFoldersParams{WorkspaceID: r.m.WorkspaceID, MigrationID: r.m.ID})
		if err != nil {
			return err
		}
		return r.s.recordFolders(ctx, q, r.m, listedFromRows(rows, nil))
	})
}

func listedFromRows(rows []store.MigrationFolder, extra []listedFolder) []listedFolder {
	out := make([]listedFolder, 0, len(rows)+len(extra))
	for _, f := range rows {
		out = append(out, listedFolder{Name: f.Name, Delim: f.Delim, Kind: f.Kind, Messages: int64(f.Total)})
	}
	return append(out, extra...)
}

type fileScanFolder struct {
	adds           int32
	size           int64
	oldest, newest *time.Time
}

// scanFile reads the export file's messages from where the last slice stopped, a page at a time,
// and stores them as a mailbox scan stores headers: one item per message and folder.
func (r *migrationRun) scanFile(ctx context.Context) (bool, error) {
	s, m := r.s, r.m
	doc := readUpload(m.Upload)
	if doc.Scan.Done {
		return false, nil
	}
	f, err := s.openUpload(ctx, m)
	if err != nil {
		return false, err
	}
	defer f.Close()
	rows, err := s.st.ListMigrationFolders(ctx, store.ListMigrationFoldersParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
	if err != nil {
		return false, err
	}
	byName := map[string]store.MigrationFolder{}
	for _, row := range rows {
		byName[row.Name] = row
	}
	now := s.now()
	var items []migrate.Item
	stats := map[int32]*fileScanFolder{}
	cursor := doc.Scan
	stat := func(id int32) *fileScanFolder {
		if stats[id] == nil {
			stats[id] = &fileScanFolder{}
		}
		return stats[id]
	}
	flush := func(done bool) error {
		cursor.Done = done
		p := store.InsertMigrationItemsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID, State: migrate.StateSkipped, Reason: migrate.ReasonUnplanned}
		for _, it := range items {
			p.SourceIds, p.FolderIds, p.DedupeKeys = append(p.SourceIds, it.SourceID), append(p.FolderIds, it.FolderID), append(p.DedupeKeys, it.DedupeKey)
			p.ReceivedAts, p.Sizes, p.Metas = append(p.ReceivedAts, it.Received), append(p.Sizes, it.Size), append(p.Metas, mustJSON(it.Meta))
		}
		err := s.st.InTx(ctx, func(q *store.Queries) error {
			if len(items) > 0 {
				if _, err := q.InsertMigrationItems(ctx, p); err != nil {
					return err
				}
			}
			var ids, adds []int32
			for id, st := range stats {
				if st.adds > 0 {
					ids, adds = append(ids, id), append(adds, st.adds)
				}
				if st.size > 0 || st.oldest != nil {
					if err := q.SetMigrationFolderScanned(ctx, store.SetMigrationFolderScannedParams{
						WorkspaceID: m.WorkspaceID, MigrationID: m.ID, ID: id, SizeBytes: st.size, OldestAt: st.oldest, NewestAt: st.newest,
					}); err != nil {
						return err
					}
				}
			}
			if len(ids) > 0 {
				if err := q.AddMigrationFolderTotals(ctx, store.AddMigrationFolderTotalsParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID, Ids: ids, Adds: adds}); err != nil {
					return err
				}
			}
			return q.SetMigrationUploadScan(ctx, store.SetMigrationUploadScanParams{WorkspaceID: m.WorkspaceID, ID: m.ID, Scan: mustJSON(cursor)})
		})
		items, stats = nil, map[int32]*fileScanFolder{}
		return err
	}
	folder := func(name string) (store.MigrationFolder, error) {
		if f, ok := byName[name]; ok {
			return f, nil
		}
		kind := migrate.FolderKind(imapread.Folder{Name: name, Delim: "/"}, doc.Layout.Takeout)
		err := s.st.InTx(ctx, func(q *store.Queries) error {
			cur, err := q.ListMigrationFolders(ctx, store.ListMigrationFoldersParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
			if err != nil {
				return err
			}
			return s.recordFolders(ctx, q, m, listedFromRows(cur, []listedFolder{{Name: name, Delim: "/", Kind: kind}}))
		})
		if err != nil {
			return store.MigrationFolder{}, err
		}
		rows, err := s.st.ListMigrationFolders(ctx, store.ListMigrationFoldersParams{WorkspaceID: m.WorkspaceID, MigrationID: m.ID})
		if err != nil {
			return store.MigrationFolder{}, err
		}
		for _, row := range rows {
			byName[row.Name] = row
		}
		f, ok := byName[name]
		if !ok {
			return f, errors.New("the folder " + name + " was not recorded")
		}
		return f, nil
	}
	err = fileread.Scan(f, f.size, doc.Layout, doc.MboxFormat, doc.Scan, s.migrations.MaxMessageBytes, func(msg fileread.Message) error {
		cursor = msg.Next
		if msg.Err != "" {
			s.addMigrationError(ctx, m, "unreadable", msg.Err, msg.Folder, nil, "")
		}
		if msg.SourceID != "" && msg.Err == "" {
			it := migrate.ScanRaw(msg.Header, msg.Parts, msg.Received, msg.Size, now)
			if it.Meta.ID == "" && msg.Raw != nil {
				it.DedupeKey = email.SynthesizedID(msg.Raw)
			}
			it.Meta.Thread = msg.Thread
			names := msg.Labels
			if msg.Folder != "" {
				names = append([]string{msg.Folder}, names...)
			}
			for i, name := range names {
				fo, err := folder(name)
				if err != nil {
					return err
				}
				st := stat(fo.ID)
				if msg.Counted {
					st.adds++
				}
				if !migrate.Readable(fo.Kind) {
					continue
				}
				c := it
				c.FolderID, c.SourceID = fo.ID, msg.SourceID
				if i > 0 || msg.Folder == "" {
					c.SourceID = msg.SourceID + "#" + strconv.Itoa(int(fo.ID))
				}
				items = append(items, c)
				at := it.Meta.At
				st.size += it.Size
				if st.oldest == nil || at.Before(*st.oldest) {
					st.oldest = &at
				}
				if st.newest == nil || at.After(*st.newest) {
					st.newest = &at
				}
			}
		}
		if len(items) < scanHeaderPage {
			return nil
		}
		if err := flush(false); err != nil {
			return err
		}
		if ok, err := r.renew(ctx); err != nil || !ok {
			if err == nil {
				err = errScanPaused
			}
			return err
		}
		if !r.timeLeft() {
			return errScanPaused
		}
		return nil
	})
	switch {
	case errors.Is(err, errScanPaused):
		return true, nil
	case err != nil:
		return false, err
	}
	return false, flush(true)
}

// fetchFile reads messages again from the export file for the copy and the preview, in the
// order they are in the file.
func (r *migrationRun) fetchFile(ctx context.Context, ids []string) (map[string]fetchedItem, int64, error) {
	if r.file == nil {
		f, err := r.s.openUpload(ctx, r.m)
		if err != nil {
			return nil, 0, err
		}
		r.file = f
	}
	doc := readUpload(r.m.Upload)
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b string) int { return fileOffset(a) - fileOffset(b) })
	out := map[string]fetchedItem{}
	var read int64
	for _, id := range sorted {
		raw, tooLarge, err := fileread.Read(r.file, r.file.size, id, doc.MboxFormat, r.s.migrations.MaxMessageBytes)
		switch {
		case errors.Is(err, fileread.ErrUnreadable):
			out[id] = fetchedItem{present: true, unreadable: true}
		case err != nil:
			return nil, read, err
		case tooLarge:
			out[id] = fetchedItem{present: true}
		default:
			out[id] = fetchedItem{raw: raw, present: true}
			read += int64(len(raw))
		}
	}
	return out, read, nil
}

func fileOffset(id string) int {
	_, rest, _ := strings.Cut(id, ":")
	n, _ := strconv.Atoi(rest[:strings.IndexAny(rest+"+", "+")])
	return n
}

// followFile finds the file migration of the same mailbox an IMAP migration into channel
// continues from, and the day its newest message is from.
func followFile(ctx context.Context, q *store.Queries, ws uuid.UUID, channel *uuid.UUID, address string) (*uuid.UUID, *time.Time, error) {
	if channel == nil {
		return nil, nil, nil
	}
	f, err := q.FindFollowedFileMigration(ctx, store.FindFollowedFileMigrationParams{WorkspaceID: ws, ChannelID: channel, SourceAddress: address})
	if store.IsNotFound(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	since := f.NewestAt.UTC().Truncate(24 * time.Hour)
	return &f.ID, &since, nil
}
