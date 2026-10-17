package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/api"
	"github.com/productdevbook/yuva/api/internal/awsses"
	"github.com/productdevbook/yuva/api/internal/email"
)

const (
	sesCaller    = "111122223333"
	sesServerKey = "AKIASERVERKEY0000001"
	sesOtherKey  = "AKIAOTHERKEY00000001"
	sesHost      = "email-smtp.eu-west-1.amazonaws.com"
	sesConfigSet = "yuva-events"
)

type fakeSES struct {
	mu         sync.Mutex
	keys       map[string]string
	keyCalls   map[string]int
	idents     []awsses.Identity
	tenants    map[string]awsses.Tenant
	assoc      map[string][]string
	entities   []awsses.Entity
	creates    []string
	reinstated []string
	deleted    []string
	deleteErr  error
	calls      int
}

func newFakeSES() *fakeSES {
	return &fakeSES{
		keys:     map[string]string{sesServerKey: sesCaller, sesOtherKey: "999988887777"},
		keyCalls: map[string]int{},
		idents: []awsses.Identity{
			{Name: "example.com", Type: "DOMAIN", Verified: true, SendingEnabled: true},
			{Name: "ops@verified.example", Type: "EMAIL_ADDRESS", Verified: true, SendingEnabled: true},
			{Name: "pending.example", Type: "DOMAIN", Verified: false, SendingEnabled: true},
			{Name: "stopped.example", Type: "DOMAIN", Verified: true, SendingEnabled: false},
		},
		tenants: map[string]awsses.Tenant{},
		assoc:   map[string][]string{},
	}
}

func (f *fakeSES) CallerAccount(context.Context) (string, error) { return sesCaller, nil }

func (f *fakeSES) AccessKeyAccount(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.keyCalls[key]++
	a, ok := f.keys[key]
	if !ok {
		return "", &awsses.Error{Operation: "GetAccessKeyInfo", Status: 400, Code: "InvalidParameterValue"}
	}
	return a, nil
}

func (f *fakeSES) CreateTenant(_ context.Context, name string, tags map[string]string) (awsses.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if _, ok := f.tenants[name]; ok {
		return awsses.Tenant{}, &awsses.Error{Operation: "CreateTenant", Status: 400, Code: "AlreadyExistsException"}
	}
	f.creates = append(f.creates, name)
	t := awsses.Tenant{Name: name, ID: "tn-" + name, ARN: fmt.Sprintf("arn:aws:ses:eu-west-1:%s:tenant/%s/tn-%s", sesCaller, name, name), SendingStatus: "ENABLED", Tags: tags}
	f.tenants[name] = t
	return t, nil
}

func (f *fakeSES) GetTenant(_ context.Context, name string) (awsses.Tenant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tenants[name]
	if !ok {
		return t, &awsses.Error{Operation: "GetTenant", Status: 404, Code: "NotFoundException"}
	}
	return t, nil
}

func (f *fakeSES) DeleteTenant(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if len(f.assoc[name]) > 0 {
		return &awsses.Error{Operation: "DeleteTenant", Status: 409, Code: "ConflictException", Message: "resources are still associated"}
	}
	f.deleted = append(f.deleted, name)
	delete(f.tenants, name)
	return nil
}

func (f *fakeSES) Associate(_ context.Context, tenant, arn string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.assoc[tenant] = append(f.assoc[tenant], arn)
	return nil
}

func (f *fakeSES) Dissociate(_ context.Context, tenant, arn string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.assoc[tenant] = slices.DeleteFunc(f.assoc[tenant], func(a string) bool { return a == arn })
	return nil
}

func (f *fakeSES) TenantResources(_ context.Context, name string) ([]awsses.Resource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tenants[name]; !ok {
		return nil, &awsses.Error{Operation: "ListTenantResources", Status: 404, Code: "NotFoundException"}
	}
	var out []awsses.Resource
	for _, a := range f.assoc[name] {
		out = append(out, awsses.Resource{Type: "EMAIL_IDENTITY", ARN: a})
	}
	return out, nil
}

func (f *fakeSES) Identities(context.Context) ([]awsses.Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.idents), nil
}

