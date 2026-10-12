# Dokumentationsseiten

Bringen Sie „War diese Seite hilfreich?“ und einen öffentlichen Diskussionsbereich auf Ihre
Dokumentationsseite. Ihre Seiten bleiben, wo sie sind; zwei Web Components senden Bewertungen und
privates Feedback in den Yuva-Posteingang und zeigen die Diskussionen der Seite: Threads mit
Antworten, Stimmen und einer akzeptierten Antwort, wie bei GitHub Discussions.

## 1. Ein Chat-Kanal

Verwenden Sie einen Chat-Kanal, eingerichtet wie für das
[Web-Widget](widget.md#1-einen-chat-kanal-anlegen); ein Kanal kann beides bedienen.

- Tragen Sie die Origin der Doku-Seite in die **Allowed origins** des Kanals ein
  (`https://docs.example.com`, dazu Ihren Entwicklungsserver, etwa `http://localhost:3000`). Auf
  jeder anderen Origin bleiben die Elemente leer.
- Geben Sie dem Posteingang einen [E-Mail-Kanal](email.md): Antworten auf Feedback erreichen Leser
  per E-Mail.
- Wählen Sie mit der Kanaleinstellung `discussions`, wie Diskussionen funktionieren: `moderated`
  (Standard: Beiträge eines Lesers warten auf Ihr Team, bis einer davon freigegeben wurde), `open`
  (sofort veröffentlicht) oder `off` (keine Diskussionen; das Element sagt das).

## 2. Die Elemente einbauen

### Reines HTML

```html
<script src="https://support.example.com/yuva-docs.js" defer></script>

<yuva-page-feedback channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-page-feedback>
<yuva-discussions channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-discussions>
```

Setzen Sie sie in die Vorlage, die alle Doku-Seiten teilen, nach dem Artikel. Ihr Yuva-Server
liefert `/yuva-docs.js` aus.

Auf npm gibt es sie als `useyuva/docs`, `useyuva/react` und `useyuva/nuxt` (`useyuva` 0.0.7 und neuer).

### Nuxt Content

Lauffähiges Beispiel: [`examples/nuxt-content`](../../examples/nuxt-content)
([so starten Sie es](../../examples/README.md#nuxt-content)). Binden Sie das Modul ein:

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["@nuxt/content", "useyuva/nuxt"],
  yuva: { server: "https://support.example.com", channel: "yuva_pk_xxxxxxxxxxxxxxxx" },
});
```

```vue
<!-- app/pages/[...slug].vue, nach <ContentRenderer> -->
<YuvaPageFeedback :page-title="page.title" />
<YuvaDiscussions :page-title="page.title" />
```

`NUXT_PUBLIC_YUVA_SERVER` und `NUXT_PUBLIC_YUVA_CHANNEL` überschreiben `yuva` zur Laufzeit. Die
Komponenten rendern auf dem Server und laden die Elemente im Browser, daher funktioniert auch
`nuxt generate`. Die Events heißen `@rating`, `@feedback`, `@discussion`, `@post` und `@reader`. Auf einer frischen Seite scheitert `nuxt build` mit „Nuxt
Content requires better-sqlite3“: Installieren Sie es, oder setzen Sie
`content.experimental.sqliteConnector: "native"` wie im Beispiel. Die Seite fragt eine
`content`-Collection ab; definieren Sie sie wie in der
[`content.config.ts`](../../examples/nuxt-content/content.config.ts) des Beispiels.

### Fumadocs

Lauffähiges Beispiel: [`examples/fumadocs`](../../examples/fumadocs)
([so starten Sie es](../../examples/README.md#fumadocs)). Die Doku-Seite ist eine Server Component;
packen Sie die React-Komponenten deshalb in eine Client Component:

```tsx
// components/yuva.tsx
"use client";
import { YuvaDiscussions, YuvaPageFeedback } from "useyuva/react";

const channel = process.env.NEXT_PUBLIC_YUVA_CHANNEL!;
const server = process.env.NEXT_PUBLIC_YUVA_SERVER;

export const PageDiscussions = ({ title }: { title: string }) =>
  <YuvaDiscussions channel={channel} server={server} pageTitle={title} />;
export const PageFeedback = ({ title }: { title: string }) =>
  <YuvaPageFeedback channel={channel} server={server} pageTitle={title} />;
```

```tsx
// app/docs/[[...slug]]/page.tsx, nach <DocsBody>
<PageFeedback title={page.data.title} />
<PageDiscussions title={page.data.title} />
```

Haben Sie die eigene `<Feedback>`-Komponente von Fumadocs eingebaut, ersetzen Sie sie durch
`<PageFeedback />`, oder behalten Sie sie und ergänzen nur `<PageDiscussions />`. Ist `useyuva` per
Pfad aus `sdk/js` verlinkt, kann Next eine zweite Kopie von React laden; setzen Sie
`turbopack.root` wie in der `next.config.mjs` des Beispiels.

### shadcn/ui

Für React-Seiten auf [shadcn/ui](https://ui.shadcn.com), die die Diskussionen als eigenen Code in
ihrem eigenen Designsystem haben wollen. Lauffähiges Beispiel:
[`examples/shadcn-next`](../../examples/shadcn-next)
([so starten Sie es](../../examples/README.md#shadcn-next)).

```sh
npx shadcn@latest add https://useyuva.com/r/yuva-discussions.json https://useyuva.com/r/yuva-page-feedback.json
```

Das kopiert die Komponenten in Ihr Projekt (`components/yuva-*.tsx`,
`hooks/use-yuva-discussions.tsx`, `lib/yuva-utils.ts`), ergänzt die shadcn/ui-Bausteine, auf denen
sie aufbauen (button, card, input, tabs, textarea, avatar, badge, separator, skeleton), und
installiert `useyuva` und `lucide-react`. Die kleineren Einträge sind `use-yuva-discussions` (nur
Provider und Hooks), `yuva-discussion-list`, `yuva-discussion-thread` und
`yuva-discussion-composer`; alle stehen in `https://useyuva.com/r/registry.json`.

```tsx
// app/layout.tsx
import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions";

<YuvaDiscussionsProvider
  server={process.env.NEXT_PUBLIC_YUVA_SERVER}
  channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL}
>
  {children}
</YuvaDiscussionsProvider>
```

```tsx
// unter dem Artikel
<YuvaPageFeedback />
<YuvaDiscussions />
```

`YuvaDiscussions` nimmt `page` und `pageTitle` (standardmäßig die URL ohne Query und Fragment und
`document.title`), `defaultSort`, `labels` und `className`; ohne Provider darum geben Sie ihm
`server` und `channel` selbst. Der Provider nimmt außerdem `identityToken` (eine Funktion, wie bei
den Elementen), `locale` für Datumsangaben und `client`, um ein eigenes `createYuvaClient()` zu
teilen.

- **Anpassen** durch Bearbeiten der Dateien: Sie gehören jetzt Ihnen. Die Farben kommen aus Ihren
  Theme-Variablen (`primary`, `muted`, `border`, `destructive` …), der Dark Mode folgt also Ihrem
  Theme. Beiträge zeigen das `body_html` des Servers in den Klassen von `yuvaProseClassName` aus
  `lib/yuva-utils.ts`; die Vorschau im Editor besteht aus React-Elementen, veröffentlicht wird, was
  der Server rendert.
- **Eigenes Layout**: Die Hooks hinter den Komponenten sind `useDiscussions(page, sort)`,
  `useDiscussion(id)`, `useYuvaReader()` (`signIn()` aus einem Klick aufrufen),
  `useDiscussionMode()` und `usePageRating(page)`.
- **Beschriftungen** sind standardmäßig Englisch (`defaultYuvaDiscussionsLabels`). Geben Sie
  `labels` an den Provider oder an eine Komponente, um sie zu ändern oder zu übersetzen; Zahlen
  sind Funktionen. Türkisch:

```tsx
import type { YuvaDiscussionsLabels } from "@/lib/yuva-utils";

const tr: Partial<YuvaDiscussionsLabels> = {
  heading: "Tartışmalar",
  sortNew: "Yeni",
  sortTop: "En çok oy alan",
  sortUnanswered: "Yanıtsız",
  newDiscussion: "Yeni tartışma",
  empty: "Bu sayfada henüz tartışma yok. İlkini siz başlatın.",
  replies: (count) => `${count} yanıt`,
  votes: (count) => `${count} oy`,
  answered: "Yanıtlandı",
  acceptedAnswer: "Kabul edilen yanıt",
  reply: "Yanıtla",
  signIn: "Giriş yap",
  signInHint: "Tartışma başlatmak, yanıtlamak ve oy vermek için giriş yapın.",
  write: "Yaz",
  preview: "Önizleme",
  startDiscussion: "Tartışmayı başlat",
  feedbackQuestion: "Bu sayfa işinize yaradı mı?",
  feedbackYes: "Evet",
  feedbackNo: "Hayır",
};

<YuvaDiscussionsProvider server={server} channel={channel} labels={tr} locale="tr">
```

Die Komponenten verwenden logische Abstände (`ps-`, `border-s`, `text-start`) und funktionieren
deshalb auch unter `dir="rtl"`. Wie die Elemente brauchen sie `useyuva` 0.0.7 oder neuer.

### Andere Generatoren

Laden Sie das Skript (oder `useyuva/docs`) und setzen Sie beide Elemente unter den Artikel.
`useyuva/docs` und `useyuva/react` lassen sich beim Server-Rendering importieren; sie registrieren
die Elemente nur im Browser. Geprüft mit Starlight 0.42, VitePress 1.6 und Docusaurus 3.10.

- **Starlight**: [`Footer` überschreiben](https://starlight.astro.build/guides/overriding-components/): den Standard-`Footer` rendern, danach beide Elemente
  und `<script>import "useyuva/docs";</script>`. Astro braucht für Custom Elements keine Einstellung.
- **VitePress**: [`Layout` umhüllen](https://vitepress.dev/guide/extending-default-theme#layout-slots) und beide Elemente in den Slot `doc-after` setzen;
  `useyuva/docs` in `.vitepress/theme/index.js` importieren und in der Konfiguration `vue: { template: { compilerOptions: { isCustomElement: (tag) => tag.startsWith("yuva-") } } }` setzen.
- **Docusaurus**: `DocItem/Footer` [wrappen](https://docusaurus.io/docs/swizzling#wrapping) (`npm run swizzle @docusaurus/theme-classic DocItem/Footer -- --wrap --danger`;
  die CLI stuft das als unsicher ein) und danach `YuvaPageFeedback` und `YuvaDiscussions` aus `useyuva/react` rendern.
- **MkDocs, Hugo, Jekyll und andere**: wie [reines HTML](#reines-html).

## Was Leser und Ihr Team sehen

- **Leser** bewerten eine Seite mit Ja oder Nein (ein anonymer Zähler) und können danach Feedback
  schreiben; es bleibt privat und nimmt eine optionale E-Mail-Adresse für die Antwort.
- **Alle** lesen die Diskussionen der Seite ohne Anmeldung: Threads, sortiert nach Neu, Top oder
  Unbeantwortet, jeder mit seinem Eröffnungsbeitrag, der akzeptierten Antwort oben angeheftet,
  Antworten (eine Ebene Antworten auf Antworten), Stimmen und einem „Beantwortet“-Abzeichen.
  Autoren erscheinen nur mit Namen und Initialen; Beiträge Ihres Teams tragen ein „Team“-Abzeichen.
- **Mitmachen** braucht eine Anmeldung. **Anmelden** öffnet ein kleines Fenster auf Ihrem
  Yuva-Server, in dem sich der Leser mit einem E-Mail-Code oder einem Passkey anmeldet; das Konto
  gilt auf jeder Seite, die diesen Yuva-Server nutzt, und das Fenster gibt der Seite ein Token für
  diesen Kanal, das 24 Stunden gilt. Leser eröffnen Threads, antworten, stimmen ab, folgen einem
  Thread (E-Mail, wenn jemand antwortet oder eine Antwort akzeptiert wird), markieren in ihrem
  eigenen Thread eine Antwort, bearbeiten ihre Beiträge 30 Minuten lang und löschen sie jederzeit.
  Beiträge sind Markdown mit Vorschau; die Seite zeigt das HTML, das der Server daraus macht, ohne
  Skripte oder rohes HTML. **Abmelden** beendet das Token nur auf dieser Seite.
- **Moderation**: In einem `moderated`-Kanal sehen Leser ihre ersten Beiträge als „Wartet auf
  Freigabe“; für alle anderen bleiben sie verborgen, bis jemand aus Ihrem Team einen freigibt.
  Danach erscheinen sie sofort. Einen Beitrag als Spam zu markieren verbirgt ihn und lässt die
  Beiträge dieses Autors wieder warten. Ein gesperrter Thread nimmt von Lesern keine Antworten,
  Stimmen oder Antwortmarkierungen mehr an.
- **Ihr Team** bekommt Feedback im Posteingang als Unterhaltungen vom Typ `feedback` und arbeitet
  im Panel mit den Diskussionen: Threads pro Site und Seite, die Warteschlange freizugebender
  Beiträge, Antworten als Team, Antworten akzeptieren, Threads sperren und umbenennen. Eine private
  Unterhaltung kann mit dem Text, den Ihr Team schreibt, zu einem öffentlichen Thread werden; sonst
  wird nichts aus der Unterhaltung veröffentlicht. Die Ansicht **Docs** listet Seiten mit ihren
  Bewertungen, ihrem Feedback, offenen und unbeantworteten Threads und wartenden Beiträgen über 7,
  30 oder 90 Tage.

## Gut zu wissen

- **Eine Seite ist ihre URL ohne Query und Fragment.** URLs pro Sprache (`/en/docs/install`,
  `/de/docs/install`) sind getrennte Seiten mit eigenen Bewertungen; übergeben Sie eine URL als
  `page`, um sie zusammen zu zählen, und setzen Sie `locale` auf die Sprache der Seite.
- **Ihre eigene Anmeldung**: Hat Ihre Seite Konten, geben Sie den Elementen mit
  `setIdentityToken` ein [Identitäts-Token](identity.md) (in React die Prop `identityToken`). Das
  Feedback-Formular hat dann kein E-Mail-Feld, Antworten gehen an die Adresse im Token, und
  `<yuva-discussions>` schreibt ohne Anmeldefenster als dieser Nutzer. Ohne **Anonymous visitors**
  am Kanal braucht Feedback ein Token; Bewertungen nie.
- **Anmeldefenster**: Browser öffnen es nur nach einem Klick, und so öffnet es das Element. Seiten,
  die `Cross-Origin-Opener-Policy: same-origin` senden, trennen das Fenster von der Seite;
  verwenden Sie dort `same-origin-allow-popups`.
- **Content Security Policy**: Erlauben Sie den Yuva-Server in `script-src` (für `/yuva-docs.js`)
  und `connect-src`.
- **Elemente bleiben leer**: Die Origin ist nicht erlaubt oder der Schlüssel ist falsch; siehe die
  Browser-Konsole.
- **Links auf einen Thread**: E-Mails an Folgende und die Kontoseite des Lesers verlinken auf
  `<Seite>#yuva-discussion-<id>`. Das Element öffnet diesen Thread und scrollt zu ihm, auch bei
  einem `hashchange`; der Weg zurück zur Liste entfernt das Fragment aus der Adresse.
- **Frühere Fragen**: Veröffentlichte Antworten wurden zu Diskussionen auf ihren Seiten,
  beantwortet von dem Mitglied, das sie veröffentlicht hat.

## Referenz

### Attribute

| Attribut | Bedeutung |
|---|---|
| `channel` | Der öffentliche Schlüssel des Chat-Kanals (`yuva_pk_…`). Pflicht. |
| `server` | URL des Yuva-Servers. Standard ist die Origin, von der `yuva-docs.js` geladen wurde. |
| `locale` | `en`, `tr` oder `de`. Standard ist die Sprache des Browsers, sonst Englisch. |
| `dir` | `ltr` oder `rtl`. Standard ist die Schreibrichtung der Sprache. |
| `identity-token` | Ein signiertes Identitäts-Token. `setIdentityToken` ist meist besser. |
| `page` | Die URL der Seite. Standard ist die aktuelle URL; clientseitige Navigation wird verfolgt. |
| `page-title` | Der Titel der Seite. Standard ist `document.title`. |

Die React-Wrapper nehmen dasselbe als Props (`channel`, `server`, `locale`, `dir`, `page`,
`pageTitle`, `identityToken`), dazu `className` und `style`.

### Properties und Methoden

| Element | Member | Bedeutung |
|---|---|---|
| beide | `channel`, `server`, `locale`, `page`, `pageTitle` | Die Attribute als Properties. |
| beide | `setIdentityToken(fn)` | Eine Funktion, die ein Token zurückgibt, oder `null` für einen nicht angemeldeten Leser. |
| `yuva-page-feedback` | `rating` | `"up"`, `"down"` oder `null` für diese Seite in diesem Browser. |
| `yuva-page-feedback` | `rate("up" \| "down")` | Dasselbe wie ein Klick auf Ja oder Nein. |
| `yuva-discussions` | `discussions` | Die aufgelisteten Threads. |
| `yuva-discussions` | `reader` | Der angemeldete Leser (`{ token, expires_at, reader: { name, initials } }`) oder `null`. |
| `yuva-discussions` | `reload()` | Lädt die Liste neu. |
| `yuva-discussions` | `open(id)` | Zeigt einen Thread. |
| `yuva-discussions` | `signIn()`, `signOut()` | Dasselbe wie die Buttons; rufen Sie `signIn()` aus einem Klick auf. |

### Ereignisse

Sie steigen aus dem Shadow DOM auf, Sie können also auf `document` lauschen.

| Ereignis | Element | `detail` | React-Prop |
|---|---|---|---|
| `yuva-rating` | `yuva-page-feedback` | `{ page, rating, previous }` | `onRating` |
| `yuva-feedback` | `yuva-page-feedback` | `{ page, rating, conversation_id }` | `onFeedback` |
| `yuva-discussion` | `yuva-discussions` | `{ page, discussion_id, state }`, wenn der Leser einen Thread eröffnet hat | `onDiscussion` |
| `yuva-post` | `yuva-discussions` | `{ page, discussion_id, post_id, state }`, wenn der Leser geantwortet hat | `onPost` |
| `yuva-reader` | `yuva-discussions` | `{ name, initials }` nach der Anmeldung, `null` nach der Abmeldung | `onReader` |

### CSS-Variablen

Die Elemente übernehmen `color` und `font` von der Seite und folgen dem Dark Mode. Die Akzentfarbe
kommt von der Launcher-Farbe des Kanals; für eine eigene setzen Sie sie mit `!important`.

| Variable | Verwendet für |
|---|---|
| `--yuva-accent` | Buttons, die gewählte Bewertung und Sortierung, Stimmen, Fokusringe, Links in Beiträgen, Team-Avatare. |
| `--yuva-on-accent` | Text auf der Akzentfarbe. |
| `--yuva-border` | Rahmen von Buttons, Feldern, Threads und Beiträgen. |
| `--yuva-soft` | Hintergrund der gewählten Bewertung, Threads unter dem Mauszeiger, Code in Beiträgen. |
| `--yuva-danger` | Fehlermeldungen. |

### Endpunkte

| Endpunkt | Verwendet für |
|---|---|
| `POST /client/v1/channels/{channel_key}/page-ratings` | Eine Bewertung zählen. |
| `POST /client/v1/feedback` | Feedback mit `page_url`, `page_title`, `rating` senden. |
| `GET /client/v1/channels/{channel_key}/discussions?page=&sort=` | Die Threads einer Seite (ohne Token eine Minute gecacht). |
| `GET /client/v1/channels/{channel_key}/discussions/{discussionId}` | Ein Thread mit seinen Antworten. |
| `POST /client/v1/discussions`, `POST /client/v1/discussions/{discussionId}/posts` | Einen Thread eröffnen, antworten. |
| `PATCH`, `DELETE /client/v1/discussion-posts/{postId}`, `PUT`, `DELETE …/vote` | Bearbeiten, löschen, abstimmen. |
| `PUT`, `DELETE /client/v1/discussions/{discussionId}/answer`, `…/follow` | Eine Antwort akzeptieren, folgen. |
| `POST /v1/account/reader-tokens`, `DELETE /client/v1/reader-token` | Das Token des Anmeldefensters, abmelden. |
| `GET /v1/discussions`, `GET /v1/discussion-posts?state=pending` und der Rest von `/v1/discussions*` | Die Ansicht und Moderation Ihres Teams. |
| `POST /v1/conversations/{conversationId}/discussion` | Eine Unterhaltung zu einem öffentlichen Thread machen. |
| `GET /v1/docs/sites`, `GET /v1/docs/pages`, `GET /v1/docs/summary`, `GET /v1/docs/page` | Die Ansicht Docs. |
| `GET /v1/conversations?page=&kind=feedback` | Das Feedback einer Seite. |

Details: [API-Vertrag](../../openapi/openapi.yaml).
