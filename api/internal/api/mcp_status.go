package api

import (
	"context"
	"time"
	"uuid"

	"github.com/productdevbook/yuva/api/internal/oas"
)

var (
	statusStateEnum    = []any{"operational", "degraded", "partial_outage", "major_outage", "maintenance"}
	statusImpactEnum   = []any{"none", "minor", "major", "critical"}
	incidentStatesEnum = []any{"investigating", "identified", "monitoring", "resolved", "scheduled", "in_progress", "completed"}
)

type mcpStatusPage struct {
	ID        uuid.UUID `json:"id"`
	InboxID   uuid.UUID `json:"inbox_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url" jsonschema:"the public page"`
	State     string    `json:"state" jsonschema:"the worst state of its visible components"`
	Published bool      `json:"published" jsonschema:"false: nobody outside the team sees it yet"`
}

func statusPageOutMCP(p oas.StatusPage) mcpStatusPage {
	return mcpStatusPage{ID: p.Id, InboxID: p.InboxId, Name: p.Name, URL: p.Url, State: string(p.State), Published: p.Published}
}

type mcpStatusComponent struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	State  string    `json:"state"`
	Since  time.Time `json:"since"`
	Source string    `json:"source" jsonschema:"member, api_key, monitor or incident"`
	Hidden bool      `json:"hidden,omitempty"`
}

type mcpStatusUpdate struct {
	State string    `json:"state"`
	Body  string    `json:"body"`
	At    time.Time `json:"at"`
}

type mcpIncident struct {
	ID         uuid.UUID         `json:"id"`
	Kind       string            `json:"kind"`
	Title      string            `json:"title"`
	Impact     string            `json:"impact"`
	State      string            `json:"state"`
	Draft      bool              `json:"draft" jsonschema:"true until a member publishes it in Yuva"`
	Components []uuid.UUID       `json:"components"`
	Updates    []mcpStatusUpdate `json:"updates" jsonschema:"newest first"`
	URL        string            `json:"url"`
}

func incidentOutMCP(in oas.StatusIncident) mcpIncident {
	out := mcpIncident{ID: in.Id, Kind: string(in.Kind), Title: in.Title, Impact: string(in.Impact), State: string(in.State),
		Draft: in.PublishedAt == nil, Components: []uuid.UUID{}, Updates: []mcpStatusUpdate{}, URL: in.Url}
	for _, c := range in.Components {
		out.Components = append(out.Components, c.ComponentId)
	}
	for _, u := range in.Updates {
		out.Updates = append(out.Updates, mcpStatusUpdate{State: string(u.State), Body: u.Body, At: u.CreatedAt})
	}
	return out
}

type mcpSubscriberCounts struct {
	Enabled      bool  `json:"enabled" jsonschema:"whether the page mails its e-mail subscribers"`
	Subscribed   int64 `json:"subscribed"`
	Pending      int64 `json:"pending" jsonschema:"waiting for their confirmation"`
	Unsubscribed int64 `json:"unsubscribed"`
}

type mcpStatusDetail struct {
	Page        mcpStatusPage        `json:"page"`
	Subscribers mcpSubscriberCounts  `json:"subscribers" jsonschema:"the page's e-mail subscribers, counts only"`
	Components  []mcpStatusComponent `json:"components"`
	Open        []mcpIncident        `json:"open" jsonschema:"open incidents and maintenance, drafts included"`
}

type statusPageIn struct {
	PageID uuid.UUID `json:"page_id" jsonschema:"a page from list_status_pages"`
}

type componentStateIn struct {
	PageID      uuid.UUID `json:"page_id"`
	ComponentID uuid.UUID `json:"component_id"`
	State       string    `json:"state"`
}

type incidentComponentIn struct {
	ComponentID uuid.UUID `json:"component_id"`
	State       string    `json:"state,omitempty" jsonschema:"its state while the incident is open; default major_outage"`
}

type createIncidentIn struct {
	PageID     uuid.UUID             `json:"page_id"`
	Title      string                `json:"title"`
	Message    string                `json:"message" jsonschema:"the first update, plain text that customers read"`
	Impact     string                `json:"impact,omitempty"`
	Components []incidentComponentIn `json:"components,omitempty"`
	Notify     *bool                 `json:"notify,omitempty" jsonschema:"mail the page's subscribers when a member publishes it; default true"`
}