func (f *fakeSES) ReputationEntities(_ context.Context, prefix string) ([]awsses.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []awsses.Entity
	for _, e := range f.entities {
		if strings.HasPrefix(e.Reference, prefix) {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeSES) Reinstate(_ context.Context, arn string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reinstated = append(f.reinstated, arn)
	return nil
}

func (f *fakeSES) associated(tenant string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Clone(f.assoc[tenant])
	slices.Sort(out)
	return out
}

func (f *fakeSES) setEntity(e awsses.Entity) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entities = slices.DeleteFunc(f.entities, func(x awsses.Entity) bool { return x.Reference == e.Reference })
	f.entities = append(f.entities, e)
}

func withSES(f *fakeSES) func(*api.Deps) {
	return func(d *api.Deps) {
		d.SES = api.SESSettings{Client: f, Region: "eu-west-1", ConfigurationSet: sesConfigSet, TenantPrefix: "yuva-"}
	}
}

func sesSMTP(host, user string) map[string]any {
	return map[string]any{"host": host, "port": 587, "username": user, "password": "ses-secret", "tls": "starttls"}
}

func createEmailChannel(t *testing.T, c *client, inbox, address string, smtp map[string]any, extra map[string]any) string {
	t.Helper()
	settings := map[string]any{"address": address, "smtp": smtp}
	for k, v := range extra {
		settings[k] = v
	}
	return c.expect(http.StatusCreated, "POST", "/v1/inboxes/"+inbox+"/channels", map[string]any{"kind": "email", "name": "Mail " + address, "email": settings}).str("id")
}

func channelSES(t *testing.T, c *client, id string) map[string]any {
	t.Helper()
	ses, _ := c.expect(http.StatusOK, "GET", "/v1/channels/"+id, nil).body["email"].(map[string]any)["ses"].(map[string]any)
	return ses
}

func syncInbox(t *testing.T, h *harness, ws, inbox string) api.SESSyncResult {
	t.Helper()
	res, err := h.srv.SyncSESTenant(context.Background(), uuid.MustParse(ws), uuid.MustParse(inbox), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Retry {
		t.Fatalf("sync asks for a retry: %+v", res)
	}
	return res
}

func identityARN(name string) string {
	return "arn:aws:ses:eu-west-1:" + sesCaller + ":identity/" + name
}

const configSetARN = "arn:aws:ses:eu-west-1:" + sesCaller + ":configuration-set/" + sesConfigSet

func TestSESTenantSync(t *testing.T) {
	f := newFakeSES()
	h := newHarnessWith(t, withSES(f))
	tm := newTeam(t, h)
	o := tm.owner
	inbox1 := tm.inbox
	inbox2 := o.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Shop", "slug": "shop"}).body["inbox"].(map[string]any)["id"].(string)

	ch1 := createEmailChannel(t, o, inbox1, unique("support")+"@example.com", sesSMTP(sesHost, sesServerKey), nil)
	ch5 := createEmailChannel(t, o, inbox1, unique("help")+"@pending.example", sesSMTP(sesHost, sesServerKey), nil)
	plain := createEmailChannel(t, o, inbox1, unique("plain")+"@example.com", map[string]any{"host": "smtp.example.com", "port": 2525, "username": "mailer", "tls": "starttls"}, nil)
	if ses := channelSES(t, o, ch1); ses["source"] != "server" || ses["state"] != "pending" {
		t.Fatalf("new SES channel: %v", ses)
	}
	if ses := channelSES(t, o, plain); ses != nil {
		t.Fatalf("a channel without SES shows %v", ses)
	}
	if n := len(h.jobs("ses_tenant_sync", tm.ws)); n != 1 {
		t.Fatalf("%d sync jobs for one inbox", n)
	}

	res := syncInbox(t, h, tm.ws, inbox1)
	if len(f.creates) != 1 || f.creates[0] != res.Tenant || !strings.HasPrefix(res.Tenant, "yuva-") {
		t.Fatalf("created %v, sync %+v", f.creates, res)
	}
	tags := f.tenants[res.Tenant].Tags
	if tags["yuva-workspace"] != tm.ws || tags["yuva-inbox"] != inbox1 || tags["yuva-server"] != "localhost" {
		t.Fatalf("tags %v", tags)
	}
	if got := f.associated(res.Tenant); !slices.Equal(got, []string{configSetARN, identityARN("example.com")}) {
		t.Fatalf("associated %v", got)
	}
	if f.keyCalls[sesServerKey] != 1 {
		t.Fatalf("the server key's account was asked %d times in one run", f.keyCalls[sesServerKey])
	}
	ses := channelSES(t, o, ch1)
	if ses["source"] != "server" || ses["state"] != "isolated" || ses["tenant"] != res.Tenant || ses["configuration_set"] != sesConfigSet {
		t.Fatalf("isolated channel: %v", ses)
	}
	if ses := channelSES(t, o, ch5); ses["state"] != "identity_missing" || !strings.Contains(ses["detail"].(string), "pending.example") {
		t.Fatalf("unverified domain: %v", ses)
	}
	o.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+inbox1+"/members/"+tm.agentID, nil)
	for _, id := range []string{ch1, ch5} {
		ses := channelSES(t, tm.agent, id)
		if ses["source"] != "server" || ses["state"] == nil || ses["tenant"] != nil || ses["configuration_set"] != nil || ses["detail"] != nil {
			t.Fatalf("an agent reads %v", ses)
		}
	}
	for _, it := range tm.agent.expect(http.StatusOK, "GET", "/v1/inboxes/"+inbox1+"/channels", nil).body["items"].([]any) {
		if ses, _ := it.(map[string]any)["email"].(map[string]any)["ses"].(map[string]any); ses != nil && (ses["tenant"] != nil || ses["detail"] != nil) {
			t.Fatalf("an agent lists %v", ses)
		}
	}
	in := o.expect(http.StatusOK, "GET", "/v1/inboxes/"+inbox1, nil).body
	if st := in["ses_tenant"].(map[string]any); st["state"] != "ready" || st["sending_status"] != "enabled" || st["paused_by"] != nil {
		t.Fatalf("inbox tenant: %v", st)
	}
	if in2 := o.expect(http.StatusOK, "GET", "/v1/inboxes/"+inbox2, nil).body; in2["ses_tenant"] != nil {
		t.Fatalf("an inbox without a tenant shows %v", in2["ses_tenant"])
	}
	if n := len(h.jobs("ses_tenant_status", tm.ws)); n != 1 {
		t.Fatalf("%d status jobs after a sync", n)
	}

	before := f.calls
	syncInbox(t, h, tm.ws, inbox1)
	if f.calls != before || len(f.creates) != 1 {
		t.Fatalf("a second sync called AWS %d times", f.calls-before)
	}

	ch2 := createEmailChannel(t, o, inbox2, unique("news")+"@mail.example.com", sesSMTP(sesHost, sesServerKey), nil)
	ch3 := createEmailChannel(t, o, inbox2, unique("theirs")+"@example.com", sesSMTP(sesHost, sesOtherKey), nil)
	ch4 := createEmailChannel(t, o, inbox2, unique("us")+"@example.com", sesSMTP("email-smtp.us-east-1.amazonaws.com", sesServerKey), nil)
	res2 := syncInbox(t, h, tm.ws, inbox2)
	if res2.Tenant == res.Tenant || len(f.creates) != 2 {
		t.Fatalf("two inboxes share a tenant: %s %s", res.Tenant, res2.Tenant)
	}
	if got := f.associated(res2.Tenant); !slices.Equal(got, []string{configSetARN, identityARN("example.com")}) {
		t.Fatalf("second tenant associated %v", got)
	}
	if ses := channelSES(t, o, ch2); ses["state"] != "isolated" || ses["tenant"] != res2.Tenant {
		t.Fatalf("subdomain channel: %v", ses)
	}
	if ses := channelSES(t, o, ch3); ses["source"] != "none" || ses["state"] != "not_isolated" || !strings.Contains(ses["detail"].(string), "another AWS account") {
		t.Fatalf("other account: %v", ses)
	}
	if ses := channelSES(t, o, ch4); ses["state"] != "not_isolated" || !strings.Contains(ses["detail"].(string), "us-east-1") {
		t.Fatalf("other region: %v", ses)
	}

	o.expectProblem(http.StatusBadRequest, "validation_failed", "PATCH", "/v1/channels/"+ch3, map[string]any{"email": map[string]any{
		"address": channelAddress(t, o, ch3), "ses": map[string]any{"tenant": "has space"},
	}})
	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+ch3, map[string]any{"email": map[string]any{
		"address": channelAddress(t, o, ch3), "smtp": sesSMTP(sesHost, sesOtherKey), "ses": map[string]any{"tenant": "their-tenant", "configuration_set": "their-set"},
	}})
	syncInbox(t, h, tm.ws, inbox2)
	if ses := channelSES(t, o, ch3); ses["source"] != "channel" || ses["state"] != "isolated" || ses["tenant"] != "their-tenant" || ses["configuration_set"] != "their-set" {
		t.Fatalf("typed tenant: %v", ses)
	}

	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+ch1, map[string]any{"email": map[string]any{
		"address": channelAddress(t, o, ch1), "from_address": "ops@verified.example", "smtp": sesSMTP(sesHost, sesServerKey),
	}})
	if ses := channelSES(t, o, ch1); ses["state"] != "pending" {
		t.Fatalf("a changed sending address keeps its verdict: %v", ses)
	}
	syncInbox(t, h, tm.ws, inbox1)
	if got := f.associated(res.Tenant); !slices.Equal(got, []string{configSetARN, identityARN("ops@verified.example")}) {
		t.Fatalf("after the sending address changed: %v", got)
	}
	if ses := channelSES(t, o, ch1); ses["state"] != "isolated" {
		t.Fatalf("address identity: %v", ses)
	}

	var acct *string
	if err := h.st.Pool.QueryRow(context.Background(), "SELECT smtp_aws_account FROM email_channels WHERE channel_id = $1", ch1).Scan(&acct); err != nil || acct == nil || *acct != sesCaller {
		t.Fatalf("stored account %v %v", acct, err)
	}
	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+ch1, map[string]any{"email": map[string]any{
		"address": channelAddress(t, o, ch1), "smtp": sesSMTP(sesHost, sesOtherKey),
	}})
	var state string
	if err := h.st.Pool.QueryRow(context.Background(), "SELECT smtp_aws_account, ses_state FROM email_channels WHERE channel_id = $1", ch1).Scan(&acct, &state); err != nil || acct != nil || state != "pending" {
		t.Fatalf("after the user name changed: account %v state %s %v", acct, state, err)
	}
	syncInbox(t, h, tm.ws, inbox1)
	if got := f.associated(res.Tenant); !slices.Equal(got, []string{configSetARN}) {
		t.Fatalf("no managed channel left, still associated: %v", got)
	}

	o.expect(http.StatusNoContent, "DELETE", "/v1/inboxes/"+inbox2, nil)
	if n := h.count("SELECT count(*) FROM river_job WHERE kind = 'ses_tenant_delete' AND args->>'name' = $1", res2.Tenant); n != 1 {
		t.Fatalf("%d delete jobs for the deleted inbox's tenant", n)
	}
	if err := h.srv.OperatorDeleteWorkspace(context.Background(), uuid.MustParse(tm.ws)); err != nil {
		t.Fatal(err)
	}
	if n := h.count("SELECT count(*) FROM river_job WHERE kind = 'ses_tenant_delete' AND args->>'name' = $1", res.Tenant); n != 1 {
		t.Fatalf("%d delete jobs for the closed workspace's tenant", n)
	}
}

