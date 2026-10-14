# API izleme

Bir API özelliği, kendi API'nizin karşıladığı istekleri sayar: kaç tane olduğunu, kaçının başarısız olduğunu ve ne
kadar sürdüğünü, rota rota. Sunucunuz yanıtladığı her isteği bildirir; Yuva bunları dakika ve saat başına toplar,
istek başına satır, başlık, gövde ya da IP adresi saklamaz. Bu sayfa [Analitik](analytics.md) sayfasının
üzerine kuruludur.

## API özelliği oluşturun

API özelliği, `kind: api` olan bir analitik özelliğidir. Alan adı ve izleme betiği yoktur. Sahipler ve yöneticiler
onu `analytics:manage` kapsamlı bir anahtarla oluşturur:

```sh
curl https://support.example.com/v1/analytics/properties \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "kind": "api", "name": "Shop API", "domains": [], "timezone": "Europe/Berlin" }'
```

Yanıt özelliğin `id` değerini taşır. Özelliğin saat dilimi, sayılarının hangi saat ve günlere göre sayılacağını
belirler; `time:minute` sorgusunun dakikaları da bu dilimdedir.

Sonra kayıtları gönderecek anahtarı **Ayarlar → API anahtarları** altında `analytics:write` kapsamıyla oluşturun ve
sunucuda tutun. Bir üyenin oturumu reddedilir (`403 forbidden`); bir web sitesi özelliği `422 property_kind`
yanıtlar. `YUVA_MONITOR=off` ile başlatılan sunucuda API özelliği yoktur.

## İstek gönderme

Yığınızı seçin. Hepsi aynı kayıtları gönderir; hangisini kullanırsanız kullanın sayılar aynıdır.

- [Go](analytics-api-go.md): `net/http` için ara katman.
- [Node, Bun ve Deno](analytics-api-js.md): srvx, h3, Nitro, Nuxt, Hono, Express, Fastify ve Next.js route handler'ları.
- [OpenTelemetry](analytics-api-otel.md): diğer tüm diller; iki ortam değişkeni, Yuva kodu yok.
- [iOS ve Android uygulamaları](analytics-api-apps.md): uygulamanızın kendi API'nize yaptığı çağrılar, cihazda ölçülür.
- Başka her şey için aşağıdaki curl.

## curl ile istek gönderme

Diğer tüm teknolojiler aynı kayıtları gönderir. `POST /v1/analytics/properties/{propertyId}/requests` 1 ile 1.000
arası kayıt alır ve `202` yanıtlar. Gövde `Content-Encoding: gzip` ile sıkıştırılabilir.

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/requests \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "records": [{
      "method": "GET",
      "route": "/users/{id}",
      "status": 200,
      "duration_us": 18400,
      "response_size": 612,
      "consumer": "mobile-app",
      "environment": "production"
    }]
  }'
