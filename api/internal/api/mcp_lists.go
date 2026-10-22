package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
	"time"
	"uuid"

	"github.com/oapi-codegen/nullable"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const mcpImportMaxBytes = 5 << 20

var (
	errNotListManager     = problem(http.StatusForbidden, "forbidden", "only owners, admins and API keys change mailing lists, subscribers and imports")
	errImportCountChanged = problem(http.StatusConflict, "count_mismatch", "confirm_count is not the import's counts.valid; show the person get_list_import's counts.valid and start only after they confirm that number")
	errConsentNotStated   = errValidation("confirmed needs consent_text and consent_where, both as the person stated them; leave confirmed false to mail the subscriber a confirmation instead")
	importFields          = []any{"email", "name", "first_name", "last_name", "locale", "external_id", "attribute", "skip"}
	subscriptionStates    = []any{"pending", "subscribed", "unsubscribed"}
)

func listTool(ops ...string) mcpTool {
	return mcpTool{ops: ops, allow: mayManage, denied: errNotListManager}
}

func senderReady(legalName, postalAddress *string) bool {
	return deref(legalName) != "" && deref(postalAddress) != ""
}

type updateInboxSenderIn struct {
	InboxID       uuid.UUID `json:"inbox_id"`
	LegalName     *string   `json:"legal_name,omitempty" jsonschema:"the sender's legal name exactly as the person gave it; an empty string clears it"`
	PostalAddress *string   `json:"postal_address,omitempty" jsonschema:"the sender's postal address exactly as the person gave it; an empty string clears it"`
	Registration  *string   `json:"registration,omitempty" jsonschema:"a registration such as a MERSİS number, VAT id or company number, as the person gave it; an empty string clears it"`
	Contact       *string   `json:"contact,omitempty" jsonschema:"an e-mail address, phone number or URL where people reach the sender, as the person gave it; an empty string clears it"`
}

type createEmailChannelIn struct {
	InboxID     uuid.UUID `json:"inbox_id"`
	Address     string    `json:"address" jsonschema:"the address customers write to, such as support@example.com"`
	Name        string    `json:"name,omitempty" jsonschema:"shown to members; the inbox's name when empty"`
	DisplayName string    `json:"display_name,omitempty" jsonschema:"the name in From; the channel's name when empty"`
}

type createEmailChannelOut struct {
	Channel            mcpChannel `json:"channel"`
	Setup              string     `json:"setup" jsonschema:"what the person does next for mail to arrive and go out; tell them"`
	ChannelSettingsURL string     `json:"channel_settings_url,omitempty" jsonschema:"the channel's settings in the panel, where the person sets the SMTP account; its workspace parameter opens it in this workspace even when another one is open in the panel"`
}

type channelIDIn struct {
	ChannelID uuid.UUID `json:"channel_id" jsonschema:"an e-mail channel from list_channels"`
}

type deliverabilityIn struct {
	ChannelID uuid.UUID `json:"channel_id" jsonschema:"an e-mail channel from list_channels"`
	Check     bool      `json:"check,omitempty" jsonschema:"true sends a test message through the channel's SMTP account and checks it again; owners, admins and keys with inboxes:manage, a few times an hour"`
}

type mcpDNSRecord struct {
	Type      string `json:"type" jsonschema:"the DNS record type"`
	Name      string `json:"name" jsonschema:"the full host name of the record"`
	Value     string `json:"value"`
	Purpose   string `json:"purpose" jsonschema:"what the record is for, such as dkim"`
	Published *bool  `json:"published,omitempty" jsonschema:"the last check found it in DNS"`
}

type mcpAuthResult struct {
	Status  string `json:"status" jsonschema:"the receiving server's result, such as pass, fail, none or unknown"`
	Aligned *bool  `json:"aligned,omitempty" jsonschema:"the domain matches From, as DMARC needs"`
	Domain  string `json:"domain,omitempty"`
	Record  string `json:"record,omitempty" jsonschema:"the record found in DNS"`
	Policy  string `json:"policy,omitempty" jsonschema:"DMARC: none, quarantine or reject"`
}

type mcpDeliverability struct {
	ChannelID uuid.UUID             `json:"channel_id"`
	Checked   bool                  `json:"checked" jsonschema:"false when the channel was never checked; call again with check true"`
	State     string                `json:"state,omitempty" jsonschema:"queued and sent while the check runs; received, unverifiable, timeout or failed when it ended; call again without check to read it"`
	Domain    string                `json:"domain,omitempty" jsonschema:"the sending domain"`
	CheckedAt *time.Time            `json:"checked_at,omitempty"`
	Error     string                `json:"error,omitempty" jsonschema:"why the test message was not sent"`
	TLS       *bool                 `json:"tls,omitempty" jsonschema:"the test went to the SMTP server encrypted"`
	SPF       *mcpAuthResult        `json:"spf,omitempty"`
	DKIM      *mcpAuthResult        `json:"dkim,omitempty"`
	DMARC     *mcpAuthResult        `json:"dmarc,omitempty"`
	MX        *oas.DeliverabilityMx `json:"mx,omitempty" jsonschema:"the MX hosts of the reply address's domain"`
	Records   []mcpDNSRecord        `json:"records" jsonschema:"the DNS records to add at the sending domain's DNS host"`
	Next      []string              `json:"next" jsonschema:"what to fix, in order"`
}

