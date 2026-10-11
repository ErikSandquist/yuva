# E-Mail-Designs und Vorlagen

Ein Design ist das Aussehen einer Mail: Blöcke wie Überschriften, Text, Bilder und Buttons sowie ein
Theme mit Farben und Schriften. Sie bauen es einmal im Panel, starten mit einer der Vorlagen, die
Yuva mitbringt, und nutzen es für Ihre Broadcasts. Der Server macht daraus Mail-HTML, das in den
gängigen Mail-Apps funktioniert, mit Nur-Text-Teil und Dark Mode.

Designs erstellen Owner und Admins. API-Keys und OAuth-Tokens brauchen den Scope `broadcasts:read`
oder `broadcasts:write`. Assistenten (Agents) können Designs nicht verwenden.

## 1. Mit einer Vorlage starten

Im Panel: Broadcasts, dann der Tab Designs. Die Galerie bietet sechs Startvorlagen: Newsletter,
Product update, Announcement, Welcome, Event und Digest, jeweils auf Englisch, Türkisch und Deutsch.
Wählen Sie eine, wählen Sie die Sprache der Vorlage und übernehmen Sie sie. Ihre Bilder werden in die
Bilder Ihres Workspace kopiert, danach können Sie alles ändern. New blank design beginnt mit einer
leeren Mail in schlichten Farben.

