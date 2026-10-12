package migrate

import (
	"slices"
	"sort"
	"strings"
	"time"
)

// Item states and reasons, as stored in migration_items.
const (
	StatePlanned   = "planned"
	StateImported  = "imported"
	StateDuplicate = "duplicate"
	StateSkipped   = "skipped"
	StateFailed    = "failed"

	ReasonCopy         = "copy"
	ReasonInYuva       = "in_yuva"
	ReasonInternal     = "internal"
	ReasonAutomatic    = "automatic"
	ReasonOutsideRange = "outside_range"
	ReasonBlocked      = "blocked"
	ReasonTooLarge     = "too_large"
	ReasonLabelOnly    = "label_only"
	ReasonUnselected   = "unselected"
	ReasonUnplanned    = "unplanned"
	ReasonGone         = "gone"
	ReasonUnparseable  = "unparseable"
	ReasonFailed       = "failed"

	StatusOpen    = "open"
	StatusPending = "pending"
	StatusClosed  = "closed"
)

// PlanFolder is how a scanned folder takes part in the plan: Label names the label its
// conversations get ("" for none), LabelsOnly folders only label messages read elsewhere.
type PlanFolder struct {
	Kind       string
	Label      string
	LabelsOnly bool
}

type PlanInput struct {
	Items           []Item
	Folders         map[int32]PlanFolder
	Team            map[string]bool
	Since           *time.Time
	SkipAutomatic   bool
	CloseAfterDays  int
	Now             time.Time
	MaxMessageBytes int64
	Blocked         map[string]bool
	InYuva          map[string]bool
	// Names, when set, is the name each address goes by across the whole mailbox; otherwise the
	// names are counted over Items.
	Names        map[string]string
	AttachmentOK func(contentType string, size int64) bool
}

type ItemPlan struct {
	State     string
	Reason    string
	ThreadKey string
}

// ThreadPlan is one conversation to create; its JSON is kept on migration_threads.plan.
type ThreadPlan struct {
	Key       string    `json:"-"`
	Contact   Address   `json:"contact"`
	Subject   string    `json:"subject"`
	Status    string    `json:"status"`
	FirstAt   time.Time `json:"first_at"`
	LastAt    time.Time `json:"last_at"`
	Labels    []string  `json:"labels,omitempty"`
	In        int       `json:"in"`
	Out       int       `json:"out"`
	SourceIDs []string  `json:"-"`
}

type Totals struct {
	Conversations      int64
	In, Out            int64
	Contacts           map[string]bool
	Labels             map[string]int64
	Skipped            map[string]int64
	Attachments        int64
	AttachmentBytes    int64
	AttachmentsSkipped int64
	Bytes              int64
	Oldest, Newest     time.Time
}

type Plan struct {
	Items   map[string]ItemPlan
	Threads []ThreadPlan
	Totals  Totals
}

var folderOrder = map[string]int{KindInbox: 0, KindSent: 1, KindArchive: 2, KindAll: 3, KindUser: 4, KindLabel: 5}

func orderOf(kind string) int {
	if o, ok := folderOrder[kind]; ok {
		return o
	}
	return 9
}

// PrimaryOrder lists the folders whose messages are imported, in the order MakePlan prefers one
// copy of a message over another.
func PrimaryOrder(folders map[int32]PlanFolder) []int32 {
	var out []int32
	for id, f := range folders {
		if !f.LabelsOnly {
			out = append(out, id)
		}
	}
	slices.SortFunc(out, func(a, b int32) int {
		if oa, ob := orderOf(folders[a].Kind), orderOf(folders[b].Kind); oa != ob {
			return oa - ob
		}
		return int(a - b)
	})
	return out
}

type unionFind map[string]string

func (u unionFind) find(x string) string {
	root := x
	for {
		p, ok := u[root]
		if !ok || p == root {
			break
		}
		root = p
	}
	for x != root {
		next := u[x]
		u[x] = root
		x = next
	}
	return root
}

func (u unionFind) union(a, b string) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	if ra < rb {
		u[rb] = ra
	} else {
		u[ra] = rb
	}
}

// Direction tells mail from a team address (out) from mail by anyone else (in).
func Direction(team map[string]bool, from string) string {
	if team[from] {
		return "out"
	}
	return "in"
}

