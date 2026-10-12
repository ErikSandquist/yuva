# Von Help Scout umziehen

Kopieren Sie die Unterhaltungen eines Help-Scout-Kontos nach Yuva und leiten Sie seine Support-Adresse an
Yuva um. Yuva liest Help Scout nur. Die Übersicht steht unter [Zu Yuva umziehen](migrate.md).

## Bevor Sie anfangen

- Erstellen Sie eine App für Yuva. Öffnen Sie in Help Scout **Your Profile → My apps** und klicken Sie auf
  **Create My App**. Nennen Sie sie „Yuva“ und tragen Sie die **Redirect URL**, die der Schritt Connect von
  Yuva zeigt, als Redirection URL ein. Erstellen Sie sie und kopieren Sie **App ID** und **App Secret**.
  Help Scouts Seite: [Authentication](https://developer.helpscout.com/mailbox-api/overview/authentication/).
- Geben Sie im Schritt Connect **App ID** und **App Secret** ein und drücken Sie **Sign in with Help Scout**.
  Melden Sie sich als Nutzer an, der jedes Postfach sieht, das Sie umziehen möchten.

Help Scout kennt keine reine Leseberechtigung, die App könnte also Daten ändern. Schreibgeschützt hält sie
Yuvas Verbindung: Außer der Anmeldung sendet sie Help Scout nur Leseanfragen, sodass sich keine Unterhaltung,
kein Kunde und keine Einstellung ändert.

## Was kopiert wird

| Help Scout | Yuva |
|---|---|
| Unterhaltungen | Unterhaltungen mit allen Nachrichten |
| Kunden-Threads | Eingehende Nachrichten |
| Antworten | Ausgehende Nachrichten |
| Chat- und Telefon-Threads | Nachrichten, eingehend oder ausgehend je nach Verfasser |
| Notizen | Notizen, die nur Ihr Team sieht |
| Status: active | Offen |
| Status: pending | Ausstehend |
| Status: closed | Geschlossen |
| Tags | Labels, wie Sie sie zuordnen (oder keine) |
| Nutzer | Teammitglieder, wie Sie sie zuordnen; andere bleiben mit Namen erhalten |
| Zuständige Person | Die zuständige Person, wenn der Nutzer zugeordnet ist |
| Kunden | Kontakte, mit Name und Telefon |
| Mailboxes | Posteingänge, je Mailbox einer, wie Sie sie zuordnen |
| Anhänge | Dateien an den Nachrichten |

Spam, Entwürfe und gelöschte Unterhaltungen werden nie importiert. Unterhaltungen behalten ihre
ursprünglichen Zeiten und kommen [still](migrate.md#wie-der-verlauf-still-bleibt) an.

## Was nicht kopiert wird

Workflows und ihre Protokollzeilen, gespeicherte Antworten, Zufriedenheitsbewertungen, eigene Felder, Docs
(die Wissensdatenbank), Beacon-Einstellungen und Berichte. Yuva kopiert nur, was die Tabelle oben nennt.

## Originalnachrichten

Bei einem E-Mail-Thread liest Yuva die Originalnachricht, sodass Header, Formatierung und Anhänge so
ankommen, wie sie gesendet wurden. Help Scout bewahrt Originalquellen nur eine Weile auf; ist eine nicht mehr
da, nimmt Yuva stattdessen den Text des Threads aus Help Scouts Daten und holt dessen Anhänge einzeln. Eine
Datei, die größer als das Anhang-Limit des Servers ist, bleibt zurück.

## Limits

Help Scout begrenzt Anfragen pro Minute je nach Tarif, und alle Apps eines Kontos teilen sich das Limit. Yuva
bleibt bei der Hälfte des Limits, das Help Scout meldet, und wird langsamer, wenn Help Scout darum bittet. Ein
großes Konto dauert deshalb eine Weile. Help Scouts Seite:
[Rate limiting](https://developer.helpscout.com/mailbox-api/overview/rate-limiting/).

## Die Umstellung

Heute erreicht Ihre Support-Adresse Help Scout, weitergeleitet vom Mail-Anbieter, der sie hostet, oder durch
eine Regel dort. Kopieren Sie im Schritt Switch die [Weiterleitungsadresse](email.md#weiterleitungsadresse)
des Kanals und ändern Sie diese Weiterleitung so, dass die Support-Adresse stattdessen Yuva erreicht. Bis
dahin liest Yuva Help Scout alle zwei Minuten, sodass keine Unterhaltung verloren geht. Antworten aus Yuva
gehen von der Adresse des Kanals ab.

Drücken Sie dann „Send a test message“: Es funktioniert, wenn die Nachricht auf dem neuen Weg Yuva erreicht.
Nach einer ruhigen Phase oder mit „Finish now“ endet der Umzug
([so funktioniert es](migrate.md#testnachricht-ruhige-phase-und-abschluss)).

## Nach dem Abschluss

Löschen Sie die App, die Sie für Yuva erstellt haben: in Help Scout unter **Your Profile → My apps**. Yuva
löscht sein Token, wenn der Umzug endet, kann es in Help Scout aber nicht widerrufen; den Zugriff beendet
deshalb das Löschen der App. Lassen Sie die Weiterleitung, wie sie ist, und behalten Sie Help Scout gern als
Archiv.
