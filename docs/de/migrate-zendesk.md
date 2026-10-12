# Von Zendesk umziehen

Kopieren Sie die Tickets eines Zendesk-Support-Kontos nach Yuva und leiten Sie seine Support-Adresse an Yuva
um. Yuva liest Zendesk nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## Bevor Sie anfangen

- Sie brauchen einen Zendesk-Admin: Nur ein Admin kann alle Tickets lesen. Der Tarif muss API-Zugriff
  enthalten.
- Erstellen Sie einen OAuth-Client für Yuva. Öffnen Sie im Admin Center **Apps and integrations → APIs →
  OAuth clients** und klicken Sie auf **Add OAuth client**. Nennen Sie ihn „Yuva“, wählen Sie als Client-Art
  **Confidential**, setzen Sie den Scope auf `read` und tragen Sie die **Redirect URL** ein, die der Schritt
  Connect von Yuva zeigt. Speichern Sie und kopieren Sie Client-Kennung und Secret sofort: Zendesk zeigt das
  Secret nur einmal vollständig. Zendesks Seite:
  [Using OAuth authentication with your application](https://support.zendesk.com/hc/en-us/articles/4408845965210-Using-OAuth-authentication-with-your-application).
- Geben Sie im Schritt Connect Ihre **Subdomain** (`acme` bei `acme.zendesk.com`), den **Client identifier**
  und das **Client secret** ein und drücken Sie **Sign in with Zendesk**. Melden Sie sich als Admin an und
  erlauben Sie den Zugriff.

Yuva fragt nur den Scope `read` an und lehnt eine Anmeldung ab, die mehr gewährt. Außer der Anmeldung sendet
seine Verbindung Zendesk nur Leseanfragen, sodass sich kein Ticket, kein Nutzer und keine Einstellung ändert.

## Was kopiert wird

| Zendesk | Yuva |
|---|---|
| Tickets | Unterhaltungen mit allen Nachrichten |
| Öffentliche Kommentare | Nachrichten: vom Anfragenden eingehend, von einem Agenten ausgehend |
| Private Kommentare | Notizen, die nur Ihr Team sieht |
| Status: new, open | Offen |
| Status: pending, on-hold | Ausstehend |
| Status: solved, closed | Geschlossen |
| Tags | Labels, wie Sie sie zuordnen (oder keine) |
| Agenten und Admins | Teammitglieder, wie Sie sie zuordnen; andere bleiben mit Namen erhalten |
| Zuständige Person | Die zuständige Person, wenn der Agent zugeordnet ist |
| Anfragende | Kontakte |
| Brands | Posteingänge, je Brand einer, wie Sie sie zuordnen |
| Anhänge | Dateien an den Nachrichten ([unten](#anhänge)) |

Spam und gelöschte Tickets werden nie importiert. Unterhaltungen behalten ihre ursprünglichen Zeiten und
kommen [still](migrate.md#wie-der-verlauf-still-bleibt) an.

## Was nicht kopiert wird

Makros, Automatisierungen, Trigger und Ansichten; Zufriedenheitsbewertungen; das Help Center (Artikel und
Community); eigene Ticket- und Nutzerfelder; Organisationen und Gruppen; Berichte. Yuva kopiert nur, was
die Tabelle oben nennt.

## Anhänge

Yuva holt die Dateien jedes Kommentars. Zendesk hält einige auf fremden Hosts; sie werden ohne Yuvas Token
geholt, das nur an Ihr Zendesk-Konto und an Zendesks eigenen Datei-Host geht. Eine Datei, die größer als das
Anhang-Limit des Servers ist, oder eine, die Zendesk nicht mehr hat, bleibt zurück; die Nachricht wird ohne
sie kopiert.

## Limits

Zendesk begrenzt Anfragen pro Minute je nach Tarif; sein inkrementeller Export, der die Tickets auflistet,
erlaubt 10 pro Minute. Yuva bleibt bei der Hälfte: höchstens 5 Export-Anfragen pro Minute und für alles
andere die Hälfte des Limits, das Zendesk für das Konto meldet. Ein großes Konto dauert deshalb eine Weile,
und das Kopieren wird von selbst langsamer, wenn Zendesk darum bittet. Zendesks Seiten:
[Incremental exports](https://developer.zendesk.com/api-reference/ticketing/ticket-management/incremental_exports/),
[Usage limits](https://developer.zendesk.com/api-reference/ticketing/account-configuration/usage_limits/).

## Die Umstellung

Heute erreicht Ihre Support-Adresse Zendesk, weitergeleitet vom Mail-Anbieter, der sie hostet, oder durch
eine Regel dort. Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse)
des Kanals und ändern Sie diese Weiterleitung so, dass die Support-Adresse stattdessen Yuva erreicht. Bis
dahin liest Yuva Zendesk alle zwei Minuten, sodass kein Ticket verloren geht. Antworten aus Yuva gehen von
der Adresse des Kanals ab.

Drücken Sie dann „Send a test message“: Es funktioniert, wenn die Nachricht auf dem neuen Weg Yuva erreicht.
Nach einer ruhigen Phase oder mit „Finish now“ endet der Umzug
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## Nach dem Abschluss

Löschen Sie den OAuth-Client, den Sie für Yuva erstellt haben: im Admin Center unter **Apps and integrations
→ APIs → OAuth clients**. Yuva löscht sein Token, wenn der Umzug endet, kann es in Zendesk aber nicht
widerrufen; den Zugriff beendet deshalb das Löschen des Clients. Lassen Sie die Weiterleitung, wie sie ist,
und behalten Sie Zendesk gern als Archiv.