// MakePlan joins the scanned messages into threads by Message-ID, In-Reply-To and References,
// never by subject, and decides what each message becomes.
func MakePlan(in PlanInput) Plan {
	p := Plan{Items: map[string]ItemPlan{}, Totals: Totals{
		Contacts: map[string]bool{}, Labels: map[string]int64{}, Skipped: map[string]int64{},
	}}
	counts := map[string]map[string]int{}
	seeName := func(a Address) {
		if a.Email == "" || a.Name == "" {
			return
		}
		if counts[a.Email] == nil {
			counts[a.Email] = map[string]int{}
		}
		counts[a.Email][a.Name]++
	}
	groups := map[string][]*Item{}
	extraLabels := map[string][]string{}
	for i := range in.Items {
		it := &in.Items[i]
		f, ok := in.Folders[it.FolderID]
		switch {
		case !ok:
			p.Items[it.SourceID] = ItemPlan{State: StateSkipped, Reason: ReasonUnselected, ThreadKey: it.DedupeKey}
		case f.LabelsOnly:
			p.Items[it.SourceID] = ItemPlan{State: StateSkipped, Reason: ReasonLabelOnly, ThreadKey: it.DedupeKey}
			if f.Label != "" {
				extraLabels[it.DedupeKey] = append(extraLabels[it.DedupeKey], f.Label)
			}
		default:
			groups[it.DedupeKey] = append(groups[it.DedupeKey], it)
			seeName(it.Meta.From)
			for _, a := range slices.Concat(it.Meta.To, it.Meta.Cc) {
				seeName(a)
			}
		}
	}
	names := in.Names
	if names == nil {
		names = bestNames(counts)
	}
	primaries := map[string]*Item{}
	labels := map[string][]string{}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g := groups[k]
		sort.SliceStable(g, func(a, b int) bool {
			oa, ob := orderOf(in.Folders[g[a].FolderID].Kind), orderOf(in.Folders[g[b].FolderID].Kind)
			if oa != ob {
				return oa < ob
			}
			if g[a].FolderID != g[b].FolderID {
				return g[a].FolderID < g[b].FolderID
			}
			return g[a].SourceID < g[b].SourceID
		})
		primaries[k] = g[0]
		var ls []string
		for i, it := range g {
			if l := in.Folders[it.FolderID].Label; l != "" && !slices.Contains(ls, l) {
				ls = append(ls, l)
			}
			if i > 0 {
				p.Items[it.SourceID] = ItemPlan{State: StateDuplicate, Reason: ReasonCopy, ThreadKey: k}
				p.Totals.Skipped["duplicate_copies"]++
			}
		}
		for _, l := range extraLabels[k] {
			if !slices.Contains(ls, l) {
				ls = append(ls, l)
			}
		}
		labels[k] = ls
	}
	uf := unionFind{}
	for _, k := range keys {
		uf.find(k)
		for _, r := range primaries[k].Meta.Refs {
			uf.union(k, r)
		}
	}
	components := map[string][]*Item{}
	for _, k := range keys {
		root := uf.find(k)
		components[root] = append(components[root], primaries[k])
	}
	roots := make([]string, 0, len(components))
	for r := range components {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	for _, r := range roots {
		p.planThread(in, components[r], labels, names)
	}
	sort.SliceStable(p.Threads, func(a, b int) bool {
		if !p.Threads[a].LastAt.Equal(p.Threads[b].LastAt) {
			return p.Threads[a].LastAt.After(p.Threads[b].LastAt)
		}
		return p.Threads[a].Key < p.Threads[b].Key
	})
	return p
}

func (p *Plan) skip(items []*Item, key, reason string) {
	for _, it := range items {
		p.Items[it.SourceID] = ItemPlan{State: StateSkipped, Reason: reason, ThreadKey: key}
		p.Totals.Skipped[reason]++
	}
}

// bestNames picks for each address the name it goes by most often, the first in byte order on a tie.
func bestNames(counts map[string]map[string]int) map[string]string {
	out := make(map[string]string, len(counts))
	for addr, n := range counts {
		best, count := "", 0
		for name, c := range n {
			if c > count || (c == count && name < best) {
				best, count = name, c
			}
		}
		if count > 0 {
			out[addr] = best
		}
	}
	return out
}

// Add adds the totals of another part of the same plan; Contacts are left to the caller.
func (t *Totals) Add(o Totals) {
	t.Conversations += o.Conversations
	t.In += o.In
	t.Out += o.Out
	t.Attachments += o.Attachments
	t.AttachmentBytes += o.AttachmentBytes
	t.AttachmentsSkipped += o.AttachmentsSkipped
	t.Bytes += o.Bytes
	if t.Labels == nil {
		t.Labels = map[string]int64{}
	}
	for k, v := range o.Labels {
		t.Labels[k] += v
	}
	if t.Skipped == nil {
		t.Skipped = map[string]int64{}
	}
	for k, v := range o.Skipped {
		t.Skipped[k] += v
	}
	if !o.Oldest.IsZero() && (t.Oldest.IsZero() || o.Oldest.Before(t.Oldest)) {
		t.Oldest = o.Oldest
	}
	if o.Newest.After(t.Newest) {
		t.Newest = o.Newest
	}
}

