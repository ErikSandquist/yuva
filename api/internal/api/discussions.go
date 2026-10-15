package api

import (
	"bytes"
	"context"
	"html"
	"net/http"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/microcosm-cc/bluemonday"
	"github.com/riverqueue/river"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	discussionEditWindow   = 30 * time.Minute
	maxDiscussionBody      = 20000
	discussionPostsPerMin  = 10
	discussionPostsPerHour = 60

	eventDiscussionCreated     = string(oas.DiscussionEventTypeDiscussionCreated)
	eventDiscussionPostCreated = string(oas.DiscussionEventTypeDiscussionPostCreated)
	eventDiscussionAnswered    = string(oas.DiscussionEventTypeDiscussionAnswered)
	eventDiscussionUpdated     = string(oas.DiscussionEventTypeDiscussionUpdated)

	authorMember    = "member"
	authorContact   = "contact"
	authorAnonymous = "anonymous"

	statePublished = string(oas.DiscussionStatePublished)
	statePending   = string(oas.DiscussionStatePending)
	stateHidden    = string(oas.DiscussionStateHidden)
)

var (
	errDiscussionGone      = problem(http.StatusNotFound, "not_found", "no such discussion")
	errDiscussionPostGone  = problem(http.StatusNotFound, "not_found", "no such post")
	errDiscussionsOff      = problem(http.StatusForbidden, "discussions_off", "discussions are off on this channel")
	errSignInRequired      = problem(http.StatusForbidden, "sign_in_required", "sign in to write in discussions")
	errDiscussionLocked    = problem(http.StatusForbidden, "discussion_locked", "the thread is locked")
	errEditWindowClosed    = problem(http.StatusForbidden, "edit_window_closed", "posts can be edited for 30 minutes after writing them")
	errNotPostAuthor       = problem(http.StatusForbidden, "forbidden", "only its author may do this")
	errOwnPostVote         = problem(http.StatusForbidden, "forbidden", "you cannot vote for your own post")
	errNoDiscussionChannel = problem(http.StatusConflict, "no_discussion_channel", "no chat channel of the inbox serves the page with discussions on")
	errAlreadyDiscussed    = problem(http.StatusConflict, "already_discussed", "this conversation is already a discussion")
	errNotAnswerable       = errValidation("the answer must be a published reply to the thread itself")
)

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.Linkify),
	goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(imagesAsLinks{}, 100))),
)

// imagesAsLinks turns every image into a link to it, so opening a post never fetches a URL its
// author chose.
type imagesAsLinks struct{}

func (imagesAsLinks) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	var images []*ast.Image
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if img, ok := n.(*ast.Image); ok && entering {
			images = append(images, img)
		}
		return ast.WalkContinue, nil
	})
	for _, img := range images {
		parent := img.Parent()
		if insideLink(img) {
			for c := img.FirstChild(); c != nil; {
				next := c.NextSibling()
				parent.InsertBefore(parent, img, c)
				c = next
			}
			parent.RemoveChild(parent, img)
			continue
		}
		link := ast.NewLink()
		link.Destination, link.Title = img.Destination, img.Title
		for c := img.FirstChild(); c != nil; {
			next := c.NextSibling()
			link.AppendChild(link, c)
			c = next
		}
		if !link.HasChildren() {
			link.AppendChild(link, ast.NewString(img.Destination))
		}
		parent.ReplaceChild(parent, img, link)
	}
}

func insideLink(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if _, ok := p.(*ast.Link); ok {
			return true
		}
	}
	return false
}

var discussionHTML = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

// renderMarkdown turns a post into HTML any client can show: goldmark drops raw HTML, bluemonday
// keeps only safe markup.
func renderMarkdown(src string) string {
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(src), &buf); err != nil {
		return "<p>" + html.EscapeString(src) + "</p>"
	}
	return discussionHTML.Sanitize(buf.String())
}

var plainText = bluemonday.StrictPolicy().AddSpaceWhenStrippingTag(true)

