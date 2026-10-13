# Events, Goals und Funnels

Events sind das, was Besucher außer dem Aufrufen einer Seite tun: sich registrieren, kaufen, eine Datei
herunterladen, einem Link folgen. Senden Sie sie von Ihren Seiten oder Apps, machen Sie die wichtigen zu
**Goals**, um zu sehen, wie viele Besucher sie erreicht haben und wie viel Umsatz sie brachten, und verketten
Sie Goals zu **Funnels**, um zu sehen, wo Besucher abspringen. Diese Seite baut auf [Analytics](analytics.md)
auf; installieren Sie es zuerst.

## Events senden

### Aus dem Skript

```js
window.yuva.track("Signup", { props: { plan: "pro", seats: 3 }, revenue: { amount: "29.90", currency: "EUR" } })
```

`track(name, { props, revenue })` sendet ein eigenes Event auf der aktuellen Seite. Der Name hat 1 bis 120
Zeichen. `pageview`, `engagement` und `screen` sowie Namen, die mit `$` oder `yuva_` beginnen, sind reserviert
und werden abgelehnt. Events folgen allem, dem auch Seitenaufrufe folgen: dem [Consent-Modus](analytics.md),
dem Opt-out, Do Not Track, Global Privacy Control und automatisierten Browsern.

Das Skript wird mit `defer` geladen, `window.yuva` existiert also noch nicht, wenn Ihr eigener Code früh
läuft. Reihen Sie den Aufruf stattdessen ein: Er läuft, sobald das Skript da ist, und funktioniert auch danach:

```js
;(window.yuvaq = window.yuvaq || []).push(["track", "Signup", { props: { plan: "pro" } }])
```

