package api

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/realtime"
)

const (
	mcpChangelogPrefix  = mcpURIPrefix + "changelog/"
	whatsNewDraftedNote = "Saved as a draft. A member publishes it in Yuva; this connection cannot publish, schedule or mail it."
	changelogResultTop  = 100
)

type mcpChangelogRef struct {
	ID            uuid.UUID `json:"id"`
	InboxID       uuid.UUID `json:"inbox_id"`
	Name          string    `json:"name"`
	URL           string    `json:"url" jsonschema:"the public home page"`
	DefaultLocale string    `json:"default_locale"`
	Locales       []string  `json:"locales" jsonschema:"the languages entries can be written in"`
	Published     bool      `json:"published" jsonschema:"false: nobody outside the team sees the changelog yet"`
	Entries       int32     `json:"entries"`
	Scheduled     int32     `json:"scheduled"`
}

func changelogRefMCP(c oas.Changelog) mcpChangelogRef {
	return mcpChangelogRef{
		ID: c.Id, InboxID: c.InboxId, Name: c.Name, URL: c.Url, DefaultLocale: c.DefaultLocale, Locales: c.Locales, Published: c.Published,
		Entries: c.Counts.Entries, Scheduled: c.Counts.Scheduled,
	}
}

type whatsNewScopeIn struct {
	ChangelogID *uuid.UUID `json:"changelog_id,omitempty" jsonschema:"a changelog from list_whats_new_entries; may be left out when you can see only one"`
}

type listWhatsNewIn struct {
	ChangelogID *uuid.UUID `json:"changelog_id,omitempty" jsonschema:"may be left out when you can see only one changelog"`
	State       string     `json:"state,omitempty" jsonschema:"draft, scheduled, published or changed (published with a newer draft)"`
	Label       string     `json:"label,omitempty" jsonschema:"new, improved or fixed"`
	Locale      string     `json:"locale,omitempty"`
	Limit       int32      `json:"limit,omitempty" jsonschema:"1 to 100, default 25"`
	Cursor      string     `json:"cursor,omitempty"`
}

type mcpWhatsNewItem struct {
	ID          uuid.UUID  `json:"id"`
	Number      int32      `json:"number"`
	Labels      []string   `json:"labels"`
	State       string     `json:"state" jsonschema:"draft, scheduled or published"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	Titles      []string   `json:"titles" jsonschema:"one per language: locale, state and title"`
}

type mcpWhatsNewList struct {
	Changelog  mcpChangelogRef   `json:"changelog"`
	Items      []mcpWhatsNewItem `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty" jsonschema:"pass as cursor to get the next page"`
}

type getWhatsNewIn struct {
	ChangelogID *uuid.UUID `json:"changelog_id,omitempty" jsonschema:"may be left out when you can see only one changelog"`
	EntryID     uuid.UUID  `json:"entry_id" jsonschema:"an entry from list_whats_new_entries"`
}

type mcpWhatsNewTranslation struct {
	Locale        string `json:"locale"`
	State         string `json:"state" jsonschema:"draft, published, or changed (published with a newer draft)"`
	Outdated      bool   `json:"outdated,omitempty" jsonschema:"the default language was published after this translation was saved"`
	Version       int32  `json:"version" jsonschema:"send it as base_version to update_whats_new_draft"`
	Title         string `json:"title"`
	Body          string `json:"body" jsonschema:"the draft, in Markdown"`
	PublishedBody string `json:"published_body,omitempty" jsonschema:"what the public sees, in Markdown, when published"`
	URL           string `json:"url,omitempty"`
}

type mcpWhatsNewEntry struct {
	ID             uuid.UUID                `json:"id"`
	ChangelogID    uuid.UUID                `json:"changelog_id"`
	Number         int32                    `json:"number"`
	Labels         []string                 `json:"labels"`
	State          string                   `json:"state" jsonschema:"draft, scheduled or published"`
	PublishAt      *time.Time               `json:"publish_at,omitempty"`
	PublishedAt    *time.Time               `json:"published_at,omitempty"`
	ScheduleFailed string                   `json:"schedule_failed,omitempty" jsonschema:"why the schedule did not publish the entry"`
	Translations   []mcpWhatsNewTranslation `json:"translations"`
}