type createDkimKeyIn struct {
	ChannelID uuid.UUID `json:"channel_id" jsonschema:"an e-mail channel from list_channels"`
	Selector  string    `json:"selector,omitempty" jsonschema:"the DNS label the key is published under: lower-case letters, digits and inner hyphens; yuva when empty"`
}

type createDkimKeyOut struct {
	Selector          string       `json:"selector"`
	Record            mcpDNSRecord `json:"record" jsonschema:"the TXT record to add at the sending domain's DNS host"`
	Enabled           bool         `json:"enabled" jsonschema:"mail is signed with the key"`
	UnsupportedReason string       `json:"unsupported_reason,omitempty" jsonschema:"why Yuva does not sign this channel's mail even when enabled"`
	Next              string       `json:"next"`
}

func (c *mcpCall) deliverability(ctx context.Context, in deliverabilityIn) (mcpDeliverability, error) {
	out := mcpDeliverability{ChannelID: in.ChannelID, Records: []mcpDNSRecord{}, Next: []string{}}
	ch, err := c.getChannel(ctx, in.ChannelID)
	if err != nil {
		return out, err
	}
	if ch.Email == nil {
		return out, errNotEmailChannel
	}
	var check oas.DeliverabilityCheck
	if in.Check {
		if err := keyMayCall(c.p, "CheckDeliverability"); err != nil {
			return out, mcpScopeError(err)
		}
		if !mayManage(c.p) {
			return out, errNotManager
		}
		res, err := c.s.CheckDeliverability(ctx, oas.CheckDeliverabilityRequestObject{ChannelId: in.ChannelID})
		if err != nil {
			return out, err
		}
		check, out.Checked = oas.DeliverabilityCheck(res.(oas.CheckDeliverability202JSONResponse)), true
	} else {
		res, err := c.s.GetDeliverability(ctx, oas.GetDeliverabilityRequestObject{ChannelId: in.ChannelID})
		if err != nil && !errors.Is(err, errNeverChecked) {
			return out, err
		}
		if err == nil {
			check, out.Checked = oas.DeliverabilityCheck(res.(oas.GetDeliverability200JSONResponse)), true
		}
	}
	if d := ch.Email.Dkim; d != nil {
		rec := mcpDNSRecord{Type: "TXT", Name: d.DnsName, Value: d.DnsValue, Purpose: "dkim"}
		if check.YuvaDkim != nil && check.YuvaDkim.DnsName == d.DnsName {
			rec.Published = &check.YuvaDkim.Published
		}
		out.Records = append(out.Records, rec)
		if rec.Published == nil {
			out.Next = append(out.Next, "the TXT record "+d.DnsName+" belongs at the domain's DNS host; a finished check shows whether it is there")
		} else if !*rec.Published {
			out.Next = append(out.Next, "add the TXT record "+d.DnsName+" at the domain's DNS host, then check again")
		} else if !d.Enabled {
			out.Next = append(out.Next, "the DKIM record is published; an owner or admin turns signing on in the channel's settings in Yuva")
		}
	} else {
		out.Next = append(out.Next, "make a DKIM key with create_dkim_key and add its TXT record, unless the SMTP provider signs the mail itself")
	}
	if !out.Checked {
		out.Next = append(out.Next, "the channel was never checked: call get_deliverability with check true")
		return out, nil
	}
	out.State, out.Domain, out.CheckedAt, out.Error, out.TLS, out.MX = string(check.State), check.Domain, check.CheckedAt, deref(check.Error), check.Tls, check.Mx
	out.SPF = &mcpAuthResult{Status: string(check.Spf.Status), Aligned: &check.Spf.Aligned, Domain: deref(check.Spf.Domain), Record: deref(check.Spf.Record)}
	out.DKIM = &mcpAuthResult{Status: string(check.Dkim.Status), Aligned: &check.Dkim.Aligned, Domain: deref(check.Dkim.Domain)}
	out.DMARC = &mcpAuthResult{Status: string(check.Dmarc.Status), Record: deref(check.Dmarc.Record)}
	if check.Dmarc.Policy != nil {
		out.DMARC.Policy = string(*check.Dmarc.Policy)
	}
	if check.Mx == nil {
		if check.State == oas.DeliverabilityQueued {
			out.Next = append(out.Next, "the check is running and has not read DNS yet: call get_deliverability again without check in a minute")
		} else {
			out.Next = append(out.Next, "the check ended before it read DNS: call get_deliverability with check true to run it again")
		}
		return out, nil
	}
	if check.Spf.Record == nil {
		out.Next = append(out.Next, "the domain "+check.Domain+" has no SPF record; add the one the SMTP provider documents")
	}
	if check.Dmarc.Record == nil {
		out.Next = append(out.Next, "the domain "+check.Domain+" has no DMARC record; add a TXT record at _dmarc."+check.Domain)
	}
	return out, nil
}

type listIDIn struct {
	ListID uuid.UUID `json:"list_id" jsonschema:"a mailing list from list_mailing_lists"`
}

