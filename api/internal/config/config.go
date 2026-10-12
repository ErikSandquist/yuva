package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Version     string
	ListenAddr  string
	MetricsAddr string
	DatabaseURL string

	PublicURL      string
	CookieSecure   bool
	ClientIPHeader string
	TrustedProxies []netip.Prefix

	WebAuthnRPID    string
	WebAuthnRPName  string
	WebAuthnOrigins []string

	SignupOpen                bool
	SignupWorkspacesPerPerson int
	SignupWorkspacesPerIP     int

	OperatorEmails []string

	SMTP SMTP

	MasterKey string

	Storage     Storage
	Attachments Attachments
	// EmailImagesMaxBytes caps each workspace's e-mail images; 0 is no limit.
	EmailImagesMaxBytes int64
	// ImportMaxRows caps the data rows of a subscriber CSV import, at most 100,000.
	ImportMaxRows int

	IngressSecret        string
	IngressAcceptV1      bool
	IngressAuthservID    string
	IngressMaxConcurrent int
	SESTopicARNs         []string
	EmailSenderHourlyCap int

	ChatEmailDelay           time.Duration
	DiscussionEmailDelay     time.Duration
	AnonymousContactsPerHour int

	WebhookAllowPrivate bool
	SMTPAllowPrivate    bool

	VAPID VAPID

	MCP    bool
	Panel  bool
	Widget bool

	// Broadcasts is on, approval or off (YUVA_BROADCASTS).
	Broadcasts               string
	BroadcastWorkers         int
	BroadcastDailyRecipients int64
	// MailLinkURL is the base of /m/, /s/, /c/ and /i/ links in mail; PublicURL when unset.
	MailLinkURL string

	// MigrationSources are the sources teams may move from (YUVA_MIGRATION_SOURCES); nil for every
	// source the server has.
	MigrationSources      []string
	MigrationAllowPrivate bool
	// MigrationWorkers is the migrations queue's workers in this process; 0 runs none here.
	MigrationWorkers int
	// MigrationMaxActive caps a workspace's scanning, copying and syncing migrations; 0 is no limit.
	MigrationMaxActive int
	// MigrationMaxBytes caps the bytes a workspace imports a month; 0 is no limit.
	MigrationMaxBytes int64
	// MigrationMaxMessageBytes is the largest message imported; larger ones are skipped.
	MigrationMaxMessageBytes int64
	// MigrationUploadMaxBytes is the largest export file; 0 turns file imports off.
	MigrationUploadMaxBytes int64
	// MigrationSyncMaxDays ends a migration's catch-up sync, and deletes its credentials, after so many days.
	MigrationSyncMaxDays int
	// MigrationAuditDays keeps a migration's audit entries so many days after it ended.
	MigrationAuditDays int
	// ForwardingDomain gives every e-mail channel the address <token>@ForwardingDomain; empty for none.
	ForwardingDomain string
	// MicrosoftClientID and MicrosoftClientSecret are the server's own Microsoft Entra app
	// registration for Sign in with Microsoft (YUVA_MICROSOFT_CLIENT_ID, _CLIENT_SECRET); without
	// them a team signs in with an app registration of its own. MicrosoftTenant is its authority
	// (YUVA_MICROSOFT_TENANT): common for a multi-tenant app, else its tenant.
	MicrosoftClientID     string
	MicrosoftClientSecret string
	MicrosoftTenant       string
}

type VAPID struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

func (v VAPID) Enabled() bool { return v.PublicKey != "" }

type Storage struct {
	Driver            string
	Dir               string
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3PathStyle       bool
}

type Attachments struct {
	MaxBytes int64
	Types    []string
}

var defaultAttachmentTypes = "image/png,image/jpeg,image/gif,image/webp,image/heic,application/pdf,text/plain,text/csv," +
	"application/zip,application/json,video/mp4,video/quicktime,audio/mpeg,audio/mp4"

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLS      string
}

func (s SMTP) Enabled() bool { return s.Host != "" }

