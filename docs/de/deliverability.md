# Zustellbarkeit

Ob Ihre Broadcasts im Posteingang ankommen, hängt vor allem davon ab, dass die Absenderdomain richtig
eingerichtet ist und dass niemand Ihre Mail als Spam markiert. Yuva hat eine Prüfung, die die Einrichtung
eines E-Mail-Kanals testet, indem sie ihm eine Testmail schickt. Diese Seite zeigt, wie Sie das Ergebnis
lesen und beheben, was sie findet.

Sie brauchen einen [E-Mail-Kanal](email.md) mit einem SMTP-Konto, das [Broadcasts](broadcasts.md) versendet,
und Zugriff auf das DNS der Domain, von der er sendet.

## 1. Die Prüfung starten

Im Panel: Settings, der Posteingang, Where customers write from, den E-Mail-Kanal öffnen, dann im Abschnitt
Broadcast sending „Check deliverability and set up DKIM“. Klicken Sie „Run a check“.

Yuva sendet über das Broadcast-Konto des Kanals eine Testmail an die eigene Adresse des Kanals, mit den
Headern, die ein Broadcast trägt, liest dann, wie der empfangende Server sie beurteilt hat, und schaut in Ihr
DNS. Das Ergebnis erscheint innerhalb einer Minute, spätestens nach 10 Minuten. Sie können sechs Prüfungen
pro Stunde und Kanal starten. Die Testmail wird nie zu einer Konversation.

Starten Sie sie vor dem ersten Broadcast und nach jeder Änderung an DNS oder SMTP-Konto. Die Broadcast-Prüfung
warnt, solange ein Kanal nie geprüft wurde.

### Das Ergebnis lesen

