# API izlemeyi çözümleme

Bu sayfa, bir API özelliğindeki her sayının ne anlama geldiğini, yüzdeliklerin nasıl hesaplandığını ve hangi
kırılımları ve karşılaştırmaları isteyebileceğinizi anlatır. [API izleme](analytics-api.md) sayfasının üzerine kuruludur.
Panel bu görünümleri özelliğin **Genel bakış** sayfasında ve her rotanın sayfasında gösterir; aynı sayılar analitik
sorgusundan da gelir.

## Sayılar ne anlama gelir

Her sayı seçtiğiniz aralık üzerinden sayılır. Örneklenmiş bir istek (`sample_rate` 1'den büyük) her sayıda o kadar
istek olarak sayılır; bu yüzden örneklenmiş kayıtlara dayanan yanıt bir tahmindir ve `meta.estimated: true` taşır.

| Metrik | Anlamı |
|---|---|
| `requests` | Yanıtlanan istekler. |
| `requests_per_minute` | `requests` bölü aralığın ya da kovanın şimdiye kadarki dakikaları: işlem hacmi. |
| `errors`, `error_rate` | Durumu 500 ile 599 olan ya da yanıt gönderilmediği için `0` olan istekler ve bunların tüm isteklerdeki yüzde payı. |
| `client_errors`, `client_error_rate` | Aynısı 400 ile 499 durumları için. |
| `p50`, `p75`, `p90`, `p95`, `p99` | İsteklerin bu payının içinde kaldığı süre, milisaniye olarak. `p95` değerinin 240 olması, 100 istekten 95'inin 240 ms ya da daha kısa sürdüğü demektir. |
| `avg_duration`, `max_duration` | Ortalama ve en uzun süre, milisaniye olarak. İkisi de kesindir. |
| `apdex` | İsteklerin ne kadarının yeterince hızlı olduğunu 0 ile 1 arasında gösteren puan; aşağıya bakın. |
| `request_bytes`, `response_bytes` | Boyut bildiren kayıtların gövde baytlarının toplamı. |
| `response_size_p50`, `response_size_p95` | Yanıtların bu payının içinde kaldığı boyut, bayt olarak. |
| `conversations` | Özelliğe bağlı gelen kutusunun aldığı görüşmeler, güne göre. |
| `latency_bands` | Süre bandı başına istek sayısı: gecikme ısı haritasının verisi. |

### Yüzdelikler nasıl hesaplanır

Yuva istek başına satır tutmaz, bu yüzden süreleri sıralayamaz. Bunun yerine her dakika, saat ve gün, kovaları 2^(1/32)
kat (yaklaşık %2,2) büyüyen küçük bir histogram tutar. Bir istek, süresinin düştüğü kovaya sayılır; farklı dakikaların
histogramları kova kova toplanır ve yüzdelik, isteklerin o sırasını tutan kova olarak okunur. Bir kova için bildirilen
değer kovanın ortasıdır; bu yüzden yüzdelik, aralık, kırılım ya da istek sayısı ne olursa olsun gerçek değerin %1,1
içindedir. Kovaları 2^(1/4) olan ikinci bir histogram yanıt boyutlarını tutar; `response_size_p50` ve
`response_size_p95` bu yüzden %9 içindedir.

Bir grubun yüzdeliği, parçalarının yüzdeliklerinden hesaplanamaz; ama histogramlar tam olarak toplanır. Yani bir rotanın
aylık `p95` değeri günlük sayıların ortalaması değil, o ayın gerçek `p95` değeridir.

### Apdex

Apdex süreleri, milisaniye cinsinden bir T eşiğine göre üç gruba ayırır: **memnun** istekler en çok T sürdü,
**katlanılır** olanlar T'den uzun ama en çok 4T sürdü, geri kalanı **hayal kırıklığı** yarattı. Puan
(memnun + katlanılır / 2) / istekler olarak bulunur; 1 her isteğin yeterince hızlı olduğu, 0 hiçbirinin olmadığı
anlamına gelir.

T, özelliğin `apdex_ms` değeridir (varsayılan 500 ms, 1 ile 60.000 arası). Doğası gereği yavaş bir rotanın, örneğin
bir dışa aktarmanın, kendi `apdex_ms` değeri olabilir; `null` özelliğinkine döner. Panelde rotanın sayfasından ya da
`PATCH /v1/analytics/properties/{propertyId}/routes/{routeId}` ile (kapsam `analytics:manage`) ayarlayın. Apdex
süre histogramlarından okunur; bu yüzden bir kovanın sınırındaki istek, yüzdeliklerde olduğu gibi bir grup öteye ya da
beriye sayılabilir.

### Gecikme bantları

`latency_bands`, toplamı `requests` eden 20 sayı verir. Band 0, en çok 256 mikrosaniye süren istekleri tutar; sonraki
her band bir sonraki ikinin kuvvetine kadar olanları (512 µs, 1,02 ms ve böyle devam ederek yaklaşık 67 saniyeye kadar),
son band ise bundan yavaş her şeyi. Üst sınırlar `meta.latency_band_bounds` içinde gelir ve her yanıtta aynıdır. Histogram
kovaları bu sınırlarda bittiği için sayılar tahmin değil, kesindir. Panel bunları rotanın sayfasında **Gecikme ısı
haritası** olarak çizer: bir eksende zaman, ötekinde bantlar.

## Kırılım ve filtre

`route`, `method`, `status`, `status_class`, `api_version`, `environment`, `service`, `source` (`server` sunucunuz,
`otlp` [OpenTelemetry](analytics-api-otel.md), `app` [uygulamalarınız](analytics-api-apps.md) için) ve `platform`
(uygulama kayıtları için `ios` ya da `android`) ile, her birleşimde ve her aralıkta kırılım yapabilirsiniz; en çok iki
boyut, biri zaman aralığı olabilir. Filtreler aynı boyutları kullanır. Panelde bunlar **Döküm** ve filtrelerdir.

`consumer`, `country` ve `app_version` üç ek boyuttur; her biri bir kez eklenebilir, sınırlarla:

- **Aynı anda bir ek boyut.** Bir sorguda ikisi, ya da biri boyut ötekisi filtre olarak, reddedilir.
- **Yalnızca `route`, `method` ve `status_class` ile.** Ek boyut, boyut ya da filtre olarak `status`, `api_version`,
  `environment`, `service`, `source`, `platform` ya da ikinci bir ek boyutla birleştirilemez.
- **Yalnızca tam yerel saatler üzerinde.** `time:hour` ya da daha uzun aralıklar, gün ya da hafta aralıkları, ya da
  saat başlarında `since` ve `until` kullanın. `15m`, `1h`, `6h`, `24h` ve `time:minute` reddedilir, çünkü dakikalar ek
  boyut başına tutulmaz.
- **Değeri olmayan istekler dışarıda kalır.** `consumer` kırılımı tüketici (ya da ülke, ya da uygulama sürümü) bildirmeyen
  istekleri atlar; bu yüzden satırların toplamı genel toplamdan az olabilir.

Tutulmayan bir birleşim `422 dimension_combination` yanıtlar; `detail` neyin değişmesi gerektiğini söyler. Panel boş bir
grafik yerine aynı iletiyi gösterir.

Bir özellik en çok 10.000 tüketiciyi ve 200 ülkeyi ya da uygulama sürümünü sayar; fazlası `(other)` altında sayılır ve
`meta.caps_reached` bunu bildirir.

## Tüketiciler

**Tüketiciler** sayfası tüketici başına istek, hata ve gecikmeyi, bir rotanın sayfasındaki **En çok çağıran tüketiciler**
ise aynısını o rota için gösterir. Tüketici, sunucunuzun `consumer` alanında gönderdiği değerdir: bir API anahtarının
adı, bir istemci, bir uygulama. Tüketici de bir ek boyuttur; yukarıdaki sınırlar geçerlidir: tam saatler ve yalnızca
`route`, `method` ya da `status_class` ile.

## Dönemleri karşılaştırma

`compare`, başka bir aralığın aynı sayılarını yanıta ekler: `previous` (hemen öncesindeki aynı uzunlukta aralık),
`year` (bir yıl önceki aynı günler) ya da `range` (`from` ile `to` arasındaki günler). Dakika aralıklarında (`15m` ile
`24h`, ya da `since` ve `until`) `previous` hemen öncesindeki o kadar dakika, `year` bir yıl önceki aynı anlardır.
Panelin karşılaştırma denetimi karşılaştırılan aralığı ikinci bir çizgi olarak çizer. API sorguları için `insights`
kullanılamaz.

## Yeni ve sessiz rotalar

`routes: new`, sunucunuzun aralık içinde ilk kez aldığı rotaları listeler; `routes: silent` ise aynı uzunluktaki önceki
dönemde çağrılıp aralık başladığından beri çağrılmayanları. Yalnızca `route` boyutuyla çalışır. Listelenen her rotanın bir
satırı vardır; hiç isteği yoksa sıfırlarla gelir, `previous` ile `compare` sessiz bir rotanın önceki sayılarını gösterir.
`(unmatched)`, `(other)` ve gizli rotalar hiçbir zaman listelenmez. Panelde bunlar rota listesinin **Yeni** ve **Sessiz**
sekmeleridir.

## Yayın işaretleri

Bir kayıt, hizmeti ve ortamı için yeni olan bir `release` getirdiğinde Yuva, o kaydın zamanına hizmet, ortam ve sürümle
bir `deploy` notu ekler. Bir hizmetin gönderdiği ilk sürüm de sayılır; daha önce görülmüş bir sürüme geri dönmek not
eklemez; özellik ve gün başına en çok 50 not eklenir. Panel bunları zaman grafiklerinde çizgi olarak çizer ve listeler.
Yuva'nın diğer notları gibi salt okunurdur. İşaret için kayıtlarınızla `release` gönderin ([OpenTelemetry](analytics-api-otel.md)
`service.version` değerini buna çevirir).