func whatsNewEntryMCP(e oas.ChangelogEntry) mcpWhatsNewEntry {
	out := mcpWhatsNewEntry{
		ID: e.Id, ChangelogID: e.ChangelogId, Number: e.Number, Labels: labelStrings(e.Labels), State: string(e.State), PublishAt: e.PublishAt,
		PublishedAt: e.PublishedAt, Translations: []mcpWhatsNewTranslation{},
	}
	if e.ScheduleFailed != nil {
		out.ScheduleFailed = string(e.ScheduleFailed.Reason)
	}
	for _, t := range e.Translations {
		item := mcpWhatsNewTranslation{
			Locale: t.Locale, State: string(t.State), Outdated: t.Outdated, Version: t.Version, Title: t.Title, Body: t.Body, URL: deref(t.Url),
		}
		if t.Published != nil {
			item.PublishedBody = t.Published.Body
		}
		out.Translations = append(out.Translations, item)
	}
	return out
}

func labelStrings(in []oas.ChangelogLabel) []string {
	out := make([]string, len(in))
	for i, l := range in {
		out[i] = string(l)
	}
	return out
}

type draftWhatsNewIn struct {
	ChangelogID *uuid.UUID `json:"changelog_id,omitempty" jsonschema:"may be left out when you can see only one changelog"`
	EntryID     *uuid.UUID `json:"entry_id,omitempty" jsonschema:"add a translation to this entry; omit to start a new entry"`
	Locale      string     `json:"locale,omitempty" jsonschema:"the language of this text; the changelog's default locale when empty"`
	Title       string     `json:"title" jsonschema:"3 to 200 characters; what changed, in a few words"`
	Body        string     `json:"body" jsonschema:"Markdown written for customers, up to 50000 characters"`
	Labels      []string   `json:"labels,omitempty" jsonschema:"a new entry only: one to three of new, improved and fixed"`
}

type updateWhatsNewIn struct {
	ChangelogID *uuid.UUID `json:"changelog_id,omitempty" jsonschema:"may be left out when you can see only one changelog"`
	EntryID     uuid.UUID  `json:"entry_id"`
	Locale      string     `json:"locale" jsonschema:"the translation to change"`
	BaseVersion int32      `json:"base_version" jsonschema:"the version from get_whats_new_entry; a draft saved by someone else since is refused"`
	Title       *string    `json:"title,omitempty" jsonschema:"unchanged when absent"`
	Body        *string    `json:"body,omitempty" jsonschema:"unchanged when absent"`
	Labels      []string   `json:"labels,omitempty" jsonschema:"replaces the entry's labels when given"`
}

type mcpWhatsNewDraft struct {
	EntryID     uuid.UUID `json:"entry_id"`
	ChangelogID uuid.UUID `json:"changelog_id"`
	Number      int32     `json:"number"`
	Locale      string    `json:"locale"`
	Version     int32     `json:"version"`
	State       string    `json:"state"`
	Title       string    `json:"title"`
	Note        string    `json:"note"`
}

type mcpChangelogResource struct {
	Changelog mcpChangelogRef   `json:"changelog"`
	Entries   []mcpWhatsNewItem `json:"entries"`
}

// changelogFor finds the changelog a call names, or the only one the caller can see.
func (c *mcpCall) changelogFor(ctx context.Context, id *uuid.UUID) (oas.Changelog, error) {
	if id != nil {
		res, err := c.s.GetChangelog(ctx, oas.GetChangelogRequestObject{ChangelogId: *id})
		if err != nil {
			return oas.Changelog{}, err
		}
		return oas.Changelog(res.(oas.GetChangelog200JSONResponse)), nil
	}
	res, err := c.s.ListChangelogs(ctx, oas.ListChangelogsRequestObject{})
	if err != nil {
		return oas.Changelog{}, err
	}
	items := res.(oas.ListChangelogs200JSONResponse).Items
	switch len(items) {
	case 0:
		return oas.Changelog{}, problem(404, "not_found", "no What's new changelog exists in the inboxes you can see; an owner creates one in Yuva")
	case 1:
		return items[0], nil
	}
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Id.String() + " (" + it.Name + ")"
	}
	return oas.Changelog{}, errValidation("changelog_id is required: you can see " + strings.Join(names, ", "))
}