// postExcerpt is the start of a post as plain text: the text of its rendered HTML, whitespace
// collapsed, cut at a word boundary.
func postExcerpt(body string) string {
	text := strings.Join(strings.Fields(html.UnescapeString(plainText.Sanitize(renderMarkdown(body)))), " ")
	r := []rune(text)
	if len(r) <= excerptRunes {
		return text
	}
	cut := string(r[:excerptRunes])
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

func discussionTitle(v string) (string, error) {
	return trimmed(strings.Join(strings.Fields(v), " "), 3, 200, "title")
}

func discussionBodyText(v string) (string, error) {
	return trimmed(v, 1, maxDiscussionBody, "body")
}

// actor is who writes: a member, or a contact (a reader or an identity-token user).
type actor struct {
	memberID  *uuid.UUID
	contactID *uuid.UUID
}

func (a actor) owns(p store.DiscussionPostView) bool {
	if a.memberID != nil {
		return p.AuthorMemberID != nil && *p.AuthorMemberID == *a.memberID
	}
	return a.contactID != nil && p.AuthorContactID != nil && *p.AuthorContactID == *a.contactID
}

func (a actor) authorType() string {
	if a.memberID != nil {
		return authorMember
	}
	return authorContact
}

// viewer decides what of a thread a caller sees and which flags they get.
type viewer struct {
	actor
	team      bool
	client    bool
	voted     map[uuid.UUID]bool
	following map[uuid.UUID]bool
}

func (v *viewer) sees(p store.DiscussionPostView) bool {
	switch {
	case v.team:
		return true
	case p.State == statePublished:
		return true
	case p.State == statePending:
		return v.owns(p)
	}
	return false
}

func discussionAuthor(p store.DiscussionPostView, team bool) oas.DiscussionAuthor {
	out := oas.DiscussionAuthor{Kind: oas.DiscussionAuthorKindAnonymous}
	switch p.AuthorType {
	case authorMember:
		out.Kind, out.Deleted = oas.DiscussionAuthorKindMember, p.AuthorMemberID == nil
		if team {
			out.MemberId = p.AuthorMemberID
		}
	case authorContact:
		out.Kind, out.Deleted = oas.DiscussionAuthorKindReader, p.AuthorContactID == nil
		if team {
			out.ContactId = p.AuthorContactID
		}
	}
	if !out.Deleted && p.AuthorType != authorAnonymous && strings.TrimSpace(p.AuthorName) != "" {
		name := strings.TrimSpace(p.AuthorName)
		out.Name, out.Initials = &name, initials(name)
	}
	return out
}

func discussionBody(d store.Discussion, op store.DiscussionPostView, v *viewer) oas.Discussion {
	out := oas.Discussion{
		Id: d.ID, Page: d.Page, Title: d.Title, OpeningPostId: d.OpeningPostID, Body: op.Body, BodyHtml: renderMarkdown(op.Body),
		Author: discussionAuthor(op, v.team), State: oas.DiscussionState(d.State), AnswerPostId: d.AnswerPostID,
		AnsweredAt: d.AnsweredAt, Locked: d.LockedAt != nil, Votes: op.Votes, ReplyCount: d.ReplyCount,
		CreatedAt: d.CreatedAt, EditedAt: op.EditedAt, LastActivityAt: d.LastActivityAt,
	}
	if d.PageTitle != "" {
		t := d.PageTitle
		out.PageTitle = &t
	}
	if op.DeletedAt != nil {
		deleted := true
		out.Deleted = &deleted
	}
	if v.team {
		inbox, channel, pending, hidden := d.InboxID, d.ChannelID, d.PendingPosts, d.HiddenPosts
		out.InboxId, out.ChannelId, out.PendingPosts, out.HiddenPosts, out.ConversationId = &inbox, &channel, &pending, &hidden, d.ConversationID
	}
	if v.client {
		mine, voted, following := v.owns(op), v.voted[op.ID], v.following[d.ID]
		out.Mine, out.Voted, out.Following = &mine, &voted, &following
	}
	return out
}

func postBody(p store.DiscussionPostView, answer *uuid.UUID, v *viewer) oas.DiscussionPost {
	out := oas.DiscussionPost{
		Id: p.ID, DiscussionId: p.DiscussionID, ParentPostId: p.ParentPostID, Opening: p.Opening, Body: p.Body,
		BodyHtml: renderMarkdown(p.Body), Author: discussionAuthor(p, v.team), State: oas.DiscussionState(p.State),
		Votes: p.Votes, Accepted: answer != nil && *answer == p.ID, Deleted: p.DeletedAt != nil,
		CreatedAt: p.CreatedAt, EditedAt: p.EditedAt,
	}
	if v.client {
		mine, voted := v.owns(p), v.voted[p.ID]
		out.Mine, out.Voted = &mine, &voted
	}
	return out
}

// thread is a discussion with every post, the opening one included.
type thread struct {
	d     store.Discussion
	posts []store.DiscussionPostView
}

func (t thread) opening() store.DiscussionPostView {
	for _, p := range t.posts {
		if p.ID == t.d.OpeningPostID {
			return p
		}
	}
	return store.DiscussionPostView{ID: t.d.OpeningPostID, AuthorType: authorAnonymous, State: t.d.State}
}

func (t thread) post(id uuid.UUID) (store.DiscussionPostView, bool) {
	for _, p := range t.posts {
		if p.ID == id {
			return p, true
		}
	}
	return store.DiscussionPostView{}, false
}

func loadThread(ctx context.Context, q *store.Queries, ws, id uuid.UUID, lock bool) (thread, error) {
	get := q.GetDiscussion
	if lock {
		get = func(ctx context.Context, arg store.GetDiscussionParams) (store.Discussion, error) {
			return q.LockDiscussion(ctx, store.LockDiscussionParams(arg))
		}
	}
	d, err := get(ctx, store.GetDiscussionParams{WorkspaceID: ws, ID: id})
	if store.IsNotFound(err) {
		return thread{}, errDiscussionGone
	}
	if err != nil {
		return thread{}, err
	}
	posts, err := q.ListDiscussionPosts(ctx, store.ListDiscussionPostsParams{WorkspaceID: ws, DiscussionID: id})
	return thread{d: d, posts: posts}, err
}

// detail is the thread as v sees it: the accepted answer first, then oldest first.
func (t thread) detail(v *viewer) oas.DiscussionDetail {
	out := oas.DiscussionDetail{Discussion: discussionBody(t.d, t.opening(), v), Posts: []oas.DiscussionPost{}}
	var rest []oas.DiscussionPost
	for _, p := range t.posts {
		if p.Opening || !v.sees(p) {
			continue
		}
		if p.DeletedAt != nil && !t.hasVisibleReplies(p.ID, v) {
			continue
		}
		b := postBody(p, t.d.AnswerPostID, v)
		if b.Accepted {
			out.Posts = append(out.Posts, b)
		} else {
			rest = append(rest, b)
		}
	}
	out.Posts = append(out.Posts, rest...)
	return out
}

func (t thread) hasVisibleReplies(id uuid.UUID, v *viewer) bool {
	for _, p := range t.posts {
		if p.ParentPostID != nil && *p.ParentPostID == id && p.DeletedAt == nil && v.sees(p) {
			return true
		}
	}
	return false
}

// fillViewer loads the caller's votes and follows for the threads shown.
func fillViewer(ctx context.Context, q *store.Queries, ws uuid.UUID, v *viewer, ids []uuid.UUID) error {
	v.voted, v.following = map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	if v.contactID == nil && v.memberID == nil {
		return nil
	}
	voted, err := q.ListVoterPosts(ctx, store.ListVoterPostsParams{WorkspaceID: ws, DiscussionIds: ids, ContactID: v.contactID, MemberID: v.memberID})
	if err != nil {
		return err
	}
	for _, id := range voted {
		v.voted[id] = true
	}
	if v.contactID == nil {
		return nil
	}
	followed, err := q.ListFollowedDiscussions(ctx, store.ListFollowedDiscussionsParams{WorkspaceID: ws, ContactID: *v.contactID, DiscussionIds: ids})
	for _, id := range followed {
		v.following[id] = true
	}
	return err
}

// discussionsBody turns discussion rows into bodies with their opening posts.
func discussionsBody(ctx context.Context, q *store.Queries, ws uuid.UUID, rows []store.Discussion, v *viewer) ([]oas.Discussion, error) {
	ids, openings := make([]uuid.UUID, len(rows)), make([]uuid.UUID, len(rows))
	for i, d := range rows {
		ids[i], openings[i] = d.ID, d.OpeningPostID
	}
	ops, err := q.ListOpeningPosts(ctx, store.ListOpeningPostsParams{WorkspaceID: ws, Ids: openings})
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]store.DiscussionPostView, len(ops))
	for _, p := range ops {
		byID[p.ID] = p
	}
	if v.client {
		if err := fillViewer(ctx, q, ws, v, ids); err != nil {
			return nil, err
		}
	}
	out := make([]oas.Discussion, len(rows))
	for i, d := range rows {
		op, ok := byID[d.OpeningPostID]
		if !ok {
			op = store.DiscussionPostView{ID: d.OpeningPostID, AuthorType: authorAnonymous, State: d.State}
		}
		out[i] = discussionBody(d, op, v)
	}
	return out, nil
}

