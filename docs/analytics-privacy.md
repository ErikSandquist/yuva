# Analytics and privacy

This page says what Yuva Analytics keeps about your visitors and for how long, whether your website needs a
cookie banner, and gives you text for your privacy notice. It is information with sources, not legal advice;
laws differ by country and change, so check your own situation with a lawyer or your data protection
authority.

## What it keeps

| Data | What happens to it |
|---|---|
| Cookies, local storage, device or advertising ids | None are set. The only thing the script ever stores on a device is a visitor's own opt-out (`yuva_optout`), and only when they ask for it. |
| IP address | Shortened to `/24` (IPv4) or `/48` (IPv6) in the first step, before anything else uses it. The location and the visitor hash come from the shortened address only. No address is stored, full or shortened. |
| Browser (user agent) | Read to find the browser, system and device type, and mixed into the visitor hash. The text itself is not stored. |
| Visitor | A keyed hash (HMAC-SHA-256) of the website, the shortened address and the browser, under a random salt that exists for one day and one website. The salt is deleted 2 hours after that day ends (`YUVA_ANALYTICS_SALT_GRACE`, up to 6 hours), after which nobody can compute the hash again. So a visitor is the same only within one day, on one website. |
| What the visitor did | Page host and path, referrer host and path, the `utm_*` parameters, country, region and city, browser, system, device type, screen size class, language. The rest of the query string is dropped, and anything shaped like an e-mail address is removed. |
| Children's websites | With **Audience: Children** only the country is kept: no region or city, no screen size, no language. |
| Do Not Track, Global Privacy Control | A browser that sends either sends nothing, and the server drops any event that carries one. This cannot be switched off. |

Yuva treats the detailed records as pseudonymous personal data until they are deleted, because while a
day's salt exists the hash can tell one device from another within that day. The daily totals count visits
and cannot be traced to a person.

## How long

| Data | Kept |
|---|---|
| Daily salt | Until 2 hours after the website's day ends |
| Detailed records (visits and events) | 90 days by default; the operator sets 7 to 400 with [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analytics) |
| Daily totals | 25 months by default; `retention_months` on the website changes it |