func channelAddress(t *testing.T, c *client, id string) string {
	t.Helper()
	return c.expect(http.StatusOK, "GET", "/v1/channels/"+id, nil).body["email"].(map[string]any)["address"].(string)
}

func TestSESTenantTakesOverOnlyItsOwn(t *testing.T) {
	f := newFakeSES()
	h := newHarnessWith(t, withSES(f))
	tm := newTeam(t, h)
	o := tm.owner
	ctx := context.Background()
	other := o.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	mine, foreign := "yuva-"+unique("crash"), "yuva-"+unique("foreign")
	for inbox, name := range map[string]string{tm.inbox: mine, other: foreign} {
		if _, err := h.st.Pool.Exec(ctx, "INSERT INTO ses_tenants (workspace_id, inbox_id, name, region, created_at) VALUES ($1, $2, $3, 'eu-west-1', now())", tm.ws, inbox, name); err != nil {
			t.Fatal(err)
		}
	}
	f.tenants[mine] = awsses.Tenant{Name: mine, ARN: "arn:aws:ses:eu-west-1:" + sesCaller + ":tenant/" + mine + "/tn-1", Tags: map[string]string{"yuva-workspace": tm.ws, "yuva-inbox": tm.inbox}}
	f.tenants[foreign] = awsses.Tenant{Name: foreign, ARN: "arn:aws:ses:eu-west-1:" + sesCaller + ":tenant/" + foreign + "/tn-2", Tags: map[string]string{"owner": "someone else"}}
	ch := createEmailChannel(t, o, tm.inbox, unique("support")+"@example.com", sesSMTP(sesHost, sesServerKey), nil)
	chOther := createEmailChannel(t, o, other, unique("other")+"@example.com", sesSMTP(sesHost, sesServerKey), nil)

	if res := syncInbox(t, h, tm.ws, tm.inbox); res.Tenant != mine || len(f.creates) != 0 {
		t.Fatalf("recovery: %+v, created %v", res, f.creates)
	}
	if ses := channelSES(t, o, ch); ses["state"] != "isolated" || ses["tenant"] != mine {
		t.Fatalf("recovered tenant: %v", ses)
	}
	if _, err := h.srv.SyncSESTenant(ctx, uuid.MustParse(tm.ws), uuid.MustParse(other), false); err != nil {
		t.Fatal(err)
	}
	if got := f.associated(foreign); len(got) != 0 {
		t.Fatalf("a foreign tenant got associations: %v", got)
	}
	if ses := channelSES(t, o, chOther); ses["state"] != "error" || !strings.Contains(ses["detail"].(string), "did not create") {
		t.Fatalf("foreign tenant: %v", ses)
	}
	if st := o.expect(http.StatusOK, "GET", "/v1/inboxes/"+other, nil).body["ses_tenant"].(map[string]any); st["state"] != "error" {
		t.Fatalf("inbox with a foreign tenant name: %v", st)
	}
}

