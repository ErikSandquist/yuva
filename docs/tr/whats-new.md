# Yenilikler

Yenilikler değişiklik günlüğü, müşterilerinize nelerin değiştiğini anlatır: Yeni, İyileştirildi veya
Düzeltildi etiketli kısa, tarihli kayıtlar. Bunlar herkese açık bir sayfada, bir Atom akışında,
okunmamış sayısıyla [web widget'ında](widget.md) ve siz isterseniz abonelerinize e-postayla görünür. Bir
gelen kutusuna aittir; bu yüzden müşteri bildirdiği hatanın düzeldiğini, soracağı yerde görür.

Her gelen kutusunun en fazla bir değişiklik günlüğü olur. Herkese açık sayfalar, zorunlu JavaScript olmadan,
Yuva sunucunuz tarafından düz HTML olarak üretilir. Burada hiçbir şey kendiliğinden e-posta göndermez: bir
kaydı e-postayla göndermek, her zamanki gibi gözden geçirip gönderdiğiniz bir [bülten](broadcasts.md)
taslağı oluşturur.

## 1. Değişiklik günlüğü oluşturun

Panelde: kenar çubuğunda Yenilikler (kısayol `g w`), sonra gelen kutusunda Değişiklik günlüğü oluştur.
Değişiklik günlüklerini sahipler ve yöneticiler oluşturur ve değiştirir; kayıtları kimin yayınlayabileceği
için bkz. [Taslaklar ve yayınlama](#taslaklar-ve-yayınlama).

| Ayar (API alanı) | Anlamı |
|---|---|
| Ad (`name`) | En fazla 100 karakter. |
| Açıklama (`description`) | En fazla 1.000 karakter, ana sayfada adın altında gösterilir. |
| Adres (`slug`) | 3 ile 48 karakter: `a-z`, `0-9` ve araya giren tireler. Sunucuda benzersiz; bazı sözcükler ayrılmıştır. |
| Diller (`locales`), ana dil (`default_locale`) | Kayıtların yazıldığı ve yayınlandığı en fazla 20 [dil](widget.md#diller); ana dil bunlardan biridir. |
| Analitik (`analytics_property_id`) | Değişiklik günlüğüyle birlikte saklanır. |
| Temsilciler yayınlayabilir (`agents_may_publish`) | Varsayılan olarak kapalı. Bkz. [Taslaklar ve yayınlama](#taslaklar-ve-yayınlama). |
| Yayında (`published`) | Varsayılan olarak kapalı. Yayınlanmamış bir değişiklik günlüğü ve tüm kayıtları ekibiniz dışında herkese `404` verir; widget'ta Yenilikler görünmez. |

Sayfaların logosu ve vurgu rengi gelen kutusunun markasıdır. Adresi değiştirirseniz eskisi 90 gün boyunca
değişiklik günlüğüne ayrılmış kalır ve altındaki herkese açık adresler yenisine yönlendirir.

API ile: `inbox_id`, `name` ve `slug` ile `POST /v1/changelogs`, sonra `{"published": true}` ile
`PATCH /v1/changelogs/{changelogId}`. `GET /v1/changelogs/{changelogId}/preview`, değişiklik günlüğü
yayında olmasa bile bir sayfayı ziyaretçilerin göreceği gibi döndürür; panel bunu taslaklar dahil Önizleme
ile gösterir (düzenleyicide kısayol `v`). `DELETE`, değişiklik günlüğünü kayıtları ve çevirileriyle siler;
kayıtlarından yapılmış bültenler kalır.

## 2. Kayıt yazın ve çevirin

Bir kaydın bir ila üç etiketi, isteğe bağlı bir görseli ve dil başına bir *çevirisi* vardır. Yenilikler'i
açın, Yeni kayıt'ı seçin, etiketleri belirleyin ve ana dilde bir başlık ile Markdown metin yazın.

| Alan | Sınır |
|---|---|
| Etiketler (`labels`) | `new`, `improved` ve `fixed` değerlerinden bir ila üçü. |
| Başlık (`title`) | 3 ile 200 karakter. |
| Metin (`body`) | Markdown, en fazla 50.000 karakter. Başlıklar, listeler, tablolar, kod, bağlantılar ve görseller; ham HTML kaldırılır. |
| Adres (`slug`) | Bir çevirinin web adresinin parçası; başlıktan üretilir, düzenlenebilir. URL'deki yanlış bir slug doğrusuna yönlendirir. |
| Görsel (`image_id`) | Çalışma alanınızın [e-posta görsellerinden](designs.md#4-görseller) biri. |

Metindeki görseller yalnızca çalışma alanınıza yüklediyseniz kalır: düzenleyiciden yükleyin veya
`POST /v1/changelogs/{changelogId}/images` kullanın (e-posta görselleriyle aynı türler, boyutlar ve yeniden
kodlama). Hiçbir kaydın, taslağın veya tasarımın kullanmadığı görsel, yüklemeden bir hafta sonra silinir.

Çevirmek için düzenleyicide bir dil seçin: ana dildeki metin çevirinin yanında kalır, eksik veya güncelliğini
yitirmiş çeviriler işaretlenir. Yuva sizin yerinize çeviri yapmaz. Yayındaki bir kayıt ana dildeki çevirisini
her zaman korur.

Bir çalışma alanının en fazla `YUVA_CHANGELOG_MAX_ENTRIES` kaydı olabilir
([Yapılandırma](configuration.md#yenilikler)); bunun ötesinde kayıt oluşturmak `422 hosted_limit` verir.

API ile, `/v1/changelogs/{changelogId}` altında: `POST /entries` ilk çevirisiyle bir kayıt yazar;
`GET /entries` `state` (`draft`, `scheduled`, `published` veya `changed`), `label` ve `locale` ile süzer;
`PATCH /entries/{entryId}` etiketleri ve görseli değiştirir; `/entries/{entryId}/translations/{locale}`
üzerindeki `PUT` ve `DELETE` bir çeviriyi yazar ve kaldırır.

## Taslaklar ve yayınlama

Bir çevirinin çalışma taslağı vardır ve yayınlandığında donmuş bir yayın kopyası olur. Kaydetmek yalnızca
taslağı değiştirir; herkese açık sayfa siz yayınlayınca değişir.

- **Kaydetme.** Her kayıt, düzenleyicinin yüklediği `base_version` değerini taşır. Arada başkası
  kaydettiyse kayıt `409 version_conflict` ile reddedilir ve yanıt çeviriyi şu anki haliyle içerir; böylece
  kimse başkasının çalışmasının üzerine yazmaz. Açık olan diğer düzenleyicilere kaydın değiştiği bildirilir.
- **Yayınlama.** Yayınla, taslağı olan her çeviriyi dondurur, işler ve kaydı web'e koyar
  (`POST .../entries/{entryId}/publish`). Ana dilde metin gerekir (`409 no_default_translation`). İlk
  yayın gösterilen tarihi belirler; yeniden yayınlamak onu korur. Sonradan eklenen veya değiştirilen bir
  çeviri, istekte `locale` ile tek başına yayınlanır. Yayından kaldır (`POST .../unpublish`) kaydı web'den
  ve widget'tan alır, tarihini koruyan bir taslağa döndürür.
- **Kim yayınlayabilir.** Gelen kutusuna erişimi olan temsilciler kayıt ve çeviri yazar. Sahipler ve
  yöneticiler ayrıca yayınlar, zamanlar, yayından kaldırır, yayındaki bir kaydın etiketlerini veya görselini
  değiştirir, yayındaki kayıtları siler ve değişiklik günlüğünün ayarlarını değiştirir. Gelen kutusuna
  erişimi olan temsilcilerin de yayınlayabilmesi için Temsilciler yayınlayabilir'i (`agents_may_publish`)
  açın. Yetkisi olmayan temsilciler Taslağı gönder'i kullanır ve bir sahip veya yönetici yayınlar.
- **Anahtarlar.** Bir API anahtarı yalnızca `changelog:publish` yetkisiyle yayınlar; örneğin kaydını
  gönderen bir sürüm hattı. Onsuz anahtar taslak yazar.
- **Asistanlar asla yayınlamaz.** OAuth ve [MCP](mcp.md#yenilikleri-bir-asistanla-kullanın) token'ları
  taslak yazar; ancak ayarlar ne olursa olsun bir değişiklik günlüğü oluşturamaz veya değiştiremez,
  yayınlayamaz, zamanlayamaz, yayından kaldıramaz ya da yayındaki bir kaydı silemez
  (`403 member_session_required`).

## Zamanlama

Bir kaydı, seçtiğiniz bir zamanda, saat diliminizde ve en fazla 365 gün sonrasına yayına girecek şekilde
zamanlayın (`publish_at` ile `POST .../entries/{entryId}/schedule`). Bir değişiklik günlüğünde en fazla 50
zamanlanmış kayıt olur (`409 too_many_scheduled`). Yeniden zamanlamak saati taşır; Zamanlamayı kaldır
(`POST .../unschedule`) kaydı taslağa döndürür.

Sunucu kaydı o saatte, onu zamanlayan kişi veya anahtar yapacakmış gibi kendisi yayınlar. Zamanlanmış bir kayıt
olduğu gibi yayına girer; bu yüzden onu yalnızca yayınlama yetkisi olanlar değiştirebilir, çevirilerini
kaydedebilir veya silebilir (`403 entry_scheduled`).

Zaman geldiğinde yetki yoksa hiçbir şey yayına girmez: kayıt, panelin gösterdiği `schedule_failed` ile
taslağa döner.

| `schedule_failed.reason` | Anlamı |
|---|---|
| `lost_right` | Kaydı zamanlayan üye artık yayınlayamıyor veya yok. |
| `key_revoked` | Kaydı zamanlayan API anahtarı iptal edildi. |
| `key_scope_lost` | Anahtar artık `changelog:publish` yetkisine sahip değil veya gelen kutusunu göremiyor. |
| `no_default_translation` | Ana dildeki metin kaldırıldı. |

Nedeni giderin ve yeniden zamanlayın; bir sonraki zamanlama veya yayın hatayı temizler.

## 3. Herkese açık sayfalar ve akış

Yayındaki bir değişiklik günlüğü `<Yuva adresiniz>/changelog/{slug}` adresindedir ve ana diline yönlendirir.
Tarayıcı diline göre yönlendirme yoktur.

| Adres | Gösterir |
|---|---|
| `/changelog/{slug}/{locale}/` | Kayıtlar yeniden eskiye, sayfada 20; `?label=` ile `new`, `improved` veya `fixed` süzer, `?before=` sayfalar. |
| `/changelog/{slug}/{locale}/entries/{number}-{slug}` | Tek bir kayıt. Yanlış slug doğrusuna yönlendirir. |
| `/changelog/{slug}/{locale}/feed.atom` | O dildeki en yeni 50 kayıt, her kaydın metni ve etiketleriyle bir Atom akışı olarak. |
| `/changelog/{slug}/sitemap.xml` | Yayındaki sayfalar. |

Bir sayfa yalnızca adresindeki dilde yayınlanmış çevirisi olan kayıtları listeler. Taslak, zamanlanmış ve
yayından kaldırılmış kayıtlar, bilinmeyen diller ve yayınlanmamış değişiklik günlükleri düz bir `404` verir.
Sayfalar kanonik ve `hreflang` bağlantıları, kaydın görselini içeren Open Graph etiketleri ve arama
motorları için yapılandırılmış veri taşır. Bir dakika önbelleğe alınır ve adres başına dakikada 120 istekle
sınırlıdır.

## 4. Widget içinde

Gelen kutusunun bir kaydı olan yayınlanmış bir değişiklik günlüğü olduğunda [Ana ekran](widget.md),
ziyaretçinin açmadığı kayıtların sayısını "9+" a kadar gösteren bir Yenilikler kutucuğu gösterir. Uygulama
panelin içinde açılır: kayıtlar yeniden eskiye etiketleri, tarihi, görseli ve metniyle, bir de herkese açık
sayfaya bağlantıyla. Değişiklik günlüğünde varsa ziyaretçinin dilini, yoksa ana dili kullanır.

- **Okunmamış.** Ziyaretçinin açtığı en yeni tarih yalnızca tarayıcısında, kanal başına tutulur; Yuva bu
  konuda hiçbir şey saklamaz ve anonim ziyaretçiler de bunu alır. İşaret yoksa 30 günden eski kayıtlar
  okunmuş sayılır. Uygulamayı açmak gösterilen her şeyi okunmuş yapar.
- **Başlatıcı rozeti.** Sohbet kanalında Güncellemeleri duyur'u (`announce_updates`) açın; başlatıcı,
  ziyaretçinin okunmamış mesajı yokken okunmamış kayıt sayısını gösterir. Mesajlar rozeti her zaman alır.
  Varsayılan olarak kapalıdır; Ana ekran kutucuğu sayıyı her durumda gösterir.
- **Canlı.** Bir yayın açık widget'lara anında ulaşır.

Kodda: `openApp("whats-new")` uygulamayı, `openApp("whats-new", { entry: "<id>" })` tek bir kaydı açar. Kendi
ekranınızı kurmak için istemcide `whatsNew({ locale, cursor, limit })` ve `whatsNewEntry(id)` vardır; React
sitelerinde `useyuva/react` içinde `useWhatsNew({ locale })` bulunur; bkz. [SDK README](../../sdk/js/README.md).
Bunların arkasındaki uç noktalar `GET /client/v1/channels/{channel_key}/whats-new` ve
`.../whats-new/{entryId}`: oturum yok, kişi oluşturulmaz, adres başına dakikada 120 okuma, bir dakika
önbelleğe alınabilir. Yayınlanmış değişiklik günlüğü yoksa `404 whats_new_off` verirler.

## 5. Bir kaydı abonelere e-postayla gönderin

Yayındaki bir kayıtta sahipler ve yöneticiler Bunu abonelere e-postayla gönder'i seçer: gelen kutusunun en
fazla 10 [listesini](lists.md), gönderim kanalını ve dili seçin. Yuva, `product-update`
[tasarımından](designs.md) bir bülten taslağı oluşturur: başlık, etiketler ve tarih, metin, çalışma
alanınızın görselleri, üstte kaydın görseli ve kaydın herkese açık sayfasına bir düğme. Bir e-posta
tasarımının taşıyamadığı tablo veya kod bloğu, kayda bağlanan bir satıra dönüşür. Panel taslağı açar; kayıt,
bültenlerini ve durumlarını listeler.

Bununla hiçbir şey zamanlanmaz veya gönderilmez. Gözden geçirme, test gönderimleri, onay, abonelikten çıkma
bağlantıları, izin, hız ve gönderim her [bültenle](broadcasts.md) aynıdır. API ile:
`locale`, `list_ids` ve isteğe bağlı `channel_id` ile
`POST /v1/changelogs/{changelogId}/entries/{entryId}/broadcast`; `broadcasts:write` ve `lists:read`
yetkilerini ister ve yayınlanmamış bir kayıt için `409 not_published` verir. Asistanlar bir kaydı asla
e-postayla göndermez.

## Olaylar ve webhook'lar

Yayınlama (bir zamanlama tetiklendiğinde ve yayındaki bir çeviri değiştirildiğinde de), yayından kaldırma ve
yayındaki bir kaydı silme, gelen kutusunu gören üyeler için bir olay saklar ve [webhook'lara](webhooks.md)
`changelog.entry.published`, `changelog.entry.unpublished` ve `changelog.entry.deleted` olarak gönderilebilir.

## Başvuru

### Yetkiler

Yetkileri adlandırmadan oluşturulan API anahtarları `changelog:read` ve `changelog:write` taşır;
`changelog:publish` adlandırılmalıdır. Bunlar var olmadan önce oluşturulmuş anahtarlara yetki eklenmez. Bkz.
[Headless](headless.md#yetkiler).

| Yetki | İzin verir |
|---|---|
| `changelog:read` | Değişiklik günlüklerini, taslaklı kayıtları okuma ve önizleme. |
| `changelog:write` | Kayıt ve çeviri yazma, görsel yükleme. |
| `changelog:publish` | Değişiklik günlüğü oluşturma, değiştirme ve silme; yayındaki kayıtları yayınlama, zamanlama, yayından kaldırma ve silme. OAuth istemcilerine asla sunulmaz. |

Gelen kutularıyla sınırlı bir anahtar yalnızca o gelen kutularının değişiklik günlüklerini görür; diğerleri
`404` verir.

### Sınırlar

| Öğe | Sınır |
|---|---|
| Çalışma alanı başına kayıt | `YUVA_CHANGELOG_MAX_ENTRIES` ([Yapılandırma](configuration.md#yenilikler)). |
| Değişiklik günlüğü başına zamanlanmış kayıt | 50. |
| Değişiklik günlüğü başına dil | 20. |
| E-postayla gönderilen kayıt başına liste | 10. |
| Herkese açık sayfalar ve akışlar | Adres başına dakikada 120 istek. |
| Önizleme | Çağıran başına dakikada 60 işleme. |

### Kapatın

`YUVA_WHATS_NEW=off` özelliği kaldırır: uç noktaları ve herkese açık sayfaları `404` verir ve widget'ta
Yenilikler görünmez.