You can delete days or everything at any time ([Keys and data](analytics.md#keys-and-data)).

## Do you need a cookie banner?

A banner is about consent to storing or reading something on the visitor's device. Yuva Analytics stores
nothing there, but a script that makes the browser send data counts as access in the EU's reading, so the
answer depends on where your visitors are. Short answer:

- **France, the United Kingdom and Türkiye:** the default mode fits the published exemptions. You need an
  entry in your privacy notice and a way to object, not a consent banner.
- **Elsewhere in the EU:** unless your country has an exemption like France's, the safe choice is
  [consent mode](analytics.md#consent-mode): nothing is sent until your consent tool says the visitor agreed.
- **Everywhere else:** check the local rule. This page does not cover it.

### By place

| Place | What the rule says | Where it is published |
|---|---|---|
| EU (ePrivacy Art. 5(3)) | The EDPB reads Art. 5(3) as applying when an entity distributes software or JavaScript that makes the browser send information: "Such access clearly falls within the scope of Article 5(3)" (paragraph 33). Collecting an IP address falls under it too, unless the entity can ensure the address does not come from the user's device (paragraph 55). It also says that falling under the article does not always mean consent is needed: each case is assessed for an exemption (paragraph 56). | [EDPB Guidelines 2/2023, version 2.0, adopted 7 October 2024](https://www.edpb.europa.eu/system/files/2024-10/edpb_guidelines_202302_technical_scope_art_53_eprivacydirective_v2_en_0.pdf) |
| France | The CNIL exempts audience measurement from consent when you inform users, let them object, use the data for audience measurement only, do not cross-check it with other processing, limit the scope to one publisher's site or app, cut the last byte of the IP address and limit trackers to 13 months. | [CNIL sheet no. 16](https://www.cnil.fr/en/sheet-ndeg16-use-analytics-your-websites-and-applications) |
| United Kingdom | Schedule A1 to the Privacy and Electronic Communications Regulations (PECR), inserted by the Data (Use and Access) Act 2025 and in force since 5 February 2026, lets a service store or access information on a device without consent for statistics on how the service is used, with a view to improving it, when the information is not shared (except to help another person improve the service), the user gets clear and comprehensive information and a simple, free way to object (paragraph 5). | [PECR Schedule A1](https://www.legislation.gov.uk/uksi/2003/2426/schedule/A1) |
| Türkiye | The KVKK cookie guide (July 2025) puts first-party analytics cookies under Criterion B, which needs no explicit consent, when their purpose is limited to measuring the audience, data that does not have to be processed is anonymised (masking the IP is the example), only anonymous statistics are produced, there is no tracking across sites and data is not passed to third parties (section 5.9). The guide is about cookies; Yuva uses none and meets the same criteria. The information duty applies whatever the legal basis, and at the latest when the data is collected (section 9). | [KVKK, Çerez Uygulamaları Hakkında Rehber](https://www.kvkk.gov.tr/SharedFolderServer/CMSFiles/fb193dbb-b159-4221-8a7b-3addc083d33f.pdf) |
| California | Businesses must treat an opt-out preference signal such as Global Privacy Control as an opt-out of the sale and sharing of personal information. Yuva neither sells nor shares what it counts, and stops collecting for browsers that send the signal. | [California Privacy Protection Agency, opt-out preference signals](https://cppa.ca.gov/meetings/materials/20260806_07_02.pdf) |

Which row applies to a website is a question about its visitors, not about where Yuva runs. A site with
visitors in several countries follows the strictest rule that applies to them, or uses consent mode.

### Consent mode in short

Install the script with `data-consent="required"` and set the website's **Collection** to **Only with
consent**. Nothing is counted until your own consent tool calls `yuva.consent({ analytics: true })`. The steps
are in [Consent mode](analytics.md#consent-mode).

## Your privacy notice

Whatever the banner answer, tell visitors what you do. The GDPR lists what the notice must contain, among it
the purposes, the legal basis, the legitimate interests, recipients and the retention period (Art. 13). Paste the
paragraph in the language of your site, fill in the brackets and put the opt-out link from
[Opt-out links](analytics.md#opt-out-links) where the brackets ask for it.

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

Where the counts leave your own server, add the provider to the notice as a recipient.

### Requests from visitors

In the default mode your records cannot be tied to a person, and the GDPR does not oblige you to collect more
information to do so: if you can show that you cannot identify the person, you tell them so where possible,
and the rights in Articles 15 to 20 (access, rectification, erasure and the others) do not apply unless they give you
information that lets you identify them (Art. 11). The right to object (Art. 21) is served by the opt-out
links and by Do Not Track and Global Privacy Control. The Articles are in the
[UK GDPR](https://www.legislation.gov.uk/eur/2016/679/article/11); the EU GDPR numbers them the same.

## Legitimate interest: a balancing test

If you rely on legitimate interest (Art. 6(1)(f)), write down the three steps the ICO describes: the
[purpose, necessity and balancing tests](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/legitimate-interests/what-is-the-legitimate-interests-basis/).
A starting point for Yuva Analytics, to adapt:

| Step | Outline |
|---|---|
| Purpose | We want to know which pages are read, where visitors come from and which devices to support, to improve the website. |
| Necessity | Counting views needs a visitor count; a daily hash is the least that tells visitors from views. No cookie, no stored address, no profile. Less data would not answer the question. |
| Balancing | Visitors expect a website to count its visits. The data is not used for advertising, not combined with other data, not followed across days or websites and not shared. The address is shortened and not stored, the salt is deleted within hours, detailed records go after 90 days, and visitors can object at any time with a link or a browser signal. For children's websites only the country is kept. |
| Result | The interest is not overridden. Review this when you add consent features or keep detailed records longer. |

## Türkiye: aydınlatma metni

The KVKK guide asks for the information to be given at the latest when the data is collected, whatever the
legal basis, in a clear and plain text that is easy to find, not buried in a long privacy notice (section 9). It
recommends a balancing test when you rely on legitimate interest (Law 6698 article 5(2)(f)). Adapt this text
and fill in the brackets:

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

## Sources

- [EDPB Guidelines 2/2023 on the technical scope of Art. 5(3) of the ePrivacy Directive, version 2.0](https://www.edpb.europa.eu/system/files/2024-10/edpb_guidelines_202302_technical_scope_art_53_eprivacydirective_v2_en_0.pdf)
- [CNIL, sheet no. 16: use analytics on your websites and applications](https://www.cnil.fr/en/sheet-ndeg16-use-analytics-your-websites-and-applications)
- [PECR Schedule A1, legislation.gov.uk](https://www.legislation.gov.uk/uksi/2003/2426/schedule/A1)
- [KVKK, Çerez Uygulamaları Hakkında Rehber (July 2025)](https://www.kvkk.gov.tr/SharedFolderServer/CMSFiles/fb193dbb-b159-4221-8a7b-3addc083d33f.pdf)
- [California Privacy Protection Agency, opt-out preference signals](https://cppa.ca.gov/meetings/materials/20260806_07_02.pdf)
- [ICO, what is the legitimate interests basis](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/legitimate-interests/what-is-the-legitimate-interests-basis/)
- UK GDPR articles [6](https://www.legislation.gov.uk/eur/2016/679/article/6), [11](https://www.legislation.gov.uk/eur/2016/679/article/11), [13](https://www.legislation.gov.uk/eur/2016/679/article/13) and [21](https://www.legislation.gov.uk/eur/2016/679/article/21)

For a hosted server, see the [data processing outline](analytics-dpa.md).
