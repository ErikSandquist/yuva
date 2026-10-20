package api

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"
	"uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/productdevbook/yuva/api/internal/oas"
	"github.com/productdevbook/yuva/api/internal/realtime"
	"github.com/productdevbook/yuva/api/internal/store"
)

const (
	mcpRoadmapPrefix   = mcpURIPrefix + "roadmap/"
	roadmapResourceTop = 100
)

var errConfirmMismatch = func(stage string, want int32) *apiError {
	return problem(409, "confirm_mismatch", "moving to "+stage+" changes the progress of "+strconv.Itoa(int(want))+
		" linked feedback; call get_roadmap_idea, show that count to the person you work for, and pass it as confirm_linked")
}

type mcpIdeaContent struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty" jsonschema:"Markdown as the reader wrote it"`
	// Comments are given by get_roadmap_idea only.
	Comments []mcpIdeaComment `json:"comments,omitempty" jsonschema:"published replies, oldest first"`
}

type mcpIdea struct {
	ID              uuid.UUID      `json:"id"`
	BoardID         uuid.UUID      `json:"board_id"`
	Stage           string         `json:"stage" jsonschema:"ideas, planned, in_progress, shipped or closed"`
	State           string         `json:"state" jsonschema:"published, pending (waits for a member) or hidden"`
	Votes           int32          `json:"votes"`
	Replies         int32          `json:"replies"`
	LinkedFeedback  int32          `json:"linked_feedback" jsonschema:"feedback conversations linked to the idea"`
	CreatedAt       time.Time      `json:"created_at"`
	StageChangedAt  *time.Time     `json:"stage_changed_at,omitempty"`
	CustomerContent mcpIdeaContent `json:"customer_content"`
}

func ideaOutMCP(d oas.Discussion, body bool) mcpIdea {
	out := mcpIdea{
		ID: d.Id, Votes: d.Votes, Replies: d.ReplyCount, State: string(d.State), CreatedAt: d.CreatedAt, StageChangedAt: d.StageChangedAt,
		CustomerContent: mcpIdeaContent{Title: d.Title},
	}
	if d.BoardId != nil {
		out.BoardID = *d.BoardId
	}
	if d.Stage != nil {
		out.Stage = string(*d.Stage)
	}
	if d.LinkedCount != nil {
		out.LinkedFeedback = *d.LinkedCount
	}
	if body {
		out.CustomerContent.Body = d.Body
	} else {
		out.CustomerContent.Body = postExcerpt(d.Body)
	}
	return out
}

type listIdeasIn struct {
	BoardID *uuid.UUID `json:"board_id,omitempty" jsonschema:"only this roadmap board's ideas"`
	Stage   string     `json:"stage,omitempty"`
	Query   string     `json:"query,omitempty" jsonschema:"searches the titles: one to eight words, the last may be the start of a word"`
	State   string     `json:"state,omitempty" jsonschema:"published, pending or hidden; every state when empty"`
	Sort    string     `json:"sort,omitempty" jsonschema:"votes (default), new or activity"`
	Cursor  string     `json:"cursor,omitempty"`
	Limit   int32      `json:"limit,omitempty" jsonschema:"1 to 100, default 25"`
}

type ideaIn struct {
	IdeaID uuid.UUID `json:"idea_id" jsonschema:"an idea from list_roadmap_ideas"`
}

type ideaLinkIn struct {
	IdeaID         uuid.UUID `json:"idea_id"`
	ConversationID uuid.UUID `json:"conversation_id" jsonschema:"a feedback conversation of the idea's inbox"`
}