func whatsNewItemMCP(e oas.ChangelogEntrySummary) mcpWhatsNewItem {
	item := mcpWhatsNewItem{
		ID: e.Id, Number: e.Number, Labels: labelStrings(e.Labels), State: string(e.State), PublishAt: e.PublishAt, PublishedAt: e.PublishedAt, Titles: []string{},
	}
	for _, t := range e.Translations {
		item.Titles = append(item.Titles, t.Locale+" ("+string(t.State)+"): "+t.Title)
	}
	return item
}

func (c *mcpCall) whatsNewDraft(e oas.ChangelogEntry, locale string) (mcpWhatsNewDraft, error) {
	for _, t := range e.Translations {
		if t.Locale == locale {
			return mcpWhatsNewDraft{
				EntryID: e.Id, ChangelogID: e.ChangelogId, Number: e.Number, Locale: t.Locale, Version: t.Version, State: string(t.State),
				Title: t.Title, Note: whatsNewDraftedNote,
			}, nil
		}
	}
	return mcpWhatsNewDraft{}, errInternal
}

func (c *mcpCall) whatsNewEntry(ctx context.Context, changelog, id uuid.UUID) (oas.ChangelogEntry, error) {
	res, err := c.s.GetChangelogEntry(ctx, oas.GetChangelogEntryRequestObject{ChangelogId: changelog, EntryId: id})
	if err != nil {
		return oas.ChangelogEntry{}, err
	}
	return oas.ChangelogEntry(res.(oas.GetChangelogEntry200JSONResponse)), nil
}

func chLabels(in []string) []oas.ChangelogLabel {
	out := make([]oas.ChangelogLabel, len(in))
	for i, l := range in {
		out[i] = oas.ChangelogLabel(l)
	}
	return out
}

type createChangelogIn struct {
	InboxID       uuid.UUID `json:"inbox_id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug" jsonschema:"a-z, 0-9 and inner hyphens, 3 to 48 characters, unique across the server; part of the page's address"`
	Description   string    `json:"description,omitempty" jsonschema:"shown on the changelog's home page"`
	DefaultLocale string    `json:"default_locale,omitempty" jsonschema:"a BCP 47 tag; the inbox's language when empty"`
	Locales       []string  `json:"locales,omitempty" jsonschema:"the languages entries can be written in; the default one is added"`
}

