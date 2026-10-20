# Roadmap-Board

Ein Roadmap-Board zeigt Ihren Kunden, woran Sie arbeiten, und lässt sie sagen, was sie brauchen: Ideen in
Phasen, Stimmen und Vorschläge von Lesern. Es gehört zu einem Posteingang, deshalb lässt sich Feedback, das Ihre
Kunden ohnehin schicken, mit einer Idee verknüpfen, statt es erneut zu beantworten. Rückt die Idee voran, sieht
jeder verknüpfte Kunde, wo sie steht; wird sie ausgeliefert, erreicht eine einzige Antwort alle auf ihrem
eigenen Kanal.

Ein Board kann privat bleiben, als Werkzeug Ihres Teams zum Sortieren von Feedback, oder öffentlich sein: eine
schlichte HTML-Seite auf Ihrem Yuva-Server ohne erforderliches JavaScript und eine Roadmap-App im
[Web-Widget](widget.md). Jeder Posteingang hat höchstens ein Board.

## 1. Board anlegen

Im Panel: Roadmap in der Seitenleiste (Kürzel `g p`), dann Board erstellen am Posteingang. Boards legen Inhaber und
Admins an und ändern sie; Agents mit Zugriff auf den Posteingang arbeiten mit den Ideen.

| Einstellung (API-Feld) | Bedeutung |
|---|---|
| Name (`name`) | Bis zu 100 Zeichen. |
| Beschreibung (`description`) | Bis zu 1.000 Zeichen, unter dem Namen angezeigt. |
| Adresse (`slug`) | 3 bis 48 Zeichen: `a-z`, `0-9` und innere Bindestriche. Auf dem Server eindeutig; einige Wörter sind reserviert. |
| Chat-Kanal (`channel_id`) | Ein [Chat-Kanal](widget.md) desselben Posteingangs. Die öffentliche Seite bettet seinen Chat ein, und von Ihrem Team angelegte Ideen werden auf ihm abgelegt. Pflicht; er lässt sich nicht löschen, solange das Board existiert. |
| Öffentlich (`public`) | Standardmäßig aus. Ein privates Board antwortet allen außer Ihrem Team mit `404`. |
| Vorschläge (`suggestions`) | `moderated` (Standard), `open` oder `off`; siehe [Vorschläge und Stimmen](#vorschläge-und-stimmen). |
| Analytics (`analytics_property_id`) | Wird mit dem Board gespeichert. |

Logo und Akzentfarbe der Seite sind das Branding des Posteingangs. Ändern Sie die Adresse, bleibt die alte 90 Tage lang
für das Board reserviert, und die öffentliche Adresse darunter leitet auf die neue weiter. Schalten Sie
Öffentlich aus, verschwindet das Board sofort aus dem Web, und das Widget verliert seine Roadmap-Kachel.

Per API: `POST /v1/roadmap-boards` mit `inbox_id`, `channel_id`, `name` und `slug`, dann
`PATCH /v1/roadmap-boards/{boardId}` mit `{"public": true}`. `GET /v1/roadmap-boards/{boardId}/preview` liefert
eine Seite so, wie Besucher sie sehen würden, auch solange das Board privat ist. `DELETE` entfernt das Board mit
Ideen, Stimmen, Antworten und Phasenverlauf; das Feedback und der bereits gesetzte Fortschritt bleiben.

## 2. Ideen und Phasen

Eine Idee ist ein Thread mit einem Titel (3 bis 200 Zeichen) und einem Markdown-Text (bis zu 20.000) und liegt
in einer von fünf Phasen:

| Phase | Bedeutung | Fortschritt, der auf verknüpftes Feedback gesetzt wird |
|---|---|---|
| `ideas` | Gesammelt, nicht entschieden. Neue Ideen beginnen hier. | keiner |
| `planned` | Sie werden es bauen. | `planned` |
| `in_progress` | Sie bauen es gerade. | `in_progress` |
| `shipped` | Fertig. Nimmt keine neuen Stimmen an. | `done` |
| `closed` | Sie werden es nicht bauen. Nimmt keine neuen Stimmen an und fehlt in öffentlichen Listen. | `wont_do` |

Ihr Team legt Ideen im Panel an oder ausgehend von einem Feedback (siehe [Feedback verknüpfen](#3-feedback-verknüpfen)).
Agents mit Zugriff auf den Posteingang, Inhaber und Admins verschieben die Phase: Ziehen Sie auf dem Board eine Karte
oder nutzen Sie ihr Phasenmenü (`m`; `[` und `]` für die vorige und nächste Phase). Das Panel nennt zuerst, wie
viel verknüpftes Feedback welchen Fortschritt erhält. Sie können einen Kommentar hinzufügen; er wird als Ihre
öffentliche Antwort auf die Idee veröffentlicht. Jede Verschiebung bleibt im Phasenverlauf der Idee erhalten, der
auch die Daten „Geplant am“ und „Ausgeliefert am“ der öffentlichen Seite liefert. Eine Verschiebung in die
Phase, in der die Idee schon ist, ändert nichts.

Per API: `POST /v1/discussions/{discussionId}/stage` mit `stage` und optional `comment` (ein Kommentar braucht
eine Mitgliedssitzung) sowie `GET /v1/discussions/{discussionId}/stage-changes`. Ideen sind Diskussionen der Art
`idea`: Listen Sie sie mit `GET /v1/discussions?kind=idea`, gefiltert nach `board_id`, `stage`, `sort`
(`votes`, `new`, `activity`) und `q`; legen Sie eine mit `POST /v1/discussions` und `kind: idea` an.

## 3. Feedback verknüpfen

Öffnen Sie eine Feedback-Konversation und wählen Sie Mit einer Idee verknüpfen (`b`): Suchen Sie auf dem Board
des Posteingangs, oder wählen Sie Neue Idee aus diesem Feedback, um mit dem von Ihnen geschriebenen Titel und Text eine
Idee anzulegen und die Konversation mit ihr zu verknüpfen. Nichts vom Feedback wird öffentlich. Das Feedback
zeigt die Idee als Chip.

- Nur `feedback`-Konversationen lassen sich verknüpfen, und nur mit einer Idee eines Boards desselben Posteingangs.
- Eine Idee hat viel verknüpftes Feedback; ein Feedback hat höchstens eine Idee. Erneutes Verknüpfen ersetzt die
  Verknüpfung. Eine Idee nimmt höchstens 5.000 Verknüpfungen auf.
- Wird eine Konversation in einen anderen Posteingang verschoben, löst sich ihre Verknüpfung mit der Idee.

Jede Karte auf dem Board zeigt, wie viel Feedback hinter ihrer Idee steht. Per API:
`PUT` und `DELETE /v1/discussions/{discussionId}/conversations/{conversationId}`;
`GET /v1/conversations?idea_id=` listet das Feedback einer Idee, `idea=none` das Feedback ohne Idee.

### Der Fortschritt folgt der Phase

Wird eine Idee nach `planned`, `in_progress`, `shipped` oder `closed` verschoben, setzt Yuva den Fortschritt
jedes verknüpften Feedbacks wie in der Tabelle oben. Jede Konversation erhält ein Verlaufsereignis („via idea …“),
der Kontakt sieht die neue Fortschrittszeile, und [Webhooks](webhooks.md) senden `conversation.updated`. Feedback,
das den Fortschritt schon hat, wird übersprungen. Eine neuere Verschiebung gewinnt immer gegen eine ältere, die
noch läuft. Eine Verschiebung zurück nach `ideas` ändert keinen Fortschritt. Wird Feedback mit einer Idee
verknüpft, die schon über `ideas` hinaus ist, wird sein Fortschritt sofort gesetzt; Entknüpfen lässt ihn, wie er
ist. Den Fortschritt eines Feedbacks können Sie weiterhin von Hand setzen; er gilt bis zur nächsten Verschiebung
der Idee.

### An Verknüpfte antworten

Wird eine Idee ausgeliefert, bietet das Panel sofort An Verknüpfte antworten an. Es ist dieselbe Funktion wie
bei [Statusseiten](status-pages.md#7-konversationen-mit-einem-vorfall-verknüpfen): Eine einmal geschriebene
Antwort geht als Sie an jede offene verknüpfte Konversation, über den jeweils eigenen Kanal (E-Mail, Chat oder
App), und kann die Konversationen schließen. Einmal gesendet, lässt sie sich nicht zurückholen; das Panel meldet
je Konversation, ob sie gesendet, übersprungen wurde oder fehlschlug. Per API:
`POST /v1/discussions/{discussionId}/linked-replies` antwortet mit `202`, und
`GET /v1/discussions/{discussionId}/linked-replies/{replyId}` meldet das Ergebnis. Ein Schlüssel braucht
`roadmap:write`, `conversations:read` und `messages:write` (zum Schließen zusätzlich `conversations:write`), und
der Workspace muss Bots das Senden erlauben ([Bots dürfen senden](headless.md#bots-may-send)).

## Vorschläge und Stimmen

Leser stimmen für Ideen und schlagen neue vor. Das erfordert eine Anmeldung: **Anmelden** öffnet auf Ihrem
Yuva-Server ein kleines Fenster, in dem sich der Leser mit einem E-Mail-Code oder einem Passkey anmeldet, wie bei
den [Dokumentationsseiten](documentation-pages.md#was-leser-und-ihr-team-sehen). Hat Ihr Produkt Konten, geben Sie
dem Widget ein [Identitäts-Token](identity.md); Ihre Nutzer stimmen und schlagen dann sofort vor. Anonyme Besucher
lesen nur. Eine Stimme abzugeben heißt auch, der Idee zu folgen: Follower erhalten eine E-Mail-Zusammenfassung,
wenn sie nach `planned`, `in_progress`, `shipped` oder `closed` rückt oder eine Antwort bekommt.

| Modus | Der Vorschlag eines Lesers |
|---|---|
| `moderated` (Standard) | Wartet auf Freigabe, bis einer seiner Beiträge im Workspace freigegeben wurde. Er sieht „Wartet auf Freigabe“. Nutzer mit Identitäts-Token und Ihr Team veröffentlichen sofort. |
| `open` | Wird sofort veröffentlicht, außer von Autoren, die Sie als Spam markiert haben. |
| `off` | Keine Vorschläge. Stimmen und Antworten funktionieren weiter. |

Wartende Ideen erscheinen im Streifen des Roadmap-Bildschirms; Inhaber und Admins geben sie frei oder blenden sie
aus (`PATCH /v1/discussion-posts/{postId}` mit `state`). Die Warteschlange ist
`GET /v1/discussion-posts?state=pending`. Ein Board hält höchstens 200 wartende Ideen (`429 suggestions_full`).
Ideen auf einem privaten Board senden ihren Autoren keine E-Mail.

## Öffentliche Seiten

Ein öffentliches Board liegt auf Ihrem Server unter `/roadmap/{slug}`:

| Adresse | Seite |
|---|---|
| `/roadmap/{slug}` | Spalten Ideen, Geplant, In Arbeit und Ausgeliefert (die letzten 90 Tage; alle unter `?stage=shipped`), nach Stimmen sortiert. Jede Karte zeigt Titel, Auszug, Stimmen und Kommentare. |
| `/roadmap/{slug}/ideas/{id}` | Eine Idee mit Phasenverlauf und veröffentlichten Antworten. Autoren erscheinen nur mit Namen und Initialen. |

Die Seite liest sich auch ohne JavaScript vollständig. Mit JavaScript öffnen Abstimmen und Idee vorschlagen die
Roadmap-App des Widgets bei dieser Idee. Ein privates Board, eine unbekannte Adresse sowie eine wartende oder
ausgeblendete Idee antworten mit einem schlichten `404`. Die Seiten setzen einen Canonical-Link, Open-Graph-Tags
und strukturierte Daten, werden eine Minute zwischengespeichert (eine Stimmenzahl kann so lange hinterherhinken)
und beantworten 120 Anfragen pro Minute und Adresse. Die Wörter der Seite gibt es auf Englisch, Türkisch und
Deutsch; die Seite nutzt die Sprache des Posteingangs, `?lang=` wählt eine andere.

## Im Web-Widget

Hat der Posteingang ein öffentliches Board, bekommt der Startbildschirm des [Widgets](widget.md#was-besucher-sehen) eine
Roadmap-Kachel. Die App zeigt Tabs nach Phase, Ideen nach den meisten Stimmen, eine Idee mit Text, Stimmen und
Antworten sowie Idee vorschlagen, das beim Tippen des Titels ähnliche Ideen auflistet. Ihre Seite kann sie öffnen:
`document.querySelector("yuva-chat").openApp("roadmap", { idea: "<Ideen-ID>" })`.

Für Ihre eigene Oberfläche nutzen die [shadcn-Komponenten](https://useyuva.com/shadcn) `use-yuva-roadmap`,
`yuva-roadmap`, `yuva-idea` und `yuva-idea-suggest` den Endpunkt `GET /client/v1/channels/{channel_key}/roadmap`
und die Diskussions-Endpunkte mit `kind=idea` des [Headless-Clients](headless.md).

## Aus einem Assistenten

Mit [MCP](mcp.md#roadmap-aus-einem-assistenten) listet ein Assistent Ideen, verknüpft Feedback mit ihnen und
verschiebt eine Phase nur dann, wenn Sie den Scope `roadmap:stage` ausdrücklich vergeben haben und nachdem Sie
bestätigt haben, wie viel Feedback sich ändert.

## API, Webhooks und Scopes

Alles liegt unter `/v1/roadmap-boards` und den Diskussions-Endpunkten, mit dem Tag `roadmap` in der
[API-Referenz](headless.md). [Webhooks](webhooks.md) senden `discussion.stage_changed` mit `discussion`, `from`,
`to` und `linked`, neben `discussion.created`, `discussion.post_created` und `discussion.updated`. Ein auf Posteingänge
beschränkter Schlüssel sieht nur die Boards dieser Posteingänge; jedes andere Board und jede andere Idee antwortet mit
`404`.

| Scope | Erlaubt |
|---|---|
| `roadmap:read` | Boards, Phasenverlauf, Ergebnis einer Antwort an Verknüpfte. Wird Schlüsseln gegeben, die ohne benannte Scopes erstellt wurden. |
| `roadmap:write` | Boards anlegen, ändern und löschen, Feedback mit Ideen verknüpfen und lösen, An Verknüpfte antworten. Wird Schlüsseln gegeben, die ohne benannte Scopes erstellt wurden. |
| `roadmap:stage` | Die Phase einer Idee verschieben. In keinem anderen Scope enthalten: Nennen Sie ihn ausdrücklich. |

Ideen zu lesen und zu verknüpfen braucht zusätzlich `conversations:read`. Bestehende Schlüssel erhalten keinen
dieser Scopes. `YUVA_ROADMAP=off` entfernt Boards ganz ([Konfiguration](configuration.md#roadmap-board)).
