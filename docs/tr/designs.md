# E-posta tasarımları ve şablonlar

Tasarım, bir e-postanın görünümüdür: başlık, metin, görsel ve düğme gibi bloklar ile renkleri ve
yazı tiplerini taşıyan bir tema. Bir kez panelde hazırlarsınız, Yuva ile gelen şablonlardan birinden
başlayabilirsiniz ve bültenlerinizde kullanırsınız. Sunucu tasarımı yaygın e-posta uygulamalarında
çalışan e-posta HTML'ine, düz metin kısmına ve koyu moda çevirir.

Tasarımları sahipler ve yöneticiler hazırlar. API anahtarları ve OAuth belirteçleri
`broadcasts:read` ya da `broadcasts:write` kapsamını ister. Asistanlar (agent) tasarımları
kullanamaz.

## 1. Şablonla başlayın

Panelde: Bültenler, sonra Tasarımlar sekmesi. Galeri altı başlangıç şablonu sunar: Bülten, Ürün
güncellemesi, Duyuru, Hoş geldiniz, Etkinlik ve Özet; her biri İngilizce, Türkçe ve Almanca. Birini
seçin, şablon dilini seçin ve kullanın. Görselleri çalışma alanınızın görsellerine kopyalanır, sonra
her şeyi değiştirebilirsiniz. Yeni boş tasarım, sade renklerle boş bir e-postayla başlar.

