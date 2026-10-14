# JavaScript'te API izleme

`useyuva` paketi, Node, Bun ya da Deno API'nizin yanıtladığı her isteği tek bir ara katmanla ve rota başına kod yazmadan
bildirir. Her çerçevenin yönlendiricisi hangi rotanın eşleştiğini söyler; böylece `/users/42`, `/users/{id}` olarak
sayılır. Bir API özelliği ve `analytics:write` kapsamlı bir anahtar gerekir: bkz.
[API özelliği oluşturun](analytics-api.md#api-özelliği-oluşturun). Anahtarı sunucuda tutun.

```sh
npm install useyuva
```

Bir izleyici (monitor) oluşturun ve çerçevenizin ara katmanına verin. Node 22 ya da üstü, Bun, Deno veya bir worker
çalışma zamanı gerekir; yalnızca `fetch` kullanılır.

```ts
// monitor.ts
import { createMonitor } from "useyuva/server"

export const monitor = createMonitor({
  server: "https://support.example.com",
  apiKey: process.env.YUVA_API_KEY!,
  propertyId: process.env.YUVA_API_PROPERTY!,
  service: "shop-api",
})
```

| Seçenek | Varsayılan | |
|---|---|---|
| `server`, `apiKey`, `propertyId` | zorunlu | Yuva adresiniz, anahtar ve API özelliğinin kimliği. |
| `environment` | `production` | `production`, `staging` gibi ortamları ayırır. |
| `service`, `release` | yok | İsteği karşılayan servis ve sürümü. Sürüm kabul edilir ama henüz gösterilmez. |
| `sampleRate` | her istek | `10`, başarılı isteklerin (durum 1 ile 399) onda birini gönderir; her biri on isteği temsil eder. Hatalar, istemci hataları ve durum `0` her zaman gönderilir. |
| `flushInterval` | 5000 | Gruplar arasındaki milisaniye. 500'lük dolu bir grup hemen gider. |
| `queueSize` | 50.000 | En çok bekleyen kayıt; fazlası atılır ve `onError`'a ulaşır. |
| `maxRetries`, `minBackoff` | 5, 1000 | Ağ hatası, `429` ya da `5xx` sonrası bir grubun yeniden deneme sayısı ve ilk bekleme (milisaniye); her denemede ikiye katlanır. Daha uzun bir `Retry-After` geçerlidir. |
| `onError` | yok | Bir `Error` ile çağrılır: Yuva'nın geçersiz diye dışarıda bıraktığı kayıt için `MonitorRecordError`, ya da bir grubun hatası. |
| `fetch` | global `fetch` | İstekleri gönderir. |

Kayıtlar gzip ile sıkıştırılarak gider. Her ara katman ayrıca şu fonksiyonları alır; her biri çerçevenin istek nesnesiyle
çağrılır:

| Seçenek | |
|---|---|
| `consumer(request)` | Kimin çağırdığı: bir API anahtarının adı ya da istemci kimliği. Asla bir kişinin adresi değil; e-posta adresi biçimindeki değer `(scrubbed)` olarak saklanır. |
| `apiVersion(request)` | İsteğin kullandığı API sürümü. |
| `clientIP(request)` | Çağıranın adresi; Yuva isteğin ülkesini sayabilsin diye. Gelişte kısaltılır ve asla saklanmaz. Verilmezse hiçbir adres gönderilmez. |

Bir ara katman yöntemi, eşleşen rotayı, durumu (yanıt gönderilmediyse `0`), süreyi, `Content-Length`'ten gövde
boyutlarını ve başlangıç zamanını kaydeder. Gövde, başlık, sorgu dizesi ve adres asla kaydedilmez (`clientIP`'in
döndürdüğü hariç). Hiçbir rotanın eşleşmediği istek yolu ile gönderilir; Yuva bu türden bir `404` ya da `405`'i
`(unmatched)` olarak sayar; bkz. [Rotalar](analytics-api.md#rotalar).

## Tek rota sözdizimi

Yönlendirici nasıl yazarsa yazsın rota aynı biçimde saklanır: `:id` ve `[id]`, `{id}` olur; her şeyi yakalayan
(catch-all) rota `{path...}` olur. Aynı API hangi çerçevede, [Go](analytics-api-go.md)'da ya da
[OpenTelemetry](analytics-api-otel.md) ile yazılırsa yazılsın aynı rotaları gösterir.

## Kayıtlar ne zaman gönderilir

- **Sunucusuz platformlar** (Cloudflare Workers, Vercel, Lambda ve benzerleri): ara katman her yanıttan sonra bir flush'ı
  isteğin `waitUntil`'ine verir; hiçbir şey aralığı beklemez.
- **Uzun süre çalışan sunucular**: kayıtlar `flushInterval` ile gruplanır ve sunucu kapanırken gönderilir. Süreç
  çıkmadan önce `await monitor.close()` çağırın, yoksa bekleyen kayıtlar kaybolur. srvx, Node, Bun ve Deno'da da
  `waitUntil` sunar; ama orada kuyruk yine aralıkla gruplar.

`monitor.record(record)` bir isteği ara katmanların yaptığı gibi kaydeder, `monitor.send(record)` olduğu gibi kuyruğa
alır.

## srvx

```ts
import { serve } from "srvx"
import { yuvaMonitor } from "useyuva/srvx"

serve({ plugins: [yuvaMonitor(monitor, { route: (request) => routeOf(request) })], fetch: app.fetch })
```

srvx'in yönlendiricisi yoktur; rota `route(request)`'in döndürdüğüdür, yoksa bir h3 uygulamasının isteğin bağlamına
koyduğu rotadır. `server.close()` flush eder.

## h3

```ts
import { H3 } from "h3"
import { defineYuvaMonitor } from "useyuva/h3"

const app = new H3().use(defineYuvaMonitor({ monitor })).get("/users/:id", getUser)
```

Diğer ara katmanlardan önce, ilk olarak kaydedin.

## Nitro ve Nuxt

İkisi de anahtarı genel yapılandırmadan değil, sunucu çalışma zamanı yapılandırmasından okur; böylece tarayıcıya
ulaşamaz. Yuva adresini ve iki değeri verin:

| | Nuxt | Nitro |
|---|---|---|
| Adres | `yuva.server` ya da `NUXT_YUVA_SERVER` | `NITRO_YUVA_SERVER` |
| Anahtar | `NUXT_YUVA_MONITOR_API_KEY` | `NITRO_YUVA_MONITOR_API_KEY` |
| Özellik | `NUXT_YUVA_MONITOR_PROPERTY_ID` | `NITRO_YUVA_MONITOR_PROPERTY_ID` |

Nuxt'ta modülü ekleyin ve adresi `nuxt.config` içinde verin:

```ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: { server: "https://support.example.com", monitor: { service: "shop-api" } },
})
```

Nitro'da `useyuva/nitro`'dan `yuva`'yı `modules`'a ekleyin: `yuva({ server, monitor: { service: "shop-api" } })`.
`monitor`; `apiKey`, `propertyId`, `environment`, `service`, `release`, `sampleRate` ve `flushInterval` alır; anahtar
ile özellik kimliği bunun yerine değişkenlerden gelebilir. İzleme, ikisi de derleme zamanında yapılandırmadan ya da
ortamdan biliniyorsa açılır; yalnızca derleme ortamında bulunan bir değer derlemeye yazılmaz, bu yüzden onu çalışma
zamanında da verin. Rotalar Nitro'nun yönlendiricisinden gelir. Genel dosyalar (public assets) için ve Nuxt'ta
`/_nuxt/` (derleme varlıkları dizini) için gelen istekler kaydedilmez; böylece varlık yolları asla rota olmaz. Kayıtlar,
platformda varsa `waitUntil` ile ve Nitro kapanırken gönderilir. Nitro v2 ve v3'te çalışır.

## Hono

```ts
import { Hono } from "hono"
import { yuvaMonitor } from "useyuva/hono"

const app = new Hono().use(yuvaMonitor(monitor)).get("/users/:id", getUser)
```

Rota, yanıtlayan işleyicinin yoludur; alt uygulamalar dahil. Cloudflare Workers'ta kayıtlar
`executionCtx.waitUntil` ile, `@hono/node-server` ile sunucu kapanırken gönderilir.

## Express

```ts
import { yuvaMonitor } from "useyuva/express"

app.use(yuvaMonitor(monitor))
app.get("/users/:id", getUser)
```

Rota, yönlendiricinin temel yolu ile rotanın yoludur. Yanıttan önce kapanan bağlantı durum `0` olur. HTTP sunucusunun
`close`'u flush eder.

## Fastify

```ts
import { yuvaMonitor } from "useyuva/fastify"

await app.register(yuvaMonitor(monitor))
app.get("/users/:id", getUser)
```

Uygulamanın her rotası kaydedilir, önekli eklentiler dahil. `app.close()` flush eder.

## Next.js route handler'ları

Bir route handler çalışma zamanında kalıbını bilmez; bu yüzden onu klasör gibi yazarak ikinci argüman olarak verirsiniz:
`[id]` ve `[...slug]`, `{id}` ve `{slug...}` olur; rota grupları ve slot'lar atlanır.

```ts
// app/users/[id]/route.ts
import { withYuvaMonitor } from "useyuva/next"
import { monitor } from "@/monitor"

export const GET = withYuvaMonitor(monitor, "/users/[id]", async (request, { params }) => {
  return Response.json(await findUser((await params).id))
})
```

`notFound()` ve `redirect()` kendi durumlarıyla sayılır. Platform Next.js'e bir `waitUntil` veriyorsa (Vercel) flush
`after()` ile gider; `next start` aralıkla gruplar.

## Sayıları okuma

Kayıtlar yaklaşık 15 saniye içinde bir sorguda görünür. Gürültülü bir rotayı gizleme, rota kuralları ve sorgu
[API izleme](analytics-api.md#rotaları-listeleme-ve-gizleme) sayfasındadır. Çerçevesiz kullanıyorsanız kayıtları
[curl](analytics-api.md#curl-ile-istek-gönderme) ya da `monitor.send` ile kendiniz gönderin.