| Zeile | Was sie sagt |
|---|---|
| SPF | Ob der sendende Server laut SPF-Eintrag senden darf. Sie sagt auch, ob die geprüfte Domain Ihre Absenderdomain ist. Besteht SPF nur für die Domain Ihres Anbieters, hängt DMARC allein an DKIM; das ist in Ordnung, solange DKIM besteht. |
| DKIM | Ob eine Signatur bestanden hat, für welche Domain und ob diese Ihre Absenderdomain ist. Sie sagt auch, ob die Signatur `List-Unsubscribe` und `List-Unsubscribe-Post` abdeckt, was Gmail und Yahoo verlangen. |
| DMARC | Ob DMARC bestanden hat, und Ihre Richtlinie: `none` (Empfänger berichten nur), `quarantine` (durchgefallene Mail geht in den Spam) oder `reject`. Ohne DMARC-Eintrag werden Sie aufgefordert, einen zu veröffentlichen. |
| MX | Ob die Domain der Kanaladresse Mailserver hat. Antworten auf einen Broadcast kommen dort an. |
| Yuva DKIM | Nur wenn der Kanal [einen eigenen Schlüssel](#4-mit-yuva-dkim-signieren) hat: ob das DNS damit antwortet. |

Die Broadcast-Prüfung nutzt dasselbe Ergebnis. Sie warnt, wenn der Kanal nie geprüft wurde, die Prüfung nicht
fertig wurde, DMARC fehlt, DKIM nicht zu Ihrer Absenderdomain passt oder die Abmelde-Header nicht signiert
sind. Sie blockiert den Versand, wenn keine DKIM-Signatur bestanden hat.

Kann die Prüfung kein Ergebnis lesen, sehen Sie eines davon:

| Meldung | Was zu tun ist |
|---|---|
| Nicht binnen 10 Minuten zurück (`timeout`) | Nur das DNS wurde geprüft. Stellen Sie sicher, dass Mail an die Adresse des Kanals Yuva erreicht (Weiterleitung oder MX), und prüfen Sie erneut. |
| Ohne Ergebnisse eines vertrauten Servers angekommen (`unverifiable`) | Der Betreiber des Servers hat den empfangenden Server, dem Yuva vertraut ([`YUVA_INGRESS_AUTHSERV_ID`](configuration.md#eingehende-e-mail)), nicht gesetzt, daher konnten SPF, DKIM und DMARC nicht gelesen werden. Nur das DNS wurde geprüft. |
| Konnte nicht gesendet werden (`failed`) | Prüfen Sie die SMTP-Einstellungen; die Antwort des SMTP-Servers wird angezeigt. |

## 2. Die Absenderdomain einrichten

Diese Einträge gehören zur Domain der From-Adresse des Kanals. SPF, DKIM und DMARC müssen bestehen, und für
DMARC muss SPF oder DKIM für Ihre eigene Domain gelten (ausgerichtet).

| Eintrag | Was zu veröffentlichen ist |
|---|---|
| SPF | Ein TXT-Eintrag pro Name, der Ihren Anbieter einbindet. |
| DKIM | Die Einträge Ihres Anbieters oder [Yuva DKIM](#4-mit-yuva-dkim-signieren). |
| DMARC | `_dmarc.example.com TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"`. Wechseln Sie zu `p=quarantine`, wenn die Berichte stimmen. |

Einträge brauchen Zeit, bis sie überall ankommen. Starten Sie die Prüfung erneut, wenn Ihr Anbieter sie als
verifiziert meldet.

### Amazon SES

1. Legen Sie in der SES-Konsole für Ihre Absenderdomain eine Domain-Identität mit Easy DKIM an. SES zeigt drei
   CNAME-Einträge; veröffentlichen Sie sie. Die Verifizierung kann bis zu 72 Stunden dauern. Easy DKIM
   signiert mit Ihrer Domain, die Abmelde-Header eingeschlossen; ein SES-Kanal verlässt sich darauf und nutzt
   [Yuva DKIM](#4-mit-yuva-dkim-signieren) nicht.
2. Richten Sie eine eigene MAIL-FROM-Domain ein, damit SPF auch für Ihre eigene Domain besteht: Wählen Sie eine
   Subdomain der Identität, die Sie für nichts anderes nutzen (etwa `bounce.example.com`), und veröffentlichen
   Sie darauf mit den Werten der Konsole einen MX-Eintrag (`10 feedback-smtp.<Region>.amazonses.com`) und einen
   TXT-Eintrag (`"v=spf1 include:amazonses.com ~all"`). Unter diesem Namen darf es nur einen MX-Eintrag geben.
3. Ein neues SES-Konto ist in der Sandbox: Es darf nur an verifizierte Adressen senden, 200 Nachrichten am Tag und
   eine pro Sekunde. Beantragen Sie vor dem ersten Broadcast in der SES-Konsole (Account dashboard) den
   Produktionszugang. Der Sandbox-Status gilt pro Region.
4. In der Konsole SMTP settings, „Create SMTP credentials“. Benutzername und Passwort, die angezeigt werden,
   sind nicht Ihre AWS-Zugangsschlüssel und gelten für eine Region: Nutzen Sie den SMTP-Endpunkt derselben Region.
   Speichern Sie sie, wenn sie angezeigt werden; sie lassen sich nicht erneut anzeigen.
5. Schalten Sie im Kanal „Use a separate account for broadcasts“ ein und tragen Endpunkt, Port, Verschlüsselung
   und diese Zugangsdaten ein.
6. Für Bounces und Beschwerden verbinden Sie SNS, wie unter [Bounces](email.md#bounces) beschrieben.

### Postmark

1. Fügen Sie in Postmark Ihre Absenderdomain hinzu und veröffentlichen Sie die DKIM- und Return-Path-Einträge,
   die es zeigt.
2. Aktivieren Sie in den Settings des Servers SMTP. Benutzername und Passwort sind das Server-API-Token (oder
   Access Key und Secret Key eines SMTP-Tokens).
3. Nutzen Sie im Kanal für den Broadcast-Stream den Host `smtp-broadcasts.postmarkapp.com`
   (`smtp.postmarkapp.com` ist der für Transaktionsmail), Port 587, 2525 oder 25, mit STARTTLS.

### Ein anderes SMTP-Relay

1. Fügen Sie im Panel Ihres Anbieters Ihre Absenderdomain hinzu. Er nennt Ihnen SMTP-Host, Port,
   Verschlüsselung, Benutzername und Passwort, die DKIM-Einträge und das `include:` für Ihren SPF-Eintrag.
2. Lassen Sie es bei einem SPF-Eintrag pro Name: Fügen Sie das `include:` in den vorhandenen Eintrag ein.
3. Kann der Anbieter nicht mit Ihrer eigenen Domain signieren, nutzen Sie
   [Yuva DKIM](#4-mit-yuva-dkim-signieren).
4. Tragen Sie die SMTP-Daten im Kanal unter „Use a separate account for broadcasts“ ein. Ein eigenes Konto hält
   Beschwerden über Marketingmail von Ihren Support-Antworten fern.

## 3. Erneut prüfen

Starten Sie die Prüfung. Ziel: SPF und DKIM bestehen, DKIM für Ihre eigene Domain und mit Abdeckung der
Abmelde-Header, und ein DMARC-Eintrag. Für den Anfang genügt ein DMARC-Eintrag mit `p=none`.

## 4. Mit Yuva DKIM signieren

Nutzen Sie es, wenn die Prüfung sagt, dass DKIM nur für die Domain Ihres Anbieters besteht oder die Signatur die
Abmelde-Header nicht abdeckt, oder wenn Ihr Anbieter gar nicht mit Ihrer Domain signieren kann. Yuva signiert
dann Broadcasts, Testmails, Bestätigungs- und Willkommensmails des Kanals selbst, über die Abmelde-Header. Die
Signatur Ihres Anbieters bleibt daneben bestehen; Support-Antworten ändern sich nicht.

Yuva DKIM ist für andere Relays als Amazon SES gedacht. SES ändert `From`, `To`, `Reply-To` und `Date`, nachdem
Yuva signiert hat, was jede vorher angebrachte Signatur bricht, und signiert jede Mail selbst mit Ihrer
verifizierten Domain. Yuva lehnt es deshalb ab, sein DKIM für einen Kanal einzuschalten, dessen Broadcasts über
SES gehen, und signiert über SES auch dann nicht, wenn ein Schlüssel früher eingeschaltet wurde; die Sendeseite
des Kanals nennt den Grund.

1. Öffnen Sie die Sendeseite des Kanals (oben) und suchen Sie Yuva DKIM. Wählen Sie einen Selector (Kleinbuchstaben,
   Ziffern und Bindestriche; standardmäßig `yuva`) und klicken Sie „Create key“. Der Schlüssel ist RSA 2048; der
   private Schlüssel verlässt den Server nie.
2. Veröffentlichen Sie den angezeigten TXT-Eintrag unter dem angezeigten Namen (er endet auf Ihre Absenderdomain).
   Begrenzt Ihr DNS-Anbieter einen Textabschnitt auf 255 Zeichen, teilen Sie den Wert auf mehrere Abschnitte eines
   Eintrags auf.
3. Warten Sie ein paar Minuten (manche Anbieter brauchen länger), dann klicken Sie „Check DNS and start signing“.
   Das macht ein Owner oder Admin, sobald der Eintrag veröffentlicht ist.
4. Starten Sie die Prüfung erneut. Die Zeile Yuva DKIM sollte bestehen.

„Stop signing“ schaltet es sofort aus; „Delete key“ entfernt den Schlüssel, danach können Sie den DNS-Eintrag
löschen. Ein neuer Schlüssel braucht einen neuen Eintrag. Wechselt die Absenderdomain des Kanals, wird das
Signieren ausgeschaltet, bis Sie es für die neue Domain wieder einschalten.

## 5. Gmail und Yahoo

Gmail und Yahoo haben Regeln für Absender großer Mailmengen. Gmail wendet sie ab 5.000 Nachrichten am Tag an
Gmail-Adressen an; Yahoos Richtlinien nennen keine Zahl.

- **Authentifizieren.** SPF und DKIM sowie ein DMARC-Eintrag (die Richtlinie `none` ist erlaubt). Die Domain der
  From-Adresse muss mit der SPF- oder DKIM-Domain übereinstimmen. Gmail verlangt DKIM-Schlüssel von mindestens
  1.024 Bit und empfiehlt 2.048.
- **Abmeldung mit einem Klick.** Ein `List-Unsubscribe`-Header, `List-Unsubscribe-Post: List-Unsubscribe=One-Click`
  ([RFC 8058](https://www.rfc-editor.org/rfc/rfc8058)) und ein sichtbarer Abmelde-Link in der Mail. Yahoo sagt,
  Anfragen seien innerhalb von zwei Tagen umzusetzen. Yuva fügt jedem Broadcast beide Header und den Link hinzu und
  meldet sofort ab.
- **Spam-Beschwerden niedrig halten.** Unter 0,3 %; Google empfiehlt unter 0,1 %. Yuva pausiert einen Broadcast
  bei 0,3 % (siehe [Pausen, die Yuva selbst macht](broadcasts.md#pausen-die-yuva-selbst-macht)).
- **TLS und gültiges DNS für sendende Server.** Gmail verlangt eine TLS-Verbindung sowie Forward- und
  Reverse-DNS für die sendenden IPs. Das ist Sache Ihres SMTP-Anbieters.

Quellen: [Googles Absenderrichtlinien](https://support.google.com/a/answer/81126) und
[Yahoos Best Practices für Absender](https://senders.yahooinc.com/best-practices/). Beide ändern sich; lesen Sie
sie bei der Einrichtung.

### Google Postmaster Tools

[Postmaster Tools](https://support.google.com/mail/answer/9981691) zeigt, was Gmail von Ihrer Mail sieht:
Spam-Rate, Reputation, Authentifizierung und Zustellfehler, für Mail an persönliche Gmail-Adressen. Melden Sie sich
mit einem Google-Konto an, fügen Sie die Domain hinzu, mit der Ihre Mail signiert oder gesendet wird (die
DKIM-`d=`-Domain oder die SPF-/Return-Path-Domain), und verifizieren Sie sie mit dem TXT- oder CNAME-Eintrag, den
Google zeigt. Bis die Domain verifiziert ist, erscheint nichts, und Tage mit wenig Mail zeigen keine Daten.

## 6. Langsam anfangen

Eine neue Absenderdomain hat keine Reputation, und ein großer erster Versand sieht genau nach Spam aus.

- **Gehostete Server.** Ein neuer Workspace muss eventuell [um Freigabe bitten](broadcasts.md#freigabe-auf-gehosteten-servern)
  und wärmt sich dann selbst auf: 500 Empfänger am ersten Tag, nach jedem sauberen Tag das Doppelte (mindestens 100
  Mails gesendet, Bounces unter 3 %, Beschwerden unter 0,1 %). Ein schlechter Tag nimmt eine Verdopplung zurück. Der
  Betreiber kann ein anderes Limit setzen.
- **Ihr eigener Server.** Es begrenzt Sie nur, was Sie selbst setzen. Setzen Sie
  [`YUVA_BROADCAST_DAILY_RECIPIENTS`](configuration.md#broadcasts), um die Empfänger pro Workspace und Tag zu
  begrenzen (ein Versand, der es erreicht, läuft am nächsten Tag weiter), und halten Sie das Tempo des Kanals unter
  dem, was Ihr Anbieter erlaubt. Erhöhen Sie das Limit in den ersten Wochen selbst.
- **In beiden Fällen:** Senden Sie zuerst an Ihre aktivsten Abonnenten, halten Sie die Liste sauber (der
  [Import](importing.md) lässt Adressen aus, die keine Mail empfangen können) und sehen Sie sich Bounces und
  Beschwerden des ersten Broadcasts an, bevor Sie den nächsten senden. Für eine dedizierte IP-Adresse folgen Sie den
  Aufwärmhinweisen Ihres Anbieters.

## Aus dem Backend

| Endpoint | Zweck |
|---|---|
| `POST /v1/channels/{channelId}/deliverability-check` | Startet eine Prüfung (`202`). Höchstens sechs pro Stunde und Kanal (`429 rate_limited`); `409 email_channel_required`, wenn der Kanal nicht senden kann. |
| `GET /v1/channels/{channelId}/deliverability` | Die letzte Prüfung: `state`, `spf`, `dkim`, `dmarc`, `mx`, `yuva_dkim`. `404`, wenn nie geprüft. |
| `POST /v1/channels/{channelId}/dkim-key` | Legt den Schlüssel aus einem `selector` an; die Antwort enthält `dns_name` und `dns_value` zum Veröffentlichen. `409 dkim_exists`, wenn es schon einen gibt. |
| `PATCH /v1/channels/{channelId}` | `email.dkim.enabled: true` schaltet das Signieren ein, sobald das DNS antwortet (`409 dkim_dns_missing`); `false` schaltet es aus. |
| `DELETE /v1/channels/{channelId}/dkim-key` | Entfernt den Schlüssel. |

`state` ist `queued`, `sent` (bis die Testmail ankommt), `received`, `unverifiable`, `timeout` oder `failed`.
Anlegen und Prüfen brauchen `inboxes:manage`, Lesen braucht `inboxes:read`. Die genauen Formen stehen im
[API-Vertrag](../../openapi/openapi.yaml).
