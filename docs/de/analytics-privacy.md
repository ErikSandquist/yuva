# Analytics und Datenschutz

Diese Seite sagt, was Yuva Analytics über Ihre Besucher speichert und wie lange, ob Ihre Website ein
Cookie-Banner braucht, und liefert Texte für Ihre Datenschutzerklärung. Es sind Informationen mit Quellen, keine
Rechtsberatung; Gesetze unterscheiden sich je nach Land und ändern sich, prüfen Sie Ihren Fall mit einer Anwältin
oder einem Anwalt oder Ihrer Datenschutzbehörde.

## Was gespeichert wird

| Daten | Was damit geschieht |
|---|---|
| Cookies, lokaler Speicher, Geräte- oder Werbe-IDs | Keine werden gesetzt. Das Einzige, was das Skript je auf einem Gerät speichert, ist der eigene Widerspruch eines Besuchers (`yuva_optout`), und nur auf dessen Wunsch. |
| IP-Adresse | Wird im ersten Schritt, bevor etwas anderes sie nutzt, auf `/24` (IPv4) oder `/48` (IPv6) gekürzt. Standort und Besucher-Hash entstehen nur aus der gekürzten Adresse. Keine Adresse wird gespeichert, weder vollständig noch gekürzt. |
| Browser (User-Agent) | Wird gelesen, um Browser, System und Gerätetyp zu bestimmen, und fließt in den Besucher-Hash ein. Der Text selbst wird nicht gespeichert. |
| Besucher | Ein Hash mit Schlüssel (HMAC-SHA-256) aus Website, gekürzter Adresse und Browser, unter einem zufälligen Salt, der einen Tag und eine Website lang existiert. Der Salt wird 2 Stunden nach Ende dieses Tages gelöscht (`YUVA_ANALYTICS_SALT_GRACE`, höchstens 6 Stunden); danach kann niemand den Hash neu berechnen. Ein Besucher ist also nur innerhalb eines Tages und einer Website derselbe. |
| Was der Besucher tat | Host und Pfad der Seite, Host und Pfad des Referrers, die `utm_*`-Parameter, Land, Region und Stadt, Browser, System, Gerätetyp, Bildschirmgrößenklasse, Sprache. Der Rest des Query-Strings wird verworfen, alles in Form einer E-Mail-Adresse entfernt. |
| Websites für Kinder | Mit **Audience: Children** wird nur das Land gespeichert: keine Region oder Stadt, keine Bildschirmgröße, keine Sprache. |
| Do Not Track, Global Privacy Control | Ein Browser, der eines von beiden sendet, sendet nichts, und der Server verwirft jedes Event, das eines trägt. Das lässt sich nicht abschalten. |

Yuva behandelt die detaillierten Datensätze bis zu ihrer Löschung als pseudonyme personenbezogene Daten, weil der
Hash, solange der Salt eines Tages existiert, innerhalb dieses Tages ein Gerät vom anderen unterscheiden kann. Die
Tagessummen zählen Besuche und lassen sich keiner Person zuordnen.

## Wie lange

| Daten | Aufbewahrung |
|---|---|
| Tages-Salt | Bis 2 Stunden nach Ende des Tages der Website |
| Detaillierte Datensätze (Besuche und Events) | Standard 90 Tage; der Betreiber stellt mit [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics) 7 bis 400 ein |
| Tagessummen | Standard 25 Monate; `retention_months` an der Website ändert das |