Bir tasarım bir gelen kutusuna ya da tüm çalışma alanına aittir. Gelen kutusunun
[gönderen bilgileri](lists.md#1-gönderen-bilgilerini-girin) alt bilgiyi doldurur.

API ile: `name` ve `template: { "slug": "newsletter", "locale": "tr" }` ile `POST /v1/email-designs`.
`GET /v1/email-templates` şablonları listeler.

## 2. Tasarımı düzenleyin

Tasarımcı tam ekran açılır. Blokları paletten (ya da Katmanlar özetinden) sürükleyin ya da seçili
bloktan sonra eklemek için üzerinde Enter'a basın. Ayarlarını sağda değiştirmek için bir bloğa
tıklayın.

| Blok | Kullanım |
|---|---|
| Bölüm | Kendi arka planı olan bir blok grubu. Bölümler iç içe geçmez. |
| Sütunlar | Telefonlarda alt alta dizilen iki ya da üç sütun. Sütun genişlikleri 100 eder. Sütunlar iç içe geçmez. |
| Başlık | Üç boyut. |
| Metin | Paragraflar, listeler ve bağlantılar. |
| Görsel | Yüklenmiş bir resim; isteğe bağlı bağlantı ve alt metin. |
| Düğme | Düğme gibi görünen bir bağlantı. |
| Ayırıcı, Boşluk | İnce bir çizgi, boş alan. |
| Sosyal bağlantılar | Profillerinize bağlantılar, simge ya da metin olarak. |
| Alt bilgi metni | Zorunlu alt bilginin üstüne kendi satırlarınız. |

Başlıklar, metinler, alt bilgi metni ve düğme etiketleri yerinde düzenlenir. Metin yalnızca e-posta
uygulamalarının iyi işlediklerini korur: kalın, italik, altı çizili, üstü çizili, bağlantı, liste,
satır sonu ve kod. Bağlantılar `https://`, `http://`, `mailto:` ya da `tel:` ile başlar.

Her e-posta, kaldıramayacağınız bir alt bilgiyle biter: gelen kutusunun yasal unvanı ve posta
adresi, kişinin e-postayı neden aldığı, abonelikten çıkma ve tercih bağlantıları. Gelen kutusunda
henüz yasal unvan ya da posta adresi yoksa bir uyarı bunu söyler.

Bir tasarım en çok 200 blok taşır.

### Birleştirme alanları

Birleştirme alanları her kişinin kendi bilgilerini e-postaya koyar. Bir metinde Ekle menüsünden
ekleyin ya da yazın. Değeri boş olan kişiler için `|` sonrasına yedek değer yazın:

```
Merhaba {{ contact.first_name | "değerli okurumuz" }},
```

| Alan | Değer |
|---|---|
| `contact.name` | Kişinin adı. |
| `contact.first_name` | Adın ilk sözcüğü. |
| `contact.email` | E-posta adresi. |
| `contact.attributes.<anahtar>` | Kişinin bir özniteliği, anahtarıyla. |
| `inbox.name` | Gelen kutusunun adı. |
| `list.name` | E-postanın gittiği liste. |
| `unsubscribe_url`, `preferences_url` | Kişinin kendi bağlantıları. Bağlantılarda birleştirme alanları yalnızca `?` sonrasında olabilir. |

Yuva'nın tanımadığı bir alan reddedilir. Yedek değer en çok 100 karakterdir. Değerler asla işaretlemeye
dönüşmez. Yedek değeri olmayan alan "Merhaba ," gibi bir boşluk bırakabilir; tasarımcı bunu uyarır.

## 3. Tema ve koyu mod

Renkler, yazı tipleri, içerik genişliği (480 ile 720 piksel), köşe yuvarlaklığı ve yön (soldan
sağa, sağdan sola ya da metne göre otomatik) için Temayı açın (kısayol `T`). Yazı tipleri sistem
yığınlarıdır: system, humanist, geometric, serif ve monospace; e-postalar web yazı tipi yüklemez.

E-posta uygulaması koyu modda olan okurlar, sizin renklerinizden türetilmiş daha koyu renkleri alır;
metin 4,5:1 kontrastta tutulur. Koyu mod bölümünde bu renklerden istediğinizi kendiniz belirleyebilirsiniz.
Saydam arka planlı koyu bir logo, koyu modda arkasına açık bir plaka alabilir.

## 4. Görseller

Görsel kitaplığını bir görsel bloğundan açın (kısayol `I`) ya da dosyaları üzerine bırakın.

| | |
|---|---|
| Türler | PNG, JPEG, GIF, WebP. SVG betik taşıyabildiği için reddedilir. |
| Yükleme boyutu | En çok 10 MiB; 40 megapiksel ve kenar başına 8.000 piksel. |
| Saklanan | Çözülüp EXIF, GPS ve renk profili bilgisi olmadan yeniden kodlanır. 1.200 pikselden genişse 1.200'e küçültülür. WebP, PNG olarak saklanır; saydamlığı yoksa JPEG olarak. |
| GIF | Olduğu gibi, animasyonuyla saklanır: en çok 1.200 piksel genişlik ve 5 MiB. |
| Aynı dosya iki kez | Elinizdeki görselle yanıtlanır. |
| Adres | Her görselin sunucunuzda herkese açık bir `/i/{token}` adresi vardır: çerez yok, bir yıl önbellek, istek başına hiçbir şey kaydedilmez. |

Her görsele ekran okuyucular için bir açıklama yazın ya da süs amaçlı olarak işaretleyin. Bir çalışma
alanının görselleri birlikte en çok `YUVA_EMAIL_IMAGES_MAX_BYTES` kadar yer tutabilir
([Yapılandırma](configuration.md)).

Arşivlenmemiş bir tasarımın kullandığı görsel silinemez; panel tasarımları adıyla söyler. Bir bültende
gönderilmiş görsel onay ister, çünkü teslim edilmiş e-postalarda kırık görsel görünür. Hiçbir tasarımın
kullanmadığı ve hiçbir bültende gönderilmemiş görseller yüklemeden 7 gün sonra silinir.

## 5. Önizlemeler ve denetimler

Önizleme sunucudan gelir, yani alıcıların göreceği şeydir. Düzenle, Masaüstü, Telefon ve Koyu
arasında `1` ile `4` tuşlarıyla geçin. Denetimler neyin düzeltileceğini gösterir:

| Uyarı | Anlamı |
|---|---|
| Zor okunuyor | Metin arka planına göre 4,5:1 kontrastın altında, açık ya da koyu renklerde. |
| 90 KB üstü | Gmail e-postaları yaklaşık 102 KB HTML'de keser ve sonrasını, abonelikten çıkma bağlantısı dahil, gizler. Tasarımcı 90 KB'ta ve 102 KB üstünde uyarır. |
| Görsel yok, açıklama yok | Görseli olmayan ya da alt metni olmayan görsel bloğu. |
| Yedek değer yok | Bazı kişiler için boş kalabilecek bir birleştirme alanı. |
| Gönderen bilgileri eksik | Gelen kutusunda yasal unvan ya da posta adresi yok. |

500 KB'ı aşan HTML'li tasarım reddedilir.

## 6. Kaydetme, sürümler ve çakışmalar

Tasarımcı son değişikliğinizden 1,5 saniye sonra kaydeder; `Ctrl`/`Cmd` + `S` hemen kaydeder ve bir
sürüm saklar. Geri al ve yinele (`Ctrl`/`Cmd` + `Z`, yinelemek için `Shift` ile) 100 adım geri
gider. Çevrimdışıyken değişiklikler cihazınızda kalır ve geri geldiğinizde kaydedilir.

Sürümler (kısayol `V`) en yeni 50'yi tutar: düzenlerken en çok 10 dakikada bir, her elle kayıtta ve
her geri yüklemede bir. Geri yükleme, o sürümü en yeni sürüm olarak kaydeder.

Başka biri (ya da sizin başka bir sekmeniz) önce kaydettiyse bir pencere üç yol sunar: benimkini
koru, benimkini kopya olarak kaydet ya da onlarınkini yükle. Düzenlemeniz hiç kaybolmaz, diğer sürüm de
sürümlerde kalır.

JSON indir, tasarımın belgesini dosya olarak kaydeder. Arşivle, tasarımı listeden gizler; yeniden
kaydetmek geri getirir. Yuva'nın daha yeni bir sürümünün yaptığı belge görüntülenebilir ama
değiştirilemez.

Kısayollar: `A` blok ekle, `T` tema, `I` görsel, `V` sürümler, `Ctrl`/`Cmd` + `D` bloğu çoğalt,
`Backspace` sil, `Alt` + `↑`/`↓` taşı, `Esc` önce metinden sonra bloktan çıkar.

## 7. API

Panelin yaptığı her şey [API sözleşmesinde](../../openapi/openapi.yaml) var. Şu istek bir tasarımı
kaydetmeden işler:

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
        { "id": "h", "type": "heading", "level": 1, "text": "Merhaba {{ contact.first_name | \"değerli okurumuz\" }}" },
        { "id": "b", "type": "button", "label": "Devamını oku", "url": "https://example.com/news" }
      ]
    }
  }'