type createMailingListIn struct {
	InboxID     uuid.UUID  `json:"inbox_id"`
	Name        string     `json:"name" jsonschema:"shown to subscribers on forms and the preference page"`
	Description string     `json:"description,omitempty" jsonschema:"what subscribers get and how often"`
	ChannelID   *uuid.UUID `json:"channel_id,omitempty" jsonschema:"the inbox's e-mail channel that sends confirmations and welcome mails"`
	DoubleOptIn *bool      `json:"double_opt_in,omitempty" jsonschema:"form subscribers confirm by mail first; true when absent"`
	Public      *bool      `json:"public,omitempty" jsonschema:"offered on subscribe forms and the preference page; true when absent"`
	Commercial  *bool      `json:"commercial,omitempty" jsonschema:"marketing content rather than service announcements; true when absent"`
	Welcome     *bool      `json:"welcome,omitempty" jsonschema:"send a receipt with an unsubscribe link right after someone subscribes; true when absent"`
}

type updateMailingListIn struct {
	ListID      uuid.UUID  `json:"list_id"`
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	ChannelID   *uuid.UUID `json:"channel_id,omitempty" jsonschema:"the inbox's e-mail channel that sends confirmations and welcome mails"`
	DoubleOptIn *bool      `json:"double_opt_in,omitempty"`
	Public      *bool      `json:"public,omitempty"`
	Commercial  *bool      `json:"commercial,omitempty"`
	Welcome     *bool      `json:"welcome,omitempty"`
}

func mailingListOut(l oas.MailingList) mcpMailingList {
	return mcpMailingList{
		ID: l.Id, InboxID: l.InboxId, ChannelID: l.ChannelId, Name: l.Name, Description: l.Description, Public: l.Public,
		Commercial: l.Commercial, DoubleOptIn: l.DoubleOptIn, SenderReady: l.SenderReady, Subscribed: l.Counts.Subscribed, Pending: l.Counts.Pending,
	}
}

type listSubscriptionsIn struct {
	ListID uuid.UUID `json:"list_id"`
	State  string    `json:"state,omitempty"`
	Query  string    `json:"query,omitempty" jsonschema:"part of an address or name"`
	Cursor string    `json:"cursor,omitempty"`
	Limit  int32     `json:"limit,omitempty" jsonschema:"1 to 100, default 25"`
}

type mcpSubscriberContent struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type mcpSubscription struct {
	ID                uuid.UUID            `json:"id"`
	ListID            uuid.UUID            `json:"list_id"`
	ContactID         uuid.UUID            `json:"contact_id"`
	State             string               `json:"state" jsonschema:"pending waits for the person to confirm by mail; subscribed; unsubscribed, which only the person can undo"`
	Source            string               `json:"source" jsonschema:"form, identity, api, member, import or preferences"`
	UnsubscribeReason string               `json:"unsubscribe_reason,omitempty"`
	ConsentAt         *time.Time           `json:"consent_at,omitempty"`
	UnsubscribedAt    *time.Time           `json:"unsubscribed_at,omitempty"`
	CreatedAt         time.Time            `json:"created_at"`
	CustomerContent   mcpSubscriberContent `json:"customer_content"`
}

func subscriptionOut(s oas.Subscription) mcpSubscription {
	out := mcpSubscription{
		ID: s.Id, ListID: s.ListId, ContactID: s.Contact.Id, State: string(s.State), Source: string(s.Source), ConsentAt: s.ConsentAt,
		UnsubscribedAt: s.UnsubscribedAt, CreatedAt: s.CreatedAt, CustomerContent: mcpSubscriberContent{Email: string(s.Email), Name: s.Contact.Name},
	}
	if s.UnsubscribeReason != nil {
		out.UnsubscribeReason = string(*s.UnsubscribeReason)
	}
	return out
}

type addSubscriptionIn struct {
	ListID       uuid.UUID `json:"list_id"`
	Email        string    `json:"email"`
	Name         string    `json:"name,omitempty"`
	Locale       string    `json:"locale,omitempty" jsonschema:"the subscriber's language, a BCP 47 tag"`
	Confirmed    bool      `json:"confirmed,omitempty" jsonschema:"true only when the person you work for states that the subscriber already agreed to this list; false mails them a confirmation first"`
	ConsentText  string    `json:"consent_text,omitempty" jsonschema:"with confirmed: the statement the subscriber agreed to, in the person's own words; never write it yourself"`
	ConsentWhere string    `json:"consent_where,omitempty" jsonschema:"with confirmed: where and when the subscriber agreed, in the person's own words; never write it yourself"`
}

type subscriptionIn struct {
	ListID         uuid.UUID `json:"list_id"`
	SubscriptionID uuid.UUID `json:"subscription_id" jsonschema:"from list_subscriptions"`
}

type importIn struct {
	ListID   uuid.UUID `json:"list_id"`
	ImportID uuid.UUID `json:"import_id"`
}

type importColumnIn struct {
	Index     int32  `json:"index" jsonschema:"the column's position, from 0"`
	Field     string `json:"field"`
	Attribute string `json:"attribute,omitempty" jsonschema:"with field attribute: the contact attribute to fill"`
}

func importMapping(cols []importColumnIn) oas.ListImportMappingUpdate {
	out := oas.ListImportMappingUpdate{Columns: make([]oas.ListImportColumnMapping, len(cols))}
	for i, c := range cols {
		out.Columns[i] = oas.ListImportColumnMapping{Index: c.Index, Field: oas.ListImportField(c.Field), Attribute: optString[string](c.Attribute)}
	}
	return out
}

