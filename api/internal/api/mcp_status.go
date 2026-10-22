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

type createStatusPageIn struct {
	InboxID     uuid.UUID `json:"inbox_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug" jsonschema:"a-z, 0-9 and inner hyphens, 3 to 48 characters, unique across the server; part of the page's address"`
	Description string    `json:"description,omitempty"`
	Locale      string    `json:"locale,omitempty" jsonschema:"the language of the page's words (en, tr, de); the inbox's when empty"`
	Timezone    string    `json:"timezone,omitempty" jsonschema:"an IANA time zone for the day bars"`
	SiteURL     string    `json:"site_url,omitempty" jsonschema:"the product's http(s) address"`
	SupportURL  string    `json:"support_url,omitempty" jsonschema:"an http(s) or mailto: address where customers get help"`
}

type createStatusComponentIn struct {
	PageID      uuid.UUID `json:"page_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Hidden      *bool     `json:"hidden,omitempty" jsonschema:"left off the public page and out of its overall state"`
}

type createStatusMonitorIn struct {
	PageID           uuid.UUID `json:"page_id"`
	ComponentID      uuid.UUID `json:"component_id" jsonschema:"the component whose state the monitor sets"`
	Kind             string    `json:"kind" jsonschema:"heartbeat: the host calls Yuva on a schedule; push: the host's monitoring sends its state"`
	Name             string    `json:"name"`
	Interval         int32     `json:"interval,omitempty" jsonschema:"heartbeat: seconds between beats, 30 to 30 days; 300 when empty"`
	Grace            int32     `json:"grace,omitempty" jsonschema:"heartbeat: seconds a beat may be late; 60 when empty"`
	FailureState     string    `json:"failure_state,omitempty" jsonschema:"the state a failing monitor sets; major_outage when empty"`
	OnFailure        string    `json:"on_failure,omitempty" jsonschema:"none, or draft: failing opens a draft incident a member publishes; draft when empty"`
	FailureThreshold int32     `json:"failure_threshold,omitempty" jsonschema:"failures in a row that fail it; 1 when empty"`
}

type mcpStatusMonitorCreated struct {
	mcpStatusMonitor
	Token string `json:"token,omitempty" jsonschema:"a secret shown only now: hand it to the person for their monitoring and never repeat it elsewhere"`
	URL   string `json:"url,omitempty" jsonschema:"where the host calls; it contains the token"`
}

type updateIncidentIn struct {
	PageID     uuid.UUID `json:"page_id"`
	IncidentID uuid.UUID `json:"incident_id"`
	Title      *string   `json:"title,omitempty"`
	Impact     string    `json:"impact,omitempty"`
}

func monitorOutMCP(m oas.StatusMonitor) mcpStatusMonitor {
	item := mcpStatusMonitor{ID: m.Id, ComponentID: m.ComponentId, Kind: string(m.Kind), Name: m.Name, State: string(m.State),
		Since: m.StateSince, Paused: m.Paused, LastResult: m.LastResultAt, IncidentID: m.IncidentId}
	if m.LastMessage != nil {
		item.CustomerContent = &mcpMonitorContent{LastMessage: *m.LastMessage}
	}
	return item
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
	defineTool("create_status_page", "Create a status page",
		"Creates a status page for an inbox's product. It stays unpublished: nobody outside the team sees it until a member publishes it in Yuva. Then add its components with create_status_component. Owners, admins and keys with status:manage. Maps to POST /v1/status-pages.",
		additive, nil, mcpTool{ops: []string{"CreateStatusPage"}, allow: mayManage, denied: errManagersOnly},
		func(ctx context.Context, c *mcpCall, in createStatusPageIn) (mcpStatusPage, error) {
			body := oas.StatusPageCreate{InboxId: in.InboxID, Name: in.Name, Slug: in.Slug, Description: optString[string](in.Description),
				Locale: optString[string](in.Locale), Timezone: optString[string](in.Timezone), SiteUrl: optString[string](in.SiteURL),
				SupportUrl: optString[string](in.SupportURL)}
			res, err := c.s.CreateStatusPage(ctx, oas.CreateStatusPageRequestObject{Body: &body})
			if err != nil {
				return mcpStatusPage{}, err
			}
			return statusPageOutMCP(oas.StatusPage(res.(oas.CreateStatusPage201JSONResponse))), nil
		}),
	defineTool("create_status_component", "Add a status page component",
		"Adds a component (a part of the product that can be up or down) to a status page, operational at first. Owners, admins and keys with status:manage. Maps to POST /v1/status-pages/{id}/components.",
		additive, nil, mcpTool{ops: []string{"CreateStatusComponent"}, allow: mayManage, denied: errManagersOnly},
		func(ctx context.Context, c *mcpCall, in createStatusComponentIn) (mcpStatusComponent, error) {
			res, err := c.s.CreateStatusComponent(ctx, oas.CreateStatusComponentRequestObject{PageId: in.PageID,
				Body: &oas.StatusComponentCreate{Name: in.Name, Description: optString[string](in.Description), Hidden: in.Hidden}})
			if err != nil {
				return mcpStatusComponent{}, err
			}
			cp := oas.StatusComponent(res.(oas.CreateStatusComponent201JSONResponse))
			return mcpStatusComponent{ID: cp.Id, Name: cp.Name, State: string(cp.State), Since: cp.StateSince, Source: string(cp.StateSource), Hidden: cp.Hidden}, nil
		}),
	defineTool("create_status_monitor", "Add a status monitor",
		"Adds a heartbeat or push monitor that sets a component's state from the host's own monitoring. The answer carries the monitor's token and url once; give them to the person to put in their monitoring and do not repeat them. A failing monitor opens at most a draft incident that a member publishes. Owners, admins and keys with status:manage. Maps to POST /v1/status-pages/{id}/monitors.",
		additive, map[string][]any{"kind": {"heartbeat", "push"}, "failure_state": {"degraded", "partial_outage", "major_outage"}, "on_failure": {"none", "draft"}},
		mcpTool{ops: []string{"CreateStatusMonitor"}, allow: mayManage, denied: errManagersOnly},
		func(ctx context.Context, c *mcpCall, in createStatusMonitorIn) (mcpStatusMonitorCreated, error) {
			body := oas.StatusMonitorCreate{ComponentId: in.ComponentID, Kind: oas.StatusMonitorKind(in.Kind), Name: in.Name,
				FailureState: optString[oas.StatusMonitorFailureState](in.FailureState), OnFailure: optString[oas.StatusMonitorOnFailure](in.OnFailure)}
			if in.Interval != 0 {
				body.Interval = &in.Interval
			}
			if in.Grace != 0 {
				body.Grace = &in.Grace
			}
			if in.FailureThreshold != 0 {
				body.FailureThreshold = &in.FailureThreshold
			}
			res, err := c.s.CreateStatusMonitor(ctx, oas.CreateStatusMonitorRequestObject{PageId: in.PageID, Body: &body})
			if err != nil {
				return mcpStatusMonitorCreated{}, err
			}
			m := oas.StatusMonitorWithToken(res.(oas.CreateStatusMonitor201JSONResponse))
			out := mcpStatusMonitorCreated{mcpStatusMonitor: monitorOutMCP(oas.StatusMonitor{
				Id: m.Id, ComponentId: m.ComponentId, Kind: m.Kind, Name: m.Name, State: m.State, StateSince: m.StateSince, Paused: m.Paused,
				LastResultAt: m.LastResultAt, IncidentId: m.IncidentId, LastMessage: m.LastMessage,
			})}
			out.Token, out.URL = deref(m.Token), deref(m.Url)
			return out, nil
		}),
	defineTool("update_incident", "Change an incident's title or impact",
		"Changes the title or impact of an open incident, a draft or a published one; the public page shows a change to a published incident at once. The text goes through post_incident_update. A resolved incident is refused. Maps to PATCH /v1/status-pages/{id}/incidents/{id}.",
		idempotent, map[string][]any{"impact": statusImpactEnum}, mcpTool{ops: []string{"UpdateStatusIncident"}},
		func(ctx context.Context, c *mcpCall, in updateIncidentIn) (mcpIncident, error) {
			body := oas.StatusIncidentPatch{Title: in.Title, Impact: optString[oas.StatusIncidentImpact](in.Impact)}
			if body.Title == nil && body.Impact == nil {
				return mcpIncident{}, errNothingToUpdate
			}
			res, err := c.s.UpdateStatusIncident(ctx, oas.UpdateStatusIncidentRequestObject{PageId: in.PageID, IncidentId: in.IncidentID, Body: &body})
			if err != nil {
				return mcpIncident{}, err
			}
			return incidentOutMCP(oas.StatusIncident(res.(oas.UpdateStatusIncident200JSONResponse))), nil
		}),
}