`window.yuva.notFound()` markiert die aktuelle Seite als 404-Seite Ihrer Website ([Rezepte unten](#404-seiten)).

### Aus React und Nuxt

In React rendern Sie `<YuvaAnalytics>` einmal wie in [Analytics](analytics.md) und rufen den Hook überall
darunter auf. Aufrufe vor dem Laden des Skripts warten darauf.

```tsx
import { useYuvaAnalytics } from "useyuva/react"

function UpgradeButton() {
  const { track } = useYuvaAnalytics()
  return (
    <button onClick={() => track("Upgrade", { revenue: { amount: "49.00", currency: "USD" } })}>Upgrade</button>
  )
}
```

`<YuvaAnalytics>` nimmt `server`, `publicKey` (nicht `key`, den React für sich behält), `hash`, `consent`,
`manual`, `outbound`, `files` und `chat`; die letzten drei sind die [Attribute unten](#links-downloads-und-chat).
`useYuvaAnalytics()` liefert `pageview`, `track`, `notFound`, `consent`, `optOut` und `optIn`.

In Nuxt lauten die Modul-Optionen unter `yuva.analytics` `key`, `server`, `hash`, `consent`, `manual`,
`outbound`, `files` und `chat`, und `useYuvaAnalytics()` wird mit denselben Methoden automatisch importiert:

```ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: {
    server: "https://support.example.com",
    analytics: { key: "yuva_ak_xxxxxxxx", outbound: true, files: true },
  },
})
```

```vue
<script setup>
const { track } = useYuvaAnalytics()
</script>
```

In Vue, Astro und anderen Bundlern liefert `createAnalytics()` aus `useyuva/analytics` ebenfalls `track` und
`notFound`.

### Links, Downloads und Chat

Drei Attribute des Skript-Tags (oder Props und Optionen gleichen Namens) senden Events ohne eigenen Code. Sie
sind aus, bis Sie sie hinzufügen:

| Attribut | Option | Sendet |
|---|---|---|
| `data-outbound` | `outbound: true` | `outbound_click`, wenn ein Besucher auf einen Link zu einem anderen Host klickt, mit der Prop `url`. |
| `data-files` | `files: true` | `file_download`, wenn ein Besucher auf einen Link zu einer pdf-, zip-, dmg-, exe-, apk-, csv-, xls-, xlsx-, doc-, docx-, ppt-, pptx-, mp3- oder mp4-Datei klickt, mit der Prop `url`. |
| `data-chat` | `chat: true` | `chat_open`, wenn ein Besucher den Launcher eines [`<yuva-chat>`](widget.md) auf der Seite öffnet. |

```html
<script defer data-key="yuva_ak_xxxxxxxx" data-outbound data-files data-chat
  src="https://support.example.com/yuva-analytics.js"></script>
```

Die `url` wird ohne Query-Zeichenfolge und Fragment gespeichert, und nur für `http`- und `https`-Links. Ein
Chat mit `layout="embedded"` steht in Ihrer Seite und wird nie „geöffnet“, sendet also kein `chat_open`.

`not_found` (aus `notFound()`) ist das vierte eingebaute Event. Zusammen sind es `outbound_click`,
`file_download`, `not_found` und `chat_open`; die Events-Ansicht nennt sie Outbound link, File download, Page
not found und Chat opened.

## Props

Props geben einem Event Details: den gewählten Tarif, den geklickten Button, das benutzte Formular.

| Regel | |
|---|---|
| Anzahl | Bis zu 30 pro Event. |
| Schlüssel | 1 bis 300 Zeichen, nicht mit `$` oder `yuva_` beginnend. |
| Werte | Text, Zahlen oder Booleans, gespeichert als Text von höchstens 2.000 Zeichen. |
| E-Mail-Adressen | Ein Wert, der wie eine E-Mail-Adresse aussieht, wird verworfen, nicht gespeichert. Das Event selbst bleibt, und **Settings → Health** zählt es als „had an address removed“. |
| Websites für Kinder | Es werden keine Props gespeichert, außer der `url` von `outbound_click` und `file_download`. |

Ein Event, das eine Regel bricht (zu viele Props, ein reservierter Schlüssel, ein Wert, der eine Liste oder
ein Objekt ist), wird mit `400` abgelehnt, und mit ihm jedes andere Event derselben Anfrage. Prüfen Sie neue
Events im Netzwerk-Tab Ihres Browsers.

Props beliebigen Namens lassen sich in der [Events-Ansicht](#die-events-ansicht) für die Tage aufschlüsseln, für
die die detaillierten Datensätze aufbewahrt werden ([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics),
standardmäßig 90). Eine Prop, die Sie zusätzlich unter **Settings → Tracking → Tracked props** eintragen (bis zu 10,
nicht bei einer Website für Kinder), wird auch in den Tagessummen gehalten; ihre Aufschlüsselung funktioniert
dann auch nach dem Ende der detaillierten Datensätze. Eine neu eingetragene Prop wird für die Tage der
detaillierten Datensätze neu gezählt; frühere Tage haben sie nicht. Tragen Sie Props, die Sie über Monate
vergleichen wollen, also früh ein.

## Umsatz

Ein Event kann das Geld tragen, das es eingebracht hat:

```js
track("Purchase", { revenue: { amount: "29.90", currency: "EUR" } })
```

- `amount` ist ein Dezimalstring wie `"29.90"` (eine Zahl geht auch), `currency` ein ISO-4217-Code.
- Yuva speichert den Betrag in der **kleinsten Einheit** der Währung: 2990 für 29,90 EUR, 1500 für 1500 JPY.
  Das Panel und die API (`digits` in der Umsatz-Antwort) rechnen zurück.
- **Währungen werden nie umgerechnet oder addiert.** Eine Website, die in EUR und USD verkauft, sieht je Währung
  eine eigene Umsatzzahl.
- Der Betrag darf nicht mehr Nachkommastellen haben als die Währung: zwei bei EUR, keine bei JPY und KRW, drei
  bei KWD. `"29.999"` in EUR wird mit `400` abgelehnt, und mit ihm jedes andere Event der Anfrage.
- Nur eigene und eingebaute Events tragen Umsatz. Umsatz an einem Seitenaufruf wird abgelehnt.

Umsatz melden Goals und die Events-Ansicht, nie die Zahlen der Seitenaufrufe.

## Die Events-Ansicht

**Analytics → Events** listet jeden Event-Namen mit Besuchern, Besuchen, Events und Umsatz im gewählten Zeitraum.
Wählen Sie ein Event, um seine Props aufzuschlüsseln: Wählen Sie eine Prop oder tippen Sie einen beliebigen
Prop-Namen, und sehen Sie die Werte, die sie trug, mit ihren Anzahlen. Wählen Sie einen Wert, bleiben nur die
Besuche mit diesem Wert. Tracked props erscheinen als Chips.

Unten führt **How to send events** zu einem Snippet mit Ihrem Schlüssel.

## Goals

Ein Goal benennt etwas, das Besucher tun sollen. **Analytics → Goals** listet sie mit ihren Conversions; Owner
und Admins legen sie an (**New goal**). Ein Goal ist eines von:

| Art | Erreicht, wenn ein Besucher |
|---|---|
| Page view | Eine Seite aufruft, deren Pfad zum Muster passt. `*` steht für einen Teil eines Pfads, `**` für beliebig viele: `/thanks`, `/blog/**`. |
| Event | Das Event sendet, und, wenn Sie **Only with prop** setzen, mit dieser Prop gleich dem Wert: `Signup` mit `plan` gleich `pro`. |
| Scroll depth | Auf einer Seite, deren Pfad passt, mindestens bis zu einem Prozentwert (1 bis 100) scrollt. |

Ein Goal meldet für den Zeitraum:

| Zahl | Bedeutung |
|---|---|
| **Conversions** | Besucher, die das Goal erreicht haben, pro Tag, addiert. |
| **Conversion rate** | Conversions pro 100 Besucher. |
| **Completions** | Wie oft es erreicht wurde, auch mehrmals durch einen Besucher. |
| **Revenue** | Was die Events des Goals trugen, je Währung, mit Zahl der Zahlungen und ihrem Durchschnitt. Es gibt kein eigenes Umsatz-Goal: Ein Event-Goal zeigt den Umsatz, den seine Events tragen. |

Wählen Sie ein Goal für seinen Verlauf und dafür, woher seine Conversions kommen: Channels, Quellen,
Einstiegsseiten, Länder und Geräte.

Eine Website hat bis zu 100 Goals mit eindeutigen Namen von höchstens 120 Zeichen.

**Ab welchem Tag ein Goal Zahlen hat.** Ein neues Goal zählt über alle Tage, für die die detaillierten
Datensätze aufbewahrt werden (standardmäßig 90), nicht erst ab heute. Frühere Tage haben keine, und das Goal
sagt „This goal has numbers from this day on“. Ein Goal, dessen Definition Sie ändern, wird über diese Tage
genauso neu gezählt; ältere Tage behalten, was sie vorher gezählt haben. Ein Goal, das ein Schritt eines
Funnels ist, lässt sich erst löschen, wenn Sie es aus dem Funnel nehmen. Ein Goal anzulegen fügt Ihren Charts
eine Notiz hinzu.

## Funnels

Ein Funnel zeigt, wie viele Besucher eine Kette von Goals der Reihe nach durchlaufen und wo sie aussteigen.
**Analytics → Funnels** listet sie; Owner und Admins legen sie an (**New funnel**), aus zwei bis acht Goals in
der Reihenfolge, in der Besucher sie erreichen. Bis zu 20 Funnels pro Website, mit eindeutigen Namen.

Der Report zeigt je Schritt die Besucher, die ihn nach allen Schritten davor abgeschlossen haben, den
Verlust gegenüber dem Schritt davor, den Anteil der Besucher des ersten Schritts und den Anteil der
Besucher des Schritts davor.

So wird ein Funnel gezählt:

- **Innerhalb eines Besuchs.** Ein Besucher zählt für Schritt 3 nur, wenn er im selben Besuch zuerst
  Schritt 1 und 2 erreicht hat. Ein Besucher existiert nur einen Tag, ein Funnel überschreitet also nie
  Mitternacht; Besucher über einen Zeitraum werden pro Tag addiert.
- **Aus den detaillierten Datensätzen.** Ein Funnel braucht die Schritte jedes Besuchs und funktioniert
  daher nur innerhalb der Tage, für die die detaillierten Datensätze aufbewahrt werden
  ([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics)). Ein Zeitraum, der früher beginnt, antwortet mit
  `422 needs_raw_window`; das Panel bietet an, ab dem ersten möglichen Tag zu zeigen.
- **Höchstens 90 Tage** pro Report, oder 31 Tage bei einer Website, die in den letzten 30 Tagen mehr als 10
  Millionen Events gezählt hat. Ein längerer Zeitraum antwortet mit `422 range_too_large`.
- Antworten werden zehn Minuten zwischengespeichert.

## Segments

Filter beschränken jede Ansicht auf bestimmte Besucher: ein Land, einen Channel, Besuche mit einem Event. Um
einen Satz Filter wiederzuverwenden, öffnen Sie **Segment** über den Zahlen und wählen **Save the filters as a
new segment**. Wählen Sie später ein Segment, werden seine Filter (höchstens 10) auf die Ansicht angewendet.
Segments werden mit allen geteilt, die die Website sehen, und haben eindeutige Namen; pro Website gibt es bis zu
100. Jeder, der die Website sieht, kann eines anlegen. Sein Autor, Owner und Admins können es ändern oder
löschen; ein Segment zu löschen löscht keine Zahlen.

## Notizen in Charts

Eine Notiz markiert in den Charts einen Tag oder mehrere Tage, etwa einen Launch oder eine Kampagne: **Add note**
am Chart der Overview, mit einem Text von höchstens 300 Zeichen sowie erstem und letztem Tag. Alle, die die
Website sehen, sehen die Notiz. Jeder, der die Website sieht, kann eine hinzufügen (bis zu 1.000 pro Website);
ihr Autor, Owner und Admins können sie bearbeiten oder löschen.

Yuva fügt eigene Notizen hinzu, für einen neuen Schlüssel der Website und für jedes Goal, das Sie anlegen. Diese
können nur Owner und Admins löschen, und niemand kann sie bearbeiten.

## 404-Seiten

Rufen Sie `notFound()` einmal auf der Seite auf, die Sie für fehlende Seiten zeigen. Es sendet `not_found` für
die Adresse, die der Besucher angefragt hat; der Besuch zählt weiter als Bounce, wenn das alles war, was er tat.
Danach erscheint `not_found` in der Events-Ansicht wie jedes Event, und ein Goal kann es zählen.

**Next.js**, in `app/not-found.tsx` (`pages/404.tsx` genauso), mit `<YuvaAnalytics>` im Root-Layout:

```tsx
"use client"

import { useEffect } from "react"
import { useYuvaAnalytics } from "useyuva/react"

export default function NotFound() {
  const { notFound } = useYuvaAnalytics()
  useEffect(() => notFound(), [notFound])
  return <h1>Seite nicht gefunden</h1>
}
```

**Nuxt**, in `error.vue`, mit gesetztem `analytics` in `nuxt.config`:

```vue
<script setup>
const props = defineProps({ error: Object })
const { notFound } = useYuvaAnalytics()
onMounted(() => props.error?.statusCode === 404 && notFound())
</script>

<template>
  <h1>{{ error?.statusCode === 404 ? "Seite nicht gefunden" : "Etwas ist schiefgelaufen" }}</h1>
</template>
```

**Astro**, in `src/pages/404.astro`, mit dem Skript-Tag in Ihrem Layout. Die Warteschlange funktioniert, ob das
Skript schon geladen ist oder nicht:

```astro
<Layout>
  <h1>Seite nicht gefunden</h1>
  <script is:inline>
    ;(window.yuvaq = window.yuvaq || []).push(["notFound"])
  </script>
</Layout>
```

Auf einem statischen Host wird die 404-Seite unter der Adresse ausgeliefert, die der Besucher angefragt hat;
Seitenaufruf und Event tragen also diese Adresse.

## API

Das Panel nutzt diese Aufrufe. Lesen braucht den Scope `analytics:read`; Goals und Funnels anzulegen und zu
ändern braucht `analytics:manage` und einen Owner, einen Admin oder einen Schlüssel ohne Inbox-Beschränkung.
Segments und Notizen schreibt jeder, der die Website sieht, mit `analytics:read`.

| Aufruf | Macht |
|---|---|
| `POST /client/v1/analytics/{key}` | Was das Skript aufruft. Ein Event sieht so aus: `{"n": "Signup", "u": "https://www.example.com/pricing", "p": {"plan": "pro"}, "$": {"a": "29.90", "c": "EUR"}}`. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/goals` | Listet und legt Goals an. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/goals/{goalId}` | Ändert und löscht ein Goal. `409 goal_in_use`, solange es ein Funnel-Schritt ist. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/funnels` | Listet und legt Funnels an. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/funnels/{funnelId}` | Ändert und löscht einen Funnel. |
| `GET /v1/analytics/properties/{propertyId}/funnels/{funnelId}/report` | Die Schritte für eine `period` oder `from` und `to`. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/segments` | Listet und speichert Segments. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/segments/{segmentId}` | Benennt um, ändert und löscht ein Segment. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/annotations` | Listet Notizen (`from`, `to`) und fügt eine hinzu. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/annotations/{annotationId}` | Ändert und löscht eine Notiz. `403 annotation_read_only` bei Yuvas eigenen. |
| `POST /v1/analytics/properties/{propertyId}/query` | Die Zahlen. Mit `goal_id` für `conversions`, `conversion_rate`, `completions` und `revenue`; Aufschlüsselung nach `event` oder `prop` (`prop` benennt den Schlüssel); mit `segment_id` wird ein Segment angewendet. |

Grenzen antworten mit `403` und `goal_limit`, `funnel_limit`, `segment_limit` oder `annotation_limit`; ein Name,
den ein anderes Goal, ein anderer Funnel oder ein anderes Segment schon hat, antwortet mit `409` und
`goal_taken`, `funnel_taken` oder `segment_taken`. Alle Felder stehen im [API-Vertrag](../../openapi/openapi.yaml).