var mcpChangelogTools = []mcpTool{
	defineTool("list_whats_new_entries", "List What's new entries",
		"Lists the entries of a What's new changelog, newest first, with their state, labels and a title per language, and the changelog itself. Look here before drafting a new entry. Maps to GET /v1/changelogs/{id}/entries.",
		readOnly, nil, mcpTool{ops: []string{"ListChangelogs", "GetChangelog", "ListChangelogEntries"}},
		func(ctx context.Context, c *mcpCall, in listWhatsNewIn) (mcpWhatsNewList, error) {
			cl, err := c.changelogFor(ctx, in.ChangelogID)
			if err != nil {
				return mcpWhatsNewList{}, err
			}
			prm := oas.ListChangelogEntriesParams{}
			if in.State != "" {
				prm.State = new(oas.ListChangelogEntriesParamsState(in.State))
			}
			if in.Label != "" {
				prm.Label = new(oas.ChangelogLabel(in.Label))
			}
			if in.Locale != "" {
				prm.Locale = &in.Locale
			}
			if in.Limit != 0 {
				prm.Limit = &in.Limit
			}
			if in.Cursor != "" {
				prm.Cursor = &in.Cursor
			}
			res, err := c.s.ListChangelogEntries(ctx, oas.ListChangelogEntriesRequestObject{ChangelogId: cl.Id, Params: prm})
			if err != nil {
				return mcpWhatsNewList{}, err
			}
			page := oas.ChangelogEntryPage(res.(oas.ListChangelogEntries200JSONResponse))
			out := mcpWhatsNewList{Changelog: changelogRefMCP(cl), Items: []mcpWhatsNewItem{}, NextCursor: deref(page.NextCursor)}
			for _, e := range page.Items {
				out.Items = append(out.Items, whatsNewItemMCP(e))
			}
			return out, nil
		}),
	defineTool("get_whats_new_entry", "Get a What's new entry",
		"A What's new entry with every translation: its draft, its version, and what is published. Maps to GET /v1/changelogs/{id}/entries/{id}.",
		readOnly, nil, mcpTool{ops: []string{"ListChangelogs", "GetChangelog", "GetChangelogEntry"}},
		func(ctx context.Context, c *mcpCall, in getWhatsNewIn) (mcpWhatsNewEntry, error) {
			cl, err := c.changelogFor(ctx, in.ChangelogID)
			if err != nil {
				return mcpWhatsNewEntry{}, err
			}
			e, err := c.whatsNewEntry(ctx, cl.Id, in.EntryID)
			if err != nil {
				return mcpWhatsNewEntry{}, err
			}
			return whatsNewEntryMCP(e), nil
		}),
	defineTool("draft_whats_new_entry", "Draft a What's new entry",
		"Writes a What's new entry as a draft: a new entry with its labels, or with entry_id a new translation of an existing one (in a language it has no translation in yet). Nothing becomes public: a member reviews and publishes it in Yuva, and this connection cannot publish, schedule or mail it. List the entries first so the change is not announced twice. Maps to POST /v1/changelogs/{id}/entries or PUT .../translations/{locale}.",
		additive, nil, mcpTool{ops: []string{"ListChangelogs", "GetChangelog", "CreateChangelogEntry", "PutChangelogEntryTranslation"}},
		func(ctx context.Context, c *mcpCall, in draftWhatsNewIn) (mcpWhatsNewDraft, error) {
			cl, err := c.changelogFor(ctx, in.ChangelogID)
			if err != nil {
				return mcpWhatsNewDraft{}, err
			}
			locale := in.Locale
			if locale == "" {
				locale = cl.DefaultLocale
			}
			if in.EntryID == nil {
				res, err := c.s.CreateChangelogEntry(ctx, oas.CreateChangelogEntryRequestObject{ChangelogId: cl.Id, Body: &oas.ChangelogEntryCreate{
					Locale: locale, Title: in.Title, Body: &in.Body, Labels: chLabels(in.Labels),
				}})
				if err != nil {
					return mcpWhatsNewDraft{}, err
				}
				return c.whatsNewDraft(oas.ChangelogEntry(res.(oas.CreateChangelogEntry201JSONResponse)), locale)
			}
			e, err := c.whatsNewEntry(ctx, cl.Id, *in.EntryID)
			if err != nil {
				return mcpWhatsNewDraft{}, err
			}
			if err := mcpMayEdit(e, false); err != nil {
				return mcpWhatsNewDraft{}, err
			}
			put, err := c.s.PutChangelogEntryTranslation(ctx, oas.PutChangelogEntryTranslationRequestObject{
				ChangelogId: cl.Id, EntryId: *in.EntryID, Locale: locale, Body: &oas.ChangelogTranslationWrite{Title: in.Title, Body: &in.Body},
			})
			if err != nil {
				return mcpWhatsNewDraft{}, err
			}
			return c.whatsNewPutDraft(ctx, cl.Id, *in.EntryID, put)
		}),
	defineTool("update_whats_new_draft", "Update a What's new draft",
		"Changes the draft of one translation of a What's new entry, and its labels when given; fields you leave out stay as they are. base_version must be the version from get_whats_new_entry, so a draft that someone saved since is not overwritten (409 version_conflict). The published text does not change: a member publishes the new draft in Yuva. Maps to PUT /v1/changelogs/{id}/entries/{id}/translations/{locale}.",
		idempotent, nil, mcpTool{ops: []string{"ListChangelogs", "GetChangelog", "GetChangelogEntry", "PutChangelogEntryTranslation", "UpdateChangelogEntry"}},
		func(ctx context.Context, c *mcpCall, in updateWhatsNewIn) (mcpWhatsNewDraft, error) {
			cl, err := c.changelogFor(ctx, in.ChangelogID)
			if err != nil {
				return mcpWhatsNewDraft{}, err
			}
			e, err := c.whatsNewEntry(ctx, cl.Id, in.EntryID)
			if err != nil {
				return mcpWhatsNewDraft{}, err
			}
			if err := mcpMayEdit(e, len(in.Labels) > 0); err != nil {
				return mcpWhatsNewDraft{}, err
			}
			var cur *oas.ChangelogEntryTranslation
			for i := range e.Translations {
				if e.Translations[i].Locale == in.Locale {
					cur = &e.Translations[i]
				}
			}
			if cur == nil {
				return mcpWhatsNewDraft{}, errChangelogTranslation
			}
			write := oas.ChangelogTranslationWrite{BaseVersion: in.BaseVersion, Title: cur.Title, Body: &cur.Body, Slug: new(oas.Slug(cur.Slug))}
			if in.Title != nil {
				write.Title = *in.Title
			}
			if in.Body != nil {
				write.Body = in.Body
			}
			put, err := c.s.PutChangelogEntryTranslation(ctx, oas.PutChangelogEntryTranslationRequestObject{ChangelogId: cl.Id, EntryId: in.EntryID, Locale: in.Locale, Body: &write})
			if err != nil {
				return mcpWhatsNewDraft{}, err
			}
			if len(in.Labels) > 0 {
				labels := chLabels(in.Labels)
				if _, err := c.s.UpdateChangelogEntry(ctx, oas.UpdateChangelogEntryRequestObject{ChangelogId: cl.Id, EntryId: in.EntryID, Body: &oas.ChangelogEntryUpdate{Labels: &labels}}); err != nil {
					return mcpWhatsNewDraft{}, err
				}
			}
			return c.whatsNewPutDraft(ctx, cl.Id, in.EntryID, put)
		}),
	defineTool("create_changelog", "Create a What's new page",
		"Creates a What's new changelog for an inbox's product. It stays unpublished until a member publishes it in Yuva. Then add entries with draft_whats_new_entry. Owners, admins and keys with changelog:publish; not OAuth connections. Maps to POST /v1/changelogs.",
		additive, nil, publicSetupTool("CreateChangelog"),
		func(ctx context.Context, c *mcpCall, in createChangelogIn) (mcpChangelogRef, error) {
			body := oas.ChangelogCreate{InboxId: in.InboxID, Name: in.Name, Slug: in.Slug, Description: optString[string](in.Description),
				DefaultLocale: optString[string](in.DefaultLocale)}
			if in.Locales != nil {
				body.Locales = &in.Locales
			}
			res, err := c.s.CreateChangelog(ctx, oas.CreateChangelogRequestObject{Body: &body})
			if err != nil {
				return mcpChangelogRef{}, err
			}
			return changelogRefMCP(oas.Changelog(res.(oas.CreateChangelog201JSONResponse))), nil
		}),
}

