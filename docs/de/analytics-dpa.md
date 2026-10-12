# Datenverarbeitung für Analytics

Wird ein Yuva-Server für Sie von jemand anderem betrieben (ein gehosteter Dienst), verarbeitet dieser Betreiber
die Daten Ihrer Besucher, wenn Sie Analytics nutzen. Dies ist der Entwurf der Vereinbarung, die Artikel 28 der
DSGVO zwischen Ihnen und dem Betreiber verlangt. Er ist ein Ausgangspunkt zum Anpassen mit Ihrer Anwältin oder
Ihrem Anwalt, keine Rechtsberatung und kein unterschriebener Vertrag. Auf Ihrem eigenen Server sind Sie der
Verantwortliche und niemand verarbeitet die Daten für Sie; nichts davon brauchen Sie dann.

## Parteien und Rollen

| Partei | Rolle | Aufgabe |
|---|---|---|
| Der Eigentümer des Workspace (Sie) | Verantwortlicher | Entscheidet, warum und wie Besucher gezählt werden: welche Websites, welche Einstellungen, wie lange. Informiert Besucher ([Datenschutzerklärung](analytics-privacy.md#ihre-datenschutzerklärung)). Beantwortet deren Anfragen. |
| Der Serverbetreiber | Auftragsverarbeiter | Betreibt den Server, empfängt die Events, die Ihre Seiten senden, speichert und löscht sie, wie es die Einstellungen sagen, und tut sonst nichts damit. |

## Was verarbeitet wird

| Punkt | Einzelheiten |
|---|---|
| Gegenstand | Reichweitenmessung für die Websites Ihres Workspace. |
| Betroffene | Besucher dieser Websites. |
| Datenkategorien | Gekürzte IP-Adresse (im Speicher genutzt, nicht gespeichert), Browser und Gerätetyp, Besucher-Hash, Host und Pfad der Seite, Referrer, `utm_*`-Parameter, Land, Region und Stadt (bei Websites für Kinder nur das Land), Sprache, Bildschirmgrößenklasse. Die vollständige Liste steht auf der [Datenschutzseite](analytics-privacy.md#was-gespeichert-wird). |
| Besondere Kategorien | Werden nicht erfragt. Der Server speichert keine Werte in Form einer E-Mail-Adresse. |
| Dauer | Detaillierte Datensätze standardmäßig 90 Tage (vom Betreiber eingestellt, 7 bis 400); Tagessummen standardmäßig 25 Monate (`retention_months` je Website); im Übrigen die Laufzeit der Vereinbarung. |

## Weisungen

Der Auftragsverarbeiter verarbeitet die Daten nur auf Ihre dokumentierte Weisung (Art. 28 Abs. 3 lit. a). Die
Einstellungen einer Website sind diese Weisungen: ihre Domains, Zielgruppe, Erfassungsmodus, nicht gezählte Seiten
und Adressen, Pfadregeln und Aufbewahrung.

## Vertraulichkeit und Sicherheit

- Mitarbeitende mit Zugriff auf die Daten sind zur Vertraulichkeit verpflichtet.
- Die Daten werden über TLS übertragen. Der Betreiber beschreibt, wie er gespeicherte Daten und Backups schützt
  und wer je nach Rolle Zugriff hat.
- Der Betreiber trifft die nach Artikel 32 erforderlichen Sicherheitsmaßnahmen (Art. 28 Abs. 3 lit. c).

## Unterauftragsverarbeiter

Yuva Analytics selbst nutzt keine: Standortabfrage und Browser-Parsing laufen im Server. Die
Infrastrukturanbieter des Betreibers (Hosting, Speicher, Backups) können die Daten in dessen Auftrag verarbeiten.
Der Betreiber nennt sie oder sagt, wie Sie davon erfahren, und legt die Bedingungen dar, unter denen er einen
weiteren Auftragsverarbeiter einsetzt (Art. 28 Abs. 3 lit. d).

## Unterstützung bei Anfragen

Im Standardmodus lassen sich die Datensätze eines Besuchers nicht finden, weil über einen Tag hinaus nichts einen
Besucher kennzeichnet. Der Auftragsverarbeiter hilft Ihnen, Anfragen mit dieser Aussage zu beantworten. Was Sie
löschen, ist endgültig gelöscht.

## Löschung und Rückgabe

Sie können Ihre Daten jederzeit selbst entnehmen und löschen:

- `POST /v1/analytics/properties/{propertyId}/query` mit `?format=csv` liefert die Zahlen als CSV.
- `POST /v1/analytics/properties/{propertyId}/delete-data` löscht einen Tagesbereich oder mit `all: true`
  alles; `DELETE /v1/analytics/properties/{propertyId}` entfernt die Website samt Daten.
- Endet die Vereinbarung, löscht der Betreiber den Rest nach Ihrer Wahl oder gibt ihn zurück und löscht Kopien,
  sofern das Gesetz keine Speicherung verlangt. Backups laufen innerhalb der vom Betreiber genannten
  Backup-Frist aus.

## Audits

Der Betreiber stellt die Informationen bereit, die zum Nachweis der Einhaltung dieser Vereinbarung nötig sind,
und ermöglicht Audits durch Sie oder einen von Ihnen benannten Prüfer (Art. 28 Abs. 3 lit. h).

## Übermittlungen

Der Betreiber nennt das Land, in dem Server und Backups stehen, und stützt sich überall dort, wo das Gesetz es
verlangt, auf einen gültigen Übermittlungsmechanismus.

## Quelle

[DSGVO Artikel 28, Auftragsverarbeiter](https://www.legislation.gov.uk/eur/2016/679/article/28) (Text der UK GDPR;
die EU-DSGVO hat denselben Artikel).
