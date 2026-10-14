# Çalışma süresi denetimleri

Çalışma süresi denetimi, Yuva sunucusundan sizin bir URL'nizi belirli aralıklarla çağırır ve beklediğiniz gibi yanıt
verip vermediğini, her adımın ne kadar sürdüğünü ve sertifikasının ne zaman sona erdiğini kaydeder. İstek kayıtlarının
söyleyemediği şey için kullanın: çalışmayan bir API istek almaz, dolayısıyla bunu hiçbir şey bildirmez. Bu sayfa
[API izleme](analytics-api.md) sayfasının üzerine kuruludur; bir denetim çöktüğünde [uyarılar](analytics-api-alerts.md)
sizi haberdar eder.

## Denetim ekleme

Sahipler ve yöneticiler ile gelen kutusu sınırı olmayan API anahtarları, `analytics:manage` yetkisiyle denetim oluşturur:

```sh
curl https://support.example.com/v1/monitoring/checks \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Shop API health",
    "url": "https://api.example.com/healthz",
    "property_id": "PROPERTY_ID"
  }'
```

Denetim hemen, sonra da aralığında çalışır. Denetimi oluşturan üye, kendisine gönderilen bir uyarı kuralı alır:
bkz. [varsayılan kurallar](analytics-api-alerts.md#varsayılan-kurallar).

| Alan | Varsayılan | Anlamı |
|---|---|---|
| `name` | zorunlu | 1 ile 100 karakter. |
| `url` | zorunlu | Bir host içeren, kimlik bilgisi ve parça içermeyen `http` ya da `https` URL'si, en çok 2.000 karakter. |
| `method` | `GET` | `GET` ya da `HEAD`. |
| `headers` | yok | En çok 5 istek başlığı, her biri `{ "name": "Authorization", "value": "Bearer …" }`. Aşağıya bakın. |
| `expected_statuses` | 200 ile 399 arası herhangi biri | Ayakta sayılan, 200 ile 399 arasında en çok 20 durum kodu. |
| `keyword` | yok | Yanıtın ilk 64 KiB'ının içermesi gereken metin; yalnızca `GET`. |
| `keyword_absent` | `false` | `keyword`'ü tersine çevirir: yanıt bunu içermemelidir. |
| `timeout_seconds` | `10` | 1 ile 30. |
| `interval_seconds` | `60`, ya da daha uzunsa sunucunun en küçük değeri | 30 ile 86.400 ve en az `YUVA_MONITOR_MIN_INTERVAL` (`422 interval_too_short`). |
| `failures_before_down` | `2` | Denetim çökmüş sayılmadan önce arka arkaya başarısız 1 ile 10 çalıştırma. |
| `cert_warning_days` | `14` | 1 ile 90. Sertifikası bu kadar gün içinde sona erecek bir `https` denetimi `certificate` uyarısı verir. |
| `max_redirects` | `5` | 0 ile 5. `0` ile yönlendirmenin kendi durumu değerlendirilir. |
| `service` | yok | Denetimin izlediği servis, kayıtlarınızın ve span'lerinizin verdiği adla. |
| `property_id` | yok | `uptime` sayıları ve raporları denetimi içeren API özelliği. |
| `paused` | `false` | Duraklatılmış denetim çalışmaz. |

`PATCH /v1/monitoring/checks/{checkId}` bunlardan herhangi birini değiştirir; denetim sonradan bütün olarak
doğrulanır. Değişen bir URL, method ya da aralık denetimi hemen bir sonraki çalıştırmaya alır, `paused: true` durdurur,
`paused: false` hemen çalıştırır. `DELETE` denetimi sonuçları, günlük satırları, uyarı kuralları ve uyarılarıyla birlikte
siler.

### Başlıklar

Başlıklar, denetimin `Authorization` başlığı ya da sağlık uç noktanızın istediği bir belirteci iletmesini sağlar.
Değerler sunucunun ana anahtarıyla mühürlenir ve yalnızca yazılabilir: API yalnızca `header_names` döndürür, hiçbir değeri
döndürmez. `headers` bir güncellemede tüm başlıkların yerine geçer, değerler dahil; bu yüzden listenin tamamını gönderin.
`Host`, `Cookie`, `Content-Length`, `Transfer-Encoding`, `Connection`, `Upgrade`, `TE`, `Trailer` ve `Proxy-*`
ayarlanamaz.

## Sunucunun neyi çağırıp neyi çağırmayacağı

Sunucu yalnızca genel adresleri çağırır. Bir URL; kimlik bilgisi, `localhost` ya da şu aralıklardan birinde bir adres
içeriyorsa `422 refused_address` ile reddedilir:

- loopback,
- özel (private),
- link-local,
- operatör düzeyinde NAT (CGNAT),
- multicast,
- ayrılmış (reserved),
- bulut metadata.

Bir host adı denetim kaydedilirken ve her çalıştırmada çözümlenir; çözümlediği adreslerden biri bunlardan biriyse
reddedilir. Her yönlendirme adımı aynı şekilde denetlenir ve bağlantı denetlenmiş adrese kurulur; böylece aramalar
arasında yanıtını değiştiren bir ad sunucuyu özel bir adrese götüremez. Sonraki bir çalıştırmada reddedilen denetim
`refused_address` hatasıyla başarısız olur.

Kendi sunucusunda iç servisleri izlemesi gereken bir kurulum `YUVA_MONITOR_ALLOW_PRIVATE=true` ayarlar
([Yapılandırma](configuration.md#api-izleme)). Bu, denetimlerin özel ve loopback adreslere ulaşmasına izin verir.
Link-local ve bulut metadata adresleri o zaman bile reddedilmeye devam eder.

Sunucu `User-Agent: Yuva-Uptime/<sürüm>` gönderir; çerez ve gövde göndermez, yanıttan en çok 64 KiB okur. Her bağlantı
denemesi, denetimin `timeout_seconds` değeri ne olursa olsun 10 saniye sonra vazgeçer.

## Sınırlar

| Sınır | Değer |
|---|---|
| Çalışma alanı başına denetim | `YUVA_MONITOR_CHECKS`: 500, açık kayıt olan sunucuda 50 (`403 check_limit`). |
| Tek bir host'taki denetim | 10 (`403 host_limit`). |
| En kısa aralık | `YUVA_MONITOR_MIN_INTERVAL`: 30 saniye, açık kayıt olan sunucuda 60. |
| Saklanan çalıştırmalar | `YUVA_MONITOR_CHECK_DAYS` gün, varsayılan 30; daha eski günler 400 gün boyunca günlük sayı olarak kalır. |

## Denetim çöktüğünde

Yanıt zamanında gelir, durumu `expected_statuses` içinde olur ve anahtar sözcük koşulu sağlanırsa çalıştırma başarılı
olur. Başarısız olan çalıştırma 20 saniye sonra yeniden denenir ve yalnızca bu ikinci deneme sayılır. Denetim, arka arkaya
`failures_before_down` sayılan çalıştırma başarısız olunca `down` olur, başarılı bir çalıştırmadan sonra yeniden `up`
olur. İlk sayılan çalıştırmasından önce `unknown` durumundadır.

Başarısız bir çalıştırma şu hatalardan birini taşır: `dns`, `refused_address`, `connect`, `tls`, `timeout`, `status`,
`keyword`, `redirects` ya da `other`.

**Bilinmeyen dakikalar.** Sunucuda bir dakikada en az 20 denetim çalışıp yarısından fazlası başarısız olduysa, sorun sizin
uç noktalarınızdan çok Yuva'nın kendi ağındadır. O dakikanın çalıştırmaları `unknown` kaydedilir: ne ayakta ne çökmüş
sayılır ve hiçbir çalışma süresi rakamında görünmez.

**Gecikme.** Bir dakika kapandıktan yaklaşık 40 saniye sonra değerlendirilir; bu yüzden durmuş bir uç nokta, varsayılan iki
başarısızlıkla 2 ile 3 dakika sonra çökmüş görünür. Henüz değerlendirilmemiş bir çalıştırmanın sonuçlarda durumu
`pending` olur.

## Sonuçları okuma

`GET /v1/monitoring/checks` denetimleri durumları, `state_since`, `next_run_at`, `cert_expires_at` ve son sonuçlarıyla
listeler. `GET /v1/monitoring/checks/{checkId}/results` sayılan çalıştırmaları en yenisi önce olmak üzere sayfa başına
en çok 500 olarak listeler (`limit`, sonraki sayfa için `before`). Her çalıştırmada şunlar vardır:

| Alan | Anlamı |
|---|---|
| `at` | Sayılan denemenin başladığı an. |
| `ok`, `state` | Başarılı olup olmadığı ve sonraki durum: `up`, `down`, `unknown` ya da `pending`. |
| `status` | Son yanıtın durumu; yanıt gelmediyse `0`. |
| `error`, `detail` | Neden başarısız olduğu. Ayrıntı sabit bir metindir; çözümlenen adresleri asla göstermez. |
| `retried` | İlk deneme başarısız oldu ve bu, 20 saniye sonraki denemedir. |
| `redirects` | İzlenen adım sayısı. |
| `timings` | `dns`, `connect`, `tls`, `ttfb` ve `total`, milisaniye olarak. |
| `cert_days_left` | `https`: sertifikanın sona ermesine kalan tam gün. |

### uptime veri kümesi

Zaman içindeki sayılar `dataset: uptime` ile sorgu işleminden gelir: `POST /v1/monitoring/query`, özelliği olmayanlar
dahil çalışma alanının her denetimini, `POST /v1/analytics/properties/{propertyId}/query` ise o özelliğin denetimlerini
okur. Boyut `check`'tir, bir zaman ayrıntısıyla birlikte; saklanan çalıştırmalar içinde `time:minute` ya da `time:hour`,
her aralık için `time:day`, `time:week` ya da `time:month` kullanın. Kovalar ve günler, özelliğin saat dilimi ne olursa
olsun UTC'dir.

| Metrik | Anlamı |
|---|---|
| `runs`, `failures` | Sayılan çalıştırmalar ve bunlardan başarısız olanlar. |
| `up` | Aralığın ya da kovanın son sayılan çalıştırması başarılı olan denetimler. |
| `uptime` | Denetimin çökmüş olmadığı izlenen sürenin yüzdesi; her çalıştırma kendi aralığını temsil eder. |
| `downtime` | Çökmüş olunan saniye. |
| `response_time_p50`, `response_time_p95`, `response_time_p99` | Yanıt alan çalıştırmaların toplam süresi, milisaniye olarak, %1,1 doğrulukla. |
| `avg_response_time`, `ttfb` | Ortalama toplam süre ve ilk bayta kadar geçen süre, milisaniye olarak. |
| `cert_days_left` | Bir sertifikanın sona ermesine kalan en az gün. |

Bilinmeyen çalıştırmalar bunların hiçbirinde sayılmaz. CSV dosyası için `POST /v1/monitoring/query` çağrısına
`?format=csv` ekleyin.
