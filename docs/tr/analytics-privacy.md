# Analitik ve gizlilik

Bu sayfa Yuva Analytics'in ziyaretçileriniz hakkında neleri ne kadar süre sakladığını, web sitenizin çerez
bandına ihtiyacı olup olmadığını anlatır ve gizlilik bildiriminiz için metinler verir. Kaynaklarıyla birlikte bir
bilgidir, hukuki danışmanlık değildir; yasalar ülkeye göre farklıdır ve zamanla değişir, kendi durumunuzu bir avukatla ya
da veri koruma otoritenizle kontrol edin.

## Ne sakladığı

| Veri | Ne olur |
|---|---|
| Çerezler, yerel depolama, cihaz ya da reklam kimlikleri | Hiçbiri yerleştirilmez. Betiğin bir cihazda sakladığı tek şey ziyaretçinin kendi vazgeçme tercihidir (`yuva_optout`) ve yalnızca ziyaretçi isterse. |
| IP adresi | İlk adımda, başka hiçbir şey kullanmadan önce `/24` (IPv4) ya da `/48` (IPv6) olarak kısaltılır. Konum ve ziyaretçi özeti yalnızca kısaltılmış adresten çıkar. Hiçbir adres, tam ya da kısaltılmış, saklanmaz. |
| Tarayıcı (user agent) | Tarayıcıyı, sistemi ve cihaz türünü bulmak için okunur ve ziyaretçi özetine katılır. Metnin kendisi saklanmaz. |
| Ziyaretçi | Web sitesinin, kısaltılmış adresin ve tarayıcının, bir gün ve bir web sitesi için var olan rastgele bir tuz (salt) altında anahtarlı özeti (HMAC-SHA-256). Tuz, o günün bitiminden 2 saat sonra silinir (`YUVA_ANALYTICS_SALT_GRACE`, en çok 6 saat); ardından özeti kimse yeniden hesaplayamaz. Yani bir ziyaretçi yalnızca bir gün içinde ve tek web sitesinde aynıdır. |
| Ziyaretçinin yaptığı | Sayfanın host'u ve yolu, yönlendirenin host'u ve yolu, `utm_*` parametreleri, ülke, bölge ve şehir, tarayıcı, sistem, cihaz türü, ekran boyutu sınıfı, dil. Sorgu dizesinin kalanı atılır, e-posta adresi biçimindeki her şey çıkarılır. |
| Çocuklara yönelik web siteleri | **Kitle: Çocuklar** seçildiğinde yalnızca ülke tutulur: bölge ve şehir, ekran boyutu ve dil yoktur. |
| Do Not Track, Global Privacy Control | İkisinden birini gönderen tarayıcı hiçbir şey göndermez; sunucu da birini taşıyan her olayı atar. Kapatılamaz. |

Yuva, ayrıntılı kayıtlar silinene kadar bunları takma adlı kişisel veri sayar; çünkü bir günün tuzu var olduğu
sürece özet o gün içinde bir cihazı ötekinden ayırabilir. Günlük toplamlar ziyaretleri sayar ve bir kişiye
bağlanamaz.

## Ne kadar süre

| Veri | Saklama |
|---|---|
| Günlük tuz | Web sitesinin gününün bitiminden 2 saat sonrasına kadar |
| Ayrıntılı kayıtlar (ziyaretler ve olaylar) | Varsayılan 90 gün; operatör [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analitik) ile 7 ila 400 arası ayarlar |
| Günlük toplamlar | Varsayılan 25 ay; web sitesindeki `retention_months` değiştirir |

