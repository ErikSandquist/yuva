# OpenTelemetry ile API izleme

OpenTelemetry ile izlenen her servis, isteklerini bir API özelliğine bildirebilir: trace dışa aktarıcısını Yuva'ya
yöneltin, sunucu span'leri istek kaydı olur; servise Yuva kodu eklenmez. Yuva paketi olmayan her dil için yol budur.
Bir API özelliği ve `analytics:write` kapsamlı bir anahtar gerekir: bkz.
[API özelliği oluşturun](analytics-api.md#api-özelliği-oluşturun).

## Dışa aktarıcıyı kurma

İki ortam değişkeni yeter:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=https://support.example.com/otlp/PROPERTY_ID
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YUVA_API_KEY
```

Dışa aktarıcı `/v1/traces` kısmını kendisi ekler; trace'ler `POST /otlp/{propertyId}/v1/traces` adresine gider. Bunun
yerine `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` verirseniz `/v1/traces` ile biten tam adresi verin. Anahtar,
`analytics:write` kapsamlı bir API anahtarıdır; bir üyenin oturumu ve OAuth belirteçleri reddedilir (`403`).

- **Protokol.** İkili protobuf (`Content-Type: application/x-protobuf`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`) ve
  JSON (`application/json`, `http/json`) çalışır. gRPC çalışmaz. Başka her içerik türü `415` yanıtlar.
- **gzip.** Gövde `Content-Encoding: gzip` ile gzip'lenebilir (`OTEL_EXPORTER_OTLP_COMPRESSION=gzip`). Her iki durumda da
  bir istek en çok 8 MiB olabilir.