// mcpMayEdit keeps every MCP caller, keys included, to drafts: a scheduled entry goes public as it is,
// and the labels of a published one are public at once.
func mcpMayEdit(e oas.ChangelogEntry, labels bool) error {
	switch {
	case e.State == oas.ChangelogStateScheduled:
		return errChangelogScheduledLocked
	case labels && e.State == oas.ChangelogStatePublished:
		return problem(403, "forbidden", "the labels of a published entry are public at once; a member changes them in Yuva")
	}
	return nil
}

func (c *mcpCall) whatsNewPutDraft(ctx context.Context, changelog, entry uuid.UUID, put oas.PutChangelogEntryTranslationResponseObject) (mcpWhatsNewDraft, error) {
	var t oas.ChangelogEntryTranslation
	switch r := put.(type) {
	case oas.PutChangelogEntryTranslation200JSONResponse:
		t = oas.ChangelogEntryTranslation(r)
	case oas.PutChangelogEntryTranslation201JSONResponse:
		t = oas.ChangelogEntryTranslation(r)
	default:
		return mcpWhatsNewDraft{}, errInternal
	}
	e, err := c.whatsNewEntry(ctx, changelog, entry)
	if err != nil {
		return mcpWhatsNewDraft{}, err
	}
	return c.whatsNewDraft(e, t.Locale)
}

