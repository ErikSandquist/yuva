# API için uyarılar ve raporlar

Uyarı kuralları bir [API özelliğini](analytics-api.md) ve [çalışma süresi denetimlerini](analytics-api-uptime.md) izler;
bir şey ters gittiğinde ve bittiğinde sizi haberdar eder. Her sorun, başladığında bir uyarı, bittiğinde bir ileti üretir;
her dakika yeni bir ileti gelmez. Haftalık ve aylık raporlar dönemi özetler.

## Bir uyarı nasıl çalışır

Sunucu her kuralı dakikada bir, kapanmış dakikalar üzerinde uygular. Her kural ve konu için en çok bir açık uyarı tutar:

1. Koşul sağlandığında uyarı bir kez **tetiklenir**.
2. Açık kaldığı sürece başka bir şey gönderilmez.
3. Değer, bir pay bırakılarak, bütün bir pencere boyunca eşiğin içinde kaldığında uyarı bir kez **çözülür**.

Pay %10'dur: %5'in üzerindeki hata oranı için bir kural, oran bütün pencere boyunca %4,5'in altında kaldığında çözülür.
Yüksek kalması gereken Apdex için pay, 1'e olan uzaklığın onda biridir: 0,8'in altındaki Apdex için bir kural 0,82'nin
üzerinde çözülür. Ani artış ve düşüşün kendi payları aşağıdadır.

Bir uyarı, pencere beklemeden şu durumlarda da çözülür:

- pencere kuralın `min_requests` değerinden az istek içeriyorsa, çünkü değerlendirilecek bir şey kalmamıştır,
- sessiz bir rota yeniden çağrıldıysa,
- denetim yeniden ayaktaysa ya da sertifikası yenilendiyse,
- denetim duraklatıldıysa ya da kural devre dışı bırakıldıysa (devre dışı bırakılan kural açık uyarılarını kimseye
  haber vermeden çözer).

### Kural türleri

Bir özelliğin istekleri üzerindeki kurallar `property_id` ister. `route` kuralı tek bir rotaya (rota deseni, bkz.
`GET /v1/analytics/properties/{propertyId}/routes`), `service` tek bir servise daraltır; ikisi de yoksa kural bütün
özelliği izler. Son `window_minutes` kapanmış dakika üzerinden (5 ile 60, varsayılan 5) ve yalnızca pencere en az
`min_requests` istek içeriyorsa (varsayılan 20) değerlendirilirler.

| `kind` | Şu durumda tetiklenir | `threshold` |
|---|---|---|
| `error_rate` | Durumu 500 ile 599 olan (ya da yanıtsız) isteklerin payı eşiğin üzerindedir. | Yüzde. |
| `client_error_rate` | Durumu 400 ile 499 olanların payı eşiğin üzerindedir. | Yüzde. |
| `p95`, `p99` | O yüzdelik süre eşiğin üzerindedir. | Milisaniye. |
| `apdex` | [Apdex](analytics-api-analysis.md#apdex) puanı eşiğin altındadır. | 0 ile 1. |
| `spike` | Son 15 kapanmış dakikanın istekleri, önceki 7 günde aynı 15 dakikanın ortalamasının en az üç katıdır. | Yok. |
| `drop` | Bu ortalamanın en çok yarısıdır. | Yok. |
| `silent` | Önceki gün dakikada ortalama en az bir istek alan bir rota 15 dakikadır hiç istek almamıştır. | Yok. |

`spike` ve `drop` yalnızca 7 günlük ortalama en az 50 istekse değerlendirilir ve kendi pencereleri yoktur. Bir ani artış,
15 dakika ortalamanın 2,7 katının altına döndüğünde, bir düşüş 0,55 katının üzerine döndüğünde çözülür. `route` içeren bir
`silent` kuralı o rotayı izler; içermeyen, özelliğin her rotasını izler.

Çalışma süresi denetimleri üzerindeki kurallar `check_id` verir; ya da o özelliğin her denetimi için `property_id`, ya da
çalışma alanının her denetimi için hiçbirini:

| `kind` | Şu durumda tetiklenir | `threshold` |
|---|---|---|
| `check_state` | Denetim çökmüştür. | Yok. |
| `certificate` | Denetimin sertifikası eşik içinde sona erer. | Gün; yoksa denetimin `cert_warning_days` değeri. |

### Varsayılan kurallar

Yeni bir API özelliği tek bir kural alır: 5 dakika boyunca %5'in üzerindeki 5xx oranı, özelliği oluşturan üyeye
gönderilir. Bir çalışma süresi denetimi oluşturan üye, kendisine gönderilen bir `check_state` kuralı alır. Bunları diğer
kurallar gibi düzenleyin ya da silin.

## Kural oluşturma

Sahipler ve yöneticiler ile gelen kutusu sınırı olmayan API anahtarları, `analytics:manage` yetkisiyle kuralları yönetir:

```sh
curl https://support.example.com/v1/monitoring/alert-rules \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "p95",
    "property_id": "PROPERTY_ID",
    "route": "/checkout",
    "threshold": 800,
    "window_minutes": 15,
    "recipients": ["MEMBER_ID"]
  }'
```

| Alan | Anlamı |
|---|---|
| `kind` | Yukarıdaki türlerden biri. Kural oluştuktan sonra, konusu (`property_id`, `route`, `service`, `check_id`) gibi değişmez. |
| `name` | 1 ile 100 karakter; yoksa türünden ve konusundan adlandırılır. |
| `threshold`, `window_minutes`, `min_requests` | Yukarıdaki gibi. `min_requests` 1 ile 1.000.000. |
| `recipients` | Her uyarıyı e-posta ve Web Push ile alan en çok 20 üye. Kuralın özelliğini görebiliyor olmalıdırlar. |
| `enabled` | `false` kuralı duraklatır. |

Bir çalışma alanında en çok 200 kural olur (`403 rule_limit`). `PATCH /v1/monitoring/alert-rules/{ruleId}` geri kalanı
değiştirir ve bir sonraki dakikadan itibaren, açık uyarılara da uygulanır. `DELETE` kuralı ve uyarılarını siler; notları
kalır. Üyeler kuralları ve uyarıları `analytics:read` ile okur; bir özellikteki kurallar özelliğin görünürlüğünü izler.

## Bir uyarı nereye gider

Tetiklenme ve çözülme her biri şunların hepsini yapar:

- Kuralın `recipients` üyelerine, her üyenin dilinde **e-posta**.
- Aynı üyelere acil bildirim olarak **Web Push**.
- Buna abone uç noktalara `monitor.alert` **webhook**'u ([Webhook'lar](webhooks.md)). `data.alert.state`, `firing` ya da
  `resolved` olur. Bir gelen kutusuyla sınırlı uç nokta, o gelen kutusuna bağlı özelliklerin uyarılarını alır.
