# Analitik

Yuva Analitik, kendi web sitelerinizin ziyaretçilerini, sayfalarını, kaynaklarını, konumlarını ve
cihazlarını çerezsiz ve IP adresi saklamadan sayar. Gelen kutunuzla aynı panelde durur, rakamlarına
[API](#api) üzerinden de ulaşılır.

## Ne sayar

| Yapar | Yapmaz |
|---|---|
| Sayfa görüntülemelerini, sayfanın ne kadar süre görünür kaldığını ve ne kadar kaydırıldığını sayar | Çerez koymaz, ziyaretçinin cihazına hiçbir şey yazmaz |
| IP adresini (IPv4 `/24`, IPv6 `/48`) başka hiçbir şey kullanmadan önce kısaltır, konumu bundan bulur ve unutur | IP adresi, hele tam bir adres, hiçbir yerde saklamaz |
| Bir ziyaretçiyi günde bir kez sayar: kısaltılmış adres ile tarayıcıdan, yalnızca bir gün var olan bir tuz ile üretilen bir karma değerinden | Bir ziyaretçiyi bir günden ötekine ya da bir siteden ötekine izlemez |
| Sayfanın alan adını ve yolunu, yönlendirenin alan adını ve yolunu ve `utm_*` parametrelerini tutar | Sorgu dizesinin kalanını ya da e-posta adresine benzeyen hiçbir şeyi tutmaz |
| Botları ve tarayıcısı Do Not Track ya da Global Privacy Control gönderen ziyaretçileri dışarıda bırakır | Bu ziyaretçileri hiç saymaz: hiçbir şey gönderilmez |

Yani **ziyaretçiler günlük sayılır**. Bir hafta için ziyaretçi sayısı her günün ziyaretçilerinin
toplamıdır: başka bir gün geri gelen kişi yeniden sayılır. Bir **ziyaret**, bir ziyaretçinin aynı gün
içinde, aralarında 30 dakikadan uzun boşluk olmayan sayfa görüntülemeleridir.

Betiğin bir cihaza yazdığı tek şey, ziyaretçinin kendi vazgeçme tercihidir
([aşağıda](#vazgeçme-bağlantıları)).

Nelerin ne kadar süre saklandığı, sitenizin çerez bandına ihtiyacı olup olmadığı ve gizlilik bildiriminiz için
metinler [Analitik ve gizlilik](analytics-privacy.md) sayfasındadır.

## Web sitesi ekleme

1. Panelde **Analitik → Web sitesi ekle** yolunu açın. Web sitesi ekleyebilenler sahipler ve yöneticilerdir.
2. Bir ad verin ve **Alan adları**nı her satıra bir tane olacak şekilde yazın. Başka alan adlarındaki
   sayfalar olay gönderemez. `*.example.com` tüm alt alan adlarını kapsar, `example.com`'un kendisini kapsamaz.
3. **Saat dilimi**ni (günler ona göre sayılır), rakamları kimlerin görebileceği **Gelen kutusu**nu (ya da
   hiçbiri; o zaman yalnızca sahipler ve yöneticiler) ve **Kitle**yi seçin. Çocuklar için yapılmış bir
   sitede Çocuklar'ı seçin: yalnızca ülke saklanır, bölge ya da şehir asla.
4. **Ekle ve kur**'a basın, sitenizin nasıl yapıldığına uyan kod parçasını (aşağıda) kopyalayıp
   sayfalarınıza koyun. İlk sayfa görüntülemesi geldiğinde sayfa yeşile döner. Do Not Track ya da Global
   Privacy Control açık bir tarayıcı hiçbir şey göndermez; denemek için başka bir tarayıcı kullanın.

Web sitesinin anahtarı (`yuva_ak_…`) sohbet anahtarı gibi herkese açıktır: her sayfada bulunur ve yalnızca
web sitesinin alan adlarında çalışır. Bir çalışma alanının web sitesi sayısı sınırlıdır; sınırı aşan istek
`403 property_limit` ile reddedilir.

## Kurulum

`https://support.example.com` yerine Yuva sunucunuzu, anahtar yerine web sitenizin anahtarını yazın. Panel
bu kod parçalarını kopyalamaya hazır olarak **Ayarlar → Kurulum** altında gösterir.

### Betik etiketi

```html
<script defer data-key="yuva_ak_xxxxxxxx" src="https://support.example.com/yuva-analytics.js"></script>
```

Her sayfanın `<head>` bölümüne koyun. Sayfa görüntülemelerini sayar, tek sayfalı bir uygulama adresi
değiştirdiğinde de; bu yüzden tek sayfalı uygulamada başka bir şey gerekmez. `YUVA_WIDGET=off` ya da
`YUVA_ANALYTICS=off` ile başlatılan bir sunucu betiği sunmaz.

| Nitelik | Anlamı |
|---|---|
| `data-key` | Web sitesinin anahtarı. Zorunlu. |
| `data-api` | Yuva sunucusunun adresi; betiğin yüklendiği yerden farklıysa ([kendi alan adınız](#kendi-alan-adınızdan-sunma)). |
| `data-hash` | Site `#`'den sonraki bölümle yönlendirme yapar: o bölüm sayfa yolunun parçası olur. |
| `data-manual` | Otomatik sayfa görüntülemesi yok; `yuva.pageview()`'u siz çağırırsınız. |
| `data-consent="required"` | `yuva.consent({ analytics: true })` çağrılana kadar hiçbir şey göndermez (izin modu, aşağıda). |

`data-manual` ile, bir sayfa gösterildiğinde `window.yuva.pageview()` çağırın; sayfayı kendiniz
adlandırmak için `window.yuva.pageview({ url, referrer })` kullanın.

### React ve Next.js

```sh
npm install useyuva
```

Bileşeni kök yerleşimde bir kez kullanın. Ekrana bir şey çizmez:

```tsx
import { YuvaAnalytics } from "useyuva/react"

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html>
      <body>
        {children}
        <YuvaAnalytics server="https://support.example.com" publicKey="yuva_ak_xxxxxxxx" />
      </body>
    </html>
  )
}
```

`hash`, `consent="required"` ve `manual` da birer prop'tur. `useYuvaAnalytics()`, altındaki her yerde
`pageview`, `consent`, `optOut` ve `optIn` yöntemlerini verir.

### Nuxt

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: {
    server: "https://support.example.com",
    analytics: { key: "yuva_ak_xxxxxxxx" },
  },
})
```

Modül bir istemci eklentisi ve `useYuvaAnalytics()` composable'ını ekler.
`NUXT_PUBLIC_YUVA_ANALYTICS_KEY` çalışma zamanında anahtarın yerine geçer.

### Vue, Astro ve diğer bundler'lar

```ts
import { createAnalytics } from "useyuva/analytics"

const analytics = createAnalytics({ server: "https://support.example.com", key: "yuva_ak_xxxxxxxx" })
```

Tarayıcıda bir kez çağırın: Vue'da `main.ts` içinde, Astro'da yerleşiminizin bir `<script>` bölümünde, ya da
giriş dosyanızda. Sunucu tarafında oluşturma sırasında hiçbir şey yapmaz. `pageview`, `consent`, `optOut`,
`optIn` ve `destroy` döndürür; `hash`, `consent: "required"` ve `manual` seçeneklerini de alır.

## İzin modu

Çerezsiz saymak çoğu gizlilik yasasında bir bant gerektirmez, ama bazı siteler önce sormak zorundadır. Onlar
için:

1. Betiği `data-consent="required"` ile kurun (React, Nuxt ve `createAnalytics`'te `consent="required"`).
   Ziyaretçinin kabul ettiğini söyleyene kadar hiçbir şey göndermez.
2. Kendi izin aracınızın geri çağrısında şunu çağırın. Ziyaretçi daha önce kabul ettiyse bunu **her sayfa
   yüklemesinde** yapın, çünkü betik yanıtı hatırlamaz:

   ```js
   window.yuva.consent({ analytics: true })
   ```

   Betik yüklenmeden önce çağrıyı kuyruğa alın:
   `;(window.yuvaq = window.yuvaq || []).push(["consent", { analytics: true }])`.
3. Web sitesinin **Ayarlar → Genel** bölümünde **Toplama**yı **Yalnızca izinle** yapın. Sunucu o zaman
   ziyaretçinin kabul ettiğini söylemeyen her olayı reddeder.

`yuva.consent({ analytics: false })` göndermeyi yeniden durdurur.

Kabul eden ziyaretçiler, cihazlarındaki en çok 13 aylık bir kimlikle günler boyunca da tanınabilir. Bu, özellik başına
isteğe bağlıdır: bkz. [İzinle açılan özellikler](analytics-consent.md).

## Vazgeçme bağlantıları

Şu iki bağlantıyı gizlilik sayfanıza koyun:

```html
<a href="#yuva-optout">Ziyaretlerimi sayma</a>
<a href="#yuva-optin">Ziyaretlerimi yeniden say</a>
```

Vazgeçme, betiğin ziyaretçinin cihazına yazdığı tek şeydir ve yalnızca ziyaretçi isterse yazılır.
`window.yuva.optOut()` ve `window.yuva.optIn()` da çağırabilirsiniz.

## Hiçbir şey göndermeyenler

Şunlar için betik hiçbir şey göndermez, sunucu da saymaz: Do Not Track ya da Global Privacy Control
kullanan tarayıcılar, otomatik tarayıcılar (`navigator.webdriver`), vazgeçmiş ziyaretçiler ve bilinen
botlar. **Ayarlar → İzleme** altında yazdığınız sayfalar ve adresler de (**Sayılmayan sayfalar**,
**Sayılmayan adresler**) dışarıda kalır; **Yol kuralları** bir kalıba uyan sayfaları tek bir yol altında
sayar, böylece bütün sipariş sayfaları `/orders/:id` olarak toplanır.

## Kendi alan adınızdan sunma

İçerik engelleyiciler başka alan adlarından gelen betikleri sık sık durdurur. Betiği ve uç noktasını
kendi sitenizden sunarsanız bu ziyaretçiler de sayılır. İki şeyi bilin:

- Tarayıcılar olayları betiğin sunucusuna göre `/client/v1/analytics/<anahtar>` adresine gönderir.
  `data-api` (ya da `server`) ile bu sunucuya `https://www.example.com/stats` gibi bir yol öneki verebilirsiniz.
- Yuva sunucunuz ziyaretçiyi değil, vekil sunucuyu görür. Vekil sunucunun ziyaretçinin adresini
  `X-Forwarded-For` başlığında göndermesini sağlayın; aşağıdaki örnekler bunu yapar. Yoksa her ziyaretçi
  vekil sunucu gibi görünür: hepsi tek kişi olarak sayılır ve adres başına dakikada 120 olay sınırı hepsine
  birlikte uygulanır.

**nginx**, sitenizin `server` bloğunda:

```nginx
location = /yuva-analytics.js {
  proxy_pass https://support.example.com/yuva-analytics.js;
  proxy_ssl_server_name on;
}
location /client/v1/analytics/ {
  proxy_pass https://support.example.com/client/v1/analytics/;
  proxy_ssl_server_name on;
  proxy_set_header X-Forwarded-For $remote_addr;
}
```

```html
<script defer data-key="yuva_ak_xxxxxxxx" src="/yuva-analytics.js"></script>
```

**Next.js**, sıradan görünen bir yolun altında: bir rewrite betiği sunar, bir route handler olayları ziyaretçinin
adresiyle iletir. Barındırma platformunuzun yazdığı adresi kullanın (`x-real-ip` ya da `x-forwarded-for`'un ilk
girdisi), ziyaretçinin yazabileceği bir adresi değil. `next.config.js` içinde:

```js
module.exports = {
  async rewrites() {
    return [{ source: "/stats/yuva-analytics.js", destination: "https://support.example.com/yuva-analytics.js" }]
  },
}
```

`app/stats/client/v1/analytics/[key]/route.ts` içinde:

```ts
export async function POST(request: Request, { params }: { params: Promise<{ key: string }> }) {
  const { key } = await params
  const visitor = request.headers.get("x-real-ip") ?? request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ?? ""
  const headers = new Headers({ "Content-Type": "text/plain", "X-Forwarded-For": visitor })
  for (const name of ["origin", "user-agent", "accept-language", "dnt", "sec-gpc"]) {
    const value = request.headers.get(name)
    if (value) headers.set(name, value)
  }
  const upstream = await fetch(`https://support.example.com/client/v1/analytics/${key}`, {
    method: "POST",
    headers,
    body: await request.text(),
  })
  return new Response(null, { status: upstream.status })
}
```

```html
<script defer data-key="yuva_ak_xxxxxxxx" data-api="https://www.example.com/stats" src="/stats/yuva-analytics.js"></script>
```

**Cloudflare Workers**, `www.example.com/stats/*` gibi bir rota ile:

```js
export default {
  async fetch(request) {
    const url = new URL(request.url)
    const target = new URL(url.pathname.slice("/stats".length) + url.search, "https://support.example.com")
    const headers = new Headers(request.headers)
    headers.set("X-Forwarded-For", request.headers.get("CF-Connecting-IP") ?? "")
    return fetch(target, { method: request.method, headers, body: request.body })
  },
}
```

Next.js'tekiyle aynı `<script>` etiketini kullanın. Her iki durumda da tarayıcının `Origin`'i web sitesinin
alan adlarından biri olmalıdır.

## Görünümler

Panelde **Analitik**'i açıp bir web sitesi seçin. Her görünüm seçtiğiniz aralığı gösterir (Bugün'den Son 12
ay'a kadar, ya da en fazla 400 günlük özel bir aralık) ve sekmeleri şunlardır:

| Görünüm | Gösterdiği |
|---|---|
| **Genel bakış** | Ziyaretçiler, ziyaretler, sayfa görüntüleme, ziyaret başına görüntüleme, hemen çıkma oranı ve ziyaret süresi zaman içinde (saatlik, günlük ya da haftalık), ve en çok ziyaret edilen sayfalar, kaynaklar, konumlar ve cihazlar. |
| **Canlı** | Son 30 dakikanın ziyaretçileri ve sayfa görüntülemeleri, dakika dakika; en çok ziyaret edilen sayfalar, kaynaklar ve ülkelerle. 15 saniyede bir yenilenir. |
| **Sayfalar** | Sayfalar, giriş sayfaları, çıkış sayfaları ve alan adları; sayfada geçen süre ve kaydırma derinliğiyle. |
| **Kaynaklar** | Kanallar, kaynaklar, yönlendirenler ve beş `utm_*` parametresi. |
| **Konumlar** | Ülkeler, bölgeler ve şehirler (Çocuklar kitlesinde yalnızca ülkeler). |
| **Cihazlar** | Cihaz, tarayıcı ve sürümü, işletim sistemi ve sürümü, ekran boyutu ve dil. |
| **Olaylar** | Özel ve yerleşik olaylar, özellikleri ve gelirleriyle. |
| **Hedefler** | Koyduğunuz hedeflerin dönüşümleri, dönüşüm oranı ve geliri. |
| **Dönüşüm hunileri** | Ziyaretçilerin bir hedef zincirinden nasıl geçtiği ve nerede ayrıldığı. |
| **Ayarlar** | Genel, İzleme, Kurulum, Durum ve Veriler. |

Olaylar, hedefler ve dönüşüm hunileri [Olaylar, hedefler ve dönüşüm hunileri](analytics-events.md) sayfasındadır.

Bir gelen kutusuna bağlı web sitesi ayrıca sayfalarının ne kadar destek doğurduğunu, öngörüler, uyarılar ve e-posta
raporlarıyla gösterir: [Destek ve trafik](analytics-support.md).

Aynı özellik iOS ve Android uygulamalarınızı ([Uygulamalarda analitik](analytics-apps.md)) ve arka ucunuzun
gönderdiği olayları ([Sunucu tarafı olaylar](analytics-server.md)) da sayar. `api` türündeki bir özellik ise kendi
API'nizin isteklerini sayar: [API izleme](analytics-api.md).

**Kanallar** bir ziyaretin nereden geldiğini ayırır: Doğrudan, Organik arama, Ücretli arama, Sosyal,
Ücretli sosyal, E-posta, Yönlendirme, **Yapay zekâ asistanları** (ChatGPT ve benzeri asistanlardan gelen
ziyaretler) ve Dahili.

Varsayılan bir sunucuda **Konumlar** DB-IP Lite veritabanından gelir. DB-IP, konumların gösterildiği her
yerde kendisine atıf yapılmasını ister; panel bunların altında “IP Geolocation by DB-IP” yazar.

### Filtreler, karşılaştırma ve CSV

- Bir tablonun satırını (bir sayfa, bir kaynak, bir ülke) seçerek **buna göre filtreleyin**; filtreler
  rakamların üstünde görünür, değiştirilebilir, kaldırılabilir ve birleşir. İşleçler: eşittir, eşit
  değildir, içerir, içermez, kalıba uyar (`*` yolun bir parçası, `**` istenen sayıda parça).
- **Karşılaştırma** aralığı önceki dönemle, bir yıl öncesiyle ya da seçtiğiniz başka günlerle karşılaştırır.
- **CSV indir**, gördüğünüz tabloyu her döküm ve metrik için bir sütunla kaydeder. Daha büyük dosyalar, günlük
  rakamlar ve ham olaylar [dışa aktarmalardır](analytics-sharing.md#dışa-aktarmalar); hesabı olmayanlara rapor ise
  bir [paylaşım bağlantısıdır](analytics-sharing.md#paylaşım-bağlantıları).
- Bazı filtre birleşimleri ayrıntılı kayıtlara ihtiyaç duyar; bunlar son
  [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analitik) gün boyunca (varsayılan 90) saklanır. Daha eski günler
  için yalnızca sayfa, kaynak, konum ve cihaz başına toplamlar vardır; panel bunu söyler.

### Durum

**Ayarlar → Durum**, son 30 gün için günlük sayılan ve atılan olayları ve nedenlerini (Botlar, Hız
sınırları, Başka siteler, Hariç tutulan, Aylık sınır, Geçersiz, Çok geç, Sunucu meşgul), son olayın ne zaman
geldiğini, ayrıntılı kayıtların hangi günden beri saklandığını ve hangi konum veritabanının kullanıldığını
gösterir. Atılan bir olaydan sayısı dışında hiçbir şey saklanmaz.

## Anahtarlar ve veriler

- **Yeni anahtar oluştur** (Ayarlar → Kurulum) web sitesine yeni bir anahtar verir. Eski anahtar 24 saat
  daha çalışır; böylece sayfalarınızı olay kaybetmeden güncelleyebilirsiniz. Bu süre içinde bir anahtar daha
  oluşturursanız eskisi hemen biter.
- **Günleri sil** (Ayarlar → Veriler) bir gün aralığının ziyaretlerini ve olaylarını siler; toplamlar bu
  günler olmadan yeniden hesaplanır. **Tüm verileri sil** web sitesini boşaltır ama web sitesi ve anahtarı
  kalır. **Mülkü sil** web sitesini, anahtarını ve tüm verilerini kaldırır. Hiçbiri geri alınamaz.
- Ayrıntılı kayıtlar [`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analitik) gün, günlük toplamlar 25 ay
  saklanır; API'de web sitesine `retention_months` verirseniz o kadar.

## Barındırılan sunucularda sınırlar

Kaydı herkese açık bir sunucuda her çalışma alanı ayda 1.000.000 olay sayabilir; operatör başka bir
[sınır](configuration.md#analitik) belirlemediyse. Sınırı aşan olaylar ay bitene kadar atılır ve panel bunu
söyler. Bir adres bir web sitesine dakikada en fazla 120 olay gönderebilir.

## API

Panelin her rakamı API'den gelir. Okumak için `analytics:read`, web sitelerini değiştirmek için
`analytics:manage` yetkisi gerekir; gelen kutularıyla sınırlı bir anahtar `analytics:manage` taşıyamaz ve bu
yetkiler var olmadan önce yapılmış anahtarlara verilmez ([yetkiler](headless.md#yetkiler)). Temsilciler
gelen kutularının web sitelerini okur. API, web sitesine mülk (property) der. `YUVA_ANALYTICS=off` ile
başlatılan bir sunucu bunların hepsine `404` döner.

| Çağrı | Ne yapar |
|---|---|
| `GET /v1/analytics/properties` | Görebildiğiniz mülkleri bugünkü ziyaretçileri, bu ayki kullanım ve sınırla listeler. |
| `POST /v1/analytics/properties` | Bir tane oluşturur. Yanıtta `public_key` bulunur. |
| `GET`, `PATCH`, `DELETE /v1/analytics/properties/{propertyId}` | Birini okur, değiştirir ve verileriyle siler. |
| `POST /v1/analytics/properties/{propertyId}/rotate-key` | Ona yeni bir anahtar verir; eskisi 24 saat daha çalışır. |
| `POST /v1/analytics/properties/{propertyId}/delete-data` | `from`–`to` günlerini ya da `all: true` ile her şeyi siler. `202` döner. |
| `POST /v1/analytics/properties/{propertyId}/query` | Rakamlar. `?format=csv` CSV döner. |
| `GET /v1/analytics/properties/{propertyId}/realtime` | Son 30 dakika. |
| `GET /v1/analytics/properties/{propertyId}/health` | Son `days` günün (1–90) toplama durumu. |
| `POST /client/v1/analytics/{key}` | Betiğin çağırdığı uç nokta: bir olay, ya da `{"e": [...]}` olarak en fazla 100 olay, en çok 64 KiB; `202` döner. Anahtar ya da çerez gerekmez. |

Metrikler: `visitors`, `visits`, `pageviews`, `views_per_visit`, `bounce_rate`, `visit_duration`,
`time_on_page`, `scroll_depth`, `events`. Boyutlar yukarıdaki görünümlerin sütunlarıdır, ayrıca `time:hour`,
`time:day`, `time:week` ve `time:month` zaman dilimleri vardır. Bir sorgu en fazla iki boyut, on filtre ve bir
karşılaştırma alır. Aşağıdaki, son 30 günün en çok ziyaret edilen on sayfasını önceki 30 günle birlikte ister:

```sh
curl -X POST https://support.example.com/v1/analytics/properties/$PROPERTY/query \
  -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  -d '{
    "metrics": ["visitors", "pageviews"],
    "dimensions": ["page"],
    "range": {"period": "30d"},
    "compare": {"with": "previous"},
    "limit": 10
  }'
```

Yanıtta `rows`, `totals`, `comparison` ve `meta` bulunur. Artık saklanmayan günler için ayrıntılı kayıt
gerektiren bir sorgu, sunabileceği ilk günle birlikte `422 needs_raw_window` döner; 400 günden uzun bir
aralık `422 range_too_large` döner. Bir çalışma alanında aynı anda dört sorgu çalışır; beşincisi iki saniye
bekler, sonra `429` alır. Tüm alanlar [API sözleşmesinde](../../openapi/openapi.yaml).