func Load(version string) (Config, error) {
	c := Config{
		Version:        version,
		ListenAddr:     env("YUVA_LISTEN_ADDR", ":8080"),
		MetricsAddr:    metricsAddr(),
		DatabaseURL:    env("YUVA_DATABASE_URL", ""),
		PublicURL:      strings.TrimRight(env("YUVA_PUBLIC_URL", "http://localhost:8080"), "/"),
		ClientIPHeader: env("YUVA_CLIENT_IP_HEADER", ""),
		WebAuthnRPName: env("YUVA_WEBAUTHN_RP_NAME", "Yuva"),
		SMTP: SMTP{
			Host:     env("YUVA_SMTP_HOST", ""),
			Username: env("YUVA_SMTP_USERNAME", ""),
			Password: os.Getenv("YUVA_SMTP_PASSWORD"),
			From:     env("YUVA_SMTP_FROM", ""),
			TLS:      env("YUVA_SMTP_TLS", "starttls"),
		},
		MasterKey: os.Getenv("YUVA_MASTER_KEY"),
		Storage: Storage{
			Driver:            env("YUVA_STORAGE", "local"),
			Dir:               env("YUVA_STORAGE_DIR", "data/attachments"),
			S3Endpoint:        env("YUVA_S3_ENDPOINT", ""),
			S3Region:          env("YUVA_S3_REGION", "auto"),
			S3Bucket:          env("YUVA_S3_BUCKET", ""),
			S3AccessKeyID:     env("YUVA_S3_ACCESS_KEY_ID", ""),
			S3SecretAccessKey: os.Getenv("YUVA_S3_SECRET_ACCESS_KEY"),
		},
		Attachments:       Attachments{Types: lowerList(env("YUVA_ATTACHMENT_TYPES", defaultAttachmentTypes))},
		IngressSecret:     strings.TrimSpace(os.Getenv("YUVA_INGRESS_SECRET")),
		IngressAuthservID: env("YUVA_INGRESS_AUTHSERV_ID", ""),
		SESTopicARNs:      list(env("YUVA_SES_TOPIC_ARNS", "")),
		VAPID: VAPID{
			PublicKey:  env("YUVA_VAPID_PUBLIC_KEY", ""),
			PrivateKey: strings.TrimSpace(os.Getenv("YUVA_VAPID_PRIVATE_KEY")),
			Subject:    env("YUVA_VAPID_SUBJECT", ""),
		},
	}
	public, err := url.Parse(c.PublicURL)
	if err != nil || (public.Scheme != "http" && public.Scheme != "https") || public.Host == "" {
		return c, fmt.Errorf("YUVA_PUBLIC_URL must be an absolute http(s) URL, got %q", c.PublicURL)
	}
	c.WebAuthnRPID = env("YUVA_WEBAUTHN_RP_ID", public.Hostname())
	c.WebAuthnOrigins = list(env("YUVA_WEBAUTHN_ORIGINS", public.Scheme+"://"+public.Host))
	if c.CookieSecure, err = boolEnv("YUVA_COOKIE_SECURE", public.Scheme == "https"); err != nil {
		return c, err
	}
	switch c.SMTP.TLS {
	case "starttls", "tls", "none":
	default:
		return c, fmt.Errorf("YUVA_SMTP_TLS must be starttls, tls or none, got %q", c.SMTP.TLS)
	}
	defaultPort := "587"
	if c.SMTP.TLS == "tls" {
		defaultPort = "465"
	}
	if c.SMTP.Port, err = strconv.Atoi(env("YUVA_SMTP_PORT", defaultPort)); err != nil {
		return c, fmt.Errorf("YUVA_SMTP_PORT: %w", err)
	}
	switch c.Storage.Driver {
	case "local", "s3":
	default:
		return c, fmt.Errorf("YUVA_STORAGE must be local or s3, got %q", c.Storage.Driver)
	}
	if c.Storage.S3PathStyle, err = boolEnv("YUVA_S3_PATH_STYLE", false); err != nil {
		return c, err
	}
	for _, v := range list(env("YUVA_TRUSTED_PROXIES", "")) {
		p, err := netip.ParsePrefix(v)
		if err != nil {
			a, aerr := netip.ParseAddr(v)
			if aerr != nil {
				return c, fmt.Errorf("YUVA_TRUSTED_PROXIES: %q is not an IP address or CIDR range", v)
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		c.TrustedProxies = append(c.TrustedProxies, p.Masked())
	}
	if c.WebhookAllowPrivate, err = boolEnv("YUVA_WEBHOOK_ALLOW_PRIVATE", false); err != nil {
		return c, err
	}
	if c.SMTPAllowPrivate, err = boolEnv("YUVA_SMTP_ALLOW_PRIVATE", false); err != nil {
		return c, err
	}
	if c.Attachments.MaxBytes, err = strconv.ParseInt(env("YUVA_ATTACHMENT_MAX_BYTES", "26214400"), 10, 64); err != nil || c.Attachments.MaxBytes < 1 {
		return c, errors.New("YUVA_ATTACHMENT_MAX_BYTES must be a positive number of bytes")
	}
	if c.EmailImagesMaxBytes, err = strconv.ParseInt(env("YUVA_EMAIL_IMAGES_MAX_BYTES", "524288000"), 10, 64); err != nil || c.EmailImagesMaxBytes < 0 {
		return c, errors.New("YUVA_EMAIL_IMAGES_MAX_BYTES must be a number of bytes, 0 for no limit")
	}
	if c.ImportMaxRows, err = strconv.Atoi(env("YUVA_IMPORT_MAX_ROWS", "100000")); err != nil || c.ImportMaxRows < 1 || c.ImportMaxRows > 100000 {
		return c, errors.New("YUVA_IMPORT_MAX_ROWS must be a number of rows from 1 to 100000")
	}
	if c.EmailSenderHourlyCap, err = strconv.Atoi(env("YUVA_EMAIL_SENDER_HOURLY_CAP", "500")); err != nil || c.EmailSenderHourlyCap < 1 {
		return c, errors.New("YUVA_EMAIL_SENDER_HOURLY_CAP must be a positive number of messages")
	}
	if c.ChatEmailDelay, err = time.ParseDuration(env("YUVA_CHAT_EMAIL_DELAY", "5m")); err != nil || c.ChatEmailDelay < time.Second {
		return c, errors.New("YUVA_CHAT_EMAIL_DELAY must be a duration of at least 1s, such as 5m")
	}
	if c.DiscussionEmailDelay, err = time.ParseDuration(env("YUVA_DISCUSSION_EMAIL_DELAY", "10m")); err != nil || c.DiscussionEmailDelay < time.Second {
		return c, errors.New("YUVA_DISCUSSION_EMAIL_DELAY must be a duration of at least 1s, such as 10m")
	}
	if c.MCP, err = switchEnv("YUVA_MCP", true); err != nil {
		return c, err
	}
	if c.Panel, err = switchEnv("YUVA_PANEL", true); err != nil {
		return c, err
	}
	if c.Widget, err = switchEnv("YUVA_WIDGET", true); err != nil {
		return c, err
	}
	if c.IngressAcceptV1, err = boolEnv("YUVA_INGRESS_ACCEPT_V1", false); err != nil {
		return c, err
	}
	if c.IngressMaxConcurrent, err = strconv.Atoi(env("YUVA_INGRESS_MAX_CONCURRENT", "8")); err != nil || c.IngressMaxConcurrent < 1 {
		return c, errors.New("YUVA_INGRESS_MAX_CONCURRENT must be a positive number of messages")
	}
	switch strings.ToLower(env("YUVA_SIGNUP", "invite")) {
	case "invite":
	case "open":
		c.SignupOpen = true
	default:
		return c, errors.New("YUVA_SIGNUP must be invite or open")
	}
	if c.SignupWorkspacesPerPerson, err = strconv.Atoi(env("YUVA_SIGNUP_WORKSPACES_PER_PERSON", "3")); err != nil || c.SignupWorkspacesPerPerson < 1 {
		return c, errors.New("YUVA_SIGNUP_WORKSPACES_PER_PERSON must be a positive number of workspaces")
	}
	if c.SignupWorkspacesPerIP, err = strconv.Atoi(env("YUVA_SIGNUP_WORKSPACES_PER_IP", "5")); err != nil || c.SignupWorkspacesPerIP < 1 {
		return c, errors.New("YUVA_SIGNUP_WORKSPACES_PER_IP must be a positive number of workspaces")
	}
	for _, v := range list(env("YUVA_OPERATOR_EMAIL", "")) {
		a, err := mail.ParseAddress(v)
		if err != nil {
			return c, fmt.Errorf("YUVA_OPERATOR_EMAIL: %q is not an e-mail address", v)
		}
		c.OperatorEmails = append(c.OperatorEmails, a.Address)
	}
	if c.AnonymousContactsPerHour, err = strconv.Atoi(env("YUVA_ANONYMOUS_CONTACTS_PER_HOUR", "20")); err != nil || c.AnonymousContactsPerHour < 1 {
		return c, errors.New("YUVA_ANONYMOUS_CONTACTS_PER_HOUR must be a positive number of visitors")
	}
	if err := c.loadVAPID(); err != nil {
		return c, err
	}
	if err := c.loadMigrations(); err != nil {
		return c, err
	}
	if err := c.loadBroadcasts(); err != nil {
		return c, err
	}
	if c.SMTP.Enabled() && c.SMTP.From == "" {
		return c, errors.New("YUVA_SMTP_FROM is required when YUVA_SMTP_HOST is set")
	}
	return c, nil
}

func (c *Config) loadBroadcasts() error {
	def := "on"
	if c.SignupOpen {
		def = "approval"
	}
	c.Broadcasts = strings.ToLower(env("YUVA_BROADCASTS", def))
	switch c.Broadcasts {
	case "on", "approval", "off":
	default:
		return errors.New("YUVA_BROADCASTS must be on, approval or off")
	}
	var err error
	if c.BroadcastWorkers, err = strconv.Atoi(env("YUVA_BROADCAST_WORKERS", "2")); err != nil || c.BroadcastWorkers < 1 {
		return errors.New("YUVA_BROADCAST_WORKERS must be a positive number of workers")
	}
	if c.BroadcastDailyRecipients, err = strconv.ParseInt(env("YUVA_BROADCAST_DAILY_RECIPIENTS", "0"), 10, 64); err != nil || c.BroadcastDailyRecipients < 0 {
		return errors.New("YUVA_BROADCAST_DAILY_RECIPIENTS must be a number of recipients, 0 for no limit")
	}
	c.MailLinkURL = strings.TrimRight(env("YUVA_MAIL_LINK_URL", c.PublicURL), "/")
	u, err := url.Parse(c.MailLinkURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("YUVA_MAIL_LINK_URL must be an absolute http(s) URL without a path, got %q", c.MailLinkURL)
	}
	return nil
}

var migrationSources = []string{"imap", "file", "zendesk", "helpscout"}

var forwardingDomainRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
var microsoftTenantRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)

func (c *Config) loadMigrations() error {
	var err error
	if c.MigrationAllowPrivate, err = boolEnv("YUVA_MIGRATION_ALLOW_PRIVATE", false); err != nil {
		return err
	}
	if c.MigrationWorkers, err = strconv.Atoi(env("YUVA_MIGRATION_WORKERS", "2")); err != nil || c.MigrationWorkers < 0 {
		return errors.New("YUVA_MIGRATION_WORKERS must be a number of workers, 0 for none in this process")
	}
	if c.MigrationMaxActive, err = strconv.Atoi(env("YUVA_MIGRATION_MAX_ACTIVE", "0")); err != nil || c.MigrationMaxActive < 0 {
		return errors.New("YUVA_MIGRATION_MAX_ACTIVE must be a number of migrations, 0 for no limit")
	}
	if c.MigrationMaxBytes, err = strconv.ParseInt(env("YUVA_MIGRATION_MAX_BYTES", "0"), 10, 64); err != nil || c.MigrationMaxBytes < 0 {
		return errors.New("YUVA_MIGRATION_MAX_BYTES must be a number of bytes, 0 for no limit")
	}
	if c.MigrationMaxMessageBytes, err = strconv.ParseInt(env("YUVA_MIGRATION_MAX_MESSAGE_BYTES", "52428800"), 10, 64); err != nil || c.MigrationMaxMessageBytes < 1 {
		return errors.New("YUVA_MIGRATION_MAX_MESSAGE_BYTES must be a positive number of bytes")
	}
	if c.MigrationUploadMaxBytes, err = strconv.ParseInt(env("YUVA_MIGRATION_UPLOAD_MAX_BYTES", "21474836480"), 10, 64); err != nil || c.MigrationUploadMaxBytes < 0 {
		return errors.New("YUVA_MIGRATION_UPLOAD_MAX_BYTES must be a number of bytes, 0 for no file imports")
	}
	if c.MigrationSyncMaxDays, err = strconv.Atoi(env("YUVA_MIGRATION_SYNC_MAX_DAYS", "30")); err != nil || c.MigrationSyncMaxDays < 1 || c.MigrationSyncMaxDays > 365 {
		return errors.New("YUVA_MIGRATION_SYNC_MAX_DAYS must be 1 to 365 days")
	}
	if c.MigrationAuditDays, err = strconv.Atoi(env("YUVA_MIGRATION_AUDIT_DAYS", "365")); err != nil || c.MigrationAuditDays < 1 || c.MigrationAuditDays > 36500 {
		return errors.New("YUVA_MIGRATION_AUDIT_DAYS must be 1 to 36500 days")
	}
	c.ForwardingDomain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(os.Getenv("YUVA_FORWARDING_DOMAIN"))), ".")
	if c.ForwardingDomain != "" && !forwardingDomainRe.MatchString(c.ForwardingDomain) {
		return fmt.Errorf("YUVA_FORWARDING_DOMAIN must be a domain name such as forward.example.com, got %q", c.ForwardingDomain)
	}
	c.MicrosoftClientID = strings.TrimSpace(os.Getenv("YUVA_MICROSOFT_CLIENT_ID"))
	c.MicrosoftClientSecret = os.Getenv("YUVA_MICROSOFT_CLIENT_SECRET")
	if (c.MicrosoftClientID == "") != (c.MicrosoftClientSecret == "") {
		return errors.New("set both YUVA_MICROSOFT_CLIENT_ID and YUVA_MICROSOFT_CLIENT_SECRET, or neither")
	}
	c.MicrosoftTenant = strings.TrimSpace(env("YUVA_MICROSOFT_TENANT", "common"))
	if !microsoftTenantRe.MatchString(c.MicrosoftTenant) {
		return fmt.Errorf("YUVA_MICROSOFT_TENANT must be common, organizations, consumers, a tenant id or a verified domain, got %q", c.MicrosoftTenant)
	}
	if _, set := os.LookupEnv("YUVA_MIGRATION_SOURCES"); set {
		c.MigrationSources = []string{}
		for _, s := range lowerList(os.Getenv("YUVA_MIGRATION_SOURCES")) {
			if !slices.Contains(migrationSources, s) {
				return fmt.Errorf("YUVA_MIGRATION_SOURCES: %q is not one of imap, file, zendesk, helpscout", s)
			}
			c.MigrationSources = append(c.MigrationSources, s)
		}
	}
	if c.MigrationUploadMaxBytes == 0 {
		if c.MigrationSources == nil {
			c.MigrationSources = slices.Clone(migrationSources)
		}
		c.MigrationSources = slices.DeleteFunc(c.MigrationSources, func(s string) bool { return s == "file" })
	}
	return nil
}