type createListImportIn struct {
	ListID        uuid.UUID        `json:"list_id"`
	CSV           string           `json:"csv" jsonschema:"the CSV file's text, header row first, at most 5 MB"`
	Filename      string           `json:"filename,omitempty" jsonschema:"the file's name, shown to members; import.csv when empty"`
	Attestation   string           `json:"attestation" jsonschema:"the person's statement that everyone in the file agreed to receive this list, word for word as they gave it; never write it yourself"`
	ConsentSource string           `json:"consent_source" jsonschema:"where and when the people in the file agreed, word for word as the person gave it; never write it yourself"`
	Reconfirm     bool             `json:"reconfirm,omitempty" jsonschema:"new addresses become pending and get a confirmation mail instead of being subscribed"`
	Columns       []importColumnIn `json:"columns,omitempty" jsonschema:"what each column holds; exactly one email column; found from the header when absent"`
}

type updateImportMappingIn struct {
	ListID   uuid.UUID        `json:"list_id"`
	ImportID uuid.UUID        `json:"import_id"`
	Columns  []importColumnIn `json:"columns" jsonschema:"columns left out are skipped; exactly one must be email"`
}

type startImportIn struct {
	ListID       uuid.UUID `json:"list_id"`
	ImportID     uuid.UUID `json:"import_id"`
	ConfirmCount int64     `json:"confirm_count" jsonschema:"counts.valid from get_list_import, which the person you work for confirmed"`
}

type listImportsIn struct {
	ListID uuid.UUID `json:"list_id"`
	Cursor string    `json:"cursor,omitempty"`
	Limit  int32     `json:"limit,omitempty" jsonschema:"1 to 100, default 25"`
}

type importErrorsIn struct {
	ListID   uuid.UUID `json:"list_id"`
	ImportID uuid.UUID `json:"import_id"`
	AfterRow int32     `json:"after_row,omitempty" jsonschema:"next_after_row of an earlier call"`
	Limit    int32     `json:"limit,omitempty" jsonschema:"1 to 500, default 100"`
}

type mcpImportColumn struct {
	Index     int32  `json:"index"`
	Header    string `json:"header" jsonschema:"the column's name in the file's header row"`
	Field     string `json:"field"`
	Attribute string `json:"attribute,omitempty"`
}

type mcpImportContent struct {
	Preview [][]string `json:"preview,omitempty" jsonschema:"the first data rows of the file"`
}

type mcpListImport struct {
	ID              uuid.UUID            `json:"id"`
	ListID          uuid.UUID            `json:"list_id"`
	State           string               `json:"state" jsonschema:"validating, then ready to check and start; importing, then done; failed or cancelled"`
	Filename        string               `json:"filename"`
	Reconfirm       bool                 `json:"reconfirm"`
	RowsTotal       int32                `json:"rows_total"`
	RowsDone        int32                `json:"rows_done"`
	Counts          oas.ListImportCounts `json:"counts" jsonschema:"valid is how many rows would be added: start_list_import needs it as confirm_count; the rest are refused rows by reason"`
	Columns         []mcpImportColumn    `json:"columns"`
	Error           string               `json:"error,omitempty"`
	Attestation     string               `json:"attestation"`
	ConsentSource   string               `json:"consent_source"`
	CreatedAt       time.Time            `json:"created_at"`
	StartedAt       *time.Time           `json:"started_at,omitempty"`
	FinishedAt      *time.Time           `json:"finished_at,omitempty"`
	ErrorsUntil     *time.Time           `json:"errors_until,omitempty" jsonschema:"when the refused rows are deleted"`
	CustomerContent mcpImportContent     `json:"customer_content"`
}

func listImportOut(imp oas.ListImport) mcpListImport {
	out := mcpListImport{
		ID: imp.Id, ListID: imp.ListId, State: string(imp.State), Filename: imp.Filename, Reconfirm: imp.Reconfirm, RowsTotal: imp.RowsTotal,
		RowsDone: imp.RowsDone, Counts: imp.Counts, Columns: []mcpImportColumn{}, Error: deref(imp.Error), Attestation: imp.Attestation,
		ConsentSource: imp.ConsentSource, CreatedAt: imp.CreatedAt, StartedAt: imp.StartedAt, FinishedAt: imp.FinishedAt, ErrorsUntil: imp.ErrorsUntil,
	}
	for _, c := range imp.Columns {
		out.Columns = append(out.Columns, mcpImportColumn{Index: c.Index, Header: c.Header, Field: string(c.Field), Attribute: deref(c.Attribute)})
	}
	if imp.Preview != nil {
		out.CustomerContent.Preview = *imp.Preview
	}
	return out
}

type mcpImportErrorContent struct {
	Email string `json:"email"`
}

type mcpImportError struct {
	Row             int32                 `json:"row" jsonschema:"the data row, from 1"`
	Reason          string                `json:"reason" jsonschema:"invalid_email, no_mail_domain, role_address, duplicate, suppressed, opted_out or unsubscribed"`
	CustomerContent mcpImportErrorContent `json:"customer_content"`
}

type importErrorsOut struct {
	Rows         []mcpImportError `json:"rows"`
	NextAfterRow int32            `json:"next_after_row,omitempty" jsonschema:"pass as after_row for the next rows"`
}