var teamViewer = viewer{team: true}

// discussionEvent records a stored event for members and webhooks with the thread as the team
// sees it and, when given, one post.
func discussionEvent(ctx context.Context, q *store.Queries, events *eventBatch, typ string, ws, id uuid.UUID, postID *uuid.UUID) error {
	return discussionEventWith(ctx, q, events, typ, ws, id, postID, nil)
}

func discussionEventWith(ctx context.Context, q *store.Queries, events *eventBatch, typ string, ws, id uuid.UUID, postID *uuid.UUID, change func(*oas.DiscussionEventData)) error {
	t, err := loadThread(ctx, q, ws, id, false)
	if err != nil {
		return err
	}
	data := oas.DiscussionEventData{Discussion: discussionBody(t.d, t.opening(), &teamViewer)}
	if postID != nil {
		if p, ok := t.post(*postID); ok {
			b := postBody(p, t.d.AnswerPostID, &teamViewer)
			data.Post = &b
		}
	}
	if change != nil {
		change(&data)
	}
	inbox := t.d.InboxID
	events.add(typ, &inbox, nil, data)
	return nil
}

func recount(ctx context.Context, q *store.Queries, ws uuid.UUID, ids ...uuid.UUID) error {
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, err := q.RecountDiscussion(ctx, store.RecountDiscussionParams{WorkspaceID: ws, ID: id}); err != nil && !store.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// newPostState is the state a new post starts in under the channel's setting.
func newPostState(ctx context.Context, q *store.Queries, ws, inbox uuid.UUID, mode string, a actor, identified bool) (string, error) {
	if a.memberID != nil {
		return statePublished, nil
	}
	st, err := q.GetContactDiscussionState(ctx, store.GetContactDiscussionStateParams{WorkspaceID: ws, ID: *a.contactID, InboxID: inbox})
	if err != nil {
		return "", err
	}
	switch {
	case st.DiscussionsBlockedAt != nil:
		return statePending, nil
	case mode == string(oas.DiscussionModeOpen), identified, st.Identified, st.Trusted:
		return statePublished, nil
	}
	return statePending, nil
}

type newThread struct {
	ws, inbox, channel uuid.UUID
	page, pageTitle    string
	title, body        string
	author             actor
	state              string
	conversation       *uuid.UUID
}

func (s *Server) createThread(ctx context.Context, q *store.Queries, events *eventBatch, n newThread) (store.Discussion, error) {
	now := s.now()
	id, opening := newID(), newID()
	d, err := q.CreateDiscussion(ctx, store.CreateDiscussionParams{
		ID: id, WorkspaceID: n.ws, InboxID: n.inbox, ChannelID: n.channel, Page: n.page, PageTitle: n.pageTitle, Title: n.title,
		OpeningPostID: opening, State: n.state, ConversationID: n.conversation, Now: now,
	})
	if err != nil {
		return d, err
	}
	if _, err := q.CreateDiscussionPost(ctx, store.CreateDiscussionPostParams{
		ID: opening, WorkspaceID: n.ws, DiscussionID: id, Opening: true, AuthorType: n.author.authorType(),
		AuthorMemberID: n.author.memberID, AuthorContactID: n.author.contactID, Body: n.body, State: n.state, Now: now,
	}); err != nil {
		return d, err
	}
	if d, err = q.RecountDiscussion(ctx, store.RecountDiscussionParams{WorkspaceID: n.ws, ID: id}); err != nil {
		return d, err
	}
	if n.author.contactID != nil {
		if err := q.FollowDiscussion(ctx, store.FollowDiscussionParams{WorkspaceID: n.ws, DiscussionID: id, ContactID: *n.author.contactID, Now: now}); err != nil {
			return d, err
		}
	}
	return d, discussionEvent(ctx, q, events, eventDiscussionCreated, n.ws, id, &opening)
}

// addReply writes a reply to a thread; the caller has checked who may write and the lock.
func (s *Server) addReply(ctx context.Context, q *store.Queries, events *eventBatch, t thread, a actor, body string, parent *uuid.UUID, state string) (uuid.UUID, error) {
	ws, now := t.d.WorkspaceID, s.now()
	if parent != nil {
		pp, ok := t.post(*parent)
		if !ok || pp.Opening || pp.ParentPostID != nil {
			return uuid.Nil(), errValidation("parent_post_id must be a reply to the thread itself")
		}
	}
	id, err := q.CreateDiscussionPost(ctx, store.CreateDiscussionPostParams{
		ID: newID(), WorkspaceID: ws, DiscussionID: t.d.ID, ParentPostID: parent, AuthorType: a.authorType(),
		AuthorMemberID: a.memberID, AuthorContactID: a.contactID, Body: body, State: state, Now: now,
	})
	if err != nil {
		return id, err
	}
	if err := recount(ctx, q, ws, t.d.ID); err != nil {
		return id, err
	}
	if a.contactID != nil {
		if err := q.FollowDiscussion(ctx, store.FollowDiscussionParams{WorkspaceID: ws, DiscussionID: t.d.ID, ContactID: *a.contactID, Now: now}); err != nil {
			return id, err
		}
	}
	if err := discussionEvent(ctx, q, events, eventDiscussionPostCreated, ws, t.d.ID, &id); err != nil {
		return id, err
	}
	if state == statePublished {
		return id, s.notifyFollowers(ctx, q, events, ws, t.d.ID, id, notifyReply, a.contactID)
	}
	return id, nil
}

// setAnswer marks or clears the accepted answer.
func (s *Server) setAnswer(ctx context.Context, q *store.Queries, events *eventBatch, t thread, postID *uuid.UUID, by actor) error {
	ws := t.d.WorkspaceID
	if postID == nil {
		if t.d.AnswerPostID == nil {
			return nil
		}
		if err := q.SetDiscussionAnswer(ctx, store.SetDiscussionAnswerParams{WorkspaceID: ws, ID: t.d.ID}); err != nil {
			return err
		}
		return discussionEvent(ctx, q, events, eventDiscussionUpdated, ws, t.d.ID, nil)
	}
	p, ok := t.post(*postID)
	if !ok || p.Opening || p.ParentPostID != nil || p.State != statePublished || p.DeletedAt != nil {
		return errNotAnswerable
	}
	if t.d.AnswerPostID != nil && *t.d.AnswerPostID == p.ID {
		return nil
	}
	now := s.now()
	if err := q.SetDiscussionAnswer(ctx, store.SetDiscussionAnswerParams{WorkspaceID: ws, ID: t.d.ID, AnswerPostID: &p.ID, AnsweredAt: &now}); err != nil {
		return err
	}
	if err := discussionEvent(ctx, q, events, eventDiscussionAnswered, ws, t.d.ID, &p.ID); err != nil {
		return err
	}
	return s.notifyFollowers(ctx, q, events, ws, t.d.ID, p.ID, notifyAnswer, by.contactID)
}

// editPost changes a post's body and, on the opening post, the thread's title.
func (s *Server) editPost(ctx context.Context, q *store.Queries, events *eventBatch, t thread, p store.DiscussionPostView, body, title *string) error {
	ws := t.d.WorkspaceID
	if title != nil && !p.Opening {
		return errValidation("title can be changed only on the opening post")
	}
	if p.DeletedAt != nil {
		return errDiscussionPostGone
	}
	if body != nil {
		b, err := discussionBodyText(*body)
		if err != nil {
			return err
		}
		if err := q.EditDiscussionPost(ctx, store.EditDiscussionPostParams{WorkspaceID: ws, ID: p.ID, Body: b, Now: s.now()}); err != nil {
			return err
		}
	}
	if title != nil {
		tt, err := discussionTitle(*title)
		if err != nil {
			return err
		}
		if err := q.SetDiscussionTitle(ctx, store.SetDiscussionTitleParams{WorkspaceID: ws, ID: t.d.ID, Title: tt}); err != nil {
			return err
		}
	}
	return discussionEvent(ctx, q, events, eventDiscussionUpdated, ws, t.d.ID, &p.ID)
}

// deletePost removes an author's post: a post with replies stays as deleted, an opening post
// takes the thread with it when nobody else wrote there.
func (s *Server) deletePost(ctx context.Context, q *store.Queries, events *eventBatch, t thread, p store.DiscussionPostView) error {
	ws := t.d.WorkspaceID
	if p.Opening {
		others, err := q.DiscussionHasOtherPosts(ctx, store.DiscussionHasOtherPostsParams{WorkspaceID: ws, DiscussionID: t.d.ID})
		if err != nil {
			return err
		}
		if !others {
			removed := func(d *oas.DiscussionEventData) {
				deleted := true
				d.Discussion.Deleted, d.Discussion.State = &deleted, oas.DiscussionStateHidden
			}
			if err := discussionEventWith(ctx, q, events, eventDiscussionUpdated, ws, t.d.ID, nil, removed); err != nil {
				return err
			}
			return q.DeleteDiscussion(ctx, store.DeleteDiscussionParams{WorkspaceID: ws, ID: t.d.ID})
		}
		if err := q.SoftDeleteDiscussionPost(ctx, store.SoftDeleteDiscussionPostParams{WorkspaceID: ws, ID: p.ID, Now: s.now()}); err != nil {
			return err
		}
	} else {
		replies, err := q.DiscussionPostHasReplies(ctx, store.DiscussionPostHasRepliesParams{WorkspaceID: ws, ID: p.ID})
		if err != nil {
			return err
		}
		if t.d.AnswerPostID != nil && *t.d.AnswerPostID == p.ID {
			if err := q.SetDiscussionAnswer(ctx, store.SetDiscussionAnswerParams{WorkspaceID: ws, ID: t.d.ID}); err != nil {
				return err
			}
		}
		if replies {
			err = q.SoftDeleteDiscussionPost(ctx, store.SoftDeleteDiscussionPostParams{WorkspaceID: ws, ID: p.ID, Now: s.now()})
		} else {
			err = q.DeleteDiscussionPost(ctx, store.DeleteDiscussionPostParams{WorkspaceID: ws, ID: p.ID})
		}
		if err != nil {
			return err
		}
	}
	if err := recount(ctx, q, ws, t.d.ID); err != nil {
		return err
	}
	return discussionEvent(ctx, q, events, eventDiscussionUpdated, ws, t.d.ID, nil)
}

// moderate approves or hides a post (the thread with its opening post).
func (s *Server) moderate(ctx context.Context, q *store.Queries, events *eventBatch, t thread, p store.DiscussionPostView, state string) error {
	ws := t.d.WorkspaceID
	if p.State == state {
		return nil
	}
	if err := q.SetDiscussionPostState(ctx, store.SetDiscussionPostStateParams{WorkspaceID: ws, ID: p.ID, State: state}); err != nil {
		return err
	}
	if state != statePublished && t.d.AnswerPostID != nil && *t.d.AnswerPostID == p.ID {
		if err := q.SetDiscussionAnswer(ctx, store.SetDiscussionAnswerParams{WorkspaceID: ws, ID: t.d.ID}); err != nil {
			return err
		}
	}
	if err := recount(ctx, q, ws, t.d.ID); err != nil {
		return err
	}
	if err := discussionEvent(ctx, q, events, eventDiscussionUpdated, ws, t.d.ID, &p.ID); err != nil {
		return err
	}
	if state == statePublished && p.State == statePending && !p.Opening && p.DeletedAt == nil {
		return s.notifyFollowers(ctx, q, events, ws, t.d.ID, p.ID, notifyReply, p.AuthorContactID)
	}
	return nil
}

// markSpam hides a contact's post and their other pending posts, and makes their later posts wait.
func (s *Server) markSpam(ctx context.Context, q *store.Queries, events *eventBatch, t thread, p store.DiscussionPostView) error {
	if p.AuthorType != authorContact || p.AuthorContactID == nil {
		return errValidation("only a reader's post can be marked spam")
	}
	ws := t.d.WorkspaceID
	if err := s.moderate(ctx, q, events, t, p, stateHidden); err != nil {
		return err
	}
	if err := q.BlockContactDiscussions(ctx, store.BlockContactDiscussionsParams{WorkspaceID: ws, ID: *p.AuthorContactID, Now: s.now()}); err != nil {
		return err
	}
	others, err := q.HidePendingPostsOfContact(ctx, store.HidePendingPostsOfContactParams{WorkspaceID: ws, ContactID: *p.AuthorContactID})
	if err != nil {
		return err
	}
	if err := recount(ctx, q, ws, others...); err != nil {
		return err
	}
	for _, id := range uniqueIDs(others) {
		if id == t.d.ID {
			continue
		}
		if err := discussionEvent(ctx, q, events, eventDiscussionUpdated, ws, id, nil); err != nil {
			return err
		}
	}
	return nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// vote adds or takes back a vote; it answers the post as the voter sees it afterwards.
func vote(ctx context.Context, q *store.Queries, ws uuid.UUID, p store.DiscussionPostView, a actor, add bool, now time.Time) error {
	if add {
		if a.owns(p) {
			return errOwnPostVote
		}
		_, err := q.AddDiscussionVote(ctx, store.AddDiscussionVoteParams{WorkspaceID: ws, PostID: p.ID, ContactID: a.contactID, MemberID: a.memberID, Now: now})
		return err
	}
	_, err := q.RemoveDiscussionVote(ctx, store.RemoveDiscussionVoteParams{WorkspaceID: ws, PostID: p.ID, ContactID: a.contactID, MemberID: a.memberID})
	return err
}

// postView reloads one post with its vote count, as v sees it.
func postView(ctx context.Context, q *store.Queries, ws, id uuid.UUID, v *viewer) (oas.DiscussionPost, error) {
	p, err := q.GetDiscussionPost(ctx, store.GetDiscussionPostParams{WorkspaceID: ws, ID: id})
	if store.IsNotFound(err) {
		return oas.DiscussionPost{}, errDiscussionPostGone
	}
	if err != nil {
		return oas.DiscussionPost{}, err
	}
	d, err := q.GetDiscussion(ctx, store.GetDiscussionParams{WorkspaceID: ws, ID: p.DiscussionID})
	if err != nil {
		return oas.DiscussionPost{}, err
	}
	if v.client {
		if err := fillViewer(ctx, q, ws, v, []uuid.UUID{d.ID}); err != nil {
			return oas.DiscussionPost{}, err
		}
	}
	return postBody(p, d.AnswerPostID, v), nil
}

func threadView(ctx context.Context, q *store.Queries, ws, id uuid.UUID, v *viewer) (oas.Discussion, error) {
	t, err := loadThread(ctx, q, ws, id, false)
	if err != nil {
		return oas.Discussion{}, err
	}
	if v.client {
		if err := fillViewer(ctx, q, ws, v, []uuid.UUID{id}); err != nil {
			return oas.Discussion{}, err
		}
	}
	return discussionBody(t.d, t.opening(), v), nil
}

const (
	notifyReply  = "reply"
	notifyAnswer = "answer"
)

// notifyFollowers tells a thread's followers about a published reply or an accepted answer:
// readers get a notification row, every follower one e-mail per delay.
func (s *Server) notifyFollowers(ctx context.Context, q *store.Queries, events *eventBatch, ws, discussionID, postID uuid.UUID, kind string, skip *uuid.UUID) error {
	followers, err := q.ListDiscussionFollowers(ctx, store.ListDiscussionFollowersParams{WorkspaceID: ws, DiscussionID: discussionID})
	if err != nil {
		return err
	}
	p, err := q.GetDiscussionPost(ctx, store.GetDiscussionPostParams{WorkspaceID: ws, ID: postID})
	if err != nil {
		return err
	}
	now := s.now()
	for _, f := range followers {
		if (skip != nil && f.ContactID == *skip) || (p.AuthorContactID != nil && f.ContactID == *p.AuthorContactID && kind == notifyReply) {
			continue
		}
		if f.PersonID != nil {
			post := postID
			if err := q.CreateReaderNotification(ctx, store.CreateReaderNotificationParams{
				ID: newID(), WorkspaceID: ws, PersonID: *f.PersonID, DiscussionID: discussionID, PostID: &post, Kind: kind, Now: now,
			}); err != nil {
				return err
			}
		}
		events.jobOnce(DiscussionDigestArgs{WorkspaceID: ws, DiscussionID: discussionID, ContactID: f.ContactID}, s.digestOpts(now.Add(s.discussionDelay())))
	}
	return nil
}

func (s *Server) digestOpts(at time.Time) *river.InsertOpts {
	return &river.InsertOpts{ScheduledAt: at, UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: s.discussionDelay()}}
}

func (s *Server) discussionDelay() time.Duration {
	if s.chat.DiscussionEmailDelay > 0 {
		return s.chat.DiscussionEmailDelay
	}
	return defaultDiscussionEmailDelay
}

// moveDiscussions hands a merged contact's posts, votes, follows and reader account to the
// contact that remains.
func moveDiscussions(ctx context.Context, q *store.Queries, ws, from, to uuid.UUID) error {
	if err := q.MoveDiscussionAuthorship(ctx, store.MoveDiscussionAuthorshipParams{WorkspaceID: ws, FromContact: &from, ToContact: &to}); err != nil {
		return err
	}
	if err := q.MoveDiscussionVotes(ctx, store.MoveDiscussionVotesParams{WorkspaceID: ws, FromContact: &from, ToContact: &to}); err != nil {
		return err
	}
	if err := q.MoveDiscussionFollows(ctx, store.MoveDiscussionFollowsParams{WorkspaceID: ws, FromContact: from, ToContact: to}); err != nil {
		return err
	}
	person, err := q.GetContactDiscussionState(ctx, store.GetContactDiscussionStateParams{WorkspaceID: ws, ID: from, InboxID: uuid.Nil()})
	if err != nil || person.PersonID == nil {
		return err
	}
	if err := q.ClearContactPerson(ctx, store.ClearContactPersonParams{WorkspaceID: ws, ID: from}); err != nil {
		return err
	}
	return q.SetContactPersonIfUnset(ctx, store.SetContactPersonIfUnsetParams{WorkspaceID: ws, ID: to, PersonID: person.PersonID})
}