Günleri ya da her şeyi istediğiniz zaman silebilirsiniz ([Anahtarlar ve veriler](analytics.md#anahtarlar-ve-veriler)).

## Çerez bandına ihtiyacınız var mı?

Bant, ziyaretçinin cihazında bir şey saklamak ya da okumak için rıza ile ilgilidir. Yuva Analytics orada hiçbir
şey saklamaz, ama AB'nin okumasında tarayıcıya veri gönderten bir betik erişim sayılır; bu yüzden cevap
ziyaretçilerinizin nerede olduğuna bağlıdır. Kısa cevap:

- **Fransa, Birleşik Krallık ve Türkiye:** varsayılan mod yayımlanmış istisnalara uyar. Gizlilik bildiriminizde
  bir madde ve itiraz yolu gerekir, rıza bandı gerekmez.
- **AB'nin geri kalanı:** ülkenizde Fransa'dakine benzer bir istisna yoksa güvenli seçim
  [izin modudur](analytics.md): izin aracınız ziyaretçinin kabul ettiğini söyleyene kadar hiçbir şey
  gönderilmez (**İzin modu** bölümü).
- **Başka yerler:** yerel kuralı kontrol edin. Bu sayfa onu kapsamaz.

### Yere göre

| Yer | Kural ne diyor | Nerede yayımlandı |
|---|---|---|
| AB (ePrivacy m. 5(3)) | EDPB, bir kuruluş tarayıcıya bilgi gönderten yazılım ya da JavaScript dağıttığında m. 5(3)'ün uygulandığını okur: "Such access clearly falls within the scope of Article 5(3)" (paragraf 33). IP adresi toplamak da, kuruluş adresin kullanıcının cihazından gelmediğini sağlayamıyorsa kapsama girer (paragraf 55). Maddenin kapsamına girmenin her zaman rıza gerektirmediğini de söyler: her olayda bir istisna olup olmadığı değerlendirilir (paragraf 56). | [EDPB Guidelines 2/2023, sürüm 2.0, 7 Ekim 2024'te kabul edildi](https://www.edpb.europa.eu/system/files/2024-10/edpb_guidelines_202302_technical_scope_art_53_eprivacydirective_v2_en_0.pdf) |
| Fransa | CNIL, kitle ölçümünü şu koşullarla rızadan muaf tutar: kullanıcıları bilgilendirmek, itiraz etmelerine izin vermek, veriyi yalnızca kitle ölçümü için kullanmak, başka işlemlerle çapraz kontrol etmemek, kapsamı tek bir yayıncının sitesi ya da uygulamasıyla sınırlamak, IP adresinin son baytını kesmek ve izleyicileri 13 ay ile sınırlamak. | [CNIL sheet no. 16](https://www.cnil.fr/en/sheet-ndeg16-use-analytics-your-websites-and-applications) |
| Birleşik Krallık | Privacy and Electronic Communications Regulations'a (PECR) Data (Use and Access) Act 2025 ile eklenen ve 5 Şubat 2026'dan beri yürürlükte olan Schedule A1, bir hizmetin, hizmetin nasıl kullanıldığına dair istatistik için, iyileştirmek amacıyla, bilgi paylaşılmadığında (başka birinin hizmeti iyileştirmesine yardım dışında), kullanıcıya açık ve kapsamlı bilgi ile basit, ücretsiz bir itiraz yolu verildiğinde cihazda rızasız bilgi saklamasına ya da okumasına izin verir (paragraf 5). | [PECR Schedule A1](https://www.legislation.gov.uk/uksi/2003/2426/schedule/A1) |
| Türkiye | KVKK Çerez Uygulamaları Hakkında Rehber (Temmuz 2025), birinci taraf analitik çerezleri; amacı kitle ölçümüyle sınırlıysa, işlenmesi zorunlu olmayan veri anonimleştirilmişse (örnek: IP maskeleme), yalnızca anonim istatistik üretiliyorsa, siteler arası takip yoksa ve veri üçüncü taraflara aktarılmıyorsa açık rıza gerektirmeyen Kriter B kapsamında değerlendirir (bölüm 5.9). Rehber çerezleri ele alır; Yuva çerez kullanmaz ve aynı ölçütleri karşılar. Aydınlatma yükümlülüğü hukuki sebepten bağımsız olarak uygulanır ve en geç veri elde edilirken yerine getirilmelidir (bölüm 9). | [KVKK, Çerez Uygulamaları Hakkında Rehber](https://www.kvkk.gov.tr/SharedFolderServer/CMSFiles/fb193dbb-b159-4221-8a7b-3addc083d33f.pdf) |
| Kaliforniya | İşletmeler, Global Privacy Control gibi bir vazgeçme tercih sinyalini kişisel bilgilerin satışından ve paylaşımından vazgeçme olarak ele almak zorundadır. Yuva saydığı şeyi ne satar ne paylaşır, sinyali gönderen tarayıcılar için de toplamayı durdurur. | [California Privacy Protection Agency, opt-out preference signals](https://cppa.ca.gov/meetings/materials/20260806_07_02.pdf) |

Bir web sitesine hangi satırın uygulandığı, Yuva'nın nerede çalıştığına değil ziyaretçilerine bağlıdır. Birkaç
ülkeden ziyaretçisi olan bir site, onlara uygulanan en katı kurala uyar ya da izin modunu kullanır.

### İzin modu kısaca

Betiği `data-consent="required"` ile kurun ve web sitesinin **Toplama** ayarını **Yalnızca izinle** yapın.
Kendi izin aracınız `yuva.consent({ analytics: true })` çağırana kadar hiçbir şey sayılmaz. Adımlar
[Analitik](analytics.md) sayfasının **İzin modu** bölümündedir. Kabul eden ziyaretçiler günler boyunca da tanınabilir;
bu [İzinle açılan özellikler](analytics-consent.md) sayfasındadır ve bir web sitesi için açmadıkça kapalıdır.

## Gizlilik bildiriminiz

Bant sorusunun cevabı ne olursa olsun, ziyaretçilere ne yaptığınızı söyleyin. GDPR bildirimde nelerin olması
gerektiğini sayar; amaçlar, hukuki sebep, meşru menfaatler, alıcılar ve saklama süresi bunların arasındadır
(m. 13). Paragrafı sitenizin dilinde yapıştırın, köşeli parantezleri doldurun ve parantezin istediği yere
[Analitik](analytics.md) sayfasındaki **Vazgeçme bağlantıları** bölümünün bağlantısını koyun.

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

Sayımlar kendi sunucunuzdan çıkıyorsa, sağlayıcıyı bildirime alıcı olarak ekleyin.

### Ziyaretçilerin talepleri

Varsayılan modda kayıtlar bir kişiye bağlanamaz ve GDPR bunun için daha fazla bilgi toplamanızı gerektirmez:
kişiyi belirleyemeyeceğinizi gösterebiliyorsanız, mümkünse bunu kişiye bildirirsiniz; kişi kimliğini belirlemenizi
sağlayacak bilgi vermedikçe 15 ila 20. maddelerdeki haklar (erişim, düzeltme, silme ve diğerleri) uygulanmaz
(m. 11). İtiraz hakkı (m. 21), vazgeçme bağlantıları ve Do Not Track ile Global Privacy Control ile karşılanır.
Maddeler [UK GDPR](https://www.legislation.gov.uk/eur/2016/679/article/11) metnindedir; AB GDPR'si aynı
numaralandırmayı kullanır.

## Meşru menfaat: bir denge testi

Meşru menfaate (m. 6(1)(f)) dayanıyorsanız, ICO'nun anlattığı üç adımı yazıya dökün:
[amaç, gereklilik ve denge testleri](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/legitimate-interests/what-is-the-legitimate-interests-basis/).
Yuva Analytics için uyarlayabileceğiniz bir başlangıç:

| Adım | Taslak |
|---|---|
| Amaç | Web sitesini iyileştirmek için hangi sayfaların okunduğunu, ziyaretçilerin nereden geldiğini ve hangi cihazların desteklenmesi gerektiğini bilmek istiyoruz. |
| Gereklilik | Görüntülemeleri saymak için ziyaretçi sayısı gerekir; günlük bir özet, ziyaretçiyi görüntülemeden ayıran en azıdır. Çerez, saklanan adres ya da profil yoktur. Daha az veri soruyu yanıtlamaz. |
| Denge | Ziyaretçiler bir web sitesinin ziyaretlerini saymasını bekler. Veri reklam için kullanılmaz, başka verilerle birleştirilmez, günler ya da web siteleri arasında izlenmez ve paylaşılmaz. Adres kısaltılır ve saklanmaz, tuz saatler içinde silinir, ayrıntılı kayıtlar 90 gün sonra gider ve ziyaretçiler bir bağlantı ya da tarayıcı sinyaliyle her an itiraz edebilir. Çocuklara yönelik web sitelerinde yalnızca ülke tutulur. |
| Sonuç | Menfaat aşılmıyor. İzin özellikleri eklediğinizde ya da ayrıntılı kayıtları daha uzun tuttuğunuzda yeniden gözden geçirin. |

## Türkiye: aydınlatma metni

KVKK rehberi, bilginin hukuki sebepten bağımsız olarak en geç veri elde edilirken, açık, sade ve kolay erişilebilir
bir metinle verilmesini ister; uzun bir gizlilik bildiriminin içinde kaybolmamalıdır (bölüm 9). Meşru menfaate
dayanıldığında bir denge testi yapılmasını önerir (6698 sayılı Kanun m. 5/2-f). Metni uyarlayın ve köşeli
parantezleri doldurun:

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

## Kaynaklar

- [EDPB Guidelines 2/2023 on the technical scope of Art. 5(3) of the ePrivacy Directive, version 2.0](https://www.edpb.europa.eu/system/files/2024-10/edpb_guidelines_202302_technical_scope_art_53_eprivacydirective_v2_en_0.pdf)
- [CNIL, sheet no. 16: use analytics on your websites and applications](https://www.cnil.fr/en/sheet-ndeg16-use-analytics-your-websites-and-applications)
- [PECR Schedule A1, legislation.gov.uk](https://www.legislation.gov.uk/uksi/2003/2426/schedule/A1)
- [KVKK, Çerez Uygulamaları Hakkında Rehber (Temmuz 2025)](https://www.kvkk.gov.tr/SharedFolderServer/CMSFiles/fb193dbb-b159-4221-8a7b-3addc083d33f.pdf)
- [California Privacy Protection Agency, opt-out preference signals](https://cppa.ca.gov/meetings/materials/20260806_07_02.pdf)
- [ICO, what is the legitimate interests basis](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/legitimate-interests/what-is-the-legitimate-interests-basis/)
- UK GDPR maddeleri [6](https://www.legislation.gov.uk/eur/2016/679/article/6), [11](https://www.legislation.gov.uk/eur/2016/679/article/11), [13](https://www.legislation.gov.uk/eur/2016/679/article/13) ve [21](https://www.legislation.gov.uk/eur/2016/679/article/21)

Barındırılan bir sunucu için [veri işleme taslağına](analytics-dpa.md) bakın.
