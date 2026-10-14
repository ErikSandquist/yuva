# Analytics

Yuva Analytics zählt Besucher, Seiten, Quellen, Standorte und Geräte Ihrer eigenen Websites, ohne Cookies
und ohne IP-Adressen zu speichern. Es lebt im selben Panel wie Ihr Posteingang, und seine Zahlen stehen
auch über die [API](#api) bereit.

## Was gezählt wird

| Es tut | Es tut nicht |
|---|---|
| Zählt Seitenaufrufe, wie lange eine Seite sichtbar war und wie weit gescrollt wurde | Setzt ein Cookie oder schreibt etwas auf das Gerät des Besuchers |
| Kürzt die IP-Adresse (IPv4 `/24`, IPv6 `/48`), bevor irgendetwas anderes sie nutzt, ermittelt daraus den Ort und vergisst sie | Speichert irgendwo eine IP-Adresse, schon gar keine vollständige |
| Zählt einen Besucher einmal pro Tag, aus einem Hash von gekürzter Adresse und Browser unter einem Salt, der nur einen Tag existiert | Verfolgt einen Besucher von einem Tag zum nächsten oder von einer Website zur anderen |
| Behält Host und Pfad der Seite, Host und Pfad des Referrers und die `utm_*`-Parameter | Behält den Rest der Query-Zeichenfolge oder irgendetwas, das wie eine E-Mail-Adresse aussieht |
| Lässt Bots aus, und Besucher, deren Browser Do Not Track oder Global Privacy Control sendet | Zählt diese Besucher gar nicht: Es wird nichts gesendet |

**Besucher werden also pro Tag gezählt.** Über eine Woche ist die Besucherzahl die Summe der Besucher jedes
Tages: Wer an einem anderen Tag wiederkommt, wird erneut gezählt. Ein **Besuch** sind die Seitenaufrufe eines
Besuchers ohne Pause von 30 Minuten, innerhalb eines Tages.

Das Einzige, was das Skript je auf einem Gerät speichert, ist die eigene Entscheidung eines Besuchers
auszusteigen ([unten](#opt-out-links)).

Was gespeichert wird und wie lange, ob Ihre Website ein Cookie-Banner braucht, und Texte für Ihre
Datenschutzerklärung stehen unter [Analytics und Datenschutz](analytics-privacy.md).

## Eine Website hinzufügen

1. Öffnen Sie im Panel **Analytics → Add a website**. Websites hinzufügen können Owner und Admins.
2. Geben Sie einen Namen ein und tragen Sie die **Domains** ein, eine pro Zeile. Seiten auf anderen Domains
   können keine Events senden. `*.example.com` deckt jede Subdomain ab, aber nicht `example.com` selbst.
3. Wählen Sie die **Time zone** (Tage werden darin gezählt), den Posteingang (**Inbox**), dessen Agents die Zahlen sehen
   dürfen (oder keinen, dann nur Owner und Admins), und die **Audience**. Wählen Sie Children für eine
   Website, die für Kinder gemacht ist: Es wird nur das Land gespeichert, nie Region oder Stadt.
4. Wählen Sie **Add and install**, kopieren Sie den Snippet, der zum Aufbau Ihrer Website passt (unten), und
   setzen Sie ihn auf Ihre Seiten. Die Seite wird grün, sobald der erste Seitenaufruf ankommt. Ein Browser mit
   Do Not Track oder Global Privacy Control sendet nichts; testen Sie also mit einem anderen.

Der Schlüssel der Website (`yuva_ak_…`) ist öffentlich wie ein Chat-Schlüssel: Jede Seite trägt ihn, und er
funktioniert nur auf den Domains der Website. Ein Workspace kann nur eine begrenzte Zahl Websites haben; eine
darüber hinaus wird mit `403 property_limit` abgelehnt.

## Installieren

Ersetzen Sie `https://support.example.com` durch Ihren Yuva-Server und den Schlüssel durch den Ihrer Website.
Das Panel zeigt diese Snippets kopierfertig unter **Settings → Install**.

### Script-Tag

```html
<script defer data-key="yuva_ak_xxxxxxxx" src="https://support.example.com/yuva-analytics.js"></script>
```

Setzen Sie ihn in den `<head>` jeder Seite. Er zählt Seitenaufrufe, auch wenn eine Single-Page-App die
Adresse ändert; eine Single-Page-App braucht also nichts weiter. Ein Server, der mit `YUVA_WIDGET=off` oder
`YUVA_ANALYTICS=off` gestartet wurde, liefert das Skript nicht aus.

| Attribut | Bedeutung |
|---|---|
| `data-key` | Der Schlüssel der Website. Pflicht. |
| `data-api` | Die Adresse des Yuva-Servers, wenn sie von der abweicht, von der das Skript geladen wurde ([eigene Domain](#von-der-eigenen-domain-ausliefern)). |
| `data-hash` | Die Website routet mit dem Teil nach `#`: Er wird Teil des Seitenpfads. |
| `data-manual` | Keine automatischen Seitenaufrufe; Sie rufen `yuva.pageview()` selbst auf. |
| `data-consent="required"` | Sendet nichts, bis `yuva.consent({ analytics: true })` aufgerufen wird ([Consent-Modus](#consent-modus)). |

Mit `data-manual` rufen Sie `window.yuva.pageview()` auf, wenn eine Seite angezeigt wird, oder
`window.yuva.pageview({ url, referrer })`, um die Seite selbst zu benennen.

### React und Next.js

```sh
npm install useyuva
```

Rendern Sie die Komponente einmal im Root-Layout. Sie rendert nichts:

```tsx
import { YuvaAnalytics } from "useyuva/react"

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html>
      <body>
        {children}
        <YuvaAnalytics server="https://support.example.com" publicKey="yuva_ak_xxxxxxxx" />
      </body>
    </html>
  )
}
```

`hash`, `consent="required"` und `manual` sind ebenfalls Props. `useYuvaAnalytics()` liefert überall darunter
`pageview`, `consent`, `optOut` und `optIn`.

### Nuxt

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: {
    server: "https://support.example.com",
    analytics: { key: "yuva_ak_xxxxxxxx" },
  },
})
```

Das Modul fügt ein Client-Plugin und das Composable `useYuvaAnalytics()` hinzu.
`NUXT_PUBLIC_YUVA_ANALYTICS_KEY` überschreibt den Schlüssel zur Laufzeit.

### Vue, Astro und andere Bundler

```ts
import { createAnalytics } from "useyuva/analytics"

const analytics = createAnalytics({ server: "https://support.example.com", key: "yuva_ak_xxxxxxxx" })
```

Rufen Sie es einmal im Browser auf: in Vue in `main.ts`, in Astro in einem `<script>` Ihres Layouts, sonst in
Ihrer Einstiegsdatei. Beim Rendern auf dem Server tut es nichts. Es liefert `pageview`, `consent`, `optOut`,
`optIn` und `destroy` und nimmt dieselben Optionen `hash`, `consent: "required"` und `manual`.

## Consent-Modus

Zählen ohne Cookies braucht nach den meisten Datenschutzgesetzen kein Banner, aber manche Websites müssen
vorher fragen. Für sie:

1. Installieren Sie das Skript mit `data-consent="required"` (`consent="required"` in React, Nuxt und
   `createAnalytics`). Es sendet nichts, bis Sie sagen, dass der Besucher zugestimmt hat.
2. Rufen Sie im Callback Ihres eigenen Consent-Tools Folgendes auf. Tun Sie das bei **jedem Laden einer
   Seite**, wenn der Besucher schon zugestimmt hat, denn das Skript merkt sich die Antwort nicht:

   ```js
   window.yuva.consent({ analytics: true })
   ```

   Ist das Skript noch nicht geladen, reihen Sie den Aufruf ein:
   `;(window.yuvaq = window.yuvaq || []).push(["consent", { analytics: true }])`.
3. Setzen Sie **Collection** unter **Settings → General** der Website auf **Only with consent**. Der Server
   lehnt dann jedes Event ab, das nicht sagt, dass der Besucher zugestimmt hat.

`yuva.consent({ analytics: false })` stoppt das Senden wieder.

Zustimmende Besucher lassen sich mit einer ID auf ihrem Gerät, bis zu 13 Monate lang, auch über Tage hinweg
wiedererkennen. Das ist pro Property optional: siehe [Consent-Funktionen](analytics-consent.md).

## Opt-out-Links

Setzen Sie diese beiden Links auf Ihre Datenschutzseite:

```html
<a href="#yuva-optout">Meine Besuche nicht zählen</a>
<a href="#yuva-optin">Meine Besuche wieder zählen</a>
```

Der Ausstieg ist das Einzige, was das Skript auf dem Gerät des Besuchers speichert, und nur, wenn er es
wünscht. Sie können auch `window.yuva.optOut()` und `window.yuva.optIn()` aufrufen.

## Was nichts sendet

Das Skript sendet nichts und der Server zählt nichts bei: Browsern mit Do Not Track oder Global Privacy
Control, automatisierten Browsern (`navigator.webdriver`), Besuchern, die ausgestiegen sind, und bekannten
Bots. Seiten und Adressen, die Sie unter **Settings → Tracking** eintragen (**Pages not counted**,
**Addresses not counted**), bleiben ebenfalls draußen, und **Path rules** zählen Seiten, die zu einem Muster
passen, unter einem Pfad, sodass alle Bestellseiten als `/orders/:id` zusammenlaufen.

## Von der eigenen Domain ausliefern

Content-Blocker stoppen oft Skripte von anderen Domains. Liefern Sie das Skript und seinen Endpunkt von Ihrer
eigenen Website aus, werden auch diese Besucher gezählt. Zwei Dinge sind zu beachten:

- Browser senden Events an `/client/v1/analytics/<Schlüssel>` relativ zum Server des Skripts. Mit `data-api`
  (oder `server`) können Sie diesem Server einen Pfadpräfix wie `https://www.example.com/stats` geben.
- Ihr Yuva-Server sieht den Proxy, nicht den Besucher. Lassen Sie den Proxy die Adresse des Besuchers im Header
  `X-Forwarded-For` senden, wie es die Beispiele unten tun. Ohne ihn sieht jeder Besucher wie der Proxy aus: alle
  werden als einer gezählt, und das Limit von 120 Events pro Minute und Adresse gilt für alle zusammen.

**nginx**, im `server`-Block Ihrer Website:

```nginx
location = /yuva-analytics.js {
  proxy_pass https://support.example.com/yuva-analytics.js;
  proxy_ssl_server_name on;
}
location /client/v1/analytics/ {
  proxy_pass https://support.example.com/client/v1/analytics/;
  proxy_ssl_server_name on;
  proxy_set_header X-Forwarded-For $remote_addr;
}
```

```html
<script defer data-key="yuva_ak_xxxxxxxx" src="/yuva-analytics.js"></script>
```

**Next.js**, unter einem neutralen Pfad: Ein Rewrite liefert das Skript, ein Route Handler reicht die Events mit der
Adresse des Besuchers weiter. Nehmen Sie die Adresse, die Ihre Hosting-Plattform setzt (`x-real-ip` oder den ersten
Eintrag von `x-forwarded-for`), nie eine, die ein Besucher selbst schreiben könnte. In `next.config.js`:

```js
module.exports = {
  async rewrites() {
    return [{ source: "/stats/yuva-analytics.js", destination: "https://support.example.com/yuva-analytics.js" }]
  },
}
```

In `app/stats/client/v1/analytics/[key]/route.ts`:

```ts
export async function POST(request: Request, { params }: { params: Promise<{ key: string }> }) {
  const { key } = await params
  const visitor = request.headers.get("x-real-ip") ?? request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ?? ""
  const headers = new Headers({ "Content-Type": "text/plain", "X-Forwarded-For": visitor })
  for (const name of ["origin", "user-agent", "accept-language", "dnt", "sec-gpc"]) {
    const value = request.headers.get(name)
    if (value) headers.set(name, value)
  }
  const upstream = await fetch(`https://support.example.com/client/v1/analytics/${key}`, {
    method: "POST",
    headers,
    body: await request.text(),
  })
  return new Response(null, { status: upstream.status })
}
```

```html
<script defer data-key="yuva_ak_xxxxxxxx" data-api="https://www.example.com/stats" src="/stats/yuva-analytics.js"></script>
```

**Cloudflare Workers**, mit einer Route wie `www.example.com/stats/*`:

```js
export default {
  async fetch(request) {
    const url = new URL(request.url)
    const target = new URL(url.pathname.slice("/stats".length) + url.search, "https://support.example.com")
    const headers = new Headers(request.headers)
    headers.set("X-Forwarded-For", request.headers.get("CF-Connecting-IP") ?? "")
    return fetch(target, { method: request.method, headers, body: request.body })
  },
}
```

Verwenden Sie denselben `<script>`-Tag wie bei Next.js. In beiden Fällen muss der `Origin` des Browsers eine
der Domains der Website sein.

## Die Ansichten

Öffnen Sie im Panel **Analytics** und wählen Sie eine Website. Jede Ansicht zeigt den gewählten Zeitraum
(Today bis Last 12 months oder ein eigener Zeitraum von höchstens 400 Tagen), und ihre Tabs sind:

| Ansicht | Zeigt |
|---|---|
| **Overview** | Visitors, Visits, Pageviews, Views per visit, Bounce rate und Visit duration im Zeitverlauf (pro Stunde, Tag oder Woche) sowie die Top-Seiten, Quellen, Standorte und Geräte. |
| **Realtime** | Besucher und Seitenaufrufe der letzten 30 Minuten, pro Minute, mit den Top-Seiten, Quellen und Ländern. Aktualisiert sich alle 15 Sekunden. |
| **Pages** | Seiten, Einstiegsseiten, Ausstiegsseiten und Hosts, mit Verweildauer und Scrolltiefe. |
| **Sources** | Channels, Quellen, Referrer und die fünf `utm_*`-Parameter. |
| **Locations** | Länder, Regionen und Städte (bei Audience Children nur Länder). |
| **Devices** | Gerät, Browser und Version, Betriebssystem und Version, Bildschirmgröße und Sprache. |
| **Events** | Eigene und eingebaute Events mit ihren Props und Umsätzen. |
| **Goals** | Conversions, Conversion-Rate und Umsatz der Ziele, die Sie festlegen. |
| **Funnels** | Wie Besucher eine Kette von Zielen durchlaufen und wo sie aussteigen. |
| **Settings** | General, Tracking, Install, Health und Data. |

Events, Goals und Funnels stehen unter [Events, Goals und Funnels](analytics-events.md).

Eine mit einem Posteingang verknüpfte Website zeigt außerdem, wie viel Support ihre Seiten auslösen, mit Insights,
Alerts und E-Mail-Berichten: [Support und Traffic](analytics-support.md).

Dieselbe Property zählt auch Ihre iOS- und Android-Apps ([Analytics in Apps](analytics-apps.md)) und die Events,
die Ihr Backend sendet ([Serverseitige Events](analytics-server.md)). Eine Property der Art `api` zählt stattdessen
die Requests Ihrer eigenen API: [API-Monitoring](analytics-api.md).

**Channels** ordnen ein, woher ein Besuch kam: Direct, Organic search, Paid search, Social, Paid social,
E-mail, Referral, **AI assistants** (Besuche von ChatGPT und anderen Assistenten) und Internal.

Auf einem Standard-Server stammen die **Locations** aus der Datenbank DB-IP Lite. DB-IP verlangt überall
dort, wo Standorte gezeigt werden, eine Namensnennung; das Panel zeigt darunter „IP Geolocation by DB-IP“.

### Filter, Vergleich und CSV

- Wählen Sie eine Tabellenzeile (eine Seite, eine Quelle, ein Land), um **danach zu filtern**; Filter stehen
  über den Zahlen, lassen sich ändern oder entfernen und kombinieren. Operatoren: is, is not, contains, does
  not contain, matches (`*` ist ein Pfadteil, `**` beliebig viele).
- **Comparison** stellt den Zeitraum der vorherigen Periode, dem Vorjahr oder anderen Tagen Ihrer Wahl
  gegenüber.
- **Download CSV** speichert die Tabelle, die Sie sehen, mit einer Spalte je Aufschlüsselung und Metrik. Größere
  Dateien, Tageszahlen und Rohereignisse sind [Exporte](analytics-sharing.md#exporte); ein Bericht für Menschen ohne
  Konto ist ein [Freigabelink](analytics-sharing.md#freigabelinks).
- Manche Filterkombinationen brauchen die Detaildaten, die für die letzten
  [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics) Tage (standardmäßig 90) aufbewahrt werden. Ältere
  Tage haben nur Summen je Seite, Quelle, Ort und Gerät, und das Panel sagt es.

### Health

**Settings → Health** zeigt für die letzten 30 Tage die gezählten und verworfenen Events pro Tag und warum
(Bots, Rate limits, Other sites, Excluded, Monthly limit, Invalid, Too late, Server busy), wann das letzte
Event ankam, seit wann Detaildaten aufbewahrt werden und welche Standortdatenbank in Gebrauch ist. Von einem
verworfenen Event wird nur die Zahl gespeichert.

## Schlüssel und Daten

- **Make a new key** (Settings → Install) gibt der Website einen neuen Schlüssel. Der alte funktioniert noch
  24 Stunden, sodass Sie Ihre Seiten ohne Eventverlust aktualisieren können. Erzeugen Sie in dieser Zeit
  einen weiteren, endet der alte sofort.
- **Delete days** (Settings → Data) entfernt Besuche und Events eines Tagesbereichs; die Summen werden ohne
  diese Tage neu berechnet. **Delete all data** leert die Website, behält aber sie und ihren Schlüssel.
  **Delete the property** entfernt die Website, ihren Schlüssel und alle Daten. Nichts davon lässt sich
  rückgängig machen.
- Detaildaten werden [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics) Tage aufbewahrt, Tagessummen
  25 Monate, es sei denn, Sie setzen über die API `retention_months` an der Website.

## Limits auf gehosteten Servern

Auf einem Server mit offener Registrierung darf jeder Workspace im Monat 1.000.000 Events zählen, sofern der
Operator kein anderes [Limit](configuration.md#analytics) gesetzt hat. Events darüber werden verworfen, bis
der Monat endet, und das Panel sagt es. Eine Adresse darf an eine Website höchstens 120 Events pro Minute
senden.

## API

Jede Zahl im Panel kommt aus der API. Zum Lesen braucht ein Key den Scope `analytics:read`, zum Ändern von
Websites `analytics:manage`; ein auf Posteingänge beschränkter Key kann `analytics:manage` nicht halten, und
Keys, die vor diesen Scopes angelegt wurden, bekommen sie nicht ([Scopes](headless.md#scopes)). Agents lesen
die Websites ihrer Posteingänge. Die API nennt eine Website Property. Ein Server, der mit `YUVA_ANALYTICS=off`
gestartet wurde, antwortet auf all das mit `404`.

| Aufruf | Tut |
|---|---|
| `GET /v1/analytics/properties` | Listet die Properties, die Sie sehen können, mit den heutigen Besuchern sowie Nutzung und Limit dieses Monats. |
| `POST /v1/analytics/properties` | Legt eine an. Die Antwort enthält ihren `public_key`. |
| `GET`, `PATCH`, `DELETE /v1/analytics/properties/{propertyId}` | Liest, ändert und löscht eine samt Daten. |
| `POST /v1/analytics/properties/{propertyId}/rotate-key` | Gibt ihr einen neuen Schlüssel; der alte funktioniert 24 Stunden weiter. |
| `POST /v1/analytics/properties/{propertyId}/delete-data` | Löscht die Tage `from` bis `to`, oder mit `all: true` alles. Antwortet `202`. |
| `POST /v1/analytics/properties/{propertyId}/query` | Die Zahlen. `?format=csv` antwortet mit CSV. |
| `GET /v1/analytics/properties/{propertyId}/realtime` | Die letzten 30 Minuten. |
| `GET /v1/analytics/properties/{propertyId}/health` | Zustand der Erfassung der letzten `days` Tage (1 bis 90). |
| `POST /client/v1/analytics/{key}` | Was das Skript aufruft: ein Event oder bis zu 100 als `{"e": [...]}`, höchstens 64 KiB, beantwortet mit `202`. Kein Key und kein Cookie. |

Metriken: `visitors`, `visits`, `pageviews`, `views_per_visit`, `bounce_rate`, `visit_duration`,
`time_on_page`, `scroll_depth`, `events`. Dimensionen sind die Spalten der obigen Ansichten sowie die
Zeitraster `time:hour`, `time:day`, `time:week` und `time:month`. Eine Abfrage nimmt bis zu zwei Dimensionen,
zehn Filter und einen Vergleich. Diese fragt die zehn meistbesuchten Seiten der letzten 30 Tage gegen die
30 davor ab:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/query \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "metrics": ["visitors", "pageviews"],
    "dimensions": ["page"],
    "range": {"period": "30d"},
    "compare": {"with": "previous"},
    "limit": 10
  }'
```

Die Antwort hat `rows`, `totals`, `comparison` und `meta`. Eine Abfrage, die Detaildaten für Tage braucht,
die nicht mehr aufbewahrt werden, antwortet mit `422 needs_raw_window` und dem ersten Tag, den sie liefern
kann; ein Zeitraum über 400 Tage antwortet mit `422 range_too_large`. Pro Workspace laufen vier Abfragen
gleichzeitig; eine fünfte wartet zwei Sekunden und bekommt dann `429`. Alle Felder stehen im
[API-Vertrag](../../openapi/openapi.yaml).
