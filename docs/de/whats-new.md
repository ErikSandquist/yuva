# Neuigkeiten

Ein Neuigkeiten-Changelog sagt Ihren Kunden, was sich geändert hat: kurze, datierte Einträge mit den Labels
New, Improved oder Fixed. Sie erscheinen auf einer öffentlichen Seite, in einem Atom-Feed, im
[Web-Widget](widget.md) mit Zähler für Ungelesenes und, wenn Sie möchten, per E-Mail bei Ihren Abonnenten.
Ein Changelog gehört zu einem Posteingang, deshalb sieht ein Kunde, dass der gemeldete Fehler behoben ist,
genau dort, wo er nachgefragt hätte.

Jeder Posteingang hat höchstens einen Changelog. Die öffentlichen Seiten sind schlichtes HTML ohne zwingendes
JavaScript, erzeugt von Ihrem Yuva-Server. Nichts hier versendet von selbst E-Mail: Einen Eintrag per E-Mail zu
senden erzeugt den Entwurf eines [Broadcasts](broadcasts.md), den Sie wie gewohnt prüfen und senden.

## 1. Changelog anlegen

Im Panel: Neuigkeiten in der Seitenleiste (Kürzel `g w`), dann am Posteingang Changelog erstellen. Changelogs
legen Inhaber und Admins an und ändern sie; wer Einträge veröffentlichen darf, steht unter
[Entwürfe und Veröffentlichung](#entwürfe-und-veröffentlichung).

| Einstellung (API-Feld) | Bedeutung |
|---|---|
| Name (`name`) | Bis zu 100 Zeichen. |
| Beschreibung (`description`) | Bis zu 1.000 Zeichen, auf der Startseite unter dem Namen angezeigt. |
| Adresse (`slug`) | 3 bis 48 Zeichen: `a-z`, `0-9` und innere Bindestriche. Auf dem Server eindeutig; einige Wörter sind reserviert. |
| Sprachen (`locales`), Hauptsprache (`default_locale`) | Bis zu 20 [Sprachen](widget.md#sprachen), in denen Einträge geschrieben und veröffentlicht werden; die Hauptsprache ist eine davon. |
| Analytics (`analytics_property_id`) | Wird mit dem Changelog gespeichert. |
| Agenten dürfen veröffentlichen (`agents_may_publish`) | Standardmäßig aus. Siehe [Entwürfe und Veröffentlichung](#entwürfe-und-veröffentlichung). |
| Veröffentlicht (`published`) | Standardmäßig aus. Ein unveröffentlichter Changelog und alle seine Einträge antworten allen außer Ihrem Team mit `404`, und das Widget zeigt keine Neuigkeiten. |

Logo und Akzentfarbe der Seiten sind das Branding des Posteingangs. Ändern Sie die Adresse, bleibt die alte
90 Tage lang für den Changelog reserviert, und die öffentlichen Adressen darunter leiten auf die neue um.

Per API: `POST /v1/changelogs` mit `inbox_id`, `name` und `slug`, dann
`PATCH /v1/changelogs/{changelogId}` mit `{"published": true}`.
`GET /v1/changelogs/{changelogId}/preview` liefert eine Seite so, wie Besucher sie sähen, auch solange der
Changelog unveröffentlicht ist; das Panel zeigt sie mit Vorschau samt Entwürfen (im Editor das Kürzel `v`).
`DELETE` entfernt den Changelog mit seinen Einträgen und Übersetzungen; aus seinen Einträgen erstellte Broadcasts
bleiben.

## 2. Einträge schreiben und übersetzen

Ein Eintrag hat ein bis drei Labels, ein optionales Bild und je Sprache eine *Übersetzung*. Öffnen Sie
Neuigkeiten, wählen Sie Neuer Eintrag, setzen Sie die Labels und schreiben Sie einen Titel und einen
Markdown-Text in der Hauptsprache.

| Feld | Grenze |
|---|---|
| Labels (`labels`) | Ein bis drei von `new`, `improved` und `fixed`. |
| Titel (`title`) | 3 bis 200 Zeichen. |
| Text (`body`) | Markdown, bis zu 50.000 Zeichen. Überschriften, Listen, Tabellen, Code, Links und Bilder; rohes HTML wird entfernt. |
| Adresse (`slug`) | Teil der Webadresse einer Übersetzung; aus dem Titel erzeugt, änderbar. Ein falscher Slug in einer URL leitet auf den richtigen um. |
| Bild (`image_id`) | Eines der [E-Mail-Bilder](designs.md#4-bilder) Ihres Workspace. |

Bilder im Text bleiben nur, wenn Sie sie in Ihren Workspace hochgeladen haben: im Editor hochladen oder mit
`POST /v1/changelogs/{changelogId}/images` (gleiche Typen, Größen und Neukodierung wie bei E-Mail-Bildern).
Ein Bild, das kein Eintrag, Entwurf oder Design verwendet, wird eine Woche nach dem Upload gelöscht.

Zum Übersetzen wählen Sie im Editor eine Sprache: Der Text der Hauptsprache bleibt neben der Übersetzung,
fehlende oder veraltete Übersetzungen sind markiert. Yuva übersetzt nicht für Sie. Ein veröffentlichter Eintrag
behält immer seine Übersetzung in der Hauptsprache.

Ein Workspace hat höchstens `YUVA_CHANGELOG_MAX_ENTRIES` Einträge
([Konfiguration](configuration.md#neuigkeiten)); darüber antwortet das Anlegen mit `422 hosted_limit`.

Per API, unter `/v1/changelogs/{changelogId}`: `POST /entries` schreibt einen Eintrag mit seiner ersten
Übersetzung; `GET /entries` filtert nach `state` (`draft`, `scheduled`, `published` oder `changed`), `label`
und `locale`; `PATCH /entries/{entryId}` ändert Labels und Bild; `PUT` und `DELETE` auf
`/entries/{entryId}/translations/{locale}` schreiben und entfernen eine Übersetzung.

## Entwürfe und Veröffentlichung

Eine Übersetzung hat einen Arbeitsentwurf und, einmal veröffentlicht, eine eingefrorene veröffentlichte Kopie.
Speichern ändert nur den Entwurf; die öffentliche Seite ändert sich, wenn Sie veröffentlichen.

- **Speichern.** Jedes Speichern trägt die `base_version`, die der Editor geladen hat. Hat dazwischen jemand
  gespeichert, wird das Speichern mit `409 version_conflict` abgelehnt, und die Antwort enthält die Übersetzung
  in ihrem jetzigen Stand, damit niemand die Arbeit eines anderen überschreibt. Andere offene Editoren erfahren,
  dass sich der Eintrag geändert hat.
- **Veröffentlichen.** Veröffentlichen friert jede Übersetzung mit Entwurf ein, rendert sie und stellt den
  Eintrag ins Web (`POST .../entries/{entryId}/publish`). Es braucht einen Text in der Hauptsprache
  (`409 no_default_translation`). Die erste Veröffentlichung setzt das angezeigte Datum; erneutes
  Veröffentlichen behält es. Eine später hinzugefügte oder geänderte Übersetzung wird allein mit `locale` in
  der Anfrage veröffentlicht. Zurückziehen (`POST .../unpublish`) nimmt den Eintrag aus Web und Widget und macht
  ihn wieder zum Entwurf, der sein Datum behält.
- **Wer veröffentlichen darf.** Agenten mit Zugriff auf den Posteingang schreiben Einträge und Übersetzungen.
  Inhaber und Admins veröffentlichen außerdem, planen, ziehen zurück, ändern Labels oder Bild eines
  veröffentlichten Eintrags, löschen veröffentlichte Einträge und ändern die Einstellungen des Changelogs.
  Schalten Sie Agenten dürfen veröffentlichen (`agents_may_publish`) ein, damit auch Agenten mit Zugriff auf den
  Posteingang veröffentlichen können. Agenten ohne dieses Recht nutzen Entwurf einreichen, und ein Inhaber oder
  Admin veröffentlicht.
- **Schlüssel.** Ein API-Schlüssel veröffentlicht nur mit dem Scope `changelog:publish`, zum Beispiel eine
  Release-Pipeline, die ihren Eintrag postet. Ohne ihn schreibt ein Schlüssel Entwürfe.
- **Assistenten veröffentlichen nie.** OAuth- und [MCP](mcp.md#neuigkeiten-aus-einem-assistenten)-Tokens
  schreiben Entwürfe, können aber, was auch eingestellt ist, keinen Changelog anlegen oder ändern, nicht
  veröffentlichen, planen, zurückziehen und keinen veröffentlichten Eintrag löschen
  (`403 member_session_required`).

## Planen

Planen Sie einen Eintrag für einen gewählten Zeitpunkt, in Ihrer Zeitzone und höchstens 365 Tage im Voraus
(`POST .../entries/{entryId}/schedule` mit `publish_at`). Ein Changelog hält höchstens 50 geplante Einträge
(`409 too_many_scheduled`). Erneutes Planen verschiebt die Zeit; Planung aufheben (`POST .../unschedule`) macht
den Eintrag wieder zum Entwurf.

Der Server veröffentlicht den Eintrag zu dieser Zeit selbst, so wie es die Person oder der Schlüssel getan hätte,
der ihn geplant hat. Ein geplanter Eintrag geht so, wie er ist, an die Öffentlichkeit; deshalb dürfen ihn nur
Veröffentlichungsberechtigte ändern, seine Übersetzungen speichern oder löschen (`403 entry_scheduled`).

Fehlt das Recht, wenn die Zeit kommt, geht nichts an die Öffentlichkeit: Der Eintrag wird mit
`schedule_failed` wieder zum Entwurf, was das Panel anzeigt.

| `schedule_failed.reason` | Bedeutung |
|---|---|
| `lost_right` | Das Mitglied, das ihn geplant hat, darf nicht mehr veröffentlichen oder ist nicht mehr da. |
| `key_revoked` | Der API-Schlüssel, der ihn geplant hat, wurde widerrufen. |
| `key_scope_lost` | Der Schlüssel hat `changelog:publish` nicht mehr oder sieht den Posteingang nicht mehr. |
| `no_default_translation` | Der Text der Hauptsprache wurde entfernt. |

Beheben Sie die Ursache und planen Sie erneut; die nächste Planung oder Veröffentlichung löscht den Fehler.

## 3. Öffentliche Seiten und Feed

Ein veröffentlichter Changelog liegt unter `<Ihre Yuva-Adresse>/changelog/{slug}` und leitet auf seine
Hauptsprache um. Eine Umleitung nach Browsersprache gibt es nicht.

| Adresse | Zeigt |
|---|---|
| `/changelog/{slug}/{locale}/` | Einträge, neueste zuerst, 20 je Seite; `?label=` mit `new`, `improved` oder `fixed` filtert, `?before=` blättert. |
| `/changelog/{slug}/{locale}/entries/{number}-{slug}` | Ein Eintrag. Ein falscher Slug leitet auf den richtigen um. |
| `/changelog/{slug}/{locale}/feed.atom` | Die neuesten 50 Einträge in dieser Sprache als Atom-Feed, mit Text und Labels jedes Eintrags. |
| `/changelog/{slug}/sitemap.xml` | Die veröffentlichten Seiten. |

Eine Seite listet nur Einträge mit veröffentlichter Übersetzung in der Sprache ihrer Adresse. Entwürfe,
geplante und zurückgezogene Einträge, unbekannte Sprachen und unveröffentlichte Changelogs antworten mit einem
schlichten `404`. Die Seiten tragen Canonical- und `hreflang`-Links, Open-Graph-Tags mit dem Bild des Eintrags
und strukturierte Daten für Suchmaschinen. Sie werden eine Minute lang zwischengespeichert und auf 120 Anfragen
pro Minute und Adresse begrenzt.

## 4. Im Widget

Hat der Posteingang einen veröffentlichten Changelog mit einem Eintrag, zeigt die [Startseite](widget.md) eine
Neuigkeiten-Kachel mit der Zahl der Einträge, die der Besucher nicht geöffnet hat, bis „9+“. Die App öffnet
sich im Panel: Einträge, neueste zuerst, mit Labels, Datum, Bild und Text sowie einem Link zur öffentlichen
Seite. Sie nutzt die Sprache des Besuchers, wenn der Changelog sie hat, sonst die Hauptsprache.

- **Ungelesen.** Das neueste geöffnete Datum wird nur im Browser des Besuchers gespeichert, je Kanal; Yuva
  speichert dazu nichts, und anonyme Besucher bekommen es auch. Ohne Markierung gelten Einträge, die älter als
  30 Tage sind, als gelesen. Das Öffnen der App markiert alles Gezeigte als gelesen.
- **Launcher-Badge.** Schalten Sie am Chat-Kanal Updates ankündigen (`announce_updates`) ein, und der Launcher
  zeigt die Zahl ungelesener Einträge, solange der Besucher keine ungelesenen Nachrichten hat. Nachrichten
  bekommen das Badge immer. Standardmäßig aus; die Kachel auf der Startseite zeigt die Zahl in jedem Fall.
- **Live.** Eine Veröffentlichung erreicht offene Widgets sofort.

Im Code: `openApp("whats-new")` öffnet die App, `openApp("whats-new", { entry: "<id>" })` einen einzelnen
Eintrag. Für einen eigenen Bildschirm hat der Client `whatsNew({ locale, cursor, limit })` und
`whatsNewEntry(id)`, und React-Sites haben `useWhatsNew({ locale })` in `useyuva/react`; siehe
[SDK-README](../../sdk/js/README.md). Die Endpunkte dahinter sind
`GET /client/v1/channels/{channel_key}/whats-new` und `.../whats-new/{entryId}`: keine Sitzung, es wird kein
Kontakt angelegt, 120 Lesezugriffe pro Minute und Adresse, eine Minute lang cachebar. Ohne veröffentlichten
Changelog antworten sie mit `404 whats_new_off`.

## 5. Einen Eintrag an Abonnenten mailen

Bei einem veröffentlichten Eintrag wählen Inhaber und Admins Per E-Mail an Abonnenten senden: bis zu 10
[Listen](lists.md) des Posteingangs, den Versandkanal und die Sprache. Yuva erstellt aus dem
[Design](designs.md) `product-update` den Entwurf eines Broadcasts: Titel, Labels und Datum, den Text, die
Bilder Ihres Workspace, oben das Bild des Eintrags und einen Button zur öffentlichen Seite des Eintrags. Eine
Tabelle oder ein Codeblock, den ein E-Mail-Design nicht fassen kann, wird zu einer Zeile mit Link auf den
Eintrag. Das Panel öffnet den Entwurf; der Eintrag listet seine Broadcasts und deren Status.

Dadurch wird nichts geplant oder gesendet. Prüfung, Testversand, Einwilligung, Abmeldelinks, Freigabe, Tempo
und Versand sind die jedes [Broadcasts](broadcasts.md). Per API:
`POST /v1/changelogs/{changelogId}/entries/{entryId}/broadcast` mit `locale`, `list_ids` und optional
`channel_id`; verlangt werden die Scopes `broadcasts:write` und `lists:read`, und für einen nicht
veröffentlichten Eintrag kommt `409 not_published`. Assistenten mailen nie einen Eintrag.

## Ereignisse und Webhooks

Veröffentlichen (auch wenn eine Planung auslöst und wenn eine veröffentlichte Übersetzung ersetzt wird),
Zurückziehen und das Löschen eines veröffentlichten Eintrags speichern ein Ereignis für die Mitglieder, die den
Posteingang sehen, und lassen sich als `changelog.entry.published`, `changelog.entry.unpublished` und
`changelog.entry.deleted` an [Webhooks](webhooks.md) senden.

## Referenz

### Scopes

API-Schlüssel, die ohne Nennung von Scopes erstellt werden, haben `changelog:read` und `changelog:write`;
`changelog:publish` muss genannt werden. Schlüssel, die vor diesen Scopes erstellt wurden, bekommen keinen
dazu. Siehe [Headless](headless.md#scopes).

| Scope | Erlaubt |
|---|---|
| `changelog:read` | Changelogs, Einträge mit Entwürfen lesen, Vorschau. |
| `changelog:write` | Einträge und Übersetzungen schreiben, Bilder hochladen. |
| `changelog:publish` | Changelogs anlegen, ändern und löschen; veröffentlichte Einträge veröffentlichen, planen, zurückziehen und löschen. OAuth-Clients wird er nie angeboten. |

Ein auf Posteingänge beschränkter Schlüssel sieht die Changelogs dieser Posteingänge; alle anderen antworten mit
`404`.

### Grenzen

| Element | Grenze |
|---|---|
| Einträge je Workspace | `YUVA_CHANGELOG_MAX_ENTRIES` ([Konfiguration](configuration.md#neuigkeiten)). |
| Geplante Einträge je Changelog | 50. |
| Sprachen je Changelog | 20. |
| Listen je gemailtem Eintrag | 10. |
| Öffentliche Seiten und Feeds | 120 Anfragen pro Minute und Adresse. |
| Vorschau | 60 Renderings pro Minute und Aufrufer. |

### Abschalten

`YUVA_WHATS_NEW=off` entfernt die Funktion: Ihre Endpunkte und öffentlichen Seiten antworten mit `404`, und das
Widget zeigt keine Neuigkeiten.