type mcpIdeaLink struct {
	IdeaID         uuid.UUID `json:"idea_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Linked         bool      `json:"linked"`
}

type mcpIdeaMove struct {
	Stage    string `json:"stage"`
	Date     string `json:"date,omitempty"`
	Progress string `json:"progress,omitempty" jsonschema:"the progress the move set on the linked feedback"`
}

type mcpWouldChange struct {
	Planned    int32 `json:"planned" jsonschema:"linked feedback a move to planned would change"`
	InProgress int32 `json:"in_progress"`
	Shipped    int32 `json:"shipped"`
	Closed     int32 `json:"closed"`
	Ideas      int32 `json:"ideas" jsonschema:"always 0: a move back to ideas sets no progress"`
}

type mcpIdeaComment struct {
	Author string `json:"author,omitempty"`
	Text   string `json:"text"`
}

type mcpIdeaDetail struct {
	mcpIdea
	History     []mcpIdeaChange `json:"history" jsonschema:"stage moves, newest first"`
	WouldChange mcpWouldChange  `json:"would_change" jsonschema:"for each stage, how many linked feedback a move there would change; pass it as confirm_linked to move_idea_stage"`
}

type mcpIdeaChange struct {
	From        string    `json:"from"`
	To          string    `json:"to"`
	At          time.Time `json:"at"`
	Linked      int32     `json:"linked"`
	ProgressSet int32     `json:"progress_set"`
}

type moveIdeaIn struct {
	IdeaID        uuid.UUID `json:"idea_id"`
	Stage         string    `json:"stage" jsonschema:"ideas, planned, in_progress, shipped or closed"`
	ConfirmLinked int32     `json:"confirm_linked" jsonschema:"the would_change count get_roadmap_idea showed for this stage, which the person you work for has seen"`
}

type mcpIdeaMoved struct {
	IdeaID   uuid.UUID `json:"idea_id"`
	From     string    `json:"from,omitempty"`
	Stage    string    `json:"stage"`
	Moved    bool      `json:"moved" jsonschema:"false when the idea was already in that stage"`
	Linked   int32     `json:"linked" jsonschema:"feedback linked to the idea"`
	Progress string    `json:"progress,omitempty" jsonschema:"the progress being set on the linked feedback; customers see it within a minute"`
}

type mcpRoadmapBoard struct {
	ID      uuid.UUID              `json:"id"`
	InboxID uuid.UUID              `json:"inbox_id"`
	Name    string                 `json:"name"`
	URL     string                 `json:"url"`
	Public  bool                   `json:"public"`
	Counts  oas.RoadmapBoardCounts `json:"counts"`
}

type mcpRoadmapResource struct {
	Board mcpRoadmapBoard `json:"board"`
	Ideas []mcpIdea       `json:"ideas" jsonschema:"the top ideas by votes"`
}

func (c *mcpCall) ideaWouldChange(ctx context.Context, idea uuid.UUID, current string) (mcpWouldChange, error) {
	var out mcpWouldChange
	for stage, dst := range map[string]*int32{stagePlanned: &out.Planned, stageInProgress: &out.InProgress, stageShipped: &out.Shipped, stageClosed: &out.Closed} {
		if stage == current {
			continue
		}
		progress := stageProgress(stage)
		n, err := c.s.st.CountIdeaLinksToChange(ctx, store.CountIdeaLinksToChangeParams{WorkspaceID: c.p.workspaceID, IdeaID: idea, Progress: progress})
		if err != nil {
			return out, err
		}
		*dst = n
	}
	return out, nil
}

func (c *mcpCall) getIdea(ctx context.Context, id uuid.UUID) (oas.DiscussionDetail, error) {
	res, err := c.s.GetDiscussion(ctx, oas.GetDiscussionRequestObject{DiscussionId: id})
	if err != nil {
		return oas.DiscussionDetail{}, err
	}
	d := oas.DiscussionDetail(res.(oas.GetDiscussion200JSONResponse))
	if d.Discussion.Kind != oas.DiscussionKindIdea {
		return d, errDiscussionGone
	}
	return d, nil
}

func (c *mcpCall) boardOut(ctx context.Context, id uuid.UUID) (mcpRoadmapBoard, error) {
	res, err := c.s.GetRoadmapBoard(ctx, oas.GetRoadmapBoardRequestObject{BoardId: id})
	if err != nil {
		return mcpRoadmapBoard{}, err
	}
	b := oas.RoadmapBoard(res.(oas.GetRoadmapBoard200JSONResponse))
	return mcpRoadmapBoard{ID: b.Id, InboxID: b.InboxId, Name: b.Name, URL: b.Url, Public: b.Public, Counts: b.Counts}, nil
}

func (c *mcpCall) listIdeas(ctx context.Context, prm oas.ListDiscussionsParams) (mcpPage[mcpIdea], error) {
	kind := oas.DiscussionKindIdea
	prm.Kind = &kind
	res, err := c.s.ListDiscussions(ctx, oas.ListDiscussionsRequestObject{Params: prm})
	if err != nil {
		return mcpPage[mcpIdea]{}, err
	}
	page := oas.DiscussionPage(res.(oas.ListDiscussions200JSONResponse))
	out := mcpPage[mcpIdea]{Items: []mcpIdea{}}
	if page.NextCursor != nil {
		out.NextCursor = *page.NextCursor
	}
	for _, d := range page.Items {
		out.Items = append(out.Items, ideaOutMCP(d, false))
	}
	return out, nil
}

var mcpRoadmapTools = []mcpTool{
	defineTool("list_roadmap_ideas", "List roadmap ideas",
		"Lists the ideas of the roadmap boards you can see: title, stage, votes, replies and how much feedback is linked. Search the titles with query before linking feedback. Needs conversations:read and roadmap:read. Maps to GET /v1/discussions with kind=idea.",
		readOnly, map[string][]any{"stage": {"ideas", "planned", "in_progress", "shipped", "closed"}, "state": {"published", "pending", "hidden"}, "sort": {"votes", "new", "activity"}},
		mcpTool{ops: []string{"ListDiscussions", "ListRoadmapBoards"}},
		func(ctx context.Context, c *mcpCall, in listIdeasIn) (mcpPage[mcpIdea], error) {
			sort := oas.DiscussionTeamSortVotes
			if in.Sort != "" {
				sort = oas.DiscussionTeamSort(in.Sort)
			}
			prm := oas.ListDiscussionsParams{
				BoardId: in.BoardID, Stage: optString[oas.IdeaStage](in.Stage), Q: optString[string](in.Query), State: optString[oas.DiscussionState](in.State),
				Sort: &sort, Cursor: optString[string](in.Cursor), Limit: optLimit(in.Limit),
			}
			return c.listIdeas(ctx, prm)
		}),
	defineTool("get_roadmap_idea", "Get a roadmap idea",
		"One idea with its text, comments, votes, stage history and how many linked feedback a move to each stage would change (would_change): show that to the person you work for before move_idea_stage. Needs conversations:read and roadmap:read. Maps to GET /v1/discussions/{id} and GET /v1/discussions/{id}/stage-changes.",
		readOnly, nil, mcpTool{ops: []string{"GetDiscussion", "ListIdeaStageChanges"}},
		func(ctx context.Context, c *mcpCall, in ideaIn) (mcpIdeaDetail, error) {
			d, err := c.getIdea(ctx, in.IdeaID)
			if err != nil {
				return mcpIdeaDetail{}, err
			}
			out := mcpIdeaDetail{mcpIdea: ideaOutMCP(d.Discussion, true), History: []mcpIdeaChange{}}
			ch, err := c.s.ListIdeaStageChanges(ctx, oas.ListIdeaStageChangesRequestObject{DiscussionId: in.IdeaID, Params: oas.ListIdeaStageChangesParams{}})
			if err != nil {
				return out, err
			}
			for _, x := range ch.(oas.ListIdeaStageChanges200JSONResponse).Items {
				out.History = append(out.History, mcpIdeaChange{From: string(x.FromStage), To: string(x.ToStage), At: x.CreatedAt, Linked: x.Linked, ProgressSet: x.ProgressSet})
			}
			for _, p := range d.Posts {
				if p.State != oas.DiscussionStatePublished || p.Deleted {
					continue
				}
				cm := mcpIdeaComment{Text: p.Body}
				if p.Author.Name != nil {
					cm.Author = *p.Author.Name
				}
				out.CustomerContent.Comments = append(out.CustomerContent.Comments, cm)
			}
			out.WouldChange, err = c.ideaWouldChange(ctx, in.IdeaID, string(orZero(d.Discussion.Stage)))
			return out, err
		}),
	defineTool("link_feedback_to_idea", "Link feedback to an idea",
		"Links a feedback conversation to a roadmap idea of its inbox, replacing the link it had; an idea has many links, a feedback one. Linking again changes nothing. When the idea is already planned, in progress, shipped or closed the feedback's progress is set at once, which customers see. Needs roadmap:write and conversations:read. Maps to PUT /v1/discussions/{id}/conversations/{id}.",
		idempotent, nil, mcpTool{ops: []string{"LinkIdeaConversation"}},
		func(ctx context.Context, c *mcpCall, in ideaLinkIn) (mcpIdeaLink, error) {
			if _, err := c.s.LinkIdeaConversation(ctx, oas.LinkIdeaConversationRequestObject{DiscussionId: in.IdeaID, ConversationId: in.ConversationID}); err != nil {
				return mcpIdeaLink{}, err
			}
			return mcpIdeaLink{IdeaID: in.IdeaID, ConversationID: in.ConversationID, Linked: true}, nil
		}),
	defineTool("unlink_feedback_from_idea", "Unlink feedback from an idea",
		"Removes the link between a feedback conversation and an idea; the progress already set on the feedback stays. Unlinking feedback that is not linked changes nothing. Needs roadmap:write and conversations:read. Maps to DELETE /v1/discussions/{id}/conversations/{id}.",
		idempotent, nil, mcpTool{ops: []string{"UnlinkIdeaConversation"}},
		func(ctx context.Context, c *mcpCall, in ideaLinkIn) (mcpIdeaLink, error) {
			if _, err := c.s.UnlinkIdeaConversation(ctx, oas.UnlinkIdeaConversationRequestObject{DiscussionId: in.IdeaID, ConversationId: in.ConversationID}); err != nil {
				return mcpIdeaLink{}, err
			}
			return mcpIdeaLink{IdeaID: in.IdeaID, ConversationID: in.ConversationID}, nil
		}),
	defineTool("move_idea_stage", "Move a roadmap idea to a stage",
		"Moves an idea to a stage. Customers see it: the progress of every linked feedback is set (planned, in progress, done, won't do) and followers are notified. Call get_roadmap_idea first, tell the person you work for how many feedback would change, and pass that number from would_change as confirm_linked; any other number answers 409 confirm_mismatch and nothing moves. Listed only to connections granted the scope roadmap:stage by name. Maps to POST /v1/discussions/{id}/stage.",
		delivering, map[string][]any{"stage": {"ideas", "planned", "in_progress", "shipped", "closed"}},
		mcpTool{ops: []string{"GetDiscussion", "MoveIdeaStage"}, scopes: []oas.ApiKeyScope{oas.RoadmapStage}, allow: namesRoadmapStage, denied: errInsufficientScope(oas.RoadmapStage)},
		func(ctx context.Context, c *mcpCall, in moveIdeaIn) (mcpIdeaMoved, error) {
			if !oas.IdeaStage(in.Stage).Valid() {
				return mcpIdeaMoved{}, errValidation("stage must be ideas, planned, in_progress, shipped or closed")
			}
			_, err := c.getIdea(ctx, in.IdeaID)
			if err != nil {
				return mcpIdeaMoved{}, err
			}
			res, err := c.s.MoveIdeaStage(context.WithValue(ctx, confirmLinkedKey, in.ConfirmLinked), oas.MoveIdeaStageRequestObject{DiscussionId: in.IdeaID, Body: &oas.RoadmapStageMove{Stage: oas.IdeaStage(in.Stage)}})
			if err != nil {
				return mcpIdeaMoved{}, err
			}
			r := oas.RoadmapStageMoveResult(res.(oas.MoveIdeaStage200JSONResponse))
			out := mcpIdeaMoved{IdeaID: in.IdeaID, Stage: in.Stage}
			if r.Change != nil {
				out.Moved, out.From, out.Linked = true, string(r.Change.FromStage), r.Change.Linked
				if r.Change.Progress != nil {
					out.Progress = string(*r.Change.Progress)
				}
			}
			return out, nil
		}),
}

// namesRoadmapStage is true only for a key or grant that lists roadmap:stage: it is never implied.
func namesRoadmapStage(p principal) bool {
	return p.scoped() && slices.Contains(p.scopes, string(oas.RoadmapStage))
}

func mayReadRoadmapResource(p principal) bool {
	return keyMayCall(p, "GetRoadmapBoard") == nil && keyMayCall(p, "ListDiscussions") == nil
}

func (c *mcpCall) roadmapResource(ctx context.Context, id uuid.UUID) (mcpRoadmapResource, error) {
	b, err := c.boardOut(ctx, id)
	if err != nil {
		return mcpRoadmapResource{}, err
	}
	lim := int32(roadmapResourceTop)
	sort := oas.DiscussionTeamSortVotes
	page, err := c.listIdeas(ctx, oas.ListDiscussionsParams{BoardId: &id, Sort: &sort, Limit: &lim})
	return mcpRoadmapResource{Board: b, Ideas: page.Items}, err
}

func roadmapEventBoard(e realtime.Event) (uuid.UUID, bool) {
	if e.Type != signalRoadmapBoard {
		return uuid.UUID{}, false
	}
	var d struct {
		BoardID uuid.UUID `json:"board_id"`
	}
	return d.BoardID, json.Unmarshal(e.Data, &d) == nil
}

func addRoadmapPrompts(srv *mcp.Server, c *mcpCall) {
	if !toolsListed(c.p, "list_feedback", "list_roadmap_ideas", "link_feedback_to_idea") {
		return
	}
	srv.AddPrompt(&mcp.Prompt{
		Name: "link_feedback_to_ideas", Title: "Link feedback to roadmap ideas",
		Description: "Find feedback that asks for something, match it to ideas on the roadmap board and propose links. Nothing changes until you agree, and no stage is ever moved.",
		Arguments: []*mcp.PromptArgument{
			{Name: "inbox_id", Description: "The inbox to go through; every inbox you can see when empty."},
			{Name: "board_id", Description: "The roadmap board to match against; the inbox's board when empty."},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		a := req.Params.Arguments
		board := "the roadmap board of the same inbox (list_roadmap_ideas without board_id shows the boards' ideas)"
		if a["board_id"] != "" {
			board = "the roadmap board " + a["board_id"]
		}
		return promptResult("Link feedback to roadmap ideas in "+inboxScope(a),
			"Match feedback in "+inboxScope(a)+" with ideas on "+board+".",
			"1. Call list_feedback with category idea and without_idea true, newest first, a page or two.\n"+
				"2. For each feedback, call list_roadmap_ideas with query set to the key words of what the customer asks for (search again with synonyms before you decide nothing fits). Ideas of other inboxes cannot be linked.\n"+
				"3. Propose a table: the feedback, the idea you would link it to and why, and the feedback that matches no idea. Do not link anything yet.",
			"After I agree to some rows, call link_feedback_to_idea for exactly those. Never call move_idea_stage in this task, and do not write to customers."), nil
	})
}