func (c *Config) loadVAPID() error {
	v := &c.VAPID
	if (v.PublicKey == "") != (v.PrivateKey == "") {
		return errors.New("set both YUVA_VAPID_PUBLIC_KEY and YUVA_VAPID_PRIVATE_KEY, or neither (yuva vapid-keys makes a pair)")
	}
	if !v.Enabled() {
		return nil
	}
	if v.Subject == "" {
		switch {
		case c.SMTP.From != "":
			addr, err := mail.ParseAddress(c.SMTP.From)
			if err != nil {
				return fmt.Errorf("YUVA_SMTP_FROM: %w", err)
			}
			v.Subject = "mailto:" + addr.Address
		case strings.HasPrefix(c.PublicURL, "https://"):
			v.Subject = c.PublicURL
		default:
			return errors.New("YUVA_VAPID_SUBJECT is required (mailto:you@example.com or an https URL)")
		}
	}
	if !strings.HasPrefix(v.Subject, "mailto:") && !strings.HasPrefix(v.Subject, "https://") {
		return fmt.Errorf("YUVA_VAPID_SUBJECT must start with mailto: or https://, got %q", v.Subject)
	}
	return nil
}

func (c Config) RequireDatabase() error {
	if c.DatabaseURL == "" {
		return errors.New("YUVA_DATABASE_URL is required")
	}
	return nil
}

func (c Config) RequireMasterKey() error {
	if strings.TrimSpace(c.MasterKey) == "" {
		return errors.New("YUVA_MASTER_KEY is required: 32 random bytes, base64-encoded (openssl rand -base64 32)")
	}
	return nil
}

func lowerList(v string) []string {
	var out []string
	for _, item := range list(v) {
		out = append(out, strings.ToLower(item))
	}
	return out
}

func metricsAddr() string {
	v, ok := os.LookupEnv("YUVA_METRICS_ADDR")
	if !ok {
		return ":9090"
	}
	v = strings.TrimSpace(v)
	if strings.EqualFold(v, "off") {
		return ""
	}
	return v
}

func env(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func boolEnv(name string, fallback bool) (bool, error) {
	v := env(name, "")
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return b, nil
}

func switchEnv(name string, fallback bool) (bool, error) {
	switch strings.ToLower(env(name, "")) {
	case "":
		return fallback, nil
	case "on":
		return true, nil
	case "off":
		return false, nil
	}
	return false, fmt.Errorf("%s must be on or off", name)
}

func list(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, strings.TrimRight(item, "/"))
		}
	}
	return out
}
