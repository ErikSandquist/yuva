# Analitiği paylaşma ve dışa aktarma

Paylaşım bağlantısı, bir web sitesinin rakamlarının bir bölümünü Yuva hesabı olmayan kişilere salt okunur gösterir
ve kendi sayfalarınızda bir iframe içinde durabilir. Dışa aktarma, bir web sitesinin rakamlarını ya da ham olaylarını
indireceğiniz bir dosyaya yazar. İkisi de bir [Yuva Analytics](analytics.md) web sitesine aittir; API buna mülk
(property) der.

Bir [API özelliği](analytics-api.md) için görünümler, dışa aktarmalar ve saklama farklıdır: bkz.
[API izlemeyi paylaşma, dışa aktarma ve saklama](analytics-api-sharing.md).

## Paylaşım bağlantıları

Paylaşım bağlantısı, `<Yuva adresiniz>/share/<token>` biçiminde bir web adresidir; bir web sitesinin salt okunur
raporunu oturum açmadan gösterir. Adresi elinde tutan herkes bağlantının izin verdiğini görür; adresi rakamların
hak ettiği özenle gönderin.

Panelde web sitesini açın, Paylaş iletişim kutusunu açın ve şunları ayarlayın:

| Ayar | Anlamı |
|---|---|
| Ad | Bağlantının ne için olduğu. Yalnızca üyeler görür. |
| Görünümler | `overview`, `realtime`, `pages`, `sources`, `locations`, `devices`, `events`, `goals` değerlerinden en az biri. Bunlar [görünümlerin](analytics.md#görünümler) sekmeleridir; listede olmayan sekme izleyici için yoktur. |
| Parola | İsteğe bağlı, 8 ile 200 karakter. İzleyici bir şey görmeden önce bunu yazar. |
| Bitiş | İsteğe bağlı. Bu andan sonra bağlantı çalışmaz. |
| Şurada göm | İsteğe bağlı: bağlantıyı bir iframe içinde gösterebilecek sitelerin listesi ([aşağıda](#bir-paylaşım-bağlantısını-gömme)). |

Adres ve token (`yuva_sh_…`) bağlantı yapıldığında **bir kez** gösterilir. Yuva token'ın ve parolanın yalnızca bir
özetini (hash) saklar, bu yüzden ikisi de bir daha gösterilemez; kaybettiyseniz yeni bir bağlantı yapın. Liste,
bağlantıları ayırt edebilmeniz için token'ın ilk karakterlerini ve her bağlantının en son ne zaman açıldığını tutar.

Bir bağlantıyı durdurmak için **iptal edin**: hemen çalışmaz olur ve listede kalır. **Silmek** onu listeden de
kaldırır. Parolayı değiştirmek ya da kaldırmak, bağlantının kilidini daha önce açmış herkesin erişimini bitirir. Süre
dolumu da aynıdır: `expires_at` geçildikten sonra adres `404` döner.

Kim ne yapabilir:

- Sahipler, yöneticiler ve gelen kutusu sınırı olmayan, `analytics:manage` yetkili API anahtarları bağlantı yapar,
  değiştirir, iptal eder ve siler.
- Web sitesini okuyabilen (`analytics:read`) herkes listeyi görür.
- Bir web sitesinde en çok 50 bağlantı olur (`403 share_limit`).

### Paylaşımın gösterdikleri

Bağlantı yalnızca bu web sitesinin, görünümlerindeki rakamları gösterir. Toplamlar ve zaman ölçekleri her görünümle
gelir. Rakamları kırılımlara ayırmak ya da süzmek, sütunun sahibi olan görünümü gerektirir:

| Görünüm | Ekler |
|---|---|
| `pages` | `page`, `entry_page`, `exit_page`, `hostname` |
| `sources` | `channel`, `source`, `referrer` ve `utm_*` sütunları |
| `locations` | `country`, `region`, `city` |
| `devices` | `device`, `browser`, `browser_version`, `os`, `os_version`, `screen_size`, `language`, `platform`, `app_version` |
| `events` | `event`, `prop` ve olay metriği |
| `goals` | web sitesinin hedefleri ve metrikleri |

Bağlantının görünümleri dışındaki bir sorgu reddedilir (`403 view_not_shared`). Paylaşım kayıtlı segmentleri
(`segment_id` reddedilir), diğer web sitelerini, ayarları, veri araçlarını, üyeleri ve hiçbir dışa aktarmayı
**asla** göstermez.

İzleyiciler panelle aynı sorgu yuvalarını ve önbelleği kullanır; paylaşım uç noktaları adres başına sınırlıdır:
dakikada 120 istek ve bağlantı başına dakikada 10 parola denemesi (toplamda 30). Bağlantının kilidini açan izleyici
12 saat erişir.

### Bir paylaşım bağlantısını gömme

Bağlantıyı kendi sayfanıza bir iframe ile koyun:

```html
<iframe
  src="https://support.example.com/share/yuva_sh_xxxxxxxx"
  title="Website statistics"
  width="100%"
  height="800"
  style="border: 0"
  loading="lazy"
></iframe>
```

Sayfanın sitesini **Şurada göm** altına ekleyin: tek bir site için `https://www.example.com`, onun tüm alt alan
adları için `https://*.example.com`. En çok 20 girdi; her biri `https://` ya da `http://` ile bir alan adı (ve
isteğe bağlı port) olur, sonrasında hiçbir şey olmaz.

Tarayıcı paylaşım sayfasını yalnızca sitesi listede olan bir sayfanın içinde gösterir. Diğer her site ve listesi boş
her bağlantı, çerçevelemeyi yasaklayan bir sayfa alır (`frame-ancestors 'none'`); kimse raporunuzu kendi sitesindeki
bir çerçeveye koyamaz. Bilinmeyen, süresi dolmuş ya da iptal edilmiş bir bağlantı için de aynısı geçerlidir. Adres,
elinde tutan herkes için yine de bir tarayıcı sekmesinde açılır; liste yalnızca kimin çerçeveleyebileceğine karar
verir. Paylaşım sayfasında analitik betiği yoktur ve token sunucunun istek günlüklerine yazılmaz.

## Dışa aktarmalar

Dışa aktarma, rakamları arka planda sunucudaki bir dosyaya yazar. Panelde Veri altında, sonra Dışa aktar ile
başlatın ya da [API](#api) ile. Dosya hazır olana kadar ilerleme gösterilir; aynı yerden indirin. Dosya, yapıldıktan
**7 gün** sonra silinir, daha önce de silebilirsiniz. 1 GiB'tan büyük dosya `too_large` ile başarısız olur ve bir
çalışma alanı aynı anda en çok üç dışa aktarma çalıştırır (`429 rate_limited`).

| Tür | İçeriği | Biçimler | Aralık |
|---|---|---|---|
| `view` | Tek bir sorgunun her satırı, panelde göreceğiniz tablo (karşılaştırma yok). | CSV, NDJSON | Sorgunun kendi aralığı, en çok 400 gün |
| `daily` | Her gün için bir satır: ziyaretçi, ziyaret, sayfa görüntüleme, ziyaret başına görüntüleme, hemen çıkma oranı, ziyaret süresi, sayfada geçen süre ve kaydırma derinliği; isteğe bağlı olarak tek bir boyuta göre bölünmüş. | CSV, NDJSON | En çok 400 gün |
| `raw` | Saklanan her olay ve ziyaretinin tanımı. | Yalnızca NDJSON | En çok 31 gün, hepsi ham veri penceresinin içinde |

`view` ve `daily` için `analytics:read` yeterlidir; yani web sitesini okuyabilen herkes rakamlarını dışa aktarabilir.

### Ham dışa aktarmalar

`raw` dışa aktarma, veri hatları ve kendi analizleriniz içindir. `analytics:manage` yetkisi ve sahip, yönetici ya da
gelen kutusu sınırı olmayan bir API anahtarı olan bir çağıran gerekir. Ham satırlar takma adlı (pseudonymous) kişisel
veridir; bu yüzden Yuva onları çalışma alanını yöneten kişilerle sınırlar: sıradan bir okuyucu bir tane yapamaz,
listeleyemez ya da indiremez.

Aralık, ayrıntılı kayıtların tutulduğu günlerin içinde olmalıdır
([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analitik), varsayılan 90); daha eski bir gün `422 needs_raw_window`
döner. Botlar dışarıda bırakılır.

Olay başına bir satır:

```json
{"day":"2026-10-07","time":"2026-10-07T09:14:31Z","visit":3,"type":"pageview","hostname":"www.example.com","path":"/pricing","channel":"Organic search","source":"Google","referrer":"www.google.com","country":"DE","city":"Berlin","device":"Desktop","browser":"Firefox","os":"Linux","screen_size":"Desktop","language":"de","platform":"web"}
```

| Alan | Anlamı |
|---|---|
| `day`, `time` | Web sitesinin saat diliminde gün ve olayın zamanı. |
| `visit` | Bu dosyada 1'den başlayıp artan ve bir ziyaretin olaylarını bağlayan sayı. |
| `type` | `pageview`, `screen`, `event` ya da `engagement`. |
| `name`, `props`, `revenue_minor`, `currency` | Özel bir olayın adı, prop'ları ve geliri (para biriminin küçük birimiyle). |
| `engaged_ms`, `scroll` | `engagement` olayında: görünürde geçen milisaniye, en derin kaydırma yüzdesi. |
| `hostname`, `path` | Sayfa, uygulamada ekran. |
| `channel`, `source`, `referrer`, `utm_*` | Ziyaretin nereden geldiği, ilk olayının tanımladığı gibi. Referrer bir alan adıdır, asla yol değildir. |
| `country`, `region`, `city`, `device`, `browser`, `browser_version`, `os`, `os_version`, `screen_size`, `language`, `platform`, `app_version` | Ziyaretin yeri ve cihazı, ilk olayının tanımladığı gibi. |

Geçerli olmayan alanlar satırdan çıkarılır.

Ham dosya ziyaretçi hash'ini, günlük tuzu, IP adresini, bir kişiyi, saklanan ziyaret kimliğini ya da referrer yolunu
**asla** içermez. `visit` her dosya için yeniden numaralanır; bu yüzden iki dosya onunla birleştirilemez ve ziyaretçi
hakkında bir şey söylemez. Ham dosyayı kişileri tanımak için başka verilerle birleştirmeyin; başkalarına verirseniz
kendi [gizlilik bildiriminiz](analytics-privacy.md) çerçevesinde kişisel veri olarak ele alın.

### CSV ve hesap tabloları

`=`, `+`, `-`, `@`, sekme ya da satır başı ile başlayan bir CSV hücresinin başına `'` konur. Sayfa yolları,
referrer'lar ve olay adları ziyaretçilerden gelir; `=HYPERLINK(...)` gibi bir hücre, dosya bir hesap tablosunda
açıldığında formül olarak çalışırdı. Sonuç: `-5` ya da `+49 30` gibi bir değer `'-5` ya da `'+49 30` olarak görünür.
`/pricing` gibi bir yol değişmez. Dosyayı bir programla okuyorsanız baştaki `'` karakterini atın; değerlere saklandığı
gibi ihtiyacınız varsa NDJSON kullanın.

## API

Çağrılar `Authorization: Bearer $YUVA_API_KEY` alır. `PROPERTY` değişkenini web sitesinin kimliğine ayarlayın.

| Çağrı | Ne yapar |
|---|---|
| `GET /v1/analytics/properties/{propertyId}/shares` | Paylaşım bağlantılarını, iptal edilmiş ve süresi dolmuşlar dahil listeler (`analytics:read`). |
| `POST /v1/analytics/properties/{propertyId}/shares` | Bağlantı yapar. Yanıt `token` ve `url` değerlerini bir kez taşır. |
| `PATCH /v1/analytics/properties/{propertyId}/shares/{shareId}` | Adı, görünümleri, parolayı (`null` kaldırır), gömme kaynaklarını ya da bitişi (`null` bağlantıyı iptale kadar tutar) değiştirir. İptal edilmiş bağlantı `409 share_revoked` döner. |
| `POST /v1/analytics/properties/{propertyId}/shares/{shareId}/revoke` | Bağlantıyı iptal eder. |
| `DELETE /v1/analytics/properties/{propertyId}/shares/{shareId}` | Bağlantıyı siler. |
| `GET /client/v1/analytics-shares/{token}` | Paylaşım sayfasının ilk okuduğu şey. Hesap gerekmez. |
| `POST /client/v1/analytics-shares/{token}/access` | Bağlantının kilidini parolasıyla açar ve 12 saatlik bir `yuva_sa_…` token döner. |
| `POST /client/v1/analytics-shares/{token}/query` | Bağlantının görünümleriyle sınırlı rakamlar. |
| `GET /client/v1/analytics-shares/{token}/realtime` | `realtime` görünümüyle son 30 dakika. |
| `GET /v1/analytics/properties/{propertyId}/exports` | Dışa aktarmaları süreleri dolana kadar listeler (`analytics:read`). |
| `POST /v1/analytics/properties/{propertyId}/exports` | Dışa aktarma başlatır. `202` döner. |
| `GET`, `DELETE /v1/analytics/properties/{propertyId}/exports/{exportId}` | Bir dışa aktarmayı ve ilerlemesini okur, ya da onu ve dosyasını siler. |
| `GET /v1/analytics/properties/{propertyId}/exports/{exportId}/download` | Dosyayı indirir. |

Genel bakış ve sayfalar için, parolalı, tek siteye gömülebilen, bir tarihte biten bağlantı yapın:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "name": "Board report",
    "views": ["overview", "pages"],
    "password": "a long password",
    "embed_origins": ["https://www.example.com"],
    "expires_at": "2027-01-01T00:00:00Z"
  }'
```

İptal edin:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares/$SHARE/revoke \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

Bir izleyicinin betiği bağlantının kilidini açar ve sorgular:

```sh
TOKEN=$(curl -s -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/access \
  -H "Content-Type: application/json" -d '{"password": "a long password"}' | jq -r .access_token)

curl -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/query \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"metrics": ["visitors"], "dimensions": ["time:day"], "range": {"period": "7d"}}'
```

Bir ayın günlük rakamlarını ülkeye göre dışa aktarın, bekleyin ve indirin:

```sh
EXPORT=$(curl -s -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "daily", "format": "csv", "range": {"period": "30d"}, "dimension": "country"}' | jq -r .id)

curl -s https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT \
  -H "Authorization: Bearer $YUVA_API_KEY" | jq '{status, progress, rows}'

curl -o daily.csv https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT/download \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

Bir haftanın ham dışa aktarması, `analytics:manage` yetkili bir anahtarla:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "raw", "format": "ndjson", "range": {"from": "2026-10-01", "to": "2026-10-07"}}'
```

İndirme, `status` `done` olana kadar `409 export_not_ready`, dosyanın süresi dolunca `404` döner. Tıkladığınız bir
bağlantı başlık gönderemez; bu yüzden indirme `?workspace_id=<id>` da kabul eder. Tüm alanlar [API
sözleşmesindedir](../openapi/openapi.yaml).