func (c *mcpCall) createListImport(ctx context.Context, in createListImportIn) (mcpListImport, error) {
	if len(in.CSV) > mcpImportMaxBytes {
		return mcpListImport{}, errValidation("csv is at most 5 MB through an assistant; a larger file is uploaded in Yuva")
	}
	filename := in.Filename
	if strings.TrimSpace(filename) == "" {
		filename = "import.csv"
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := [][2]string{{"attestation", in.Attestation}, {"consent_source", in.ConsentSource}}
	if in.Reconfirm {
		fields = append(fields, [2]string{"reconfirm", "true"})
	}
	if len(in.Columns) > 0 {
		m, err := json.Marshal(importMapping(in.Columns))
		if err != nil {
			return mcpListImport{}, err
		}
		fields = append(fields, [2]string{"mapping", string(m)})
	}
	for _, f := range fields {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return mcpListImport{}, err
		}
	}
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return mcpListImport{}, err
	}
	if _, err := fw.Write([]byte(in.CSV)); err != nil {
		return mcpListImport{}, err
	}
	if err := w.Close(); err != nil {
		return mcpListImport{}, err
	}
	res, err := c.s.CreateListImport(ctx, oas.CreateListImportRequestObject{ListId: in.ListID, Body: multipart.NewReader(&buf, w.Boundary())})
	if err != nil {
		return mcpListImport{}, err
	}
	return listImportOut(oas.ListImport(res.(oas.CreateListImport202JSONResponse))), nil
}

func (c *mcpCall) getListImport(ctx context.Context, listID, id uuid.UUID) (oas.ListImport, error) {
	res, err := c.s.GetListImport(ctx, oas.GetListImportRequestObject{ListId: listID, ImportId: id})
	if err != nil {
		return oas.ListImport{}, err
	}
	return oas.ListImport(res.(oas.GetListImport200JSONResponse)), nil
}

func (c *mcpCall) importErrors(ctx context.Context, in importErrorsIn) (importErrorsOut, error) {
	lim := in.Limit
	if lim <= 0 {
		lim = 100
	}
	if lim > 500 {
		return importErrorsOut{}, errValidation("limit is 1 to 500")
	}
	res, err := c.s.GetListImportErrors(ctx, oas.GetListImportErrorsRequestObject{ListId: in.ListID, ImportId: in.ImportID})
	if err != nil {
		return importErrorsOut{}, err
	}
	stream, ok := res.(importErrorsStream)
	if !ok {
		return importErrorsOut{}, errInternal
	}
	rows, err := c.s.st.ListListImportErrors(ctx, store.ListListImportErrorsParams{WorkspaceID: stream.ws, ImportID: stream.id, AfterRow: in.AfterRow, Lim: lim + 1})
	if err != nil {
		return importErrorsOut{}, err
	}
	out := importErrorsOut{Rows: []mcpImportError{}}
	if len(rows) > int(lim) {
		rows = rows[:lim]
		out.NextAfterRow = rows[lim-1].Row
	}
	for _, r := range rows {
		out.Rows = append(out.Rows, mcpImportError{Row: r.Row, Reason: r.Reason, CustomerContent: mcpImportErrorContent{Email: r.Email}})
	}
	return out, nil
}