- **Örnekleme.** Her sunucu span'ini dışa aktarın: `OTEL_TRACES_SAMPLER=always_on` verin ya da örnekleyicinizi yalnızca
  diğer dışa aktarıcılar için tutun. Bkz. [Örnekleme](#örnekleme).

Servisi ve ortamını standart kaynak öznitelikleriyle adlandırın; örneğin `OTEL_SERVICE_NAME=shop-api` ve
`OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=production,service.version=1.4.2`.

## Neler okunur

Yalnızca `SERVER` türünde olan ve bir HTTP yöntemi taşıyan span'ler sayılır; her biri bir istektir. İstemci span'leri,
iç span'ler ve gRPC sunucuları reddedilmeden atlanır. Yuva bir span'in özniteliklerini güncel HTTP anlamsal
kurallarındaki adlarıyla, eksikse 1.20 ve öncesinin adlarıyla okur; yaygın olarak kullanılan birçok SDK ve otomatik
enstrümantasyon hâlâ bu eski adları gönderir.

| İstek kaydı | Öznitelik | Eksikse kullanılan eski ad |
|---|---|---|
| Yöntem | `http.request.method` | `http.method` |
| Rota | `http.route` | |
| Rota çıkarmak için yol | `url.path` | `http.target` (sorgusuz), `http.url`'in yolu |
| Durum | `http.response.status_code` | `http.status_code` |
| İstek gövdesi boyutu | `http.request.body.size` | `http.request_content_length` |
| Yanıt gövdesi boyutu | `http.response.body.size` | `http.response_content_length` |
| Çağıranın adresi, ülke için | `client.address` | `http.client_ip`, `net.sock.peer.addr`, `net.peer.ip` |
| Tüketici | `yuva.consumer` | |
| API sürümü | `yuva.api_version` | |

Span'in başlangıcı isteğin zamanı, bitişi ile başlangıcı arasındaki fark süredir. Durumu olmayan span durum `0` sayılır.
Kalanı kaynak (resource) verir: `service.name` servistir, `deployment.environment.name` (ya da eski
`deployment.environment`) ortamdır, hiçbiri yoksa `production`; `service.version` sürümdür.

Rota `http.route`'tan gelir. Bu olmayan span yoluyla sayılır; Yuva ondan [Rotalar](analytics-api.md#rotalar)
bölümündeki gibi bir rota çıkarır, bu yüzden enstrümantasyonun `http.route` vermesi iyi olur. Adres gelişte kısaltılır,
ülke için kullanılır ve atılır; sorgu dizesi, başlık ya da gövde okunmaz.

`yuva.consumer` ve `yuva.api_version` OpenTelemetry öznitelikleri değildir: onları sunucu span'ine kendiniz, örneğin bir
ara katmanda, API anahtarının ya da istemcinin adı ve isteğin API sürümüyle verin. Asla bir kişinin adresi değil.

```js
import { trace } from "@opentelemetry/api"

trace.getActiveSpan()?.setAttribute("yuva.consumer", apiKeyName)
trace.getActiveSpan()?.setAttribute("yuva.api_version", "2026-10")
```

Bir span, [curl](analytics-api.md#curl-ile-istek-gönderme) ile gönderilen bir kayıtla aynı denetimlerden geçer: rota,
tüketici, sürüm ve ortam sınırları, kaydın sayılmadığı 72 saat ve [aylık sınır](configuration.md#api-izleme) dahil.
OpenTelemetry'den gelen kayıtlar `otlp` kaynağıyla saklanır; SDK'lardan gelenlerle aynı rotaların altında toplanır.

## Örnekleme

Bir span, örneklemesinin söylediği kadar isteği temsil eder. Yuva, span'in `tracestate` değerindeki eşiği
(`ot=th:…`, OpenTelemetry olasılık örneklemesi) okur: `th:8` span'i 2, `th:c` 4 sayar; eşik yoksa 1 sayılır. Span'leri
eşiği yazmadan atan bir örnekleyici sayıları olması gerekenden düşük yapar. Bu yüzden her sunucu span'ini Yuva'ya
dışa aktarın, yalnızca diğer sistemlere gönderdiklerinizi örnekleyin. 10.000'de 1'den düşük bir eşik span'i geçersiz
yapar.

Doğrudan gönderilen bir kaydın `sample_rate` değerinden farklı olarak, örneklenmiş bir span hata durumu taşıyabilir;
çünkü baştan örnekleme yanıt bilinmeden önce karar verir.

## Elle deneme

Aynı adres OTLP/JSON alır; ilk deneme için yeter. Zamanlar 1970'ten beri nanosaniyedir ve yakın zamanda olmalıdır:

```sh
NOW=$(date +%s)000000000
curl https://support.example.com/otlp/PROPERTY_ID/v1/traces \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": { "attributes": [{ "key": "service.name", "value": { "stringValue": "shop-api" } }] },
      "scopeSpans": [{ "spans": [{
        "kind": 2,
        "startTimeUnixNano": "'$NOW'",
        "endTimeUnixNano": "'$((NOW + 12500000))'",
        "attributes": [
          { "key": "http.request.method", "value": { "stringValue": "GET" } },
          { "key": "http.route", "value": { "stringValue": "/users/{id}" } },
          { "key": "http.response.status_code", "value": { "intValue": "200" } }
        ]
      }] }]
    }]
  }'
```

```json
{}
```

## Ne döner

| Yanıt | |
|---|---|
| `200` | Span'ler alındı. Tüm span'ler geçerliyse JSON'da `{}` (protobuf'ta boş gövde). Aksi halde `partialSuccess`, geçersiz diye dışarıda bırakılanların sayısını (`rejectedSpans`) ve ilk nedeni (`errorMessage`) taşır. 72 saatten eski span'ler ve çalışma alanının aylık sınırını aşanlar alınır ama sayılmaz. |
| `429`, `503` | `Retry-After` ile: özelliğin ya da anahtarın `YUVA_MONITOR_RECORDS_PER_MINUTE` sınırı span'lerin hiçbirini almadı, ya da sunucu açabileceği kadar açık satır tutuyor. Dışa aktarıcı bunları kendiliğinden yeniden dener. |
| `400`, `401`, `403`, `404`, `415`, `422` | Reddedildi: biçimi bozan gövde, eksik ya da yanlış anahtar, `analytics:write` kapsamı olmayan anahtar ya da üye oturumu, bilinmeyen özellik, Yuva'nın okumadığı içerik türü ya da kodlama, web sitesi özelliği. |

Hata, isteğin kodlamasında bir `google.rpc.Status`'tur; mesajı sorun koduyla başlar, örneğin `insufficient_scope: …`.
Dışa aktarıcı bir reddi yeniden denemez; sayılar görünmezse onun günlüğüne bakın. Bir span yaklaşık 15 saniye içinde
sorguda görünür; bkz. [Sayıları okuma](analytics-api.md#sayıları-okuma).
