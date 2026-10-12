// Package migrate holds what moving a mailbox into Yuva needs before any mail is read: the
// provider presets, detection from an address, the address checks for the connection, and the
// connection test.
package migrate

import "strings"

type Server struct {
	Host string
	Port int
	TLS  string
}

type Budget struct {
	Connections    int
	BytesPerDay    int64
	BytesPerSecond int64
}

type Preset struct {
	ID          string
	Source      string
	Name        string
	Servers     []Server
	Auth        []string
	AppPassword bool
	Username    string
	Folders     []string
	Budget      Budget
	Guide       string
	Available   bool
}

const (
	AuthPassword  = "password"
	AuthMicrosoft = "microsoft"

	UsernameAddress   = "address"
	UsernameLocalPart = "local_part"

	FolderUser = "user"

	PresetGmail           = "gmail"
	PresetGoogleWorkspace = "google_workspace"
	PresetMicrosoft       = "microsoft"
	PresetYandex          = "yandex"
	PresetICloud          = "icloud"
	PresetZoho            = "zoho"
	PresetYahoo           = "yahoo"
	PresetFastmail        = "fastmail"
	PresetGMX             = "gmx"
	PresetIMAP            = "imap"
)

func tls993(hosts ...string) []Server {
	out := make([]Server, len(hosts))
	for i, h := range hosts {
		out[i] = Server{Host: h, Port: 993, TLS: "tls"}
	}
	return out
}

var (
	gmailBudget   = Budget{Connections: 1, BytesPerDay: 1500 * 1000 * 1000}
	defaultBudget = Budget{Connections: 1, BytesPerSecond: 4 * 1000 * 1000}
	inboxSentUser = []string{"INBOX", `\Sent`, FolderUser}
	withArchive   = []string{"INBOX", `\Sent`, `\Archive`, FolderUser}
)

var zohoHosts = func() []string {
	var out []string
	for _, tld := range []string{"com", "eu", "in", "com.au", "jp", "ca", "sa"} {
		out = append(out, "imappro.zoho."+tld, "imap.zoho."+tld)
	}
	return out
}()

// Presets are the providers in the order the panel shows them.
var Presets = []Preset{
	{ID: PresetGmail, Name: "Gmail", Servers: tls993("imap.gmail.com"), Auth: []string{AuthPassword}, AppPassword: true,
		Username: UsernameAddress, Folders: []string{`\All`}, Budget: gmailBudget, Guide: "migrate-gmail", Available: true},
	{ID: PresetGoogleWorkspace, Name: "Google Workspace", Servers: tls993("imap.gmail.com"), Auth: []string{AuthPassword}, AppPassword: true,
		Username: UsernameAddress, Folders: []string{`\All`}, Budget: gmailBudget, Guide: "migrate-google-workspace", Available: true},
	{ID: PresetMicrosoft, Name: "Outlook.com and Microsoft 365", Servers: tls993("outlook.office365.com"), Auth: []string{AuthMicrosoft},
		Username: UsernameAddress, Folders: withArchive, Budget: defaultBudget, Guide: "migrate-microsoft"},
	{ID: PresetYandex, Name: "Yandex", Servers: tls993("imap.yandex.com"), Auth: []string{AuthPassword}, AppPassword: true,
		Username: UsernameAddress, Folders: inboxSentUser, Budget: defaultBudget, Guide: "migrate-yandex", Available: true},
	{ID: PresetICloud, Name: "iCloud Mail", Servers: tls993("imap.mail.me.com"), Auth: []string{AuthPassword}, AppPassword: true,
		Username: UsernameLocalPart, Folders: withArchive, Budget: defaultBudget, Guide: "migrate-icloud", Available: true},
	{ID: PresetZoho, Name: "Zoho Mail", Servers: tls993(zohoHosts...), Auth: []string{AuthPassword},
		Username: UsernameAddress, Folders: inboxSentUser, Budget: defaultBudget, Guide: "migrate-zoho", Available: true},
	{ID: PresetYahoo, Name: "Yahoo Mail", Servers: tls993("imap.mail.yahoo.com"), Auth: []string{AuthPassword}, AppPassword: true,
		Username: UsernameAddress, Folders: withArchive, Budget: defaultBudget, Guide: "migrate-yahoo", Available: true},
	{ID: PresetFastmail, Name: "Fastmail", Servers: tls993("imap.fastmail.com"), Auth: []string{AuthPassword}, AppPassword: true,
		Username: UsernameAddress, Folders: withArchive, Budget: defaultBudget, Guide: "migrate-fastmail", Available: true},
	{ID: PresetGMX, Name: "GMX", Servers: tls993("imap.gmx.net", "imap.gmx.com", "imap.mail.com"), Auth: []string{AuthPassword},
		Username: UsernameAddress, Folders: inboxSentUser, Budget: defaultBudget, Guide: "migrate-gmx", Available: true},
	{ID: PresetIMAP, Name: "Other IMAP server", Auth: []string{AuthPassword},
		Username: UsernameAddress, Folders: withArchive, Budget: defaultBudget, Guide: "migrate-imap", Available: true},
}

func init() {
	for i := range Presets {
		Presets[i].Source = "imap"
	}
}

func LookupPreset(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// ServerFor picks the preset's server named by host, or its first when host is empty.
func (p Preset) ServerFor(host string) (Server, bool) {
	if len(p.Servers) == 0 {
		return Server{}, false
	}
	if host == "" {
		return p.Servers[0], true
	}
	for _, s := range p.Servers {
		if strings.EqualFold(s.Host, host) {
			return s, true
		}
	}
	return Server{}, false
}

// MicrosoftHost reports a Microsoft server, which accepts no passwords.
func MicrosoftHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, s := range []string{"office365.com", "outlook.com", "hotmail.com", "live.com"} {
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}