Sie können Tage oder alles jederzeit löschen ([Schlüssel und Daten](analytics.md#schlüssel-und-daten)).

Eine [API-Property](analytics-api.md) speichert keine Besucherdaten: Ihre Zahlen sind Request-Zählungen pro Minute,
Stunde und Tag. Standardmäßig bleiben Minuten 8 Tage, Stunden 90 Tage und Tage 25 Monate. Ein Consumer-Name kann eine
Person identifizieren, wenn Sie Consumer nach Personen benennen; er lässt sich auf Anfrage
[löschen](analytics-api-sharing.md#einen-consumer-löschen).

## Brauchen Sie ein Cookie-Banner?

Ein Banner betrifft die Einwilligung dazu, etwas auf dem Gerät des Besuchers zu speichern oder auszulesen. Yuva
Analytics speichert dort nichts, aber ein Skript, das den Browser Daten senden lässt, gilt nach Lesart der EU als
Zugriff; die Antwort hängt also davon ab, wo Ihre Besucher sind. Kurz:

- **Frankreich, Vereinigtes Königreich und Türkiye:** Der Standardmodus passt zu den veröffentlichten
  Ausnahmen. Sie brauchen einen Eintrag in Ihrer Datenschutzerklärung und einen Widerspruchsweg, kein
  Einwilligungsbanner.
- **Übrige EU:** Wenn Ihr Land keine Ausnahme wie Frankreich hat, ist der sichere Weg der
  [Consent-Modus](analytics.md#consent-modus): Es wird nichts gesendet, bis Ihr Consent-Tool sagt, dass der
  Besucher zugestimmt hat.
- **Anderswo:** Prüfen Sie die örtliche Regel. Diese Seite deckt sie nicht ab.

### Nach Ort

| Ort | Was die Regel sagt | Wo sie veröffentlicht ist |
|---|---|---|
| EU (ePrivacy Art. 5 Abs. 3) | Der EDSA liest Art. 5 Abs. 3 so, dass er gilt, wenn eine Stelle Software oder JavaScript verteilt, die den Browser Informationen senden lässt: "Such access clearly falls within the scope of Article 5(3)" (Absatz 33). Auch das Erheben einer IP-Adresse fällt darunter, außer die Stelle kann sicherstellen, dass die Adresse nicht vom Gerät des Nutzers stammt (Absatz 55). Er sagt auch, dass das Fallen unter den Artikel nicht immer eine Einwilligung bedeutet: Im Einzelfall wird geprüft, ob eine Ausnahme greift (Absatz 56). | [EDPB Guidelines 2/2023, Version 2.0, angenommen am 7. Oktober 2024](https://www.edpb.europa.eu/system/files/2024-10/edpb_guidelines_202302_technical_scope_art_53_eprivacydirective_v2_en_0.pdf) |
| Frankreich | Die CNIL nimmt Reichweitenmessung von der Einwilligung aus, wenn Sie die Nutzer informieren, ihnen den Widerspruch ermöglichen, die Daten nur für Reichweitenmessung verwenden, sie nicht mit anderen Verarbeitungen abgleichen, den Umfang auf die Website oder App eines Herausgebers begrenzen, das letzte Byte der IP-Adresse abschneiden und Tracker auf 13 Monate begrenzen. | [CNIL sheet no. 16](https://www.cnil.fr/en/sheet-ndeg16-use-analytics-your-websites-and-applications) |
| Vereinigtes Königreich | Schedule A1 der Privacy and Electronic Communications Regulations (PECR), durch den Data (Use and Access) Act 2025 eingefügt und seit 5. Februar 2026 in Kraft, erlaubt einem Dienst, ohne Einwilligung Informationen auf einem Gerät zu speichern oder abzurufen, für Statistiken darüber, wie der Dienst genutzt wird, um ihn zu verbessern, wenn die Informationen nicht geteilt werden (außer um einer anderen Person bei der Verbesserung des Dienstes zu helfen), der Nutzer klare und umfassende Informationen und einen einfachen, kostenlosen Widerspruchsweg erhält (Absatz 5). | [PECR Schedule A1](https://www.legislation.gov.uk/uksi/2003/2426/schedule/A1) |
| Türkiye | Der KVKK-Cookie-Leitfaden (Juli 2025) ordnet Erstanbieter-Analyse-Cookies dem Kriterium B zu, das keine ausdrückliche Einwilligung braucht, wenn ihr Zweck auf die Messung der Reichweite beschränkt ist, nicht nötige Daten anonymisiert werden (Beispiel: IP-Maskierung), nur anonyme Statistiken entstehen, kein seitenübergreifendes Tracking stattfindet und keine Daten an Dritte gehen (Abschnitt 5.9). Der Leitfaden behandelt Cookies; Yuva nutzt keine und erfüllt dieselben Kriterien. Die Informationspflicht gilt unabhängig von der Rechtsgrundlage und ist spätestens bei der Datenerhebung zu erfüllen (Abschnitt 9). | [KVKK, Çerez Uygulamaları Hakkında Rehber](https://www.kvkk.gov.tr/SharedFolderServer/CMSFiles/fb193dbb-b159-4221-8a7b-3addc083d33f.pdf) |
| Kalifornien | Unternehmen müssen ein Opt-out-Präferenzsignal wie Global Privacy Control als Widerspruch gegen Verkauf und Weitergabe personenbezogener Daten behandeln. Yuva verkauft und teilt nichts von dem, was es zählt, und hört bei Browsern, die das Signal senden, auf zu erheben. | [California Privacy Protection Agency, opt-out preference signals](https://cppa.ca.gov/meetings/materials/20260806_07_02.pdf) |

Welche Zeile für eine Website gilt, hängt von ihren Besuchern ab, nicht davon, wo Yuva läuft. Eine Website mit
Besuchern aus mehreren Ländern folgt der strengsten Regel, die für sie gilt, oder nutzt den Consent-Modus.

### Consent-Modus in Kürze

Installieren Sie das Skript mit `data-consent="required"` und stellen Sie **Collection** der Website auf **Only with
consent**. Es wird nichts gezählt, bis Ihr eigenes Consent-Tool `yuva.consent({ analytics: true })` aufruft.
Die Schritte stehen unter [Consent-Modus](analytics.md#consent-modus). Zustimmende Besucher lassen sich auch über Tage hinweg wiedererkennen; das sind die
[Consent-Funktionen](analytics-consent.md), aus, solange Sie sie nicht für eine Website einschalten.

## Ihre Datenschutzerklärung

Egal, wie die Banner-Frage ausgeht: Sagen Sie Besuchern, was Sie tun. Die DSGVO listet auf, was die Erklärung
enthalten muss, darunter Zwecke, Rechtsgrundlage, berechtigte Interessen, Empfänger und Speicherdauer (Art. 13).
Fügen Sie den Absatz in der Sprache Ihrer Website ein, füllen Sie die Klammern aus und setzen Sie den Link aus
[Opt-out-Links](analytics.md#opt-out-links) dorthin, wo die Klammer ihn verlangt.

**English**

> We measure how our website is used with Yuva Analytics, which we run [on our own server / through (provider)].
> It sets no cookie and stores nothing on your device. Your IP address is shortened before anything else uses
> it and is never stored; we get your approximate location from the shortened address. To count visitors, we
> compute a hash of the shortened address and your browser under a random key that is deleted within hours of
> the end of the day, so we can count you once a day but cannot recognise you the next day or on another
> website. We keep individual page views for [90] days and daily totals for [25] months. We use this only to
> understand and improve our website, and we do not use it for advertising or share it with others. The legal
> basis is our legitimate interest in understanding and improving our website (Art. 6(1)(f) GDPR). You can object
> at any time: [link: Do not count my visits]. We also count nothing for browsers that send Do Not
> Track or Global Privacy Control.

**Türkçe**

> Web sitemizin nasıl kullanıldığını [kendi sunucumuzda / (sağlayıcı) üzerinden] çalıştırdığımız Yuva Analytics
> ile ölçüyoruz. Çerez yerleştirmez ve cihazınızda hiçbir şey saklamaz. IP adresiniz başka bir şey kullanmadan
> önce kısaltılır ve hiçbir zaman saklanmaz; yaklaşık konumunuzu kısaltılmış adresten buluruz. Ziyaretçileri
> saymak için kısaltılmış adresin ve tarayıcınızın, günün bitiminden saatler sonra silinen rastgele bir anahtarla
> özetini (hash) hesaplarız; böylece sizi günde bir kez sayabiliriz, ertesi gün ya da başka bir web sitesinde
> tanıyamayız. Tek tek sayfa görüntülemelerini [90] gün, günlük toplamları [25] ay saklarız. Bunu yalnızca web
> sitemizi anlamak ve iyileştirmek için kullanırız; reklam için kullanmaz, başkalarıyla paylaşmayız. Hukuki
> sebep, web sitemizi anlamak ve iyileştirmek yönündeki meşru menfaatimizdir (6698 sayılı Kanun m. 5/2-f). Dilediğiniz
> zaman itiraz edebilirsiniz: [bağlantı: Ziyaretlerimi sayma]. Do Not Track ya da Global Privacy Control
> gönderen tarayıcıları da saymayız.

**Deutsch**

> Wir messen mit Yuva Analytics, das wir [auf unserem eigenen Server / über (Anbieter)] betreiben, wie unsere
> Website genutzt wird. Es setzt kein Cookie und speichert nichts auf Ihrem Gerät. Ihre IP-Adresse wird gekürzt,
> bevor etwas anderes sie nutzt, und nie gespeichert; Ihren ungefähren Standort ermitteln wir aus der gekürzten
> Adresse. Um Besucher zu zählen, berechnen wir einen Hash aus der gekürzten Adresse und Ihrem Browser mit einem
> zufälligen Schlüssel, der wenige Stunden nach Ende des Tages gelöscht wird; so zählen wir Sie einmal am Tag,
> erkennen Sie aber am nächsten Tag oder auf einer anderen Website nicht wieder. Einzelne Seitenaufrufe
> speichern wir [90] Tage, Tagessummen [25] Monate. Wir nutzen das nur, um unsere Website zu verstehen und zu
> verbessern, nicht für Werbung, und geben es nicht weiter. Rechtsgrundlage ist unser berechtigtes Interesse
> daran, unsere Website zu verstehen und zu verbessern (Art. 6 Abs. 1 lit. f DSGVO). Sie können jederzeit
> widersprechen: [Link: Meine Besuche nicht zählen]. Browser, die Do Not Track oder Global Privacy
> Control senden, zählen wir ebenfalls nicht.

Wenn die Zählung Ihren eigenen Server verlässt, nehmen Sie den Anbieter als Empfänger in die Erklärung auf.

### Anfragen von Besuchern

Im Standardmodus lassen sich die Datensätze keiner Person zuordnen, und die DSGVO verlangt nicht, dafür mehr
Informationen zu sammeln: Können Sie nachweisen, dass Sie die Person nicht identifizieren können, teilen Sie
ihr das nach Möglichkeit mit, und die Rechte der Artikel 15 bis 20 (Auskunft, Berichtigung, Löschung und die
übrigen) gelten nicht, es sei denn, die Person liefert Informationen, mit denen Sie sie identifizieren können
(Art. 11). Das Widerspruchsrecht (Art. 21) erfüllen die Opt-out-Links sowie Do Not Track und Global Privacy
Control. Die Artikel stehen in der [UK GDPR](https://www.legislation.gov.uk/eur/2016/679/article/11); die
EU-DSGVO nummeriert sie gleich.

## Berechtigtes Interesse: eine Abwägung

Stützen Sie sich auf berechtigtes Interesse (Art. 6 Abs. 1 lit. f), halten Sie die drei Schritte fest, die die
ICO beschreibt: [Zweck-, Erforderlichkeits- und Abwägungstest](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/legitimate-interests/what-is-the-legitimate-interests-basis/).
Ein Ausgangspunkt für Yuva Analytics zum Anpassen:

| Schritt | Entwurf |
|---|---|
| Zweck | Wir wollen wissen, welche Seiten gelesen werden, woher Besucher kommen und welche Geräte zu unterstützen sind, um die Website zu verbessern. |
| Erforderlichkeit | Zum Zählen von Aufrufen braucht es eine Besucherzahl; ein Tages-Hash ist das Mindeste, was Besucher von Aufrufen unterscheidet. Kein Cookie, keine gespeicherte Adresse, kein Profil. Weniger Daten würden die Frage nicht beantworten. |
| Abwägung | Besucher erwarten, dass eine Website ihre Besuche zählt. Die Daten dienen nicht der Werbung, werden nicht mit anderen Daten verbunden, nicht über Tage oder Websites hinweg verfolgt und nicht weitergegeben. Die Adresse wird gekürzt und nicht gespeichert, der Salt binnen Stunden gelöscht, detaillierte Datensätze nach 90 Tagen entfernt, und Besucher können jederzeit per Link oder Browser-Signal widersprechen. Bei Websites für Kinder wird nur das Land gespeichert. |
| Ergebnis | Das Interesse wird nicht überwogen. Prüfen Sie das neu, wenn Sie Consent-Funktionen hinzufügen oder detaillierte Datensätze länger aufbewahren. |

## Türkei: Aydınlatma-Text

Der KVKK-Leitfaden verlangt, die Information unabhängig von der Rechtsgrundlage spätestens bei der Datenerhebung
in einem klaren, schlichten und leicht auffindbaren Text zu geben, der nicht in einer langen Datenschutzerklärung
untergeht (Abschnitt 9). Er empfiehlt einen Abwägungstest, wenn man sich auf berechtigtes Interesse stützt (Gesetz
Nr. 6698 Art. 5 Abs. 2 lit. f). Passen Sie den Text an und füllen Sie die Klammern aus:

> **Web Sitesi Ziyaretçileri İçin Aydınlatma Metni**
>
> 6698 sayılı Kişisel Verilerin Korunması Kanunu'nun 10. maddesi uyarınca, veri sorumlusu sıfatıyla [unvan, adres,
> iletişim] olarak sizi bilgilendiriyoruz.
>
> **İşlenen veriler:** Kısaltılmış IP adresi (son bölümü silinir ve saklanmaz), tarayıcı ve cihaz türü, ziyaret
> edilen sayfa, yönlendiren site, yaklaşık konum (ülke, bölge, şehir), dil ve ekran boyutu. Çerez ya da benzeri bir
> teknoloji ile cihazınızda bilgi saklanmaz.
>
> **Amaç:** Web sitemizin kullanımını istatistiksel olarak ölçmek, hangi sayfaların okunduğunu anlamak ve sitemizi
> iyileştirmek.
>
> **Hukuki sebep:** İlgili kişinin temel hak ve özgürlüklerine zarar vermemek kaydıyla, veri sorumlusunun meşru
> menfaatleri için veri işlenmesinin zorunlu olması (Kanun m. 5/2-f).
>
> **Toplama yöntemi:** Web sitemizi ziyaret ettiğinizde otomatik olarak, Yuva Analytics ile.
>
> **Aktarım:** Veriler [kendi sunucumuzda tutulur / hizmet sağlayıcımız (unvan) tarafından bizim adımıza işlenir]; reklam
> amacıyla ya da üçüncü kişilerle paylaşılmaz. [Yurt dışına aktarım varsa: ülke ve Kanun m. 9 dayanağı.]
>
> **Saklama süresi:** Tek tek sayfa görüntülemeleri [90] gün, günlük toplamlar [25] ay.
>
> **Haklarınız:** Kanun'un 11. maddesinde sayılan haklara sahipsiniz. Ziyaretlerinizin sayılmasını
> [bu bağlantıdan: ziyaretlerimi sayma] ya da tarayıcınızda Do Not Track veya Global Privacy Control ayarını açarak
> durdurabilirsiniz. Başvurularınızı [başvuru kanalı] üzerinden iletebilirsiniz.

## Quellen

- [EDPB Guidelines 2/2023 on the technical scope of Art. 5(3) of the ePrivacy Directive, version 2.0](https://www.edpb.europa.eu/system/files/2024-10/edpb_guidelines_202302_technical_scope_art_53_eprivacydirective_v2_en_0.pdf)
- [CNIL, sheet no. 16: use analytics on your websites and applications](https://www.cnil.fr/en/sheet-ndeg16-use-analytics-your-websites-and-applications)
- [PECR Schedule A1, legislation.gov.uk](https://www.legislation.gov.uk/uksi/2003/2426/schedule/A1)
- [KVKK, Çerez Uygulamaları Hakkında Rehber (Juli 2025)](https://www.kvkk.gov.tr/SharedFolderServer/CMSFiles/fb193dbb-b159-4221-8a7b-3addc083d33f.pdf)
- [California Privacy Protection Agency, opt-out preference signals](https://cppa.ca.gov/meetings/materials/20260806_07_02.pdf)
- [ICO, what is the legitimate interests basis](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/legitimate-interests/what-is-the-legitimate-interests-basis/)
- UK-GDPR-Artikel [6](https://www.legislation.gov.uk/eur/2016/679/article/6), [11](https://www.legislation.gov.uk/eur/2016/679/article/11), [13](https://www.legislation.gov.uk/eur/2016/679/article/13) und [21](https://www.legislation.gov.uk/eur/2016/679/article/21)

Für einen gehosteten Server siehe den [Entwurf zur Datenverarbeitung](analytics-dpa.md).
