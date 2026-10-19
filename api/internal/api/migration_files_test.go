package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
)

const partSize = 8 << 20

// partBody is a part's multipart body; cut leaves it off after so many bytes of the file, as a
// connection that dropped would.
func partBody(data []byte, sum string, cut int) ([]byte, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("sha256", sum)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="part"`)
	hdr.Set("Content-Type", "application/octet-stream")
	pw, _ := w.CreatePart(hdr)
	if cut >= 0 {
		_, _ = pw.Write(data[:cut])
		return buf.Bytes(), w.FormDataContentType()
	}
	_, _ = pw.Write(data)
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func (c *client) sendPart(id string, n int, data []byte, sum string, cut int) response {
	c.h.t.Helper()
	body, ct := partBody(data, sum, cut)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/v1/migrations/%s/upload/parts/%d", c.h.url, id, n), bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Forwarded-For", c.ip)
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := response{status: res.StatusCode, raw: raw, header: res.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// uploadFile sends a file in 8 MiB parts and completes the upload.
func (c *client) uploadFile(t *testing.T, id string, data []byte, filename string) response {
	t.Helper()
	parts := 0
	for off := 0; off < len(data); off += partSize {
		part := data[off:min(off+partSize, len(data))]
		if r := c.sendPart(id, parts, part, sha(part), -1); r.status != http.StatusOK {
			t.Fatalf("part %d: %d %s", parts, r.status, r.raw)
		}
		parts++
	}
	return c.do("POST", "/v1/migrations/"+id+"/upload/complete", map[string]any{"parts": parts, "size_bytes": len(data), "filename": filename})
}

func newFileRun(t *testing.T, configure func(*api.Deps)) *dovecotRun {
	t.Helper()
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Migrations = api.MigrationSettings{AllowPrivate: true}
		d.Attachments = api.AttachmentSettings{MaxBytes: 25 << 20, Types: []string{"image/png", "application/pdf", "text/plain"}}
		if configure != nil {
			configure(d)
		}
	})
	return &dovecotRun{h: h, tm: newTeam(t, h)}
}

// emailChannel adds an e-mail channel to the team's inbox.
func (d *dovecotRun) emailChannel(t *testing.T) (string, string) {
	t.Helper()
	address := unique("support") + "@example.com"
	channel := d.tm.owner.expect(http.StatusCreated, "POST", "/v1/inboxes/"+d.tm.inbox+"/channels", map[string]any{
		"kind": "email", "name": "Support mail",
		"email": map[string]any{"address": address, "display_name": "Support",
			"smtp": map[string]any{"host": "smtp.example.com", "port": 2525, "username": "mailer", "password": "smtp-secret", "tls": "starttls"}},
	}).str("id")
	return address, channel
}

func (d *dovecotRun) fileMigration(t *testing.T, preset, address, channel string, team []string) string {
	t.Helper()
	return d.tm.owner.expect(http.StatusCreated, "POST", "/v1/migrations", map[string]any{
		"source": "file", "preset": preset, "source_address": address, "channel_id": channel, "options": map[string]any{"team_addresses": team},
	}).str("id")
}

// uploadedFiles lists the stored objects under a migration's upload prefix.
func (d *dovecotRun) uploadedFiles(id string) []string {
	var out []string
	_ = filepath.WalkDir(filepath.Join(d.h.storage.Dir, d.tm.ws, "migrations", id), func(p string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// bigMbox is an mbox of n one-message threads of about body bytes each, from customers to address.
func bigMbox(n, body int, address string) []byte {
	var b strings.Builder
	filler := strings.Repeat("All work and no play makes a long message.\n", body/43+1)
	at := time.Date(2024, 5, 1, 9, 0, 0, 0, time.UTC)
	for i := range n {
		d := at.Add(time.Duration(i) * time.Hour)
		fmt.Fprintf(&b, "From customer%d@example.org %s\n", i, d.Format("Mon Jan 02 15:04:05 -0700 2006"))
		fmt.Fprintf(&b, "From: Customer %d <customer%d@example.org>\nTo: %s\nSubject: Question %d\nDate: %s\nMessage-ID: <q-%d@example.org>\n\nQuestion %d.\n>From the start of the line.\n%s\n",
			i, i, address, i, d.Format(time.RFC1123Z), i, i, filler)
	}
	return []byte(b.String())
}

func TestFileMigrationUploadResumesAndImports(t *testing.T) {
	t.Parallel()
	d := newFileRun(t, nil)
	h, tm := d.h, d.tm
	ctx := context.Background()
	address, channel := d.emailChannel(t)
	id := d.fileMigration(t, "file", address, channel, []string{address})
	path := "/v1/migrations/" + id
	m := d.migration(id)
	if m.str("state") != "draft" || m.str("source") != "file" || !reflect.DeepEqual(m.body["actions"], []any{"update", "upload", "delete"}) {
		t.Fatalf("created: %s", m.raw)
	}
	if up := m.body["upload"].(map[string]any); num(up["parts"]) != 0 || num(up["part_size"]) != partSize || up["format"] != nil {
		t.Fatalf("upload before any part: %v", up)
	}
	tm.owner.expectProblem(http.StatusConflict, "migration_state", "PUT", path+"/credentials", map[string]any{"username": "a", "password": "b"})
	tm.owner.expectProblem(http.StatusConflict, "migration_state", "POST", path+"/test", nil)
	tm.owner.expectProblem(http.StatusConflict, "migration_state", "POST", path+"/scan", nil)

	file := bigMbox(220, 100<<10, address)
	if len(file) <= 2*partSize || len(file) > 3*partSize {
		t.Fatalf("the file is %d bytes; it must take three parts", len(file))
	}
	sixty := len(file) * 6 / 10
	part0, part1, part2 := file[:partSize], file[partSize:2*partSize], file[2*partSize:]
	if r := tm.owner.sendPart(id, 0, part0, sha(part0), -1); r.status != http.StatusOK || num(r.body["size_bytes"]) != partSize || r.str("sha256") != sha(part0) {
		t.Fatalf("part 0: %d %s", r.status, r.raw)
	}
	if r := tm.owner.sendPart(id, 1, part1, sha(part1), sixty-partSize); r.status != http.StatusBadRequest {
		t.Fatalf("part 1 cut at 60%%: %d %s", r.status, r.raw)
	}
	up := tm.owner.expect(http.StatusOK, "GET", path+"/upload", nil)
	if parts := up.body["parts"].([]any); len(parts) != 1 || num(parts[0].(map[string]any)["part"]) != 0 || num(up.body["received_bytes"]) != partSize ||
		num(up.body["part_size"]) != partSize || num(up.body["max_bytes"]) != 20<<30 {
		t.Fatalf("after the cut: %s", up.raw)
	}
	r := tm.owner.do("POST", path+"/upload/complete", map[string]any{"parts": 3, "size_bytes": len(file), "filename": "Clients.mbox"})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "upload_incomplete" || !reflect.DeepEqual(r.body["missing_parts"], []any{1.0, 2.0}) {
		t.Fatalf("complete with parts missing: %d %s", r.status, r.raw)
	}
	if r := tm.owner.sendPart(id, 1, part1, sha(part2), -1); r.status != http.StatusUnprocessableEntity || r.str("code") != "checksum_mismatch" {
		t.Fatalf("part 1 with a wrong sha256: %d %s", r.status, r.raw)
	}
	for n, p := range map[int][]byte{1: part1, 2: part2} {
		if r := tm.owner.sendPart(id, n, p, sha(p), -1); r.status != http.StatusOK {
			t.Fatalf("part %d: %d %s", n, r.status, r.raw)
		}
	}
	if r := tm.owner.sendPart(id, 2, part2, sha(part2), -1); r.status != http.StatusOK {
		t.Fatalf("part 2 again: %d %s", r.status, r.raw)
	}
	if n := len(d.uploadedFiles(id)); n != 3 {
		t.Fatalf("%d stored parts, want 3: a part sent again replaces the one before", n)
	}
	other := newTeam(t, h)
	other.owner.expectProblem(http.StatusNotFound, "not_found", "GET", path+"/upload", nil)
	if r := other.owner.sendPart(id, 0, part0, sha(part0), -1); r.status != http.StatusNotFound {
		t.Fatalf("another workspace's part: %d %s", r.status, r.raw)
	}
	other.owner.expectProblem(http.StatusNotFound, "not_found", "POST", path+"/upload/complete", map[string]any{"parts": 3, "size_bytes": len(file)})
	tm.agent.expectProblem(http.StatusForbidden, "forbidden", "GET", path+"/upload", nil)

	m = tm.owner.expect(http.StatusOK, "POST", path+"/upload/complete", map[string]any{"parts": 3, "size_bytes": len(file), "filename": "Clients.mbox"})
	fu := m.body["upload"].(map[string]any)
	if m.str("state") != "connected" || fu["format"] != "mbox" || fu["mbox_format"] != "mboxrd" || fu["filename"] != "Clients.mbox" ||
		num(fu["size_bytes"]) != len(file) || num(fu["parts"]) != 3 || !reflect.DeepEqual(m.body["actions"], []any{"update", "scan", "cancel", "delete"}) {
		t.Fatalf("completed: %s", m.raw)
	}
	tm.owner.expect(http.StatusOK, "POST", path+"/upload/complete", map[string]any{"parts": 3, "size_bytes": len(file)})
	if r := tm.owner.sendPart(id, 0, part0, sha(part0), -1); r.status != http.StatusConflict {
		t.Fatalf("a part after completion: %d %s", r.status, r.raw)
	}
	folders := tm.owner.expect(http.StatusOK, "GET", path+"/folders", nil).body["items"].([]any)
	if len(folders) != 1 || folders[0].(map[string]any)["name"] != "Clients" || folders[0].(map[string]any)["kind"] != "archive" || folders[0].(map[string]any)["use"] != "import" {
		t.Fatalf("folders: %v", folders)
	}
	tm.owner.expect(http.StatusOK, "POST", path+"/scan", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	m = d.migration(id)
	est, _ := m.body["estimate"].(map[string]any)
	if m.str("state") != "ready" || num(est["conversations"]) != 220 || num(est["messages_in"]) != 220 {
		t.Fatalf("after the scan: %s", m.raw)
	}
	tm.owner.expect(http.StatusOK, "POST", path+"/start", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	m = d.migration(id)
	counts := m.body["counts"].(map[string]any)
	fu = m.body["upload"].(map[string]any)
	if m.str("state") != "completed" || num(counts["imported"]) != 220 || num(counts["failed"]) != 0 || fu["deleted_at"] == nil {
		t.Fatalf("after the copy: %s", m.raw)
	}
	if n := h.count("SELECT count(*) FROM conversations WHERE workspace_id = $1 AND migration_id = $2 AND channel_id = $3", tm.ws, id, channel); n != 220 {
		t.Fatalf("%d conversations, want 220", n)
	}
	var body string
	if err := h.st.Pool.QueryRow(ctx, `SELECT e.full_text FROM message_emails e WHERE e.workspace_id = $1 AND e.header_message_id = 'q-7@example.org'`, tm.ws).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Question 7.\r\nFrom the start of the line.") && !strings.Contains(body, "Question 7.\nFrom the start of the line.") {
		t.Fatalf("message 7 read back as %q", body[:min(len(body), 200)])
	}
	if n := h.count("SELECT count(*) FROM migration_upload_parts WHERE workspace_id = $1 AND migration_id = $2", tm.ws, id); n != 0 {
		t.Fatalf("%d upload parts left after completion", n)
	}
	if files := d.uploadedFiles(id); len(files) != 0 {
		t.Fatalf("stored parts left after completion: %v", files)
	}
	if up := tm.owner.expect(http.StatusOK, "GET", path+"/upload", nil); len(up.body["parts"].([]any)) != 0 || up.body["deleted_at"] == nil {
		t.Fatalf("upload after completion: %s", up.raw)
	}
	for _, a := range []string{"upload_completed", "file_deleted"} {
		if h.count("SELECT count(*) FROM migration_audit WHERE workspace_id = $1 AND migration_id = $2 AND action = $3", tm.ws, id, a) != 1 {
			t.Errorf("no %s audit row", a)
		}
	}
	checkNoOrphans(t, d)
}

func zipOf(t *testing.T, files map[string]string, modes map[string]fs.FileMode) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		hdr := &zip.FileHeader{Name: n, Method: zip.Deflate}
		if m, ok := modes[n]; ok {
			hdr.SetMode(m)
		}
		fw, err := w.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte(files[n]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFileMigrationRefusesBadFiles(t *testing.T) {
	t.Parallel()
	d := newFileRun(t, func(dp *api.Deps) { dp.Migrations.UploadMaxBytes = 12 << 20 })
	h, tm := d.h, d.tm
	address, channel := d.emailChannel(t)
	eml := "From: Bob <bob@example.net>\r\nTo: " + address + "\r\nSubject: Hi\r\nMessage-ID: <hi@example.net>\r\n\r\nHello.\r\n"
	cases := []struct {
		name, reason, entry string
		data                []byte
	}{
		{"zip bomb", "zip_bomb", "INBOX/bomb.eml", zipOf(t, map[string]string{"INBOX/ok.eml": eml, "INBOX/bomb.eml": eml + strings.Repeat("\x00", 10<<20)}, nil)},
		{"traversal", "path_traversal", "../../etc/cron.d/evil.eml", zipOf(t, map[string]string{"INBOX/ok.eml": eml, "../../etc/cron.d/evil.eml": eml}, nil)},
		{"symlink", "symlink", "INBOX/link.eml", zipOf(t, map[string]string{"INBOX/ok.eml": eml, "INBOX/link.eml": "/etc/passwd"}, map[string]fs.FileMode{"INBOX/link.eml": fs.ModeSymlink | 0o777})},
		{"not mail", "unknown_format", "", []byte("%PDF-1.4\nnot a mailbox\n")},
		{"empty zip", "empty", "", zipOf(t, map[string]string{"readme.txt": "hello"}, nil)},
	}
	for _, c := range cases {
		id := d.fileMigration(t, "file", address, channel, nil)
		r := tm.owner.uploadFile(t, id, c.data, c.name+".zip")
		if r.status != http.StatusUnprocessableEntity || r.str("code") != "file_refused" || r.str("reason") != c.reason || r.str("entry") != c.entry || r.str("detail") == "" {
			t.Errorf("%s: %d %s", c.name, r.status, r.raw)
			continue
		}
		m := d.migration(id)
		if m.str("state") != "draft" || num(m.body["upload"].(map[string]any)["parts"]) != 0 {
			t.Errorf("%s: after the refusal: %s", c.name, m.raw)
		}
		if files := d.uploadedFiles(id); len(files) != 0 {
			t.Errorf("%s: the refused file's parts are still stored: %v", c.name, files)
		}
		if h.count("SELECT count(*) FROM migration_audit WHERE workspace_id = $1 AND migration_id = $2 AND action = 'upload_refused' AND detail->>'reason' = $3", tm.ws, id, c.reason) != 1 {
			t.Errorf("%s: refusal not audited", c.name)
		}
	}

	id := d.fileMigration(t, "file", address, channel, nil)
	full := bytes.Repeat([]byte("x"), partSize)
	if r := tm.owner.sendPart(id, 0, full, sha(full), -1); r.status != http.StatusOK {
		t.Fatalf("part 0: %d %s", r.status, r.raw)
	}
	over := bytes.Repeat([]byte("y"), 5<<20)
	r := tm.owner.sendPart(id, 1, over, sha(over), -1)
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "hosted_limit" || r.str("limit") != "YUVA_MIGRATION_UPLOAD_MAX_BYTES" {
		t.Fatalf("a part that ends beyond the server's limit: %d %s", r.status, r.raw)
	}
	small := bytes.Repeat([]byte("y"), 4<<20)
	if r := tm.owner.sendPart(id, 1, small, sha(small), -1); r.status != http.StatusOK {
		t.Fatalf("a part that ends at the limit: %d %s", r.status, r.raw)
	}
	if r := tm.owner.sendPart(id, 2, small, sha(small), -1); r.status != http.StatusUnprocessableEntity || r.str("limit") != "YUVA_MIGRATION_UPLOAD_MAX_BYTES" {
		t.Fatalf("a part after the limit: %d %s", r.status, r.raw)
	}
	r = tm.owner.do("POST", "/v1/migrations/"+id+"/upload/complete", map[string]any{"parts": 3, "size_bytes": 13 << 20})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "hosted_limit" || r.str("limit") != "YUVA_MIGRATION_UPLOAD_MAX_BYTES" {
		t.Fatalf("a file over the server's limit: %d %s", r.status, r.raw)
	}
	tooBig := bytes.Repeat([]byte("z"), partSize+1)
	if r := tm.owner.sendPart(id, 0, tooBig, sha(tooBig), -1); r.status != http.StatusRequestEntityTooLarge || r.str("code") != "part_too_large" {
		t.Fatalf("a part over 8 MiB: %d %s", r.status, r.raw)
	}

	imap := tm.owner.expect(http.StatusCreated, "POST", "/v1/migrations", map[string]any{"preset": "imap", "source_address": address}).str("id")
	if r := tm.owner.sendPart(imap, 0, small, sha(small), -1); r.status != http.StatusConflict || r.str("code") != "migration_state" {
		t.Fatalf("a part for a mailbox migration: %d %s", r.status, r.raw)
	}
	tm.owner.expectProblem(http.StatusConflict, "migration_state", "GET", "/v1/migrations/"+imap+"/upload", nil)
	tm.owner.expectProblem(http.StatusBadRequest, "validation_failed", "POST", "/v1/migrations", map[string]any{"source": "file", "preset": "gmail", "source_address": address})

	tm.owner.expect(http.StatusNoContent, "DELETE", "/v1/migrations/"+id, nil)
	if files := d.uploadedFiles(id); len(files) != 0 {
		t.Fatalf("a deleted migration's parts are still stored: %v", files)
	}

	off := newFileRun(t, func(dp *api.Deps) { dp.Migrations.Sources = []string{"imap"} })
	off.tm.owner.expectProblem(http.StatusUnprocessableEntity, "hosted_limit", "POST", "/v1/migrations", map[string]any{"source": "file", "preset": "file", "source_address": address})
}

// conversationSignature describes what a migration imported into its channel, by thread: subject,
// contact, status, labels and each message's Message-ID with its direction.
func conversationSignature(t *testing.T, d *dovecotRun, channel string) []string {
	t.Helper()
	rows, err := d.h.st.Pool.Query(context.Background(), `
SELECT c.subject || ' | ' || ce.email || ' | ' || c.status || ' | ' ||
       coalesce((SELECT string_agg(l.name, ',' ORDER BY l.name) FROM conversation_labels cl JOIN labels l ON l.id = cl.label_id WHERE cl.conversation_id = c.id), '') || ' | ' ||
       (SELECT string_agg(e.header_message_id || ':' || e.direction, ',' ORDER BY e.header_message_id) FROM message_emails e WHERE e.conversation_id = c.id)
FROM conversations c JOIN contact_emails ce ON ce.contact_id = c.contact_id AND ce.position = 0
WHERE c.workspace_id = $1 AND c.channel_id = $2
ORDER BY 1`, d.tm.ws, channel)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestDovecotTakeoutMatchesIMAPAndIMAPAddsOnlyNewerMail(t *testing.T) {
	d := newDovecotRun(t)
	tm := d.tm
	ctx := context.Background()
	want := expectedFixture(t, d.fixtures, "gmail")
	box := copyMailbox(t, d.fixtures, "gmail")

	imapID := d.connectGmail(t, box, want.TeamAddresses)
	imapChannel := d.migration(imapID).str("channel_id")
	tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+imapID+"/scan", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	imapUses := map[string]string{}
	for _, f := range tm.owner.expect(http.StatusOK, "GET", "/v1/migrations/"+imapID+"/folders", nil).body["items"].([]any) {
		imapUses[f.(map[string]any)["name"].(string)] = f.(map[string]any)["use"].(string)
	}
	tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+imapID+"/start", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	checkImportedThreads(t, d, imapID, want)
	viaIMAP := conversationSignature(t, d, imapChannel)
	tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+imapID+"/cancel", nil)

	d2 := &dovecotRun{h: d.h, tm: newTeam(t, d.h), host: d.host, port: d.port, pass: d.pass, fixtures: d.fixtures}
	tm2 := d2.tm
	takeout, err := os.ReadFile(filepath.Join(d.fixtures, "files", "gmail-takeout.mbox"))
	if err != nil {
		t.Fatal(err)
	}
	address, channel := d2.emailChannel(t)
	id := d2.fileMigration(t, "takeout", address, channel, want.TeamAddresses)
	path := "/v1/migrations/" + id
	m := tm2.owner.uploadFile(t, id, takeout, "All mail Including Spam and Trash.mbox")
	if m.status != http.StatusOK || m.body["upload"].(map[string]any)["format"] != "takeout" {
		t.Fatalf("takeout upload: %d %s", m.status, m.raw)
	}
	tm2.owner.expect(http.StatusOK, "POST", path+"/scan", nil)
	if err := d2.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	uses := map[string]string{}
	for _, f := range tm2.owner.expect(http.StatusOK, "GET", path+"/folders", nil).body["items"].([]any) {
		uses[f.(map[string]any)["name"].(string)] = f.(map[string]any)["use"].(string)
	}
	if !reflect.DeepEqual(uses, imapUses) {
		t.Fatalf("Takeout folders %v, IMAP folders %v", uses, imapUses)
	}
	m = d2.migration(id)
	if m.str("state") != "ready" {
		t.Fatalf("after the scan: %s", m.raw)
	}
	checkDryRun(t, m.body["estimate"].(map[string]any), want)
	tm2.owner.expect(http.StatusOK, "POST", path+"/start", nil)
	if err := d2.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	m = d2.migration(id)
	if m.str("state") != "completed" || num(m.body["counts"].(map[string]any)["imported"]) != want.MessagesIn+want.MessagesOut {
		t.Fatalf("after the copy: %s", m.raw)
	}
	checkImportedThreads(t, d2, id, want)
	if viaTakeout := conversationSignature(t, d2, channel); !reflect.DeepEqual(viaTakeout, viaIMAP) {
		t.Fatalf("the Takeout import differs from the IMAP import:\n%s\n%s", strings.Join(viaTakeout, "\n"), strings.Join(viaIMAP, "\n"))
	}
	if n := d2.h.count("SELECT count(*) FROM migration_upload_parts WHERE workspace_id = $1 AND migration_id = $2", tm2.ws, id); n != 0 || len(d2.uploadedFiles(id)) != 0 {
		t.Fatal("the Takeout file's parts are still there")
	}

	base := time.Date(2025, 3, 3, 9, 0, 0, 0, time.UTC)
	reply := mailText("setup-4@example.org", "Carol White <carol@example.org>", "Ece Tan <ece@example.com>", "Re: Setup call", base.Add(430*time.Hour),
		"In-Reply-To: <setup-3@example.org>\r\nReferences: <setup-1@example.com> <setup-2@example.com> <setup-3@example.org>\r\n", "See you Wednesday.")
	fresh := mailText("refund-1@example.org", "Dave Green <dave@example.org>", "Acme Help <help@example.com>", "Refund", base.Add(431*time.Hour), "", "Where is my refund?")
	d2.deliver(t, box, "[Gmail].All Mail", reply, base.Add(430*time.Hour))
	d2.deliver(t, box, "[Gmail].All Mail", fresh, base.Add(431*time.Hour))

	followID, _ := d2.connect(t, box, want.TeamAddresses, channel)
	if _, err := d2.h.st.Pool.Exec(ctx, "UPDATE migrations SET preset = 'google_workspace' WHERE workspace_id = $1 AND id = $2", tm2.ws, followID); err != nil {
		t.Fatal(err)
	}
	tm2.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+followID+"/test", nil)
	f := d2.migration(followID)
	if f.str("follows_migration_id") != id || f.body["options"].(map[string]any)["since"] != "2025-03-20T00:00:00Z" {
		t.Fatalf("the IMAP migration after the Takeout import: %s", f.raw)
	}
	tm2.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+followID+"/scan", nil)
	if err := d2.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	est := d2.migration(followID).body["estimate"].(map[string]any)
	if num(est["messages"]) != 2 || num(est["conversations"]) != 1 || num(est["skipped"].(map[string]any)["already_in_yuva"]) != want.MessagesIn+want.MessagesOut {
		t.Fatalf("the follow-up dry run: %v", est)
	}
	tm2.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+followID+"/start", nil)
	if err := d2.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	f = d2.migration(followID)
	if f.str("state") != "syncing" || f.str("live_from") == "" || num(f.body["counts"].(map[string]any)["imported"]) != 2 {
		t.Fatalf("the follow-up after its copy: %s", f.raw)
	}
	after := conversationSignature(t, d2, channel)
	if len(after) != len(viaIMAP)+1 {
		t.Fatalf("conversations after the follow-up:\n%s", strings.Join(after, "\n"))
	}
	var setup int
	if err := d2.h.st.Pool.QueryRow(ctx, `SELECT count(*) FROM messages m JOIN conversations c ON c.id = m.conversation_id
WHERE c.workspace_id = $1 AND c.channel_id = $2 AND c.migration_id = $3 AND c.subject = 'Setup call'`, tm2.ws, channel, id).Scan(&setup); err != nil {
		t.Fatal(err)
	}
	if setup != 4 {
		t.Fatalf("the Takeout's Setup call conversation holds %d messages, want the 3 imported and the newer reply", setup)
	}
	if n := d2.h.count("SELECT count(*) FROM conversations WHERE workspace_id = $1 AND migration_id = $2", tm2.ws, followID); n != 1 {
		t.Fatalf("the follow-up created %d conversations, want the new thread only", n)
	}
	f = tm2.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+followID+"/finish", map[string]any{"force": true})
	if f.str("state") != "completed" || f.body["credentials"] != nil {
		t.Fatalf("finished: %s", f.raw)
	}
}

func TestFileMigrationZipsImportFoldersAsLabels(t *testing.T) {
	_, _, _, fixtures := dovecotTarget(t)
	want := expectedFixture(t, fixtures, "acme")
	for _, name := range []string{"acme-eml.zip", "acme-maildir.zip"} {
		t.Run(name, func(t *testing.T) {
			d := newFileRun(t, nil)
			d.fixtures = fixtures
			ctx := context.Background()
			data, err := os.ReadFile(filepath.Join(fixtures, "files", name))
			if err != nil {
				t.Fatal(err)
			}
			address, channel := d.emailChannel(t)
			id := d.fileMigration(t, "file", address, channel, want.TeamAddresses)
			m := d.tm.owner.uploadFile(t, id, data, name)
			format := map[string]string{"acme-eml.zip": "eml_zip", "acme-maildir.zip": "maildir_zip"}[name]
			if m.status != http.StatusOK || m.body["upload"].(map[string]any)["format"] != format {
				t.Fatalf("upload: %d %s", m.status, m.raw)
			}
			folders := map[string]string{}
			for _, f := range d.tm.owner.expect(http.StatusOK, "GET", "/v1/migrations/"+id+"/folders", nil).body["items"].([]any) {
				f := f.(map[string]any)
				folders[f["name"].(string)] = fmt.Sprintf("%s %s %d", f["kind"], f["use"], num(f["messages"]))
			}
			for _, w := range want.Folders {
				use := "skip"
				if w.Read {
					use = "import"
				}
				if got := folders[w.Name]; got != fmt.Sprintf("%s %s %d", w.Kind, use, w.Messages) {
					t.Errorf("folder %s: %q, want %s %s %d", w.Name, got, w.Kind, use, w.Messages)
				}
			}
			d.tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/scan", nil)
			if err := d.work(t, ctx); err != nil {
				t.Fatal(err)
			}
			m = d.migration(id)
			checkDryRun(t, m.body["estimate"].(map[string]any), want)
			d.tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/start", nil)
			if err := d.work(t, ctx); err != nil {
				t.Fatal(err)
			}
			if m = d.migration(id); m.str("state") != "completed" {
				t.Fatalf("after the copy: %s", m.raw)
			}
			checkImportedThreads(t, d, id, want)
			if len(d.uploadedFiles(id)) != 0 {
				t.Fatal("parts left after completion")
			}
		})
	}
}

func TestTakeoutThreadsByGmailThreadIDs(t *testing.T) {
	t.Parallel()
	d := newFileRun(t, nil)
	ctx := context.Background()
	address, channel := d.emailChannel(t)
	mail := func(thread, id, subject, refs string, at time.Time) string {
		return "From " + thread + "@xxx " + at.Format("Mon Jan 02 15:04:05 -0700 2006") + "\n" +
			"X-GM-THRID: " + thread + "\nX-Gmail-Labels: Inbox,Opened\nFrom: Bob <bob@example.net>\nTo: " + address +
			"\nSubject: " + subject + "\nDate: " + at.Format(time.RFC1123Z) + "\nMessage-ID: <" + id + ">\n" + refs + "\nBody of " + id + "\n\n"
	}
	at := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC)
	file := mail("100", "a@example.net", "Plans", "", at) +
		mail("100", "b@example.net", "Plans for Monday", "", at.Add(time.Hour)) +
		mail("200", "c@example.net", "Re: Plans", "In-Reply-To: <a@example.net>\nReferences: <a@example.net>\n", at.Add(2*time.Hour))
	id := d.fileMigration(t, "takeout", address, channel, []string{address})
	if r := d.tm.owner.uploadFile(t, id, []byte(file), "takeout.mbox"); r.status != http.StatusOK {
		t.Fatalf("upload: %d %s", r.status, r.raw)
	}
	d.tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/scan", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	d.tm.owner.expect(http.StatusOK, "POST", "/v1/migrations/"+id+"/start", nil)
	if err := d.work(t, ctx); err != nil {
		t.Fatal(err)
	}
	got := conversationSignature(t, d, channel)
	want := []string{
		"Plans | bob@example.net | closed |  | a@example.net:in,b@example.net:in",
		"Plans | bob@example.net | closed |  | c@example.net:in",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("conversations:\n%s\nwant Gmail's threads:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