## Aralıklar ve güncellik

[Analitik](analytics.md) dönemlerinin yanında bir API sorgusu, geçerli dakikaya kadar `15m`, `1h`, `6h` ve `24h` ile tam
dakikadaki anlar olarak `since` ve `until` alır (`until` dahil değil, en çok 400 gün arayla). Yanıttaki dakika ve saat
etiketleri, özelliğin yerel saati ve UTC farkıdır, örneğin `2026-10-12T14:05+03:00`; böylece saat değişiminde tekrarlanan
saat ayrı kalır. Kapanmış günler ve saatler günlük ve saatlik sayılardan, geri kalanı dakikalardan okunur; yeni bir kayıt
yaklaşık 15 saniye içinde görünür. Dakikalar [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-izleme) gün saklanır.

## API

Tüm sorgular, `analytics:read` kapsamlı bir anahtarla `POST /v1/analytics/properties/{propertyId}/query` çağrısıdır.
Aşağıdaki JSON'u gövde olarak verin:

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/query \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '<BODY>'
```

Yüzdelikler, Apdex ve boyutlar, en yavaş rotalar önde:

```json
{
  "dataset": "api",
  "metrics": ["requests", "p75", "p90", "p95", "p99", "max_duration", "apdex", "response_size_p95"],
  "dimensions": ["route"],
  "range": { "period": "7d" },
  "order_by": [{ "metric": "p95", "dir": "desc" }],
  "limit": 10
}
```

Filtre ve kırılım (bir hizmette 5xx, rota ve API sürümüne göre):

```json
{
  "dataset": "api",
  "metrics": ["requests", "error_rate"],
  "dimensions": ["route", "api_version"],
  "filters": [
    { "dimension": "service", "op": "is", "values": ["checkout"] },
    { "dimension": "status_class", "op": "is", "values": ["5xx"] }
  ],
  "range": { "period": "24h" }
}
```

Bir rotanın tüketicileri (`route` ile bir ek boyut, tam günler üzerinde):

```json
{
  "dataset": "api",
  "metrics": ["requests", "errors", "p95"],
  "dimensions": ["consumer"],
  "filters": [{ "dimension": "route", "op": "is", "values": ["/users/{id}"] }],
  "range": { "period": "30d" }
}
```

Aynı toplamlar, bir önceki dönemle karşılaştırılmış:

```json
{
  "dataset": "api",
  "metrics": ["requests", "error_rate", "p95"],
  "range": { "period": "7d" },
  "compare": { "with": "previous" }
}
```

Son haftanın yeni rotaları (duran rotalar için `"silent"` kullanın):

```json
{
  "dataset": "api",
  "metrics": ["requests"],
  "dimensions": ["route"],
  "routes": "new",
  "range": { "period": "7d" }
}
```

Gecikme ısı haritası, bugünün her saati için bir bant kümesi (sınırlar `meta.latency_band_bounds` içinde):

```json
{
  "dataset": "api",
  "metrics": ["latency_bands"],
  "dimensions": ["time:hour"],
  "range": { "period": "today" }
}
```

`since` ve `until` ile bir dakika penceresi:

```json
{
  "dataset": "api",
  "metrics": ["requests_per_minute", "p95"],
  "dimensions": ["time:minute"],
  "range": { "since": "2026-10-12T14:00:00+03:00", "until": "2026-10-12T15:00:00+03:00" }
}
```

Bağlı gelen kutusunun aldığı görüşmeler, günlük (filtre ve başka boyut yok; özelliğin bağlı gelen kutusu yoksa
`422 no_linked_inbox`):

```json
{
  "dataset": "api",
  "metrics": ["conversations"],
  "dimensions": ["time:day"],
  "range": { "period": "30d" }
}
```

Bir rotanın kendi Apdex eşiği:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID/routes/ROUTE_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "apdex_ms": 5000 }'
```

Yayın işaretleri, özelliğin `deploy` türündeki notlarıdır:

```sh
curl "https://support.example.com/v1/analytics/properties/PROPERTY_ID/annotations?from=2026-10-01&to=2026-10-14" \
  -H "Authorization: Bearer $YUVA_API_KEY"
```
