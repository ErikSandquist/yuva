# Destek ve trafik

Bir [Yuva Analytics](analytics.md) web sitesini bir gelen kutusuna bağlayın; sayfaları ne kadar destek doğurduğunu
göstersin: konuşmalar, sayfa geri bildirimleri, değerlendirmeler ve tartışmalar, sayfanın görüntülemelerinin yanında
sayfa ve gün bazında sayılır. Aynı rakamlar öngörüleri, uyarıları, haftalık ve aylık e-posta raporlarını ve
[yapay zekâ asistanlarının](mcp.md#analitiği-bir-asistanla-kullanın) analitik araçlarını besler. Yuva bunların hiçbiri
için model çalıştırmaz: her öngörü ve uyarı aşağıda okuyabileceğiniz sabit bir kuraldır.

## Neyle birleştirilir

Web sitesinin ayarlarında gelen kutusunu seçin (**Gelen kutusu**, [Web sitesi ekleme](analytics.md#web-sitesi-ekleme)).
Bundan sonra Yuva, o gelen kutusunun web sitesinin sayfalarından aldıklarını **sayfa ve gün** bazında sayar:

| Rakam | Neyi sayar |
|---|---|
| `conversations` | Sayfada [sohbet widget'ı](widget.md) ile başlayan konuşmalar, başladıkları günde. |
| `feedback` | Sayfadan [`<yuva-page-feedback>`](documentation-pages.md) ile gönderilen geri bildirimler. |
| `ratings_up`, `ratings_down` | Sayfanın "Bu sayfa yardımcı oldu mu?" değerlendirmeleri, verildikleri günde (UTC). |
| `helpful_rate` | Değerlendirmelerin yüzde kaçı olumlu. |
| `discussions` | Sayfada başlatılan tartışma konuları. |
| `support_rate` | Sayfanın 1.000 sayfa görüntülemesi başına konuşma sayısı. |

Bunlar aynı sayfanın `visitors` ve `pageviews` değerlerinin yanında durur. Sayfalar, web sitesinin yol kurallarından
sonra host ve yola göre eşleşir; yani `/orders/:id` burada da tek bir sayfadır.

**Asla birleştirilmeyen:** Analytics'in hiçbir ziyaretçisi bir kişiyle ya da konuşmayla ilişkilendirilmez. İki taraf
yalnızca sayfada buluşur. Sayfanın bilinmesi için sohbet widget'ı, konuşmanın başladığı sayfanın adresini **sorgu
dizesi ve parça olmadan** ve sayfa başlığını gönderir; Analytics'ten başka hiçbir şey göndermez. Bir konuşma başladığı
sayfayı gösterir; sayılar kimin okuduğunu söylemez. Analytics'in kendisi yine çerez ve adres saklamaz
([Analitik ve gizlilik](analytics-privacy.md)).

Gelen kutusu bağlı olmayan bir web sitesinin destek rakamı yoktur; destek sorgusu `422 no_linked_inbox` döner.

## Destek oranı

Destek oranı **1.000 sayfa görüntülemesi başına konuşma** sayısıdır: 3.000 görüntülemeli bir sayfada 12 konuşma 4
eder. Bir orandır; bu yüzden yoğun bir sayfa yalnızca yoğun olduğu için öne geçmez. Aralıkta 100'den az
görüntülemesi olan sayfa diğerlerinden sonra listelenir, çünkü birkaç görüntülemenin oranı pek bir şey söylemez.

Sayfaları bu orana göre sıralı görmek, geri bildirim, değerlendirme ve tartışmalarıyla birlikte, ayrıca aralığı
öncekiyle karşılaştırmak için panelde web sitesinin destek görünümünü açın. API'de bu, `dataset: "support"` ile tek
bir sorgudur:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/query \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "dataset": "support",
    "metrics": ["pageviews", "conversations", "support_rate"],
    "dimensions": ["page"],
    "range": {"period": "30d"},
    "order_by": [{"metric": "conversations"}],
    "limit": 10
  }'
```

Destek veri kümesi `page`, `hostname` boyutlarını ve gün, hafta ya da ay ayrıntısını alır (en çok bir sayfa ya da host
ve bir ayrıntı), `page` ve `hostname` üzerinde filtre kabul eder; hedef, özellik ya da segment kabul etmez. Satırları,
aralıkta herhangi bir desteği olan sayfalardır.

## Öngörüler

Sorguya `"insights": true` ekleyin ya da panelin genel bakışını açın; Yuva, yanıtın kendi rakamlarından çıkarılan
kısa bulgular ekler. Bunlar sabit kurallardır:

| Öngörü | Kural | Örnek |
|---|---|---|
| `metric_change` | Bir toplam, karşılaştırma aralığına göre %20 ya da daha fazla değişti; öncesinde ya da sonrasında en az 10. | "Ziyaretçiler %34 arttı." |
| `row_change` | Tek kırılımla: bir satır %50 ya da daha fazla değişti, öncesinde ya da sonrasında en az 10. En büyük üçü. | "`news.example.com` kaynaklı ziyaretler ikiye katlandı." |
| `new_value` | Karşılaştırma aralığında hiç olmayan, şimdi en az 10 olan bir satır: yeni bir yönlendiren, sayfa ya da kampanya. En büyük üçü. | "`/pricing` bu ay yeni." |
| `support_hotspot` | Sayfa bazında destek veri kümesinde: en az 3 konuşması olan ve oranı aralığınkinin en az iki katı olan sayfa. En yüksek üçü. | "`/docs/install` 5 konuşma, 1.000 görüntülemede 9; genelde 3." |

Değişim kuralları karşılaştırma (`compare`) ister. Bir sayfayı, yönlendireni ya da kampanyayı adlandıran öngörüler,
ziyaretçilerin tarayıcılarının gönderdiği metni taşır.

## Uyarılar

Uyarılar, siz bakmadan bir şey değiştiğinde haber verir. Her saat denetlenir ve **yalnızca en az bir üyenin uyarı
seçtiği web siteleri için** çalışır (aşağıda). Her tür sabit bir kuraldır:

| Tür | Şu durumda tetiklenir | Gereken |
|---|---|---|
| `spike` | Bitmiş son iki saatin sayfa görüntülemesi, önceki 7 günde aynı iki saatin ortalamasının 3 katı ya da fazlası. | En az 20 görüntülemelik bir taban. |
| `drop` | Bitmiş son iki saatin sayfa görüntülemesi o ortalamanın yarısı ya da altı. | En az 20 görüntülemelik bir taban. |
| `goal_drop` | Bir hedefin dünkü dönüşümleri, önceki 7 günün ortalamasının yarısı ya da altı. Günde bir kez, web sitesinin saat diliminde sabaha karşı denetlenir. | Ortalama günde en az 10 dönüşüm. |
| `support_load` | Dünkü 1.000 sayfa görüntülemesi başına konuşma, önceki 28 günün oranının iki katı ya da fazlası. Günde bir kez denetlenir. | Bağlı bir gelen kutusu ve dün en az 5 konuşma. |
| `new_referrer` | Dün en az 10 ziyaret gönderen bir site, önceki 28 günde hiç göndermemişti. Günde bir kez denetlenir. | Yok. |

Bir tür web sitesi başına **günde bir kez** tetiklenir (yeni bir yönlendiren site başına bir kez). Tetiklendiğinde Yuva:

1. grafiğe, ilgili güne bir not koyar ([not](analytics-events.md); bulgunun metniyle, kendiniz de not ekleyebilirsiniz);
2. bir `analytics.alert` olayı saklar ve `analytics.alert` [webhook'unu](#analyticsalert-webhooku) gönderir;
3. o türü seçen üyelere e-posta gönderir.

`GET /v1/analytics/properties/{propertyId}/alerts`, web sitesinin tetiklediği uyarıları en yenisi önce, 100'erli
listeler (eskiler için `before`, son uyarının `created_at` değerini alır; yetki `analytics:read`).

### analytics.alert webhook'u

[Webhook uç noktanıza](webhooks.md) `analytics.alert` olayını ekleyin. Bir gelen kutusuyla sınırlı uç nokta, o gelen
kutusuna bağlı web sitelerinin uyarılarını alır; tüm çalışma alanı için olan hepsini alır. Gövde olağan
[zarftır](webhooks.md#olaylar) (`type`, `timestamp`, `workspace_id`, web sitesinin gelen kutusu varsa `inbox_id`,
`data`):

```json
{
  "type": "analytics.alert",
  "timestamp": "2026-10-13T06:00:05Z",
  "workspace_id": "6f1e0c5a-0d2e-4a39-9f55-5d0c0d7f2a10",
  "inbox_id": "b1c4d8e2-7a3f-4c0e-8d61-2f9a5e3b7c40",
  "data": {
    "alert": {
      "id": "0a8f3c1d-6b52-4e17-9c44-7d2e1f0b9a63",
      "property_id": "c3d5e7f9-1a2b-4c6d-8e0f-3a5b7c9d1e2f",
      "kind": "support_load",
      "day": "2026-10-12",
      "metric": "support_rate",
      "current": 6.4,
      "baseline": 2.1,
      "text": "6.4 conversations per 1,000 page views yesterday against 2.1 on the 28 days before",
      "annotation_id": "5e9b2d7c-3f41-4a86-b0d5-8c1a6f4e2b97",
      "created_at": "2026-10-13T06:00:05Z"
    },
    "property": { "id": "c3d5e7f9-1a2b-4c6d-8e0f-3a5b7c9d1e2f", "name": "Acme website" }
  }
}
```

`metric` değeri `pageviews`, `conversions`, `support_rate` ya da `visits` olur; `baseline`, kuralın `current` değerini
neyle karşılaştırdığıdır (yeni bir yönlendiren için 0). `spike` ve `drop` ayrıca `hours_from` ve `hours_to`,
`goal_drop` `goal_id` ve `goal_name`, `new_referrer` ise `referrer` host'unu taşır. `text` nottaki gibi İngilizcedir.

## E-posta raporları ve uyarı e-postaları

Her üye kendi adına, web sitesi başına seçer:

- **Rapor**: yok (varsayılan), haftalık ya da aylık.
- **Uyarılar**: yukarıdaki beş türden istediğiniz.

Rapor ve uyarı ayarlarını panelde web sitesinin sayfalarında bulun; ya da
`GET` ve `PUT /v1/analytics/properties/{propertyId}/subscription` çağırın (bir üyenin oturumuyla, API anahtarıyla değil):

```json
{ "report": "weekly", "alerts": ["spike", "drop", "support_load"] }
```

| Rapor | Gider | Kapsar |
|---|---|---|
| Haftalık | Pazartesi 08:00, web sitesinin saat diliminde | Önceki hafta, Pazartesi–Pazar. |
| Aylık | Ayın 1'i 08:00, web sitesinin saat diliminde | Önceki ay. |

Rapor üyenin dilinde yazılır ve düz rakamlar içerir: önceki döneme göre başlıca rakamlar, ilk beş sayfa ve kaynak,
hedeflerin dönüşümleri ve gelen kutusu olan bir web sitesi için konuşmalar, geri bildirimler ve değerlendirmeler ile en
çok konuşmaya yol açan sayfa. **Görsel yoktur ve açılmayı ya da tıklamayı izleyen hiçbir şey yoktur.** Uyarı e-postası
neyin bulunduğunu söyler ve panelde web sitesine bağlantı verir.

Raporun neye benzediğini görmek için aynı yerdeki hemen gönder düğmesini kullanın ya da
`POST /v1/analytics/properties/{propertyId}/subscription/send-report` çağırın. Son yedi bitmiş günü, aylık seçtiyseniz
son ayı gönderir; üye başına saatte beş.

## Bülten kampanyalarını izleme

Bir [bülten](broadcasts.md) gittiğinde Yuva, web bağlantılarına UTM parametreleri ekler; böylece getirdiği ziyaretler
Analytics'te Kaynaklar, `utm_campaign` ve `E-posta` kanalı altında görünür. Bu **varsayılan olarak açıktır**. Bir
bağlantı şunu alır:

```
?utm_source=newsletter&utm_medium=email&utm_campaign=<kampanya>
```

Kampanya, bülten oluşturulurken adından yapılır (küçük harf, rakam, `-`, `_` ve `.`; en çok 100 karakter); taslakta
değiştirebilirsiniz. Dokunulmayan bağlantılar:

- kendi `utm_` parametresi olan bağlantılar;
- `http` ya da `https` olmayan bağlantılar, örneğin `mailto:`;
- Yuva'nın kendi bağlantıları: abonelikten çıkma, tercihler ve tıklama takibi.

Bültenin seçeneklerinden ya da taslakta `"utm": false` ile (`PATCH /v1/broadcasts/{broadcastId}`) kapatın;
`utm_campaign` kampanya alanıdır. [Tıklama takibiyle](broadcasts.md#tıklama-takibi) ya da onsuz çalışır ve üçüncü bir
tarafa hiçbir şey göndermez: ziyareti kendi Analytics'iniz sayar. UTM bağlantıları gelmeden önce taslaktan çıkmış
bültenler bağlantılarını gönderildiği gibi korur.

Bültenin raporu bundan sonra web sitesi başına **bu kampanyadan gelen ziyaretleri** gösterir: `utm_campaign` değeri
bültenin kampanyası olan ziyaretçiler, ziyaretler ve sayfa görüntülemeleri; gönderimin başladığı günden itibaren,
görmeye yetkili olduğunuz web siteleri için (`analytics:read` ister; API'de `campaign_visits`).

## Bir asistana sorun

[MCP](mcp.md) ile bağlı bir yapay zekâ asistanı aynı rakamları okuyabilir. Örneğin:

> Bu ay en çok konuşmaya yol açan sayfalar hangileri?

Asistan `analytics_support_report` aracını çağırır ve sayfaları, konuşmalarını, görüntülemelerini ve oranını yanıtlar.
Araçlar, prompt'lar ve yetkiler [Analitiği bir asistanla kullanın](mcp.md#analitiği-bir-asistanla-kullanın) bölümünde.

## Başvuru

| Çağrı | Ne yapar |
|---|---|
| `POST /v1/analytics/properties/{propertyId}/query` | Rakamlar; yukarıdaki gibi `dataset: "support"` ve `insights: true`. Yetki `analytics:read`. |
| `GET /v1/analytics/properties/{propertyId}/alerts` | Web sitesinin tetiklediği uyarılar. Yetki `analytics:read`. |
| `GET`, `PUT /v1/analytics/properties/{propertyId}/subscription` | Benim rapor ve uyarı seçimlerim. Üye oturumu. |
| `POST /v1/analytics/properties/{propertyId}/subscription/send-report` | Bana hemen bir rapor gönderir. Üye oturumu. |
| `GET /v1/analytics/properties/{propertyId}/annotations` | Grafikteki notlar, uyarılar dahil. |
| `GET /v1/broadcasts/{broadcastId}/report` | `campaign_visits` ile bültenin raporu. |

Tüm alanlar [API sözleşmesindedir](../../openapi/openapi.yaml).