- Özelliğin grafiklerinde bir **not**, böylece an sayıların yanında görünür. Özelliği olmayan bir denetimin uyarısı,
  varsa denetimin özelliğine not düşer.
- Bağlı üyelere ve ajanlara `monitor.alert` **gerçek zamanlı** iletisi.

`analytics.alert` webhook'ları ve bir web sitesinin rapor aboneliğindeki uyarılar ayrıdır ve değişmemiştir. Bir API
özelliği aboneliğinde `alerts` almaz; uyarıları bu kurallardır.

## Uyarıları okuma

```sh
curl "https://support.example.com/v1/monitoring/alerts?state=firing" \
  -H "Authorization: Bearer $YUVA_API_KEY"
```

`GET /v1/monitoring/alerts` uyarıları en yenisi önce olmak üzere `limit` kadar (varsayılan 50, en çok 100) listeler;
`state` (`firing` ya da `resolved`), `property_id`, `check_id` ya da `rule_id` ile süzülür, sonraki sayfa için `before`
kullanılır. Çözülmüş uyarılar 400 gün saklanır. Bir uyarıda şunlar vardır:

| Alan | Anlamı |
|---|---|
| `rule_id`, `rule_name`, `kind` | Onu üreten kural. |
| `key` | Kural içinde ne ile ilgili olduğu: kuralın tüm konusu için boş, `silent` için rota deseni, denetim kuralları için denetim kimliği. |
| `state` | `firing` ya da `resolved`. |
| `value`, `threshold` | Tetiklendiğindeki değer ve kuralın eşiği. `spike` ve `drop` için değer 15 dakikanın istek sayısı, `certificate` için kalan gün, `check_state` için 1'dir. |
| `baseline` | `spike` ve `drop`: karşılaştırıldığı ortalama. `silent`: önceki gün dakika başına istek. |
| `resolved_value`, `resolved_at` | Ne zaman çözüldüğü. |
| `fired_at` | Onu tetikleyen pencerenin sonu. |
| `text` | Bulunanın, nottaki gibi İngilizce metni. |

`GET /v1/monitoring/alert-rules` kuralları listeler; her biri açık uyarılarının sayısı olan `firing` değerini taşır.

## Haftalık ve aylık raporlar

Bir API özelliği, rapor isteyen üyelere kendi raporunu gönderir; ayarlar bir web sitesininkiyle aynıdır
([E-posta raporları](analytics-support.md#e-posta-raporları-ve-uyarı-e-postaları)):

```sh
curl -X PUT https://support.example.com/v1/analytics/properties/PROPERTY_ID/subscription \
  -H "Content-Type: application/json" \
  -d '{ "report": "weekly" }'
```

Bu çağrı API anahtarı değil, bir üyenin oturumunu ister. Haftalık raporlar Pazartesi 08:00'de, özelliğin saat diliminde
çıkar ve önceki haftayı kapsar; aylık raporlar ayın 1'inde çıkar ve önceki ayı kapsar. Rapor; istekleri, hata ve istemci
hata oranlarını ve p95'i önceki döneme göre, en yavaş ve en çok hata veren rotaları, özelliğin denetimlerinin çalışma
süresini ve dönemin uyarılarını içerir. Görsel yoktur ve açılmayı ya da tıklamayı izleyen hiçbir şey yoktur.
`POST …/subscription/send-report` hemen bir tane gönderir.

## Bir asistandan

`monitoring_alerts` ve `uptime_checks` araçları aynı rakamlardan yanıt verir; bkz.
[Bir asistanla API izleme](mcp.md#bir-asistanla-api-izleme).
