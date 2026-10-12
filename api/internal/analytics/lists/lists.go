// Package lists holds the third-party lists analytics matches against, as fetched by update.go:
// crawler user agents, referrer spam domains, and search engines, social networks and AI
// assistants by host. Lines starting with # are comments.
package lists

import _ "embed"

//go:embed crawlers.txt
var Crawlers string

//go:embed spammers.txt
var Spammers string

//go:embed sources.tsv
var Sources string