```

```json
{ "accepted": 1, "errors": [] }
```

| Alan | |
|---|---|
| `method` | Zorunlu, 1 ile 20 karakter. RFC 9110 yöntemleri ve `PATCH` korunur; diğerleri `_OTHER` sayılır. |
| `route` | Yönlendiricinin eşleştirdiği kalıp; `/` ile başlar, en çok 300 karakter, yöntem ve alan adı olmadan. Bunu ya da `path` verin. |
| `path` | Yönlendiricinin kalıp bilmediği durumda isteğin yolu. Yalnızca ondan çıkarılan kalıp saklanır. |
| `status` | Zorunlu, 100 ile 599; yanıt gönderilmediyse `0`. |
| `duration_us` | Zorunlu, mikrosaniye cinsinden süre, en çok 24 saat. |
| `request_size`, `response_size` | Gövdelerin bayt sayısı. |
| `time` | İsteğin başladığı an, RFC 3339. Boşsa şimdi. |
| `consumer` | En çok 100 karakter; yukarıya bakın. |
| `api_version`, `environment`, `service`, `release` | Sırasıyla en çok 40, 40, 100 ve 80 karakter. Boş bırakılan `environment` `production` olur. |
| `ip` | Arayanın adresi, yalnızca ülkesini bulmak için. |
| `sample_rate` | 1 ile 10.000: bu kayıt o kadar isteği temsil eder. Yalnızca 1-399 durumu örneklenebilir; `sample_rate` 1'den büyük ve hata durumlu kayıt reddedilir. |

## Rotalar

Bir rota, API'nizin bir bölümünü adlandırır, hiçbir zaman tek bir kaydı; bu yüzden bir kimlik asla kendi başına rota
olmaz.

- **Gönderdiğiniz rotalar temizlenir.** Bir `route` ya da `path` içinde kimliğe benzeyen her bölüm `{id}` olur:
  sayılar, UUID'ler, tarihler, 16 ya da daha çok karakterlik onaltılık dizeler, en az iki rakam (ya da iki harf
  büyüklüğü ve bir rakam) içeren 16 ya da daha çok harf, rakam, `-` ve `_` dizileri, e-posta biçimindeki değerler
  ve 40 karakterden uzun her şey. Yönlendirici yer tutucusu gibi başlayan bölümler (`{id}`, `:id`, `*`, `[id]`,
  `<id>`) olduğu gibi kalır. Yani `/users/42` ve `/users/9f8e7d6c5b4a3210` ikisi de `/users/{id}` sayılır.
- **Kalıpsız 404 ya da 405 `(unmatched)` olur.** Yalnızca `path` veren ve `404` ya da `405` ile yanıtlanan kayıt
  `(unmatched)` rotasında sayılır; böylece `/wp-login.php` yoklayan tarayıcılar rota üretmez.
- **Rota kuralları.** Özelliğin `path_rules` alanı, yukarıdaki temizlikten önce bir rotayı ya da yolu sizin
  rotanıza eşler. Kural `{"pattern": "/legacy/**", "replacement": "/legacy"}` biçimindedir: `*` bir bölümle, `**` her
  sayıda bölümle eşleşir, ilk eşleşen kural geçerlidir ve karşılık olduğu gibi kullanılır.
  `PATCH /v1/analytics/properties/{propertyId}` ile ayarlanır.
- **Sınırlar.** Bir özellik en çok 1.000 rota, 10.000 tüketici, 50 API sürümü ve 20 ortam; bir çalışma alanı 100
  servis sayar. Sınırdan sonra yeni değerler `(other)` olarak sayılır ve sorgu yanıtı sınırı
  `meta.caps_reached` içinde listeler.

### Rotaları listeleme ve gizleme

`GET /v1/analytics/properties/{propertyId}/routes` (kapsam `analytics:read`), özelliğin saydığı her rotayı kalıba
göre listeler; her biri `id`'si, yoldan `inferred` (çıkarılmış) olup olmadığı, `hidden` (gizli) olup olmadığı ve ilk
ile son görülme zamanıyla gelir. `(unmatched)` listededir; `(other)` da property'nin saydığı ilk rotadan itibaren
listededir ve bir rota onun altında sayılana kadar isteği yoktur.

Yalnızca gürültü ekleyen bir rota, örneğin yük dengeleyicinizin çağırdığı sağlık denetimi, gizlenebilir:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID/routes/ROUTE_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "hidden": true }'
```

Gizleme, rotayı o andan itibaren her `dataset: api` yanıtından çıkarır; satırlardan da toplamlardan da, ve o rotaya
göre filtre hiçbir şeyle eşleşmez. İstekleri yine sayılır ve hiçbir şey silinmez: `"hidden": false` rotayı tüm
sayılarıyla geri getirir. Gizlemek `analytics:manage` kapsamı ister ve sahipler ile yöneticiler içindir. Bir yolu
başka bir rota altında saymak için rota kuralı kullanın.

## Ne kaydedilir

Dakika, saat ve gün başına kaydedilenler: istek sayısı, durumu 500-599 ya da `0` olanlar (hatalar) ve 400-499
olanlar (istemci hataları), p50, p95 ve p99'u veren süre dağılımı; rota, yöntem, durum ve ortama göre. Servis, API
sürümü ve tüketici bunlarla birlikte saklanır.

Hiçbir zaman kaydedilmeyen ve API'nin kabul etmediği şeyler: istek ya da yanıt gövdeleri, başlıklar, sorgu
dizeleri, parçalar (fragment) ve user agent'lar. Postgres'te istek başına satır yoktur. `ip` kısaltılır (IPv4
`/24`, IPv6 `/48`), ülkesi bulunur ve aynı çağrıda atılır; hiçbir adres saklanmaz. Go ara katmanı adresi yalnızca
`Country: true` ile gönderir.

## Sayıları okuma

Panelde özelliğin **Genel bakış** sayfası zaman içinde istekleri, hata oranlarını ve gecikmeyi, **Rotalar** ise
rotaları trafikleri, hataları ve gecikmeleriyle gösterir.

