package api_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"image/color"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/productdevbook/yuva/api/internal/api"
)

var (
	ownerListTools = []string{
		"update_inbox_sender", "create_email_channel", "create_dkim_key", "create_mailing_list", "update_mailing_list", "add_subscription",
		"unsubscribe", "create_list_import", "update_list_import_mapping", "cancel_list_import", "start_list_import", "list_email_images",
		"upload_email_image", "list_designs", "get_design", "duplicate_design",
	}
	readListTools     = []string{"get_deliverability", "list_subscriptions", "get_list_import", "list_list_imports", "get_list_import_errors"}
	broadcastControls = []string{"pause_broadcast", "resume_broadcast", "unschedule_broadcast", "cancel_broadcast"}
)

type importMX struct{}

func (importMX) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	return []*net.MX{{Host: "mx." + name + ".", Pref: 10}}, nil
}

func (importMX) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("203.0.113.10")}, nil
}

func TestMCPListAndDesignToolsByRoleAndScope(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	tm.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+tm.inbox+"/members/"+tm.agentID, nil)
	all := []string{"inboxes:read", "inboxes:manage", "lists:read", "lists:write", "broadcasts:read", "broadcasts:write", "broadcasts:send"}

	owner := h.mcpSession(h.oauthToken(tm.owner, tm.ws, "Claude Code", testOrigin+"/mcp", all))
	tools := toolNames(t, owner)
	for _, name := range slices.Concat(ownerListTools, readListTools) {
		if !slices.Contains(tools, name) {
			t.Fatalf("owner with every scope lacks %s: %v", name, tools)
		}
	}
	for _, name := range broadcastControls {
		if slices.Contains(tools, name) {
			t.Fatalf("%s listed while broadcasts_by_keys is off", name)
		}
	}
	tm.owner.expect(http.StatusOK, "PATCH", "/v1/workspace", map[string]any{"broadcasts_by_keys": true})
	tools = toolNames(t, h.mcpSession(h.oauthToken(tm.owner, tm.ws, "Claude Code", testOrigin+"/mcp", all)))
	for _, name := range broadcastControls {
		if !slices.Contains(tools, name) {
			t.Fatalf("%s not listed with broadcasts:send and broadcasts_by_keys: %v", name, tools)
		}
	}

	agent := h.mcpSession(h.oauthToken(tm.agent, tm.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read", "lists:read", "broadcasts:read"}))
	tools = toolNames(t, agent)
	for _, name := range readListTools {
		if !slices.Contains(tools, name) {
			t.Fatalf("agent lacks %s: %v", name, tools)
		}
	}
	for _, name := range slices.Concat(ownerListTools, broadcastControls) {
		if slices.Contains(tools, name) {
			t.Fatalf("agent lists %s", name)
		}
	}

	noWrite := h.mcpSession(h.oauthToken(tm.owner, tm.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read", "lists:read"}))
	if slices.Contains(toolNames(t, noWrite), "create_mailing_list") {
		t.Fatal("create_mailing_list listed without lists:write")
	}
	if _, errText := callTool(t, noWrite, "create_mailing_list", map[string]any{"inbox_id": tm.inbox, "name": "News"}); !strings.Contains(errText, "insufficient_scope") || !strings.Contains(errText, "lists:write") {
		t.Fatalf("create_mailing_list without lists:write: %q", errText)
	}
}

func TestMCPInboxSender(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	cs := h.mcpSession(h.oauthToken(tm.owner, tm.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read", "inboxes:manage", "lists:read", "lists:write"}))

	if _, errText := callTool(t, cs, "create_mailing_list", map[string]any{"inbox_id": tm.inbox, "name": "Product news"}); errText != "" {
		t.Fatal(errText)
	}
	state, errText := callTool(t, cs, "get_setup_state", map[string]any{})
	if missing, _ := state["inboxes_without_sender"].([]any); errText != "" || len(missing) != 1 || missing[0].(map[string]any)["id"] != tm.inbox {
		t.Fatalf("get_setup_state before the sender: %v %s", state["inboxes_without_sender"], errText)
	}
	inboxes, _ := callTool(t, cs, "list_inboxes", map[string]any{})
	if in := inboxes["items"].([]any)[0].(map[string]any); in["sender_ready"] != false {
		t.Fatalf("list_inboxes before the sender: %v", in)
	}

	if _, errText := callTool(t, cs, "update_inbox_sender", map[string]any{"inbox_id": tm.inbox}); !strings.Contains(errText, "name at least one field") {
		t.Fatalf("update_inbox_sender without fields: %q", errText)
	}
	in, errText := callTool(t, cs, "update_inbox_sender", map[string]any{
		"inbox_id": tm.inbox, "legal_name": "Acme Yazılım A.Ş.", "postal_address": "Örnek Cad. 1, 34000 İstanbul",
	})
	if errText != "" {
		t.Fatal(errText)
	}
	sender := in["sender"].(map[string]any)
	if in["sender_ready"] != true || sender["legal_name"] != "Acme Yazılım A.Ş." || sender["registration"] != nil {
		t.Fatalf("update_inbox_sender: %v", in)
	}
	api := tm.owner.expect(http.StatusOK, "GET", "/v1/inboxes/"+tm.inbox, nil).body["sender"].(map[string]any)
	if api["postal_address"] != "Örnek Cad. 1, 34000 İstanbul" {
		t.Fatalf("sender in /v1: %v", api)
	}
	if in, _ = callTool(t, cs, "update_inbox_sender", map[string]any{"inbox_id": tm.inbox, "postal_address": ""}); in["sender_ready"] != false || in["sender"].(map[string]any)["legal_name"] != "Acme Yazılım A.Ş." {
		t.Fatalf("clearing the postal address: %v", in)
	}
	callTool(t, cs, "update_inbox_sender", map[string]any{"inbox_id": tm.inbox, "postal_address": "Örnek Cad. 1"})
	if state, _ = callTool(t, cs, "get_setup_state", map[string]any{}); state["inboxes_without_sender"] != nil {
		t.Fatalf("get_setup_state after the sender: %v", state["inboxes_without_sender"])
	}

	other := newTeam(t, h)
	ocs := h.mcpSession(h.oauthToken(other.owner, other.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read", "inboxes:manage"}))
	if _, errText := callTool(t, ocs, "update_inbox_sender", map[string]any{"inbox_id": tm.inbox, "legal_name": "Evil"}); !strings.Contains(errText, "not_found") {
		t.Fatalf("update_inbox_sender from another workspace: %q", errText)
	}
}

func runListImports(t *testing.T, h *harness, ws string) {
	t.Helper()
	for range 10 {
		jobs := h.jobs("list_import", ws)
		if len(jobs) == 0 {
			return
		}
		for _, j := range jobs {
			var a api.ListImportArgs
			if err := json.Unmarshal(j.args, &a); err != nil {
				t.Fatal(err)
			}
			if _, err := h.srv.RunListImport(context.Background(), a, time.Minute); err != nil {
				t.Fatal(err)
			}
			h.dropJob(j.id)
		}
	}
	t.Fatal("list imports did not finish")
}

func TestMCPListImportNeedsTheConfirmedCount(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) { d.Imports.Resolver = importMX{} })
	tm := newTeam(t, h)
	cs := h.mcpSession(h.oauthToken(tm.owner, tm.ws, "Claude Code", testOrigin+"/mcp", []string{"lists:read", "lists:write"}))
	list, errText := callTool(t, cs, "create_mailing_list", map[string]any{"inbox_id": tm.inbox, "name": "Product news", "double_opt_in": false})
	if errText != "" {
		t.Fatal(errText)
	}
	listID := list["id"].(string)
	csv := "email,name\nada@example.com,Ada\nbob@example.com,Bob\ncem@example.com,Cem\n"

	if _, errText := callTool(t, cs, "create_list_import", map[string]any{"list_id": listID, "csv": csv, "attestation": "", "consent_source": "x"}); !strings.Contains(errText, "attestation_required") {
		t.Fatalf("import without attestation: %q", errText)
	}
	imp, errText := callTool(t, cs, "create_list_import", map[string]any{
		"list_id": listID, "csv": csv, "filename": "customers.csv",
		"attestation": "Everyone in this file opted in to product news.", "consent_source": "Sign-up form at example.com, 2026",
	})
	if errText != "" {
		t.Fatal(errText)
	}
	if imp["state"] != "validating" || imp["filename"] != "customers.csv" || imp["attestation"] != "Everyone in this file opted in to product news." {
		t.Fatalf("create_list_import: %v", imp)
	}
	importID := imp["id"].(string)
	runListImports(t, h, tm.ws)

	got, errText := callTool(t, cs, "get_list_import", map[string]any{"list_id": listID, "import_id": importID})
	if errText != "" {
		t.Fatal(errText)
	}
	if got["state"] != "ready" || got["counts"].(map[string]any)["valid"] != 3.0 {
		t.Fatalf("get_list_import after validation: %v", got)
	}
	if fields := got["columns"].([]any); fields[0].(map[string]any)["field"] != "email" || fields[1].(map[string]any)["field"] != "name" {
		t.Fatalf("columns: %v", fields)
	}
	if preview := got["customer_content"].(map[string]any)["preview"].([]any); len(preview) != 3 {
		t.Fatalf("preview: %v", got["customer_content"])
	}
	if p := outsideCustomerContent(got, "ada@example.com", ""); len(p) > 0 {
		t.Fatalf("an address outside customer_content at %v", p)
	}

	if _, errText := callTool(t, cs, "start_list_import", map[string]any{"list_id": listID, "import_id": importID, "confirm_count": 2}); !strings.Contains(errText, "count_mismatch") {
		t.Fatalf("start with the wrong count: %q", errText)
	}
	if subs := tm.owner.expect(http.StatusOK, "GET", "/v1/lists/"+listID+"/subscriptions", nil).body["items"].([]any); len(subs) != 0 {
		t.Fatalf("subscribers after a refused start: %v", subs)
	}

	other := newTeam(t, h)
	ocs := h.mcpSession(h.oauthToken(other.owner, other.ws, "Claude Code", testOrigin+"/mcp", []string{"lists:read", "lists:write"}))
	for name, args := range map[string]map[string]any{
		"get_list_import":    {"list_id": listID, "import_id": importID},
		"start_list_import":  {"list_id": listID, "import_id": importID, "confirm_count": 3},
		"create_list_import": {"list_id": listID, "csv": csv, "attestation": "x", "consent_source": "y"},
	} {
		if _, errText := callTool(t, ocs, name, args); !strings.Contains(errText, "not_found") {
			t.Fatalf("%s from another workspace: %q", name, errText)
		}
	}

	started, errText := callTool(t, cs, "start_list_import", map[string]any{"list_id": listID, "import_id": importID, "confirm_count": 3})
	if errText != "" || started["state"] != "importing" {
		t.Fatalf("start with the confirmed count: %v %s", started, errText)
	}
	runListImports(t, h, tm.ws)
	if done, _ := callTool(t, cs, "get_list_import", map[string]any{"list_id": listID, "import_id": importID}); done["state"] != "done" || done["counts"].(map[string]any)["added"] != 3.0 {
		t.Fatalf("after the import: %v", done)
	}
	subs, errText := callTool(t, cs, "list_subscriptions", map[string]any{"list_id": listID, "state": "subscribed"})
	if errText != "" {
		t.Fatal(errText)
	}
	var emails []string
	for _, s := range subs["items"].([]any) {
		emails = append(emails, s.(map[string]any)["customer_content"].(map[string]any)["email"].(string))
	}
	slices.Sort(emails)
	if !slices.Equal(emails, []string{"ada@example.com", "bob@example.com", "cem@example.com"}) {
		t.Fatalf("subscribers: %v", subs)
	}
}

func TestMCPUploadEmailImage(t *testing.T) {
	h := newHarnessWith(t, func(d *api.Deps) {
		d.Webhooks.AllowPrivate = true
		d.Webhooks.Resolver = staticResolver{"logo.example.com": "10.1.2.3"}
	})
	tm := newTeam(t, h)
	cs := h.mcpSession(h.oauthToken(tm.owner, tm.ws, "Claude Code", testOrigin+"/mcp", []string{"broadcasts:read", "broadcasts:write"}))

	for _, u := range []string{"https://logo.example.com/logo.png", "https://127.0.0.1/logo.png", "https://localhost/logo.png", "https://[::1]/logo.png"} {
		if _, errText := callTool(t, cs, "upload_email_image", map[string]any{"url": u}); !strings.Contains(errText, "url_refused") {
			t.Fatalf("%s: %q", u, errText)
		}
	}
	if _, errText := callTool(t, cs, "upload_email_image", map[string]any{"url": "http://example.com/logo.png"}); !strings.Contains(errText, "https") {
		t.Fatalf("http URL: %q", errText)
	}
	if _, errText := callTool(t, cs, "upload_email_image", map[string]any{}); !strings.Contains(errText, "one of them") {
		t.Fatalf("no source: %q", errText)
	}

	pic := base64.StdEncoding.EncodeToString(testPNG(t, 300, 100, color.RGBA{30, 64, 175, 255}))
	up, errText := callTool(t, cs, "upload_email_image", map[string]any{"data_base64": pic, "filename": "logo.png"})
	if errText != "" {
		t.Fatal(errText)
	}
	img := up["image"].(map[string]any)
	if up["created"] != true || img["width"] != 300.0 || img["content_type"] != "image/png" || img["filename"] != "logo.png" {
		t.Fatalf("upload_email_image: %v", up)
	}
	if again, _ := callTool(t, cs, "upload_email_image", map[string]any{"data_base64": pic}); again["created"] != false || again["image"].(map[string]any)["id"] != img["id"] {
		t.Fatalf("same image again: %v", again)
	}
	if _, errText := callTool(t, cs, "upload_email_image", map[string]any{"data_base64": base64.StdEncoding.EncodeToString([]byte("<svg/>"))}); !strings.Contains(errText, "image_type") {
		t.Fatalf("svg bytes: %q", errText)
	}
	listed, _ := callTool(t, cs, "list_email_images", map[string]any{})
	if items := listed["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != img["id"] {
		t.Fatalf("list_email_images: %v", listed)
	}

	other := newTeam(t, h)
	if listed, _ := callTool(t, h.mcpSession(h.oauthToken(other.owner, other.ws, "Claude Code", testOrigin+"/mcp", []string{"broadcasts:read"})), "list_email_images", map[string]any{}); len(listed["items"].([]any)) != 0 {
		t.Fatalf("another workspace sees the image: %v", listed)
	}
}

func TestMCPDeliverabilityAdviceWaitsForDNS(t *testing.T) {
	h, bs := newBroadcastHarness(t, nil)
	bt := newBroadcastTeam(t, h, bs)
	bt.owner.expect(http.StatusCreated, "POST", "/v1/channels/"+bt.emailChannel+"/dkim-key", map[string]any{"selector": "yuva"})
	cs := h.mcpSession(h.oauthToken(bt.owner, bt.ws, "Claude Code", testOrigin+"/mcp", []string{"inboxes:read", "inboxes:manage"}))

	next := func(args map[string]any) string {
		t.Helper()
		out, errText := callTool(t, cs, "get_deliverability", args)
		if errText != "" {
			t.Fatal(errText)
		}
		b, _ := json.Marshal(out["next"])
		return string(b)
	}
	fresh := next(map[string]any{"channel_id": bt.emailChannel, "check": true})
	if strings.Contains(fresh, "no SPF") || strings.Contains(fresh, "no DMARC") || strings.Contains(fresh, "add the TXT") {
		t.Fatalf("a fresh check gives missing-record advice: %s", fresh)
	}
	if !strings.Contains(fresh, "running") || !strings.Contains(fresh, "yuva._domainkey") {
		t.Fatalf("a fresh check lacks the running notice or the DKIM record: %s", fresh)
	}

	if _, err := h.st.Pool.Exec(t.Context(), `UPDATE deliverability_checks SET state = 'sent', result = '{"domain":"example.com","dns":{"mx":{"domain":"example.com","hosts":[]}}}' WHERE channel_id = $1`, bt.emailChannel); err != nil {
		t.Fatal(err)
	}
	read := next(map[string]any{"channel_id": bt.emailChannel})
	if !strings.Contains(read, "no SPF") || !strings.Contains(read, "no DMARC") {
		t.Fatalf("a check that read DNS lacks the missing-record advice: %s", read)
	}
}
