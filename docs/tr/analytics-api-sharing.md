# API izlemeyi paylaşma, dışa aktarma ve saklama

Paylaşım bağlantısı, bir [API özelliğinin](analytics-api.md) bir kısmını Yuva hesabı olmayan kişilere gösterir; dışa
aktarma rakamlarını bir dosyaya yazar; saklama ise her tür rakamın ne kadar tutulacağını belirler. Tek bir tüketiciyle
ilgili silme talebi tek bir çağrıyla karşılanır.

## Paylaşım bağlantıları

API özellikleri web siteleriyle aynı bağlantıları kullanır: bağlantıları aynı çağrılarla
[oluşturur, değiştirir, iptal eder ve silersiniz](analytics-sharing.md#paylaşım-bağlantıları); belirteç, parola, son
kullanma tarihi, gömme ve 50 bağlantı sınırı da aynıdır. Fark görünümlerdedir. Bir API özelliğinin bağlantısı `api_`
görünümlerini, bir web sitesinin bağlantısı web sitesi görünümlerini listeler; ikisini karıştırmak
`422 property_kind` döndürür.

| Görünüm | İzleyici ne görür |
|---|---|
| `api_overview` | Tüm metriklerin toplamları ve grafikleri; `status`, `status_class`, `environment`, `service`, `api_version`, `source`, `platform`, `country` ve `app_version` kırılımlarıyla. |
| `api_routes` | `route`, `method` ve `routes` (yeni ve sessiz rotalar) ekler. |
| `api_consumers` | `consumer` ekler; kırılım ya da filtre olarak. |

Toplamlar ve zaman ölçekleri her görünümle gelir. Bağlantının görünümlerinden hiçbirine ait olmayan bir boyut ya da
filtre isteyen sorgu `403 view_not_shared` ile reddedilir. Bu yüzden tüketici adları **yalnızca** bağlantı
`api_consumers` görünümünü listeliyorsa gösterilir; bu görünüm olmadan bir bağlantı API'nizi kimin çağırdığını asla
göstermez.

Bir API özelliğinin bağlantısı `uptime`, `support` ve `conversations` verilerini asla paylaşmaz: `uptime` ya da
`support` veri kümesi veya `conversations` metriği için yapılan sorgu `403 view_not_shared` döndürür. Kayıtlı
segmentler web sitelerindeki gibi reddedilir ve paylaşımın dışa aktarması yoktur.

Genel bakış ve rotalar için, tüketicisiz bir bağlantı oluşturun:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/shares \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "name": "Ortaklar için durum",
    "views": ["api_overview", "api_routes"],
    "expires_at": "2027-01-01T00:00:00Z"
  }'
```

İzleyicinin betiği bunu `POST /client/v1/analytics-shares/{token}/query` ile okur; gövde,
`POST /v1/analytics/properties/{propertyId}/query` ile aynıdır ve `"dataset": "api"` içerir:

```sh
curl -X POST https://support.example.com/client/v1/analytics-shares/yuva_sh_xxxxxxxx/query \
  -H "Content-Type: application/json" \
  -d '{"dataset": "api", "metrics": ["requests", "error_rate", "p95"], "dimensions": ["route"], "range": {"period": "24h"}}'
```

Bir bağlantıyı iptal etmek, web sitelerinde olduğu gibi onu hemen durdurur. Paylaşım sayfası izleyicinin tarayıcısına
hangi tür özelliği gösterdiğini bildirir; böylece API grafiklerini çizer.

## Dışa aktarmalar

Dışa aktarma arka planda çalışır ve dosyasını **7 gün** saklar; bkz.
[Dışa aktarmalar](analytics-sharing.md#dışa-aktarmalar). Bir API özelliği için:

| Tür | İçeriği | Veri kümeleri |
|---|---|---|
| `view` | Bir `query` sorgusunun her satırı, panelde göreceğiniz tablo; karşılaştırma yok. | `api`, `uptime` |
| `daily` | `range` içindeki her gün için bir satır (en çok 400 gün); isteğe bağlı olarak tek bir `dimension` ile bölünmüş. | `api` (varsayılan), `uptime` |

İkisi de CSV (varsayılan) ya da NDJSON olarak gelir. Bir API özelliği için **`raw` dışa aktarma yoktur**, çünkü istek
başına satır tutmaz: `raw` dışa aktarma `422 property_kind` döndürür, `traffic` veri kümesi de öyle. `support` veri
kümesi dışa aktarılmaz.

Aralık, dışa aktarma yapılırken sabitlenir. `"period": "24h"` içeren bir `view` dışa aktarması, dosya ne kadar
beklerse beklesin, siz istediğinizde biten 24 saati içerir; daha sonra yeniden istemek daha yeni bir aralık verir.
Panelin çalıştırdığı aynı sorgu aynı satırları verir. Geçersiz bir sorgu, iş başlamadan hemen reddedilir.

Bir `daily` dışa aktarmada önce gün, varsa sonra boyut, ardından şu sütunlar gelir:

| Veri kümesi | Sütunlar, sırayla | Günler |
|---|---|---|
| `api` | `requests`, `errors`, `error_rate`, `client_errors`, `client_error_rate`, `p50`, `p95`, `p99`, `avg_duration`, `apdex`, `request_bytes`, `response_bytes` | Özelliğin yerel günleri |
| `uptime` | `runs`, `failures`, `uptime`, `downtime`, `response_time_p50`, `response_time_p95`, `response_time_p99`, `avg_response_time` | UTC günleri |

Bir `api` dışa aktarmasının `dimension` değeri herhangi bir API boyutu, bir `uptime` dışa aktarmasınınki `check`
olabilir. Oranlar yüzdedir, süreler milisaniyedir.

Bir aylık istekleri rotaya göre dışa aktarın ve indirin:

```sh
EXPORT=$(curl -s -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/exports \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "daily", "dataset": "api", "format": "csv", "range": {"period": "30d"}, "dimension": "route"}' | jq -r .id)