func (p *Plan) planThread(in PlanInput, msgs []*Item, labels map[string][]string, names map[string]string) {
	sort.SliceStable(msgs, func(a, b int) bool {
		if !msgs[a].Meta.At.Equal(msgs[b].Meta.At) {
			return msgs[a].Meta.At.Before(msgs[b].Meta.At)
		}
		return msgs[a].SourceID < msgs[b].SourceID
	})
	key := msgs[0].DedupeKey
	var cands []*Item
	for _, it := range msgs {
		if in.MaxMessageBytes > 0 && it.Size > in.MaxMessageBytes {
			p.skip([]*Item{it}, key, ReasonTooLarge)
			continue
		}
		cands = append(cands, it)
	}
	if len(cands) == 0 {
		return
	}
	last := cands[len(cands)-1].Meta.At
	if in.Since != nil && last.Before(*in.Since) {
		p.skip(cands, key, ReasonOutsideRange)
		return
	}
	internal, automatic := true, true
	var contact Address
	for _, it := range cands {
		if !it.Meta.Auto {
			automatic = false
		}
		if Direction(in.Team, it.Meta.From.Email) == "in" {
			internal = false
			if contact.Email == "" {
				contact = it.Meta.From
			}
			continue
		}
		for _, a := range slices.Concat(it.Meta.To, it.Meta.Cc) {
			if !in.Team[a.Email] {
				internal = false
			}
		}
	}
	if contact.Email == "" {
		for _, it := range cands {
			for _, a := range slices.Concat(it.Meta.To, it.Meta.Cc) {
				if !in.Team[a.Email] && contact.Email == "" {
					contact = a
				}
			}
		}
	}
	switch {
	case internal || contact.Email == "":
		p.skip(cands, key, ReasonInternal)
		return
	case automatic && in.SkipAutomatic:
		p.skip(cands, key, ReasonAutomatic)
		return
	case in.Blocked[contact.Email]:
		p.skip(cands, key, ReasonBlocked)
		return
	}
	if name, ok := names[contact.Email]; ok {
		contact.Name = name
	}
	t := ThreadPlan{Key: key, Contact: contact, Subject: BaseSubject(cands[0].Meta.Subject)}
	var planned []*Item
	for _, it := range cands {
		dir := Direction(in.Team, it.Meta.From.Email)
		switch {
		case in.InYuva[it.DedupeKey]:
			p.Items[it.SourceID] = ItemPlan{State: StateDuplicate, Reason: ReasonInYuva, ThreadKey: key}
			p.Totals.Skipped["already_in_yuva"]++
		case dir == "in" && in.Blocked[it.Meta.From.Email]:
			p.skip([]*Item{it}, key, ReasonBlocked)
		default:
			p.Items[it.SourceID] = ItemPlan{State: StatePlanned, ThreadKey: key}
			planned = append(planned, it)
		}
	}
	if len(planned) == 0 {
		return
	}
	for _, it := range planned {
		if Direction(in.Team, it.Meta.From.Email) == "out" {
			t.Out++
		} else {
			t.In++
			p.Totals.Contacts[it.Meta.From.Email] = true
		}
		for _, l := range labels[it.DedupeKey] {
			if !slices.Contains(t.Labels, l) {
				t.Labels = append(t.Labels, l)
			}
		}
		for _, part := range it.Meta.Parts {
			if in.AttachmentOK == nil || in.AttachmentOK(part.Type, part.Size) {
				p.Totals.Attachments++
				p.Totals.AttachmentBytes += part.Size
			} else {
				p.Totals.AttachmentsSkipped++
			}
		}
		p.Totals.Bytes += it.Size
		t.SourceIDs = append(t.SourceIDs, it.SourceID)
	}
	sort.Strings(t.Labels)
	t.FirstAt, t.LastAt = planned[0].Meta.At, planned[len(planned)-1].Meta.At
	lastDir := Direction(in.Team, planned[len(planned)-1].Meta.From.Email)
	switch {
	case in.CloseAfterDays == 0 || t.LastAt.Before(in.Now.AddDate(0, 0, -in.CloseAfterDays)):
		t.Status = StatusClosed
	case lastDir == "in":
		t.Status = StatusOpen
	default:
		t.Status = StatusPending
	}
	p.Totals.Contacts[contact.Email] = true
	p.Totals.Conversations++
	p.Totals.In += int64(t.In)
	p.Totals.Out += int64(t.Out)
	for _, l := range t.Labels {
		p.Totals.Labels[l]++
	}
	if p.Totals.Oldest.IsZero() || t.FirstAt.Before(p.Totals.Oldest) {
		p.Totals.Oldest = t.FirstAt
	}
	if t.LastAt.After(p.Totals.Newest) {
		p.Totals.Newest = t.LastAt
	}
	p.Threads = append(p.Threads, t)
}

// LabelName is the label a folder becomes: its name without the provider's prefix.
func LabelName(name, delim string) string {
	n := name
	for _, prefix := range []string{"[Gmail]", "[Google Mail]", "INBOX"} {
		if delim != "" && strings.HasPrefix(strings.ToUpper(n), strings.ToUpper(prefix)+delim) {
			n = n[len(prefix)+len(delim):]
		}
	}
	if delim != "" && delim != "/" {
		n = strings.ReplaceAll(n, delim, "/")
	}
	return truncate(strings.TrimSpace(n), 64)
}