func sesHeader(t *testing.T, raw []byte) (string, string) {
	t.Helper()
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return m.Header.Get("X-Ses-Tenant"), m.Header.Get("X-Ses-Configuration-Set")
}

func TestSESTenantHeaders(t *testing.T) {
	f := newFakeSES()
	h := newHarnessWith(t, withSES(f))
	lt := newListTeam(t, h)
	o := lt.owner
	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+lt.emailChannel, map[string]any{"email": map[string]any{
		"address": channelAddress(t, o, lt.emailChannel), "smtp": sesSMTP(sesHost, sesServerKey),
	}})
	res := syncInbox(t, h, lt.ws, lt.chatInbox)

	c, token := lt.form(h)
	h.clock.Advance(3 * time.Second)
	c.expect(http.StatusAccepted, "POST", "/client/v1/channels/"+lt.key+"/subscriptions", lt.subscribeBody(unique("reader")+"@example.org", token))
	before := h.smtp.count()
	h.runListMail(lt.ws)
	if h.smtp.count() != before+1 {
		t.Fatal("no list mail")
	}
	if tenant, set := sesHeader(t, h.smtp.sent[before].raw); tenant != res.Tenant || set != sesConfigSet {
		t.Fatalf("list mail headers %q %q", tenant, set)
	}

	rcpt := unique("customer") + "@example.net"
	addr := channelAddress(t, o, lt.emailChannel)
	r := h.ingest(addr, buildMail(mailOpts{from: rcpt, to: addr, subject: "Q", messageID: newMessageID(), body: "Q?"}), nil)
	reply := o.expect(http.StatusCreated, "POST", "/v1/conversations/"+r.str("conversation_id")+"/messages", map[string]any{"kind": "message", "body": "A."})
	h.sendQueued(t, lt.ws, reply.str("id"))
	sent, _ := h.smtp.last(t)
	if tenant, set := sesHeader(t, sent.raw); tenant != res.Tenant || set != sesConfigSet {
		t.Fatalf("reply headers %q %q", tenant, set)
	}

	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+lt.emailChannel, map[string]any{"email": map[string]any{
		"address": addr, "smtp": sesSMTP(sesHost, sesOtherKey), "ses": map[string]any{"tenant": "their-tenant"},
	}})
	syncInbox(t, h, lt.ws, lt.chatInbox)
	reply = o.expect(http.StatusCreated, "POST", "/v1/conversations/"+r.str("conversation_id")+"/messages", map[string]any{"kind": "message", "body": "B."})
	h.sendQueued(t, lt.ws, reply.str("id"))
	sent, _ = h.smtp.last(t)
	if tenant, set := sesHeader(t, sent.raw); tenant != "their-tenant" || set != "" {
		t.Fatalf("typed tenant headers %q %q", tenant, set)
	}

	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+lt.emailChannel, map[string]any{"email": map[string]any{
		"address": addr, "smtp": sesSMTP(sesHost, sesServerKey), "from_address": "someone@pending.example", "ses": map[string]any{"tenant": ""},
	}})
	syncInbox(t, h, lt.ws, lt.chatInbox)
	if ses := channelSES(t, o, lt.emailChannel); ses["state"] != "identity_missing" {
		t.Fatalf("not isolated: %v", ses)
	}
	reply = o.expect(http.StatusCreated, "POST", "/v1/conversations/"+r.str("conversation_id")+"/messages", map[string]any{"kind": "message", "body": "C."})
	h.sendQueued(t, lt.ws, reply.str("id"))
	sent, _ = h.smtp.last(t)
	if tenant, set := sesHeader(t, sent.raw); tenant != "" || set != sesConfigSet {
		t.Fatalf("a managed channel without isolation: headers %q %q", tenant, set)
	}

	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+lt.emailChannel, map[string]any{"email": map[string]any{
		"address": addr, "smtp": sesSMTP(sesHost, sesServerKey), "from_address": "ops@verified.example",
	}})
	if ses := channelSES(t, o, lt.emailChannel); ses["state"] != "pending" {
		t.Fatalf("after a save: %v", ses)
	}
	reply = o.expect(http.StatusCreated, "POST", "/v1/conversations/"+r.str("conversation_id")+"/messages", map[string]any{"kind": "message", "body": "D."})
	h.sendQueued(t, lt.ws, reply.str("id"))
	sent, _ = h.smtp.last(t)
	if tenant, set := sesHeader(t, sent.raw); tenant != "" || set != sesConfigSet {
		t.Fatalf("a managed channel waiting for its sync: headers %q %q", tenant, set)
	}
	syncInbox(t, h, lt.ws, lt.chatInbox)
	reply = o.expect(http.StatusCreated, "POST", "/v1/conversations/"+r.str("conversation_id")+"/messages", map[string]any{"kind": "message", "body": "E."})
	h.sendQueued(t, lt.ws, reply.str("id"))
	sent, _ = h.smtp.last(t)
	if tenant, set := sesHeader(t, sent.raw); tenant != res.Tenant || set != sesConfigSet {
		t.Fatalf("isolated again: headers %q %q", tenant, set)
	}

	et := newEmailTeam(t, h, false)
	r = h.ingest(et.address, buildMail(mailOpts{from: rcpt, to: et.address, subject: "Q", messageID: newMessageID(), body: "Q?"}), nil)
	reply = et.owner.expect(http.StatusCreated, "POST", "/v1/conversations/"+r.str("conversation_id")+"/messages", map[string]any{"kind": "message", "body": "A."})
	h.sendQueued(t, et.ws, reply.str("id"))
	sent, _ = h.smtp.last(t)
	if tenant, _ := sesHeader(t, sent.raw); tenant != "" {
		t.Fatalf("a channel without SES sent %q", tenant)
	}
}