curl -o routes.csv https://support.example.com/v1/analytics/properties/$PROPERTY/exports/$EXPORT/download \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

İndirmeden önce `GET …/exports/{exportId}` yanıtında `status: done` görünmesini bekleyin. Çağrılar, yetkiler ve
sınırlar [web sitesi dışa aktarmalarındakilerle](analytics-sharing.md#api) aynıdır.

## Rakamlar ne kadar saklanır

API izleme rakamlarını katmanlar halinde tutar; her biri gerektiği kadar:

| Rakamlar | Saklama | Belirleyen |
|---|---|---|
| Dakikalar (`time:minute`) | Varsayılan 8 gün | Operatör, sunucu genelinde: [`YUVA_MONITOR_MINUTE_DAYS`](configuration.md#api-izleme), 2 ile 31 arası |
| Saatler | Varsayılan 90 gün | Özelliğin `monitor_hour_days` değeri, 7 ile 400 arası |
| Günler ve boyut kırılımları | Varsayılan 25 ay | Özelliğin `retention_months` değeri, 1 ile 120 arası, ya da sonsuza kadar için `null` |
| Bir çalışma süresi denetiminin çalıştırmaları | Varsayılan 30 gün | Operatör: [`YUVA_MONITOR_CHECK_DAYS`](configuration.md#api-izleme), 2 ile 90 arası |
| Bir denetimin günlük rakamları, durum değişiklikleri ve hizmet geçmişi | 400 gün | Sabit |

Özellik başına iki değeri `PATCH /v1/analytics/properties/{propertyId}` ile değiştirin:

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/$PROPERTY \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{"monitor_hour_days": 30, "retention_months": 13}'
```

Bir arka plan işi, katmanının süresini aşanı siler. Daha kısa bir değer eski rakamları kalıcı olarak siler; daha uzun
bir değer, daha önce silinmiş olanı geri getirmez. Web sitelerinin çalışma alanı genelindeki `retention_days` değeri
API özelliklerine uygulanmaz.

Bir API özelliğini silmek, rakamlarını her katmanda siler. Çalışma süresi denetimleri silinmez: özelliksiz çalışmaya
devam ederler ve kendi sonuçları kendi saklama sürelerine tabi kalır.

## Bir tüketiciyi silme

Tüketici, kayıtlarınızın API'nizi çağırana verdiği addır. Biri kendisininkini silmenizi isterse, kimliğini bulun ve
silin. İki çağrı da API özellikleri içindir; bir web sitesi `422 property_kind` döndürür.

```sh
curl "https://support.example.com/v1/analytics/properties/$PROPERTY/consumers?q=acme" \
  -H "Authorization: Bearer $YUVA_API_KEY"

curl -X DELETE https://support.example.com/v1/analytics/properties/$PROPERTY/consumers/$CONSUMER \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

`GET …/consumers`, özelliğin saydığı tüketicileri `id`, `name` ve `first_seen_at` ile listeler; `q` adı bu metni
içerenleri tutar, imleç listeyi sayfalar. `analytics:read` gerekir. 10.000 tüketici sınırını aşan isteklerin sayıldığı
`(other)` listelenmez ve silinemez (`400 validation_failed`). `DELETE` için `analytics:manage` ve bir sahip, yönetici
ya da gelen kutusu sınırı olmayan bir API anahtarı gerekir; `204` döndürür.

Silmenin yaptıkları:

- Tüketicinin saatlik ve günlük satırlarını her katmanda ve adını siler. Panel ve API onu artık bir `consumer`
  kırılımında göstermez, filtre olarak da kabul etmez.
- Toplamlarınız değişmez. Dakika, saat ve gün toplamları tüketiciyi hiç tutmadı; istekleri sayılmaya devam eder,
  yalnızca kimseye atfedilmez.
- Siz istediğinizde bir sunucu sürecinin bellekte hâlâ topladığını silmek için 2 ve 11 dakika sonra iki kez daha
  tekrarlanır.
- Kimlik başka bir tüketiciye asla verilmez. Kayıtlarınız aynı tüketiciyi yeniden adlandırırsa yeni bir kimlik altında
  sayılır.
- Öncesinde yapılmış dışa aktarma dosyaları, yapıldıktan 7 gün sonra süreleri dolana kadar adı tutar. Daha erken
  silmek için `DELETE /v1/analytics/properties/{propertyId}/exports/{exportId}` kullanın.