type incidentUpdateIn struct {
	PageID     uuid.UUID `json:"page_id"`
	IncidentID uuid.UUID `json:"incident_id"`
	Body       string    `json:"body" jsonschema:"plain text that customers read"`
	State      string    `json:"state,omitempty" jsonschema:"the incident's new state; its current one when empty"`
	Notify     *bool     `json:"notify,omitempty" jsonschema:"mail the page's subscribers about it; default true when the state changes, false when it only adds text"`
}

type resolveIncidentIn struct {
	PageID     uuid.UUID `json:"page_id"`
	IncidentID uuid.UUID `json:"incident_id"`
	Body       string    `json:"body" jsonschema:"plain text that customers read"`
}

type linkConversationIn struct {
	PageID         uuid.UUID `json:"page_id"`
	IncidentID     uuid.UUID `json:"incident_id"`
	ConversationID uuid.UUID `json:"conversation_id" jsonschema:"a conversation from search_conversations or get_conversation"`
}

type mcpIncidentLink struct {
	IncidentID     uuid.UUID `json:"incident_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Linked         bool      `json:"linked"`
}

type mcpStatusMonitor struct {
	ID          uuid.UUID  `json:"id"`
	ComponentID uuid.UUID  `json:"component_id" jsonschema:"the component whose state it sets"`
	Kind        string     `json:"kind" jsonschema:"heartbeat or push"`
	Name        string     `json:"name"`
	State       string     `json:"state" jsonschema:"pending, up or down"`
	Since       time.Time  `json:"since" jsonschema:"its last change between up and down"`
	Paused      bool       `json:"paused,omitempty"`
	LastResult  *time.Time `json:"last_result,omitempty"`
	IncidentID  *uuid.UUID `json:"incident_id,omitempty" jsonschema:"the open incident it opened"`
	// The message comes from whoever holds the monitor's token, so it is treated like customer text.
	CustomerContent *mcpMonitorContent `json:"customer_content,omitempty"`
}

type mcpMonitorContent struct {
	LastMessage string `json:"last_message" jsonschema:"what the host's monitoring sent last"`
}

func (c *mcpCall) statusIncidentUpdate(ctx context.Context, page, incident uuid.UUID, body string, state *oas.StatusIncidentState, notify *bool) (mcpIncident, error) {
	res, err := c.s.CreateStatusIncidentUpdate(ctx, oas.CreateStatusIncidentUpdateRequestObject{PageId: page, IncidentId: incident,
		Body: &oas.StatusIncidentUpdateCreate{Body: body, State: state, Notify: notify}})
	if err != nil {
		return mcpIncident{}, err
	}
	return incidentOutMCP(oas.StatusIncident(res.(oas.CreateStatusIncidentUpdate201JSONResponse))), nil
}

var mcpStatusTools = []mcpTool{
	defineTool("list_status_pages", "List status pages",
		"Lists the public status pages you can see with their overall state and whether they are published. Maps to GET /v1/status-pages.",
		readOnly, nil, mcpTool{ops: []string{"ListStatusPages"}},
		func(ctx context.Context, c *mcpCall, _ noInput) (mcpPage[mcpStatusPage], error) {
			res, err := c.s.ListStatusPages(ctx, oas.ListStatusPagesRequestObject{})
			if err != nil {
				return mcpPage[mcpStatusPage]{}, err
			}
			out := mcpPage[mcpStatusPage]{Items: []mcpStatusPage{}}
			for _, p := range res.(oas.ListStatusPages200JSONResponse).Items {
				out.Items = append(out.Items, statusPageOutMCP(p))
			}
			return out, nil
		}),
	defineTool("get_status_page", "Get a status page",
		"A status page with its components and their states, and its open incidents and maintenance. Maps to GET /v1/status-pages/{id}, its components and incidents.",
		readOnly, nil, mcpTool{ops: []string{"GetStatusPage", "ListStatusComponents", "ListStatusIncidents"}},
		func(ctx context.Context, c *mcpCall, in statusPageIn) (mcpStatusDetail, error) {
			res, err := c.s.GetStatusPage(ctx, oas.GetStatusPageRequestObject{PageId: in.PageID})
			if err != nil {
				return mcpStatusDetail{}, err
			}
			pg := oas.StatusPage(res.(oas.GetStatusPage200JSONResponse))
			es := pg.EmailSubscribers
			out := mcpStatusDetail{Page: statusPageOutMCP(pg), Components: []mcpStatusComponent{}, Open: []mcpIncident{},
				Subscribers: mcpSubscriberCounts{Enabled: es.Enabled, Subscribed: es.Counts.Subscribed, Pending: es.Counts.Pending, Unsubscribed: es.Counts.Unsubscribed}}
			comps, err := c.s.ListStatusComponents(ctx, oas.ListStatusComponentsRequestObject{PageId: in.PageID})
			if err != nil {
				return mcpStatusDetail{}, err
			}
			for _, cp := range comps.(oas.ListStatusComponents200JSONResponse).Items {
				out.Components = append(out.Components, mcpStatusComponent{ID: cp.Id, Name: cp.Name, State: string(cp.State), Since: cp.StateSince,
					Source: string(cp.StateSource), Hidden: cp.Hidden})
			}
			yes := true
			incs, err := c.s.ListStatusIncidents(ctx, oas.ListStatusIncidentsRequestObject{PageId: in.PageID, Params: oas.ListStatusIncidentsParams{Open: &yes}})
			if err != nil {
				return mcpStatusDetail{}, err
			}
			for _, i := range incs.(oas.ListStatusIncidents200JSONResponse).Items {
				out.Open = append(out.Open, incidentOutMCP(i))
			}
			return out, nil
		}),
	defineTool("list_monitors", "List a status page's monitors",
		"Lists the heartbeat and push monitors of a status page with their state (pending, up, down), the component each sets and the incident a failing one opened. Maps to GET /v1/status-pages/{id}/monitors.",
		readOnly, nil, mcpTool{ops: []string{"ListStatusMonitors"}},
		func(ctx context.Context, c *mcpCall, in statusPageIn) (mcpPage[mcpStatusMonitor], error) {
			res, err := c.s.ListStatusMonitors(ctx, oas.ListStatusMonitorsRequestObject{PageId: in.PageID})
			if err != nil {
				return mcpPage[mcpStatusMonitor]{}, err
			}
			out := mcpPage[mcpStatusMonitor]{Items: []mcpStatusMonitor{}}
			for _, m := range res.(oas.ListStatusMonitors200JSONResponse).Items {
				item := mcpStatusMonitor{ID: m.Id, ComponentID: m.ComponentId, Kind: string(m.Kind), Name: m.Name, State: string(m.State),
					Since: m.StateSince, Paused: m.Paused, LastResult: m.LastResultAt, IncidentID: m.IncidentId}
				if m.LastMessage != nil {
					item.CustomerContent = &mcpMonitorContent{LastMessage: *m.LastMessage}
				}
				out.Items = append(out.Items, item)
			}
			return out, nil
		}),
	defineTool("set_component_state", "Set a component's state",
		"Sets one component of a status page to a state; the public page shows it at once. Maps to POST /v1/status-pages/{id}/components/{id}/state.",
		idempotent, map[string][]any{"state": statusStateEnum}, mcpTool{ops: []string{"SetStatusComponentState"}},
		func(ctx context.Context, c *mcpCall, in componentStateIn) (mcpStatusComponent, error) {
			res, err := c.s.SetStatusComponentState(ctx, oas.SetStatusComponentStateRequestObject{PageId: in.PageID, ComponentId: in.ComponentID,
				Body: &oas.StatusComponentStateChange{State: oas.StatusComponentStateValue(in.State)}})
			if err != nil {
				return mcpStatusComponent{}, err
			}
			cp := oas.StatusComponent(res.(oas.SetStatusComponentState200JSONResponse))
			return mcpStatusComponent{ID: cp.Id, Name: cp.Name, State: string(cp.State), Since: cp.StateSince, Source: string(cp.StateSource), Hidden: cp.Hidden}, nil
		}),
	defineTool("create_incident", "Draft an incident",
		"Drafts an incident on a status page with its first update and the components it touches. It stays a draft that only the team sees, and changes no component, until a member publishes it in Yuva. Maps to POST /v1/status-pages/{id}/incidents.",
		additive, map[string][]any{"impact": statusImpactEnum, "components.state": statusStateEnum}, mcpTool{ops: []string{"CreateStatusIncident"}},
		func(ctx context.Context, c *mcpCall, in createIncidentIn) (mcpIncident, error) {
			body := oas.StatusIncidentCreate{Title: in.Title, Message: in.Message, Impact: optString[oas.StatusIncidentImpact](in.Impact), Notify: in.Notify}
			if len(in.Components) > 0 {
				comps := make([]oas.StatusIncidentComponentInput, len(in.Components))
				for i, cp := range in.Components {
					comps[i] = oas.StatusIncidentComponentInput{ComponentId: cp.ComponentID, State: optString[oas.StatusComponentStateValue](cp.State)}
				}
				body.Components = &comps
			}
			res, err := c.s.CreateStatusIncident(ctx, oas.CreateStatusIncidentRequestObject{PageId: in.PageID, Body: &body})
			if err != nil {
				return mcpIncident{}, err
			}
			return incidentOutMCP(oas.StatusIncident(res.(oas.CreateStatusIncident201JSONResponse))), nil
		}),
	defineTool("post_incident_update", "Post an incident update",
		"Adds an update to an incident or maintenance and optionally moves it to another state. On a published incident customers see it at once. Maps to POST /v1/status-pages/{id}/incidents/{id}/updates.",
		additive, map[string][]any{"state": incidentStatesEnum}, mcpTool{ops: []string{"CreateStatusIncidentUpdate"}},
		func(ctx context.Context, c *mcpCall, in incidentUpdateIn) (mcpIncident, error) {
			return c.statusIncidentUpdate(ctx, in.PageID, in.IncidentID, in.Body, optString[oas.StatusIncidentState](in.State), in.Notify)
		}),
	defineTool("link_conversation_to_incident", "Link a conversation to an incident",
		"Links a conversation to an incident or maintenance of a status page, so the team sees it with the incident and a member's \"Reply to linked\" reaches it. Linking twice changes nothing. Needs the scopes status:manage and conversations:read. Maps to PUT /v1/status-pages/{id}/incidents/{id}/conversations/{id}.",
		idempotent, nil, mcpTool{ops: []string{"LinkStatusIncidentConversation"}},
		func(ctx context.Context, c *mcpCall, in linkConversationIn) (mcpIncidentLink, error) {
			if _, err := c.s.LinkStatusIncidentConversation(ctx, oas.LinkStatusIncidentConversationRequestObject{PageId: in.PageID, IncidentId: in.IncidentID,
				ConversationId: in.ConversationID}); err != nil {
				return mcpIncidentLink{}, err
			}
			return mcpIncidentLink{IncidentID: in.IncidentID, ConversationID: in.ConversationID, Linked: true}, nil
		}),
	defineTool("resolve_incident", "Resolve an incident",
		"Resolves an incident, or completes maintenance, with a closing update; its components go back to operational unless another open incident holds them. Maps to POST /v1/status-pages/{id}/incidents/{id}/updates.",
		additive, nil, mcpTool{ops: []string{"GetStatusIncident", "CreateStatusIncidentUpdate"}},
		func(ctx context.Context, c *mcpCall, in resolveIncidentIn) (mcpIncident, error) {
			res, err := c.s.GetStatusIncident(ctx, oas.GetStatusIncidentRequestObject{PageId: in.PageID, IncidentId: in.IncidentID})
			if err != nil {
				return mcpIncident{}, err
			}
			state := oas.StatusResolved
			if res.(oas.GetStatusIncident200JSONResponse).Kind == oas.StatusKindMaintenance {
				state = oas.StatusCompleted
			}
			return c.statusIncidentUpdate(ctx, in.PageID, in.IncidentID, in.Body, &state, nil)
		}),
}