var mcpListTools = []mcpTool{
	defineTool("update_inbox_sender", "Set an inbox's sender identity",
		"Sets who sends the inbox's list mail, which the footer of every broadcast shows as the law asks (CAN-SPAM, CASL, Türkiye's commercial communication rules): the legal name, the postal address, a registration and a contact. Broadcasts are blocked until the legal name and postal address are set. Every value comes from the person you work for, word for word; never invent, guess or complete one, and ask when one is missing. Fields left out stay as they are. Owners, admins and keys only. Maps to PATCH /v1/inboxes/{id} with sender.",
		idempotent, nil,
		mcpTool{ops: []string{"UpdateInbox"}, allow: mayManage, denied: errNotManager},
		func(ctx context.Context, c *mcpCall, in updateInboxSenderIn) (mcpInbox, error) {
			sender := oas.InboxSenderUpdate{}
			set := false
			for _, f := range []struct {
				v   *string
				out *nullable.Nullable[string]
			}{{in.LegalName, &sender.LegalName}, {in.PostalAddress, &sender.PostalAddress}, {in.Registration, &sender.Registration}, {in.Contact, &sender.Contact}} {
				if f.v != nil {
					*f.out, set = nullable.NewNullableWithValue(*f.v), true
				}
			}
			if !set {
				return mcpInbox{}, errNothingToUpdate
			}
			res, err := c.s.UpdateInbox(ctx, oas.UpdateInboxRequestObject{InboxId: in.InboxID, Body: &oas.InboxUpdate{Sender: &sender}})
			if err != nil {
				return mcpInbox{}, err
			}
			return inboxOut(oas.Inbox(res.(oas.UpdateInbox200JSONResponse))), nil
		}),
	defineTool("create_email_channel", "Create an e-mail channel",
		"Adds a support or sending address to an inbox, without an SMTP account: Yuva never takes a password through an assistant. The answer carries the forwarding address the mailbox forwards to so its mail reaches Yuva, and channel_settings_url, where the person sets the SMTP server that replies and broadcasts go out through. Owners, admins and keys only. Maps to POST /v1/inboxes/{id}/channels with kind email.",
		additive, nil,
		mcpTool{ops: []string{"CreateChannel"}, allow: mayManage, denied: errNotManager},
		func(ctx context.Context, c *mcpCall, in createEmailChannelIn) (createEmailChannelOut, error) {
			name := strings.TrimSpace(in.Name)
			if name == "" && keyMayCall(c.p, "GetInbox") != nil {
				name = in.Address
			}
			if name == "" {
				inbox, err := visibleInbox(ctx, c.s.st.Queries, c.p, in.InboxID)
				if err != nil {
					return createEmailChannelOut{}, err
				}
				name = inbox.Name
			}
			input := oas.EmailChannelInput{Address: oas.Email(in.Address), DisplayName: optString[string](in.DisplayName)}
			res, err := c.s.CreateChannel(ctx, oas.CreateChannelRequestObject{InboxId: in.InboxID, Body: &oas.ChannelCreate{
				Kind: oas.ChannelKindEmail, Name: name, Email: &input,
			}})
			if err != nil {
				return createEmailChannelOut{}, err
			}
			ch := channelOut(oas.Channel(res.(oas.CreateChannel201JSONResponse)))
			out := createEmailChannelOut{Channel: ch}
			inbound := "Mail to " + ch.EmailAddress + " reaches Yuva once the mail for that address is routed to this server."
			if ch.ForwardingAddress != "" {
				inbound = "Mail to " + ch.EmailAddress + " reaches Yuva once the mailbox forwards it, keeping a copy, to forwarding_address (" + ch.ForwardingAddress + "), or once the domain's MX points at this server."
			}
			out.Setup = "Yuva created the e-mail channel for " + ch.EmailAddress + " without an SMTP account. " + inbound +
				" Replies and broadcasts are sent only once the person sets the mailbox's SMTP server (host, port, username and password) in the panel at channel_settings_url; Yuva never takes that password through an assistant."
			if !c.s.noPanel {
				out.ChannelSettingsURL = c.panelURL("/settings/inboxes/" + in.InboxID.String() + "/channels/" + ch.ID.String())
			}
			return out, nil
		}),
	defineTool("get_deliverability", "Check an e-mail channel's deliverability",
		"Returns the DNS records to add for an e-mail channel (its DKIM key's TXT record) and what the last check found: SPF, DKIM and DMARC as a receiving server judged a test message, the DMARC policy and the reply domain's MX. check true sends a new test message through the channel's SMTP account first (owners, admins and keys with inboxes:manage); the check ends within minutes, so call again without check to read it. Maps to GET /v1/channels/{id}/deliverability and, with check, POST /v1/channels/{id}/deliverability/check.",
		delivering, nil, mcpTool{ops: []string{"GetChannel", "GetDeliverability"}},
		func(ctx context.Context, c *mcpCall, in deliverabilityIn) (mcpDeliverability, error) {
			return c.deliverability(ctx, in)
		}),
	defineTool("create_dkim_key", "Create a DKIM key",
		"Makes a DKIM key for an e-mail channel and returns the TXT record to add at the sending domain's DNS host. Once get_deliverability shows the record published, an owner or admin turns signing on in the channel's settings in Yuva. A channel has one key. Owners, admins and keys only. Maps to POST /v1/channels/{id}/dkim-key.",
		additive, nil,
		mcpTool{ops: []string{"CreateDkimKey"}, allow: mayManage, denied: errNotManager},
		func(ctx context.Context, c *mcpCall, in createDkimKeyIn) (createDkimKeyOut, error) {
			selector := strings.TrimSpace(in.Selector)
			if selector == "" {
				selector = "yuva"
			}
			res, err := c.s.CreateDkimKey(ctx, oas.CreateDkimKeyRequestObject{ChannelId: in.ChannelID, Body: &oas.DkimKeyCreate{Selector: selector}})
			if err != nil {
				return createDkimKeyOut{}, err
			}
			d := oas.EmailDkim(res.(oas.CreateDkimKey201JSONResponse))
			return createDkimKeyOut{
				Selector: d.Selector, Record: mcpDNSRecord{Type: "TXT", Name: d.DnsName, Value: d.DnsValue, Purpose: "dkim"},
				Enabled: d.Enabled, UnsupportedReason: deref(d.UnsupportedReason),
				Next: "add the TXT record at the domain's DNS host, check it with get_deliverability, then an owner or admin turns signing on in the channel's settings in Yuva",
			}, nil
		}),
	defineTool("create_mailing_list", "Create a mailing list",
		"Creates a newsletter or announcement list for an inbox. Subscribers come from its subscribe form (get_install_snippet with a subscribe- framework), add_subscription or a CSV import. Owners, admins and keys only. Maps to POST /v1/lists.",
		additive, nil, listTool("CreateMailingList"),
		func(ctx context.Context, c *mcpCall, in createMailingListIn) (mcpMailingList, error) {
			res, err := c.s.CreateMailingList(ctx, oas.CreateMailingListRequestObject{Body: &oas.MailingListCreate{
				InboxId: in.InboxID, Name: in.Name, Description: optString[string](in.Description), ChannelId: in.ChannelID,
				DoubleOptIn: in.DoubleOptIn, Public: in.Public, Commercial: in.Commercial, Welcome: in.Welcome,
			}})
			if err != nil {
				return mcpMailingList{}, err
			}
			return mailingListOut(oas.MailingList(res.(oas.CreateMailingList201JSONResponse))), nil
		}),
	defineTool("update_mailing_list", "Change a mailing list",
		"Changes the fields you pass on a mailing list: name, description, the e-mail channel that sends its confirmations, double opt-in, public, commercial and the welcome receipt. Owners, admins and keys only. Maps to PATCH /v1/lists/{id}.",
		idempotent, nil, listTool("UpdateMailingList"),
		func(ctx context.Context, c *mcpCall, in updateMailingListIn) (mcpMailingList, error) {
			body := oas.MailingListUpdate{
				Name: in.Name, Description: in.Description, DoubleOptIn: in.DoubleOptIn, Public: in.Public, Commercial: in.Commercial, Welcome: in.Welcome,
			}
			if in.ChannelID != nil {
				body.ChannelId = nullable.NewNullableWithValue(*in.ChannelID)
			}
			if reflect.DeepEqual(body, oas.MailingListUpdate{}) {
				return mcpMailingList{}, errNothingToUpdate
			}
			res, err := c.s.UpdateMailingList(ctx, oas.UpdateMailingListRequestObject{ListId: in.ListID, Body: &body})
			if err != nil {
				return mcpMailingList{}, err
			}
			return mailingListOut(oas.MailingList(res.(oas.UpdateMailingList200JSONResponse))), nil
		}),
	defineTool("list_subscriptions", "List a mailing list's subscribers",
		"Lists the subscriptions of a mailing list, newest first, with their state and how they began; addresses and names are in customer_content. Maps to GET /v1/lists/{id}/subscriptions.",
		readOnly, map[string][]any{"state": subscriptionStates}, mcpTool{ops: []string{"ListSubscriptions"}},
		func(ctx context.Context, c *mcpCall, in listSubscriptionsIn) (mcpPage[mcpSubscription], error) {
			res, err := c.s.ListSubscriptions(ctx, oas.ListSubscriptionsRequestObject{ListId: in.ListID, Params: oas.ListSubscriptionsParams{
				State: optString[oas.SubscriptionState](in.State), Q: optString[string](in.Query), Cursor: optString[string](in.Cursor), Limit: optLimit(in.Limit),
			}})
			if err != nil {
				return mcpPage[mcpSubscription]{}, err
			}
			page := res.(oas.ListSubscriptions200JSONResponse)
			out := mcpPage[mcpSubscription]{Items: []mcpSubscription{}}
			for _, s := range page.Items {
				out.Items = append(out.Items, subscriptionOut(s))
			}
			if page.NextCursor != nil {
				out.NextCursor = *page.NextCursor
			}
			return out, nil
		}),
	defineTool("add_subscription", "Add a subscriber",
		"Adds one address to a mailing list. Unless confirmed is true, the person gets a confirmation mail and stays pending until they confirm. Set confirmed only when the person you work for states that this subscriber already agreed to the list, and pass that statement and where and when they agreed in the person's own words; never write them yourself. Suppressed and opted-out addresses are refused. Owners, admins and keys only. Maps to POST /v1/lists/{id}/subscriptions.",
		delivering, nil, listTool("CreateSubscription"),
		func(ctx context.Context, c *mcpCall, in addSubscriptionIn) (mcpSubscription, error) {
			consent := oas.SubscriptionConsent{Confirmed: in.Confirmed}
			if in.Confirmed {
				if strings.TrimSpace(in.ConsentText) == "" || strings.TrimSpace(in.ConsentWhere) == "" {
					return mcpSubscription{}, errConsentNotStated
				}
				consent.Text, consent.Reference = &in.ConsentText, &in.ConsentWhere
			}
			body := oas.SubscriptionCreate{Email: &in.Email, Name: optString[string](in.Name), Locale: optString[oas.LanguageTag](in.Locale), Consent: consent}
			res, err := c.s.CreateSubscription(ctx, oas.CreateSubscriptionRequestObject{ListId: in.ListID, Body: &body})
			if err != nil {
				return mcpSubscription{}, err
			}
			switch r := res.(type) {
			case oas.CreateSubscription200JSONResponse:
				return subscriptionOut(oas.Subscription(r)), nil
			case oas.CreateSubscription201JSONResponse:
				return subscriptionOut(oas.Subscription(r)), nil
			case oas.CreateSubscription202JSONResponse:
				return subscriptionOut(oas.Subscription(r)), nil
			}
			return mcpSubscription{}, errInternal
		}),
	defineTool("unsubscribe", "Unsubscribe a subscriber",
		"Unsubscribes one subscription from a mailing list. Only the person themselves can subscribe again, so do it only when the person you work for asks. The subscription and its consent record stay. Owners, admins and keys only. Maps to PATCH /v1/lists/{id}/subscriptions/{sid} with state unsubscribed.",
		destructive, nil, listTool("UpdateSubscription"),
		func(ctx context.Context, c *mcpCall, in subscriptionIn) (mcpSubscription, error) {
			res, err := c.s.UpdateSubscription(ctx, oas.UpdateSubscriptionRequestObject{ListId: in.ListID, SubscriptionId: in.SubscriptionID, Body: &oas.SubscriptionUpdate{
				State: oas.SubscriptionStateUnsubscribed,
			}})
			if err != nil {
				return mcpSubscription{}, err
			}
			return subscriptionOut(oas.Subscription(res.(oas.UpdateSubscription200JSONResponse))), nil
		}),
	defineTool("create_list_import", "Import subscribers from a CSV",
		"Uploads a CSV file (as text, at most 5 MB) of people to add to a mailing list, the same upload the panel sends. attestation and consent_source are stored as the proof of consent: take both from the person you work for, word for word, and never write, shorten or improve them. Nothing is added yet: Yuva validates the file (state validating, then ready); read the counts with get_list_import, show the person counts.valid and start it with start_list_import once they confirm. One import of the workspace runs at a time. Owners, admins and keys only. Maps to POST /v1/lists/{id}/imports.",
		additive, map[string][]any{"columns.field": importFields}, listTool("CreateListImport"),
		func(ctx context.Context, c *mcpCall, in createListImportIn) (mcpListImport, error) {
			return c.createListImport(ctx, in)
		}),
	defineTool("get_list_import", "Get a CSV import",
		"Returns an import with its state, the columns and what each holds, the counts (valid rows and refused rows by reason) and, until it ends, a preview of the first rows. Maps to GET /v1/lists/{id}/imports/{import_id}.",
		readOnly, nil, mcpTool{ops: []string{"GetListImport"}},
		func(ctx context.Context, c *mcpCall, in importIn) (mcpListImport, error) {
			imp, err := c.getListImport(ctx, in.ListID, in.ImportID)
			if err != nil {
				return mcpListImport{}, err
			}
			return listImportOut(imp), nil
		}),
	defineTool("list_list_imports", "List a mailing list's CSV imports",
		"Lists the imports of a mailing list, newest first. Maps to GET /v1/lists/{id}/imports.",
		readOnly, nil, mcpTool{ops: []string{"ListListImports"}},
		func(ctx context.Context, c *mcpCall, in listImportsIn) (mcpPage[mcpListImport], error) {
			res, err := c.s.ListListImports(ctx, oas.ListListImportsRequestObject{ListId: in.ListID, Params: oas.ListListImportsParams{
				Cursor: optString[string](in.Cursor), Limit: optLimit(in.Limit),
			}})
			if err != nil {
				return mcpPage[mcpListImport]{}, err
			}
			page := res.(oas.ListListImports200JSONResponse)
			out := mcpPage[mcpListImport]{Items: []mcpListImport{}}
			for _, imp := range page.Items {
				out.Items = append(out.Items, listImportOut(imp))
			}
			if page.NextCursor != nil {
				out.NextCursor = *page.NextCursor
			}
			return out, nil
		}),
	defineTool("update_list_import_mapping", "Change what an import's columns hold",
		"Sets what each column of a ready import holds (exactly one email column) and validates the file again; read the new counts with get_list_import. Owners, admins and keys only. Maps to PATCH /v1/lists/{id}/imports/{import_id}.",
		idempotent, map[string][]any{"columns.field": importFields}, listTool("UpdateListImport"),
		func(ctx context.Context, c *mcpCall, in updateImportMappingIn) (mcpListImport, error) {
			body := importMapping(in.Columns)
			res, err := c.s.UpdateListImport(ctx, oas.UpdateListImportRequestObject{ListId: in.ListID, ImportId: in.ImportID, Body: &body})
			if err != nil {
				return mcpListImport{}, err
			}
			return listImportOut(oas.ListImport(res.(oas.UpdateListImport200JSONResponse))), nil
		}),
	defineTool("get_list_import_errors", "Get an import's refused rows",
		"Lists the rows an import refused, with the reason; the addresses are in customer_content. Kept 7 days after the import ends. Maps to GET /v1/lists/{id}/imports/{import_id}/errors.",
		readOnly, nil, mcpTool{ops: []string{"GetListImportErrors"}},
		func(ctx context.Context, c *mcpCall, in importErrorsIn) (importErrorsOut, error) {
			return c.importErrors(ctx, in)
		}),
	defineTool("cancel_list_import", "Cancel a CSV import",
		"Cancels an import that is validating, ready or importing and deletes the uploaded file; rows already added stay. Owners, admins and keys only. Maps to POST /v1/lists/{id}/imports/{import_id}/cancel.",
		idempotent, nil, listTool("CancelListImport", "GetListImport"),
		func(ctx context.Context, c *mcpCall, in importIn) (mcpListImport, error) {
			if _, err := c.s.CancelListImport(ctx, oas.CancelListImportRequestObject{ListId: in.ListID, ImportId: in.ImportID}); err != nil {
				return mcpListImport{}, err
			}
			imp, err := c.getListImport(ctx, in.ListID, in.ImportID)
			if err != nil {
				return mcpListImport{}, err
			}
			return listImportOut(imp), nil
		}),
	defineTool("start_list_import", "Start a CSV import",
		"Adds the valid rows of a ready import to the list. confirm_count must be the import's counts.valid that get_list_import showed and the person you work for confirmed; when it differs nothing starts (count_mismatch). With reconfirm, new addresses get a confirmation mail. Owners, admins and keys only. Maps to POST /v1/lists/{id}/imports/{import_id}/start.",
		delivering, nil, listTool("GetListImport", "StartListImport"),
		func(ctx context.Context, c *mcpCall, in startImportIn) (mcpListImport, error) {
			res, err := c.s.startListImport(ctx, oas.StartListImportRequestObject{ListId: in.ListID, ImportId: in.ImportID}, &in.ConfirmCount)
			if err != nil {
				return mcpListImport{}, err
			}
			return listImportOut(oas.ListImport(res.(oas.StartListImport202JSONResponse))), nil
		}),
}
