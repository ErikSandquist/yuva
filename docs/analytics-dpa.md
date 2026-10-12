# Data processing for Analytics

If a Yuva server is run for you by someone else (a hosted service), that operator processes your visitors'
data when you use Analytics. This is the outline of the agreement that Article 28 of the GDPR asks for between
you and the operator. It is a starting point to adapt with your lawyer, not legal advice, and not a signed
contract. On your own server you are the controller and nobody processes the data for you, so you need none of
this.

## Parties and roles

| Party | Role | Does |
|---|---|---|
| The workspace owner (you) | Controller | Decides why and how visitors are counted: which websites, which settings, how long. Tells visitors ([privacy notice](analytics-privacy.md#your-privacy-notice)). Answers their requests. |
| The server operator | Processor | Runs the server, receives the events your pages send, keeps and deletes them as the settings say, and does nothing else with them. |

## What is processed

| Item | Detail |
|---|---|
| Subject matter | Audience measurement for the websites of your workspace. |
| Data subjects | Visitors to those websites. |
| Categories of data | Shortened IP address (used in memory and not stored), browser and device type, visitor hash, page host and path, referrer, `utm_*` parameters, country, region and city (country only for a children's website), language, screen size class. The [privacy page](analytics-privacy.md#what-it-keeps) has the full list. |
| Special categories | None are asked for. The server does not keep values shaped like an e-mail address. |
| Duration | Detailed records 90 days by default (set by the operator, 7 to 400); daily totals 25 months by default (`retention_months` per website); the agreement's term for the rest. |

## Instructions

The processor handles the data only on your documented instructions (Art. 28(3)(a)). The settings of a
website are those instructions: its domains, audience, collection mode, excluded pages and addresses, path
rules and retention.

## Confidentiality and security

- Staff who can reach the data are bound to confidentiality.
- The data travels over TLS. The operator describes how it protects stored data and backups, and who has
  access by role.
- The operator takes the security measures Article 32 requires (Art. 28(3)(c)).

## Sub-processors

Yuva Analytics itself uses none: the location lookup and the browser parsing run inside the server. The
operator's infrastructure providers (hosting, storage, backups) can handle the data on its behalf. The operator
names them, or says how you learn about them, and states the conditions under which it engages another
processor (Art. 28(3)(d)).

## Helping with requests

In the default mode a visitor's records cannot be found, because nothing identifies a visitor beyond a day. The
processor helps you answer requests with that statement. When you delete data, it is deleted for good.

## Deletion and return

You can take and delete your data yourself, at any time:

- `POST /v1/analytics/properties/{propertyId}/query` with `?format=csv` returns the numbers as CSV.
- `POST /v1/analytics/properties/{propertyId}/delete-data` deletes a range of days, or everything with
  `all: true`; `DELETE /v1/analytics/properties/{propertyId}` removes the website with its data.
- When the agreement ends, the operator deletes what is left at your choice, or returns it, and deletes copies
  unless the law requires keeping them. Backups age out in the operator's backup period, which the operator
  states.

## Audits

The operator gives you the information needed to show that it follows this agreement, and allows audits by you or
an auditor you name (Art. 28(3)(h)).

## Transfers

The operator states in which country the server and its backups are, and relies on a valid transfer mechanism
wherever the law asks for one.

## Source

[GDPR Article 28, processor](https://www.legislation.gov.uk/eur/2016/679/article/28) (UK GDPR text; the EU
GDPR has the same article).