// sesBroadcastTeam is a broadcast team whose channel sends support and broadcast mail through the
// server's SES account, synced.
func sesBroadcastTeam(t *testing.T, h *harness, bs *broadcastSender) (broadcastTeam, string) {
	t.Helper()
	bt := newBroadcastTeam(t, h, bs)
	bt.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+bt.emailChannel, map[string]any{"email": map[string]any{
		"address": channelAddress(t, bt.owner, bt.emailChannel), "smtp": sesSMTP(sesHost, sesServerKey), "broadcast_smtp": sesSMTP(sesHost, sesServerKey),
	}})
	res := syncInbox(t, h, bt.ws, bt.chatInbox)
	return bt, res.Tenant
}

func TestSESTenantBroadcasts(t *testing.T) {
	f := newFakeSES()
	h, bs := newBroadcastHarness(t, func(d *api.Deps) {
		withSES(f)(d)
		d.OperatorEmails = []string{unique("operator") + "@example.com"}
	})
	bt, tenant := sesBroadcastTeam(t, h, bs)
	bt.subscribe(t, "ada@example.org")
	id := bt.draft(t, nil)
	if item := reviewItem(bt.owner.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id+"/review", nil).body, "ses_tenant"); item["status"] != "ok" {
		t.Fatalf("review item: %v", item)
	}
	bt.owner.expect(http.StatusAccepted, "POST", "/v1/broadcasts/"+id+"/test", nil)
	for _, j := range h.jobs("broadcast_test", bt.ws) {
		var a api.BroadcastTestArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := h.srv.SendBroadcastTestMail(context.Background(), a, false); err != nil {
			t.Fatal(err)
		}
		h.dropJob(j.id)
	}
	if bs.count() != 1 {
		t.Fatalf("%d test mails", bs.count())
	}
	if got, set := sesHeader(t, bs.sent[0].raw); got != tenant || set != sesConfigSet {
		t.Fatalf("broadcast test mail headers %q %q", got, set)
	}

	other, otherTenant := sesBroadcastTeam(t, h, bs)
	other.subscribe(t, "grace@example.org")
	otherID := other.draft(t, nil)
	sameWS := bt.owner.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Second", "slug": "second"}).body["inbox"].(map[string]any)["id"].(string)
	bt.owner.expect(http.StatusOK, "PATCH", "/v1/inboxes/"+sameWS, map[string]any{
		"sender": map[string]any{"legal_name": "Acme Ltd", "postal_address": "1 Main Street, Springfield"},
	})
	ch2 := createEmailChannel(t, bt.owner, sameWS, unique("second")+"@example.com", sesSMTP(sesHost, sesServerKey), map[string]any{"display_name": "Second"})
	list2 := bt.owner.expect(http.StatusCreated, "POST", "/v1/lists", map[string]any{"inbox_id": sameWS, "name": "Second list", "channel_id": ch2}).str("id")
	bt.owner.expect(http.StatusCreated, "POST", "/v1/lists/"+list2+"/subscriptions", map[string]any{"email": "linus@example.org", "consent": map[string]any{"confirmed": true}})
	secondTenant := syncInbox(t, h, bt.ws, sameWS).Tenant
	id2 := bt.owner.expect(http.StatusCreated, "POST", "/v1/broadcasts", map[string]any{
		"inbox_id": sameWS, "channel_id": ch2, "name": "Second news", "subject": "News", "design_id": bt.design, "audience": map[string]any{"list_ids": []string{list2}},
	}).str("id")
	for _, b := range []struct {
		c  *client
		id string
	}{{bt.owner, id}, {other.owner, otherID}, {bt.owner, id2}} {
		if st := b.c.expect(http.StatusOK, "POST", "/v1/broadcasts/"+b.id+"/schedule", nil).body["state"]; st != "scheduled" {
			t.Fatalf("schedule %s: %v", b.id, st)
		}
	}

	tenantARN := f.tenants[tenant].ARN
	f.setEntity(awsses.Entity{Reference: tenantARN, Type: "RESOURCE", Aggregate: "DISABLED", Impact: "HIGH",
		Customer: awsses.Status{Status: "DISABLED", Cause: "bounce rate above the policy"}, AWS: awsses.Status{Status: "ENABLED"}})
	f.setEntity(awsses.Entity{Reference: f.tenants[otherTenant].ARN, Aggregate: "ENABLED"})
	f.setEntity(awsses.Entity{Reference: f.tenants[secondTenant].ARN, Aggregate: "ENABLED"})
	if err := h.srv.RefreshSESTenants(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	b := bt.get(t, id)
	if b["state"] != "paused" || b["pause_reason"] != "ses_tenant" || !strings.Contains(b["state_detail"].(string), "bounce rate above the policy") {
		t.Fatalf("broadcast of the paused tenant: %v %v %v", b["state"], b["pause_reason"], b["state_detail"])
	}
	if st := other.get(t, otherID)["state"]; st != "scheduled" {
		t.Fatalf("another workspace's broadcast: %v", st)
	}
	if st := bt.get(t, id2)["state"]; st != "scheduled" {
		t.Fatalf("another inbox's broadcast: %v", st)
	}
	evs := broadcastEvents(t, bt, id)
	if len(evs) == 0 || evs[len(evs)-1]["broadcast"].(map[string]any)["state"] != "paused" {
		t.Fatalf("events %v", evs)
	}
	notice := "SELECT count(*) FROM operator_notices WHERE kind = 'ses_tenant_paused' AND detail LIKE '%' || $1 || '%'"
	if n := h.count(notice, tenant); n != 1 {
		t.Fatalf("%d operator notices", n)
	}
	st := bt.owner.expect(http.StatusOK, "GET", "/v1/inboxes/"+bt.chatInbox, nil).body["ses_tenant"].(map[string]any)
	if st["sending_status"] != "disabled" || st["paused_by"] != "policy" || st["reputation_impact"] != "high" || st["cause"] != "bounce rate above the policy" {
		t.Fatalf("paused tenant: %v", st)
	}
	bt.owner.expect(http.StatusNoContent, "PUT", "/v1/inboxes/"+bt.chatInbox+"/members/"+bt.agentID, nil)
	if st := bt.agent.expect(http.StatusOK, "GET", "/v1/inboxes/"+bt.chatInbox, nil).body["ses_tenant"].(map[string]any); st["sending_status"] != "disabled" || st["cause"] != nil {
		t.Fatalf("an agent reads the paused tenant as %v", st)
	}
	if item := reviewItem(bt.owner.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id+"/review", nil).body, "ses_tenant"); item["status"] != "blocking" {
		t.Fatalf("review item while paused: %v", item)
	}
	bt.owner.expectProblem(http.StatusConflict, "ses_tenant_blocked", "POST", "/v1/broadcasts/"+id+"/resume", nil)

	if err := h.srv.RefreshSESTenants(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if n := h.count(notice, tenant); n != 1 {
		t.Fatalf("a second poll of a paused tenant wrote %d notices", n)
	}

	if _, err := h.srv.OperatorReinstateSESTenant(context.Background(), uuid.MustParse(bt.ws), uuid.MustParse(bt.chatInbox)); err != nil {
		t.Fatal(err)
	}
	if len(f.reinstated) != 1 || f.reinstated[0] != tenantARN {
		t.Fatalf("reinstated %v", f.reinstated)
	}
	f.setEntity(awsses.Entity{Reference: tenantARN, Aggregate: "REINSTATED", Impact: "LOW", Customer: awsses.Status{Status: "REINSTATED"}, AWS: awsses.Status{Status: "ENABLED"}})
	if err := h.srv.RefreshSESTenants(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if st := bt.get(t, id)["state"]; st != "paused" {
		t.Fatalf("reinstating changed the broadcast: %v", st)
	}
	if st := bt.owner.expect(http.StatusOK, "POST", "/v1/broadcasts/"+id+"/resume", nil).body["state"]; st == "paused" {
		t.Fatalf("resume after reinstatement: %v", st)
	}
	bt.run(t)
	if m := bs.to("ada@example.org"); len(m) != 1 || m[0].Header.Get("X-Ses-Tenant") != tenant || m[0].Header.Get("X-Ses-Configuration-Set") != sesConfigSet {
		t.Fatalf("broadcast mail to ada: %d", len(m))
	}
	if _, err := h.st.Pool.Exec(context.Background(), "UPDATE email_channels SET ses_state = 'identity_missing', ses_detail = 'second.example is not verified' WHERE channel_id = $1", ch2); err != nil {
		t.Fatal(err)
	}
	if _, err := h.srv.SendBroadcasts(context.Background(), uuid.MustParse(bt.ws), uuid.MustParse(ch2), time.Minute); err != nil {
		t.Fatal(err)
	}
	if b := bt.get(t, id2); b["state"] != "paused" || b["pause_reason"] != "ses_tenant" || b["state_detail"] != "second.example is not verified" {
		t.Fatalf("a batch through a channel that lost isolation: %v %v %v", b["state"], b["pause_reason"], b["state_detail"])
	}
	if len(bs.to("linus@example.org")) != 0 {
		t.Fatal("mail went out through a channel that lost isolation")
	}

	f.setEntity(awsses.Entity{Reference: tenantARN, Aggregate: "DISABLED", Customer: awsses.Status{Status: "ENABLED"}, AWS: awsses.Status{Status: "DISABLED", Cause: "abuse"}})
	if err := h.srv.RefreshSESTenants(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if st := bt.owner.expect(http.StatusOK, "GET", "/v1/inboxes/"+bt.chatInbox, nil).body["ses_tenant"].(map[string]any); st["paused_by"] != "aws" {
		t.Fatalf("paused by SES: %v", st)
	}
	if _, err := h.srv.OperatorReinstateSESTenant(context.Background(), uuid.MustParse(bt.ws), uuid.MustParse(bt.chatInbox)); err == nil || !strings.Contains(err.Error(), "AWS Support") {
		t.Fatalf("reinstating a pause by SES: %v", err)
	}
}

func TestSESTenantNotifications(t *testing.T) {
	f := newFakeSES()
	h := newHarnessWith(t, withSES(f))
	signer := newSNSSigner(t, h.web)
	et := newEmailTeam(t, h, false)
	o := et.owner
	o.expect(http.StatusOK, "PATCH", "/v1/channels/"+et.channel, map[string]any{"email": map[string]any{
		"address": channelAddress(t, o, et.channel), "smtp": sesSMTP(sesHost, sesServerKey),
	}})
	tenant := syncInbox(t, h, et.ws, et.inbox).Tenant
	for _, j := range h.jobs("ses_tenant_status", et.ws) {
		h.dropJob(j.id)
	}
	post := func(payload any) {
		t.Helper()
		b, _ := json.Marshal(payload)
		note := &email.SNSMessage{Type: "Notification", MessageId: unique("m"), TopicArn: testSESTopic, Message: string(b), Timestamp: time.Now().UTC().Format(time.RFC3339)}
		if st := h.postSNS(signer.sign(t, note)); st != http.StatusOK {
			t.Fatalf("notification: %d", st)
		}
	}
	event := func(arn string) map[string]any {
		return map[string]any{"version": "0", "source": "aws.ses", "detail-type": "Sending Status Disabled", "resources": []string{arn}, "detail": map[string]any{}}
	}
	post(event("arn:aws:ses:eu-west-1:" + sesCaller + ":tenant/yuva-unknown/tn-x"))
	if n := len(h.jobs("ses_tenant_status", et.ws)); n != 0 {
		t.Fatalf("an unknown tenant queued %d jobs", n)
	}
	post(event(f.tenants[tenant].ARN))
	jobs := h.jobs("ses_tenant_status", et.ws)
	if len(jobs) != 1 || !strings.Contains(string(jobs[0].args), et.inbox) {
		t.Fatalf("status jobs %v", jobs)
	}

	bounce := func(rcpt, msgID, tenant string) map[string]any {
		return map[string]any{
			"notificationType": "Bounce",
			"bounce": map[string]any{"bounceType": "Permanent", "bounceSubType": "OnTenantSuppressionList",
				"bouncedRecipients": []any{map[string]any{"emailAddress": rcpt, "status": "5.1.1"}}},
			"mail": map[string]any{"messageId": "ses-" + unique("id"), "commonHeaders": map[string]any{"messageId": "<" + msgID + ">"},
				"tags": map[string]any{"ses:tenant-name": []string{tenant}}},
		}
	}
	send := func(rcpt string) (string, string) {
		r := h.ingest(et.address, buildMail(mailOpts{from: rcpt, to: et.address, subject: "Q", messageID: newMessageID(), body: "Q?"}), nil)
		conv := r.str("conversation_id")
		reply := o.expect(http.StatusCreated, "POST", "/v1/conversations/"+conv+"/messages", map[string]any{"kind": "message", "body": "A."})
		h.sendQueued(t, et.ws, reply.str("id"))
		return conv, lastOf(messages(o, conv), "message")["email"].(map[string]any)["message_id"].(string)
	}
	rcpt := unique("someone") + "@example.net"
	conv, msgID := send(rcpt)
	post(bounce(rcpt, msgID, "yuva-someoneelse"))
	if d := lastOf(messages(o, conv), "message")["delivery"].(map[string]any); d["state"] == "failed" {
		t.Fatalf("a bounce of another tenant was applied: %v", d)
	}
	post(bounce(rcpt, msgID, tenant))
	if d := lastOf(messages(o, conv), "message")["delivery"].(map[string]any); d["state"] != "failed" {
		t.Fatalf("a tenant suppression bounce was not applied: %v", d)
	}

	other := o.expect(http.StatusCreated, "POST", "/v1/inboxes", map[string]any{"name": "Other", "slug": "other"}).body["inbox"].(map[string]any)["id"].(string)
	createEmailChannel(t, o, other, unique("other")+"@example.com", sesSMTP(sesHost, sesServerKey), nil)
	otherTenant := syncInbox(t, h, et.ws, other).Tenant
	moved := unique("moved") + "@example.net"
	conv, msgID = send(moved)
	o.expect(http.StatusOK, "POST", "/v1/conversations/"+conv+"/move", map[string]any{"inbox_id": other})
	post(bounce(moved, msgID, otherTenant))
	if d := lastOf(messages(o, conv), "message")["delivery"].(map[string]any); d["state"] == "failed" {
		t.Fatalf("a bounce naming the new inbox's tenant was applied to mail the old inbox sent: %v", d)
	}
	post(bounce(moved, msgID, tenant))
	if d := lastOf(messages(o, conv), "message")["delivery"].(map[string]any); d["state"] != "failed" {
		t.Fatalf("a bounce of mail sent before the conversation moved was dropped: %v", d)
	}

	orphan := unique("orphan") + "@example.net"
	conv, msgID = send(orphan)
	o.expect(http.StatusNoContent, "DELETE", "/v1/channels/"+et.channel, nil)
	post(bounce(orphan, msgID, "yuva-someoneelse"))
	if d := lastOf(messages(o, conv), "message")["delivery"].(map[string]any); d["state"] != "failed" {
		t.Fatalf("a bounce whose sending channel is gone was dropped: %v", d)
	}
}

func TestSESTenantsOff(t *testing.T) {
	h := newHarness(t)
	tm := newTeam(t, h)
	ch := createEmailChannel(t, tm.owner, tm.inbox, unique("support")+"@example.com", sesSMTP(sesHost, sesServerKey), nil)
	if ses := channelSES(t, tm.owner, ch); ses["source"] != "none" || ses["state"] != "not_isolated" {
		t.Fatalf("without server keys: %v", ses)
	}
	if n := len(h.jobs("ses_tenant_sync", tm.ws)); n != 0 {
		t.Fatalf("%d sync jobs without server keys", n)
	}
	if _, err := h.srv.OperatorSyncSESTenant(context.Background(), uuid.MustParse(tm.ws), uuid.MustParse(tm.inbox), false); err != api.ErrSESNotConfigured {
		t.Fatalf("operator sync without keys: %v", err)
	}
}

func TestSESTenantSetupWaitsAndMissingTenant(t *testing.T) {
	f := newFakeSES()
	h, bs := newBroadcastHarness(t, withSES(f))
	bt, tenant := sesBroadcastTeam(t, h, bs)
	addr := channelAddress(t, bt.owner, bt.emailChannel)
	bt.subscribe(t, "ada@example.org", "grace@example.org")
	id := bt.draft(t, nil)
	bt.schedule(t, id)
	bt.owner.expect(http.StatusOK, "PATCH", "/v1/channels/"+bt.emailChannel, map[string]any{"email": map[string]any{
		"address": addr, "from_address": "news@example.com", "smtp": sesSMTP(sesHost, sesServerKey), "broadcast_smtp": sesSMTP(sesHost, sesServerKey),
	}})
	if ses := channelSES(t, bt.owner, bt.emailChannel); ses["state"] != "pending" {
		t.Fatalf("after a save: %v", ses)
	}
	if item := reviewItem(bt.owner.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id+"/review", nil).body, "ses_tenant"); item["status"] != "warning" {
		t.Fatalf("review item while the tenant is set up: %v", item)
	}
	if wait := bt.run(t); wait <= 0 {
		t.Fatalf("a pending tenant did not hold sending back: wait %v", wait)
	}
	if b := bt.get(t, id); b["state"] == "paused" || bs.count() != 0 {
		t.Fatalf("a pending tenant: state %v, %d mails", b["state"], bs.count())
	}
	syncInbox(t, h, bt.ws, bt.chatInbox)
	bt.run(t)
	if b := bt.get(t, id); b["state"] == "paused" || len(bs.to("ada@example.org")) != 1 || len(bs.to("grace@example.org")) != 1 {
		t.Fatalf("after the sync: state %v, %d mails", b["state"], bs.count())
	}

	f.mu.Lock()
	delete(f.tenants, tenant)
	delete(f.assoc, tenant)
	f.mu.Unlock()
	for _, j := range h.jobs("ses_tenant_sync", bt.ws) {
		h.dropJob(j.id)
	}
	if err := h.srv.RefreshSESTenants(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	if st := bt.owner.expect(http.StatusOK, "GET", "/v1/inboxes/"+bt.chatInbox, nil).body["ses_tenant"].(map[string]any); st["state"] != "missing" {
		t.Fatalf("a tenant gone from SES: %v", st)
	}
	if n := len(h.jobs("ses_tenant_sync", bt.ws)); n != 1 {
		t.Fatalf("%d sync jobs for a missing tenant", n)
	}
	id2 := bt.draft(t, map[string]any{"name": "November news"})
	if item := reviewItem(bt.owner.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id2+"/review", nil).body, "ses_tenant"); item["status"] != "blocking" {
		t.Fatalf("review item of a missing tenant: %v", item)
	}
	before := bs.count()
	bt.owner.expect(http.StatusAccepted, "POST", "/v1/broadcasts/"+id2+"/test", nil)
	for _, j := range h.jobs("broadcast_test", bt.ws) {
		var a api.BroadcastTestArgs
		if err := json.Unmarshal(j.args, &a); err != nil {
			t.Fatal(err)
		}
		if err := h.srv.SendBroadcastTestMail(context.Background(), a, false); err != nil {
			t.Fatal(err)
		}
		h.dropJob(j.id)
	}
	if bs.count() != before+1 {
		t.Fatal("no test mail")
	}
	if got, set := sesHeader(t, bs.sent[before].raw); got != "" || set != sesConfigSet {
		t.Fatalf("mail of a missing tenant: headers %q %q", got, set)
	}
	if res := syncInbox(t, h, bt.ws, bt.chatInbox); res.Tenant != tenant || f.tenants[tenant].ARN == "" {
		t.Fatalf("set up again: %+v", res)
	}
	if got := f.associated(tenant); !slices.Equal(got, []string{configSetARN, identityARN("example.com")}) {
		t.Fatalf("associated again: %v", got)
	}
	if item := reviewItem(bt.owner.expect(http.StatusOK, "GET", "/v1/broadcasts/"+id2+"/review", nil).body, "ses_tenant"); item["status"] != "ok" {
		t.Fatalf("review item after the tenant is back: %v", item)
	}
}

func TestSESTenantDelete(t *testing.T) {
	f := newFakeSES()
	h := newHarnessWith(t, withSES(f))
	ctx := context.Background()
	name := "yuva-" + unique("del")
	f.tenants[name] = awsses.Tenant{Name: name}
	f.assoc[name] = []string{configSetARN, identityARN("example.com")}
	f.deleteErr = &awsses.Error{Operation: "DeleteTenant", Status: 409, Code: "ConflictException"}
	if err := h.srv.DeleteSESTenant(ctx, api.SESTenantDeleteArgs{Name: name, Region: "eu-west-1"}); err == nil {
		t.Fatal("a conflict was not retried")
	}
	if got := f.associated(name); len(got) != 0 {
		t.Fatalf("still associated before the delete: %v", got)
	}
	f.deleteErr = nil
	if err := h.srv.DeleteSESTenant(ctx, api.SESTenantDeleteArgs{Name: name, Region: "eu-west-1"}); err != nil || !slices.Contains(f.deleted, name) {
		t.Fatalf("delete: %v, deleted %v", err, f.deleted)
	}
	if err := h.srv.DeleteSESTenant(ctx, api.SESTenantDeleteArgs{Name: name, Region: "eu-west-1"}); err != nil {
		t.Fatalf("a tenant already gone: %v", err)
	}
	f.deleteErr = &awsses.Error{Operation: "DeleteTenant", Status: 503, Code: "ServiceUnavailable"}
	f.tenants["yuva-temp"] = awsses.Tenant{Name: "yuva-temp"}
	if err := h.srv.DeleteSESTenant(ctx, api.SESTenantDeleteArgs{Name: "yuva-temp", Region: "eu-west-1"}); err == nil {
		t.Fatal("a temporary failure was not retried")
	}
	f.tenants["other-x"] = awsses.Tenant{Name: "other-x"}
	if err := h.srv.DeleteSESTenant(ctx, api.SESTenantDeleteArgs{Name: "other-x", Region: "eu-west-1"}); err != nil || slices.Contains(f.deleted, "other-x") {
		t.Fatalf("a tenant outside the prefix: %v, deleted %v", err, f.deleted)
	}
	if err := h.srv.DeleteSESTenant(ctx, api.SESTenantDeleteArgs{Name: "yuva-temp", Region: "us-east-1"}); err != nil {
		t.Fatalf("another region: %v", err)
	}
}