```

Yanıtta `html`, `text`, `size_bytes` ve `warnings` bulunur. Koyu palet için `"dark": true` gönderin.
Kayıt, başladığı `version` değerini `base_version` olarak gönderir; tasarım bu arada değiştiyse yanıt,
güncel tasarımı `design` içinde taşıyan `409 version_conflict` olur. Geçersiz belge, her sorun için bir
kayıtla `errors` taşıyan `422 invalid_design` döner.

## Başvuru

### Uç noktalar

| Uç nokta | Kullanım |
|---|---|
| `GET`, `POST /v1/email-designs` | Listele (`inbox_id`, `q`, `archived=true`) ve bir `document`, bir `template` ya da boş olarak oluştur. |
| `GET`, `PUT`, `DELETE /v1/email-designs/{designId}` | Oku, kaydet (`base_version` ile), arşivle. |
| `POST /v1/email-designs/{designId}/duplicate` | Tasarımı kopyala. |
| `POST /v1/email-designs/render` | Bir belgeyi işle: HTML, metin, boyut ve uyarılar. Dakikada en çok 120. |
| `GET /v1/email-designs/{designId}/versions` | Saklanan sürümler. |
| `GET /v1/email-designs/{designId}/versions/{version}` | Belgesiyle bir sürüm. |
| `POST /v1/email-designs/{designId}/versions/{version}/restore` | Bir sürümü en yeni olarak kaydet. |
| `GET`, `POST /v1/email-images` | Görselleri listele ve yükle (multipart, tek `file`). |
| `DELETE /v1/email-images/{imageId}` | Görseli sil (gönderilmiş olan için `force=true`). |
| `GET /v1/email-templates`, `GET /v1/email-templates/{slug}` | Galeri (`locale=en`, `tr` ya da `de`). |
| `GET /i/{token}` | Bir görsel, herkese açık. |
| `GET /email-templates/{slug}/{locale}.html`, `.webp` | Bir şablonun önizlemesi, herkese açık. |

### Kapsamlar

| Kapsam | Anlamı |
|---|---|
| `broadcasts:read` | Tasarımları, sürümleri, görselleri ve şablonları oku; işle. |
| `broadcasts:write` | Tasarım oluştur, kaydet, kopyala, arşivle ve geri yükle; görsel yükle ve sil. |

Gelen kutularıyla sınırlı bir anahtar, kendi gelen kutularının ve tüm çalışma alanına ait tasarımları
okur, yalnızca kendi gelen kutularının tasarımlarını değiştirir. Bu kapsamlar var olmadan önce
oluşturulan anahtarlara verilmez.

### Sınırlar

| Sınır | Değer |
|---|---|
| Bir tasarımdaki blok | 200 |
| Tasarım belgesi | 300 KiB JSON |
| İşlenmiş HTML | 90 KB uyarı, 102 KB kesilir, 500 KB reddedilir |
| Saklanan sürüm | 50 |
| Görsel yükleme | 10 MiB, 40 megapiksel, kenar başına 8.000 piksel |
| Yüklemeden sonra görsel genişliği | 1.200 piksel |
| Birleştirme alanı yedek değeri | 100 karakter |
| Bağlantı | 2.048 karakter |
| İşleme | Çağıran başına dakikada 120 |
| Kullanılmayan görseller | Yüklemeden 7 gün sonra silinir |