Ein Design gehört zu einem Posteingang oder zum ganzen Workspace. Die
[Absenderangaben](lists.md#1-absenderangaben-eintragen) des Posteingangs füllen die Fußzeile.

Per API: `POST /v1/email-designs` mit `name` und `template: { "slug": "newsletter", "locale": "de" }`.
`GET /v1/email-templates` listet die Vorlagen.

## 2. Das Design bearbeiten

Der Designer öffnet sich im Vollbild. Ziehen Sie Blöcke aus der Palette (oder der Ebenenübersicht)
oder drücken Sie Enter darauf, um sie nach dem gewählten Block einzufügen. Klicken Sie auf einen
Block, um seine Einstellungen rechts zu ändern.

| Block | Verwendung |
|---|---|
| Section | Eine Gruppe von Blöcken auf eigenem Hintergrund. Sections lassen sich nicht verschachteln. |
| Columns | Zwei oder drei Spalten, die sich auf Handys untereinander stapeln. Die Spaltenbreiten ergeben 100. Columns lassen sich nicht verschachteln. |
| Heading | Drei Größen. |
| Text | Absätze, Listen und Links. |
| Image | Ein hochgeladenes Bild, optional mit Link und Alt-Text. |
| Button | Ein Link, der wie ein Button aussieht. |
| Divider, Spacer | Eine dünne Linie, Leerraum. |
| Social links | Links zu Ihren Profilen, als Icons oder Text. |
| Footer text | Eigene Zeilen über der vorgeschriebenen Fußzeile. |

Überschriften, Text, Fußzeilentext und Button-Beschriftungen bearbeiten Sie direkt an Ort und Stelle.
Text behält nur, was Mail-Apps gut darstellen: fett, kursiv, unterstrichen, durchgestrichen, Links,
Listen, Zeilenumbrüche und Code. Links beginnen mit `https://`, `http://`, `mailto:` oder `tel:`.

Jede Mail endet mit einer Fußzeile, die Sie nicht entfernen können: der juristische Name und die
Postanschrift des Posteingangs, der Grund, warum die Person die Mail erhält, sowie die Links zum
Abmelden und zu den Einstellungen. Eine Warnung weist darauf hin, wenn dem Posteingang noch
juristischer Name oder Postanschrift fehlen.

Ein Design enthält höchstens 200 Blöcke.

### Merge-Felder

Merge-Felder setzen die Angaben der jeweiligen Person in die Mail. Fügen Sie eines im Text über das
Menü Insert ein oder tippen Sie es. Nach `|` steht ein Ersatzwert für Personen, bei denen der Wert
leer ist:

```
Hallo {{ contact.first_name | "liebe Leserin, lieber Leser" }},
```

| Feld | Wert |
|---|---|
| `contact.name` | Der Name des Kontakts. |
| `contact.first_name` | Das erste Wort des Namens. |
| `contact.email` | Die E-Mail-Adresse. |
| `contact.attributes.<Schlüssel>` | Ein Attribut des Kontakts, über seinen Schlüssel. |
| `inbox.name` | Der Name des Posteingangs. |
| `list.name` | Die Liste, an die die Mail geht. |
| `unsubscribe_url`, `preferences_url` | Die eigenen Links der Person. In Links dürfen Merge-Felder nur nach dem `?` stehen. |

Ein Feld, das Yuva nicht kennt, wird abgelehnt. Ein Ersatzwert hat höchstens 100 Zeichen. Werte werden
nie zu Markup. Ohne Ersatzwert kann das Feld eine Lücke wie „Hallo ," lassen; der Designer warnt davor.

## 3. Theme und Dark Mode

Unter Theme (Kürzel `T`) stellen Sie Farben, Schriften, Inhaltsbreite (480 bis 720 Pixel),
Eckenradius und Richtung ein (links nach rechts, rechts nach links oder automatisch je Text). Die
Schriften sind Systemstapel: system, humanist, geometric, serif und monospace; Mails laden keine
Webfonts.

Leser, deren Mail-App im Dark Mode läuft, bekommen dunklere Farben, die aus Ihren abgeleitet werden,
mit Text bei 4,5:1 Kontrast. Unter Dark mode können Sie jede dieser Farben selbst festlegen. Ein
dunkles Logo auf transparentem Hintergrund kann im Dark Mode eine helle Platte dahinter bekommen.

## 4. Bilder

Öffnen Sie die Bildbibliothek an einem Image-Block (Kürzel `I`) oder legen Sie Dateien darauf ab.

| | |
|---|---|
| Typen | PNG, JPEG, GIF, WebP. SVG wird abgelehnt, weil es Skripte enthalten kann. |
| Upload-Größe | Höchstens 10 MiB; 40 Megapixel und 8.000 Pixel je Seite. |
| Gespeichert als | Dekodiert und ohne EXIF-, GPS- und Farbprofildaten neu kodiert. Breiter als 1.200 Pixel wird auf 1.200 verkleinert. WebP wird als PNG gespeichert, ohne Transparenz als JPEG. |
| GIF | Bleibt unverändert, mit Animation: höchstens 1.200 Pixel breit und 5 MiB. |
| Dieselbe Datei zweimal | Antwortet mit dem Bild, das Sie schon haben. |
| Adresse | Jedes Bild hat auf Ihrem Server eine öffentliche URL `/i/{token}`: keine Cookies, ein Jahr im Cache, nichts wird pro Aufruf aufgezeichnet. |

Geben Sie jedem Bild eine Beschreibung für Screenreader oder markieren Sie es als dekorativ. Die
Bilder eines Workspace dürfen zusammen höchstens `YUVA_EMAIL_IMAGES_MAX_BYTES` belegen
([Konfiguration](configuration.md)).

Ein Bild, das ein nicht archiviertes Design verwendet, lässt sich nicht löschen; das Panel nennt die
Designs. Bei einem Bild, das in einem Broadcast verschickt wurde, fragt es nach, weil bereits
zugestellte Mails dann ein defektes Bild zeigen. Bilder, die kein Design verwendet und kein Broadcast
versendet hat, werden 7 Tage nach dem Upload gelöscht.

## 5. Vorschauen und Prüfungen

Die Vorschau kommt vom Server, zeigt also, was Empfänger erhalten. Mit `1` bis `4` wechseln Sie
zwischen Edit, Desktop, Phone und Dark. Checks zeigt, was zu beheben ist:

| Warnung | Bedeutung |
|---|---|
| Hard to read | Text unter 4,5:1 Kontrast zum Hintergrund, in hellen oder dunklen Farben. |
| Über 90 KB | Gmail schneidet Mails bei etwa 102 KB HTML ab und blendet den Rest aus, auch den Abmeldelink. Der Designer warnt ab 90 KB und erneut über 102 KB. |
| Bild fehlt, Beschreibung fehlt | Ein Image-Block ohne Bild oder ohne Alt-Text. |
| Kein Ersatzwert | Ein Merge-Feld, das für manche Personen leer sein kann. |
| Absenderangaben fehlen | Dem Posteingang fehlen juristischer Name oder Postanschrift. |

Ein Design mit mehr als 500 KB HTML wird abgelehnt.

## 6. Speichern, Versionen und Konflikte

Der Designer speichert 1,5 Sekunden nach Ihrer letzten Änderung; `Strg`/`Cmd` + `S` speichert sofort
und behält eine Version. Rückgängig und Wiederholen (`Strg`/`Cmd` + `Z`, mit `Umschalt` zum
Wiederholen) gehen 100 Schritte zurück. Offline bleiben Änderungen auf Ihrem Gerät und werden
gespeichert, sobald Sie wieder online sind.

Versions (Kürzel `V`) behält die neuesten 50: eine höchstens alle 10 Minuten beim Bearbeiten, eine je
manuellem Speichern und eine je Wiederherstellung. Eine Wiederherstellung speichert diese Version als
neueste.

Hat jemand anderes (oder ein anderer Tab von Ihnen) zuerst gespeichert, bietet ein Dialog drei Wege:
meine behalten, meine als Kopie speichern oder ihre laden. Ihre Änderung geht nie verloren, und die
andere Version bleibt in den Versionen. Was Sie auch wählen, die Version der Gegenseite wird dort als
„Aus einem Konflikt behalten“ aufbewahrt; keine Änderung geht verloren.

Download JSON speichert das Dokument des Designs als Datei. Archive blendet ein Design aus der Liste
aus; erneutes Speichern holt es zurück. Ein Dokument, das ein neueres Yuva erstellt hat, lässt sich
ansehen, aber nicht ändern.

Kürzel: `A` Block hinzufügen, `T` Theme, `I` Bild, `V` Versionen, `Strg`/`Cmd` + `D` Block
duplizieren, `Backspace` löschen, `Alt` + `↑`/`↓` verschieben, `Esc` verlässt erst den Text, dann den
Block.

## 7. Die API

Alles, was das Panel tut, steht im [API-Vertrag](../../openapi/openapi.yaml). Dieser Aufruf rendert
ein Design, ohne es zu speichern:

```sh
curl -X POST https://support.example.com/v1/email-designs/render \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "inbox_id": "INBOX-ID",
    "document": {
      "schema": 1,
      "theme": {
        "width": 600,
        "font": "system",
        "colors": {
          "page": "#f4f4f5", "card": "#ffffff", "text": "#18181b", "muted": "#52525b",
          "accent": "#1d4ed8", "link": "#1d4ed8", "border": "#e4e4e7"
        }
      },
      "blocks": [
        { "id": "h", "type": "heading", "level": 1, "text": "Hallo {{ contact.first_name | \"liebe Leserin, lieber Leser\" }}" },
        { "id": "b", "type": "button", "label": "Weiterlesen", "url": "https://example.com/news" }
      ]
    }
  }'
```

Die Antwort enthält `html`, `text`, `size_bytes` und `warnings`. Senden Sie `"dark": true` für die
dunkle Palette. Ein Speichern sendet die `version`, von der es ausging, als `base_version`; hat sich das
Design inzwischen geändert, lautet die Antwort `409 version_conflict` mit dem aktuellen Design in
`design`. Ein ungültiges Dokument ergibt `422 invalid_design` mit einem Eintrag je Problem in `errors`.

## Referenz

### Endpunkte

| Endpunkt | Verwendung |
|---|---|
| `GET`, `POST /v1/email-designs` | Auflisten (`inbox_id`, `q`, `archived=true`) und aus einem `document`, einem `template` oder leer erstellen. |
| `GET`, `PUT`, `DELETE /v1/email-designs/{designId}` | Lesen, speichern (mit `base_version`), archivieren. |
| `POST /v1/email-designs/{designId}/duplicate` | Ein Design kopieren. |
| `POST /v1/email-designs/render` | Ein Dokument rendern: HTML, Text, Größe und Warnungen. Höchstens 120 pro Minute. |
| `GET /v1/email-designs/{designId}/versions` | Die gespeicherten Versionen. |
| `GET /v1/email-designs/{designId}/versions/{version}` | Eine Version mit ihrem Dokument. |
| `POST /v1/email-designs/{designId}/versions/{version}/restore` | Eine Version als neueste speichern. |
| `GET`, `POST /v1/email-images` | Bilder auflisten und hochladen (multipart, eine `file`). |
| `DELETE /v1/email-images/{imageId}` | Ein Bild löschen (`force=true` für ein versendetes). |
| `GET /v1/email-templates`, `GET /v1/email-templates/{slug}` | Die Galerie (`locale=en`, `tr` oder `de`). |
| `GET /i/{token}` | Ein Bild, öffentlich. |
| `GET /email-templates/{slug}/{locale}.html`, `.webp` | Die Vorschau einer Vorlage, öffentlich. |

### Scopes

| Scope | Bedeutung |
|---|---|
| `broadcasts:read` | Designs, Versionen, Bilder und Vorlagen lesen; rendern. |
| `broadcasts:write` | Designs erstellen, speichern, kopieren, archivieren und wiederherstellen; Bilder hochladen und löschen. |

Ein auf Posteingänge beschränkter Key liest die Designs seiner Posteingänge und die workspaceweiten
und ändert nur Designs seiner Posteingänge. Keys, die vor diesen Scopes angelegt wurden, erhalten sie
nicht.

### Grenzen

| Grenze | Wert |
|---|---|
| Blöcke in einem Design | 200 |
| Design-Dokument | 300 KiB JSON |
| Gerendertes HTML | 90 KB Warnung, 102 KB abgeschnitten, 500 KB abgelehnt |
| Gespeicherte Versionen | 50 |
| Bild-Upload | 10 MiB, 40 Megapixel, 8.000 Pixel je Seite |
| Bildbreite nach dem Upload | 1.200 Pixel |
| Ersatzwert eines Merge-Felds | 100 Zeichen |
| Link | 2.048 Zeichen |
| Renderings | 120 pro Minute und Aufrufer |
| Ungenutzte Bilder | Werden 7 Tage nach dem Upload gelöscht |