func mayReadChangelogResource(p principal) bool {
	return keyMayCall(p, "GetChangelog") == nil && keyMayCall(p, "ListChangelogEntries") == nil
}

func (c *mcpCall) changelogResource(ctx context.Context, id uuid.UUID) (mcpChangelogResource, error) {
	cl, err := c.changelogFor(ctx, &id)
	if err != nil {
		return mcpChangelogResource{}, err
	}
	out := mcpChangelogResource{Changelog: changelogRefMCP(cl), Entries: []mcpWhatsNewItem{}}
	lim := int32(changelogResultTop)
	res, err := c.s.ListChangelogEntries(ctx, oas.ListChangelogEntriesRequestObject{ChangelogId: id, Params: oas.ListChangelogEntriesParams{Limit: &lim}})
	if err != nil {
		return out, err
	}
	for _, e := range res.(oas.ListChangelogEntries200JSONResponse).Items {
		out.Entries = append(out.Entries, whatsNewItemMCP(e))
	}
	return out, nil
}

func changelogEventChangelog(e realtime.Event) (uuid.UUID, bool) {
	switch e.Type {
	case changelogEventPublished, changelogEventUnpublished, changelogEventDeleted:
		var d struct {
			Changelog struct {
				ID uuid.UUID `json:"id"`
			} `json:"changelog"`
		}
		return d.Changelog.ID, json.Unmarshal(e.Data, &d) == nil
	case changelogSignalUpdated:
		var d struct {
			ChangelogID uuid.UUID `json:"changelog_id"`
		}
		return d.ChangelogID, json.Unmarshal(e.Data, &d) == nil
	}
	return uuid.UUID{}, false
}

func addChangelogPrompts(srv *mcp.Server, c *mcpCall) {
	if !toolsListed(c.p, "list_whats_new_entries", "draft_whats_new_entry") {
		return
	}
	srv.AddPrompt(&mcp.Prompt{
		Name: "draft_release_notes", Title: "Draft release notes",
		Description: "Turn commit subjects or notes you give into What's new entries as drafts, one per change a customer would notice, labelled new, improved or fixed. Nothing is published.",
		Arguments: []*mcp.PromptArgument{
			{Name: "notes", Description: "The commit subjects, changelog lines or notes to write from; paste them here, Yuva fetches nothing.", Required: true},
			{Name: "changelog_id", Description: "The changelog to write in; the only one you can see when empty."},
			{Name: "locale", Description: "The language to write in; the changelog's default when empty."},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		a := req.Params.Arguments
		target := "the changelog " + a["changelog_id"]
		args := " with changelog_id " + a["changelog_id"]
		if a["changelog_id"] == "" {
			target, args = "my What's new changelog", ""
		}
		language := "the changelog's default language"
		if a["locale"] != "" {
			language = a["locale"]
		}
		return promptResult("Draft release notes for "+target,
			"Write What's new entries for "+target+" from these notes:\n\n"+a["notes"],
			"1. Call list_whats_new_entries"+args+" so nothing is announced twice.\n"+
				"2. Keep only changes a customer notices or cares about; drop refactors, dependency bumps and internal work. Group related commits into one entry.\n"+
				"3. For each entry write a title that says what changed in a few words and a short Markdown body in "+language+" that tells a customer what it means for them. Pick the labels: new for something added, improved for something better, fixed for a bug removed. Say nothing the notes do not support.\n"+
				"4. Store each with draft_whats_new_entry"+args+". Do not try to publish or schedule: a member reviews and publishes them in Yuva.",
			"Then list each entry's title, labels and number."), nil
	})
}