Aynı sayılar analitik sorgusundan gelir: `POST /v1/analytics/properties/{propertyId}/query`, `"dataset": "api"`
ile (kapsam `analytics:read`):

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/query \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "dataset": "api",
    "metrics": ["requests", "error_rate", "p95"],
    "dimensions": ["route"],
    "range": { "period": "24h" },
    "limit": 20
  }'
```

| | |
|---|---|
| Metrikler | `requests`, `requests_per_minute`, `errors`, `error_rate`, `client_errors`, `client_error_rate`, `p50`, `p95`, `p99`, `avg_duration`. Oranlar yüzde, süreler milisaniyedir. Yüzdelikler %1,1 doğrulukla verilir. |
| Boyutlar ve filtreler | `route`, `method`, `status`, `status_class` (`2xx`, `3xx`, `4xx`, `5xx`; `0` durumu için `none`), `environment`; en çok iki boyut, biri zaman aralığı olabilir. |
| Zaman aralıkları | `time:minute`, `time:hour`, `time:day`. Dakikalar ve saatler özelliğin yerel saatindedir. |
| Aralık | Geçerli dakikaya kadar `15m`, `1h`, `6h`, `24h`; ya da `today`, `7d`, `30d` ve diğer dönemler; ya da `from` ve `to`. |

`time:minute`, [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-izleme) gün (varsayılan 8) geriye gider ve en çok
1.440 dakika döndürür; daha uzun ya da daha eski aralık `422 range_too_large` ile reddedilir. Saatlik sayılar
özelliğin `monitor_hour_days` değeri kadar (varsayılan 90, 7 ile 400 arası), günlük sayılar özelliğin
`retention_months` değeri kadar saklanır. `sample_rate` 1'den büyük gönderilen kayıtlara dayanan yanıtta
`meta.estimated: true` bulunur. Bir web sitesi özelliğinde `dataset: api` ve bir API özelliğinde bu alanı
vermeyen sorgu `422 property_kind` yanıtlar. API sorgusu `goal_id`, `prop`, `segment_id`, `compare` ya da
`insights` almaz.

Bir kayıt sorguda yaklaşık 15 saniye içinde görünür.

## Ne döner

İstek geçerliyse, kayıtlara ne olduğundan bağımsız olarak yanıt `202`'dir.

- **`errors`**, geçersiz diye dışarıda bırakılan kayıtları, her biri `records` içindeki `index`'i ve bir `detail`
  ile listeler. Diğerleri alınır.
- **Alınır ama sayılmaz:** 72 saatten eski kayıtlar; özellik ya da anahtar için `YUVA_MONITOR_RECORDS_PER_MINUTE`
  değerini aşan kayıtlar; çalışma alanının aylık sınırını aşan kayıtlar.
- **İsteğin tamamı reddedilir**: yukarıdaki biçimi bozan gövde (`400`), eksik ya da yanlış anahtar (`401`),
  `analytics:write` kapsamı olmayan anahtar ya da üye oturumu (`403`), bilinmeyen özellik (`404`), web sitesi
  özelliği (`422`) ve gzip dışındaki kodlama (`415`) için. İzin verdiği kadar açık satır tutan bir sunucu
  `Retry-After` ile `503` yanıtlar: o süreden sonra grubu yeniden gönderin.

Aylık sınır, kayıtların temsil ettiği istekleri değil, alınan kayıtları sayar: örneklenmiş bir kayıt `sample_rate`
değeri ne olursa olsun bir kayıttır. Sınırı aşan bir grup sınıra kadar alınır: hâlâ sığan kayıtlar sayılır,
kalanlar sınır aşıldığı için atılır. Bir ayın kayıtları sınıra ulaştığında çalışma alanının sahipleri ayda bir kez
e-posta alır. `GET /v1/analytics/properties`, ayın `requests` ve `requests_limit` değerlerini `usage` altında,
sınıra ulaşıldığında da `requests_limit_reached_at` değerini gösterir.

## Saat dilimini değiştirme

Bir API özelliğinin `timezone` değerini değiştirmek (`PATCH /v1/analytics/properties/{propertyId}`) bir günü ikiye
bölmez. Özellik, eski dilimin bir sonraki yerel gece yarısına kadar eski dilimde saymaya devam eder, ondan sonra
yenisinde. O ana kadar özellik, yeni dilimin başladığı an olan `timezone_applies_at` değerini taşır. Daha önce
sayılmış günler sayıldıkları dilimde kalır.
