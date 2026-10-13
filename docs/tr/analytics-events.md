# Olaylar, hedefler ve dönüşüm hunileri

Olaylar, ziyaretçilerin sayfa görüntülemenin dışında yaptığı şeylerdir: kayıt olmak, satın almak, bir dosya
indirmek, bir bağlantıya tıklamak. Bunları sayfalarınızdan ya da uygulamalarınızdan gönderin, önemli olanları
**hedef**e çevirip kaç ziyaretçinin oraya ulaştığını ve ne kadar gelir getirdiğini görün, hedefleri **dönüşüm
hunisi**nde zincirleyip ziyaretçilerin nerede ayrıldığına bakın. Bu sayfa [Analitik](analytics.md) üzerine
kuruludur; önce onu kurun.

## Olay gönderme

### Betikten

```js
window.yuva.track("Signup", { props: { plan: "pro", seats: 3 }, revenue: { amount: "29.90", currency: "EUR" } })
```

`track(ad, { props, revenue })` geçerli sayfada özel bir olay gönderir. Ad 1 ile 120 karakter arasındadır.
`pageview`, `engagement` ve `screen` ile `$` ya da `yuva_` ile başlayan adlar ayrılmıştır ve reddedilir. Olaylar,
sayfa görüntülemelerin uyduğu her şeye uyar: [izin modu](analytics.md), vazgeçme, Do Not Track, Global Privacy
Control ve otomatik tarayıcılar.

Betik `defer` ile yüklenir; bu yüzden kendi kodunuz erken çalışırsa `window.yuva` henüz yoktur. Çağrıyı kuyruğa
koyun: betik gelir gelmez çalışır, betik yüklendikten sonra da işe yarar:

```js
;(window.yuvaq = window.yuvaq || []).push(["track", "Signup", { props: { plan: "pro" } }])
```

`window.yuva.notFound()` geçerli sayfayı sitenizin 404 sayfası olarak işaretler ([örnekler aşağıda](#404-sayfaları)).

### React ve Nuxt'tan

React'te `<YuvaAnalytics>` bileşenini [Analitik](analytics.md) sayfasındaki gibi bir kez yerleştirin ve hook'u
altındaki her yerde kullanın. Betik yüklenmeden yapılan çağrılar onu bekler.

```tsx
import { useYuvaAnalytics } from "useyuva/react"

function UpgradeButton() {
  const { track } = useYuvaAnalytics()
  return (
    <button onClick={() => track("Upgrade", { revenue: { amount: "49.00", currency: "USD" } })}>Upgrade</button>
  )
}
```

`<YuvaAnalytics>` şu prop'ları alır: `server`, `publicKey` (React `key` adını kendine ayırdığı için `key`
değil), `hash`, `consent`, `manual`, `outbound`, `files` ve `chat`; son üçü [aşağıdaki
nitelikler](#bağlantılar-indirmeler-ve-sohbet) ile aynıdır. `useYuvaAnalytics()` `pageview`, `track`,
`notFound`, `consent`, `optOut` ve `optIn` döndürür.

Nuxt'ta `yuva.analytics` altındaki modül seçenekleri `key`, `server`, `hash`, `consent`, `manual`, `outbound`,
`files` ve `chat`'tir; `useYuvaAnalytics()` aynı yöntemlerle kendiliğinden içe aktarılır:

```ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: {
    server: "https://support.example.com",
    analytics: { key: "yuva_ak_xxxxxxxx", outbound: true, files: true },
  },
})
```

```vue
<script setup>
const { track } = useYuvaAnalytics()
</script>
```

Vue, Astro ve diğer bundler'larda `useyuva/analytics` içindeki `createAnalytics()` da `track` ve `notFound`
döndürür.

### Bağlantılar, indirmeler ve sohbet

Betik etiketinin üç niteliği (ya da aynı adlı prop ve seçenekler) kod yazmadan olay gönderir. Siz ekleyene kadar
kapalıdırlar:

| Nitelik | Seçenek | Gönderdiği |
|---|---|---|
| `data-outbound` | `outbound: true` | Ziyaretçi başka bir alan adına giden bir bağlantıya tıklayınca `outbound_click`, `url` özelliğiyle. |
| `data-files` | `files: true` | Ziyaretçi bir pdf, zip, dmg, exe, apk, csv, xls, xlsx, doc, docx, ppt, pptx, mp3 ya da mp4 dosyasına giden bir bağlantıya tıklayınca `file_download`, `url` özelliğiyle. |
| `data-chat` | `chat: true` | Ziyaretçi sayfadaki bir [`<yuva-chat>`](widget.md) başlatıcısını açınca `chat_open`. |

```html
<script defer data-key="yuva_ak_xxxxxxxx" data-outbound data-files data-chat
  src="https://support.example.com/yuva-analytics.js"></script>
```

`url`, sorgu dizesi ve parça olmadan, yalnızca `http` ve `https` bağlantıları için saklanır. `layout="embedded"`
ile gösterilen sohbet sayfanızın içinde durur ve hiç "açılmaz"; bu yüzden `chat_open` göndermez.

`not_found` (`notFound()` çağrısından) dördüncü yerleşik olaydır. Hepsi: `outbound_click`, `file_download`,
`not_found` ve `chat_open`. Olaylar görünümü bunları Dış bağlantı, Dosya indirme, Sayfa bulunamadı ve Sohbet
açıldı diye adlandırır.

## Özellikler (prop)

Özellikler bir olaya ayrıntı ekler: seçilen plan, tıklanan düğme, kullanılan form.

| Kural | |
|---|---|
| Sayı | Olay başına en fazla 30. |
| Anahtarlar | 1 ile 300 karakter; `$` ya da `yuva_` ile başlamaz. |
| Değerler | Metin, sayı ya da doğru/yanlış; en fazla 2.000 karakterlik metin olarak saklanır. |
| E-posta adresleri | E-posta adresine benzeyen bir değer saklanmaz, atılır. Olayın kendisi kalır ve **Ayarlar → Durum** bunu "adres çıkarıldı" olarak sayar. |
| Çocuklara yönelik web siteleri | `outbound_click` ve `file_download` olaylarının `url` değeri dışında hiçbir özellik saklanmaz. |

Bir kuralı çiğneyen olay (çok fazla özellik, ayrılmış anahtar, liste ya da nesne olan değer) `400` ile reddedilir;
aynı istekte gönderilen diğer olaylar da öyle. Yeni olayları tarayıcınızın ağ sekmesinde deneyin.

Herhangi bir adın özellikleri, ayrıntılı kayıtların tutulduğu günler için [Olaylar görünümünde](#olaylar-görünümü)
kırılımlanabilir ([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analitik), varsayılan 90). **Ayarlar → İzleme →
İzlenen özellikler** altında listelediğiniz bir özellik (en fazla 10; çocuklara yönelik sitede olmaz) günlük
toplamlarda da tutulur; kırılımı ayrıntılı kayıtlar silindikten sonra da çalışır. Eklediğiniz özellik,
ayrıntılı kayıtların tutulduğu günler boyunca yeniden sayılır; daha eski günlerde olmaz, bu yüzden aylar boyunca
karşılaştırmak isteyeceğiniz özellikleri erken ekleyin.

## Gelir

Bir olay kazandırdığı parayı taşıyabilir:

```js
track("Purchase", { revenue: { amount: "29.90", currency: "EUR" } })
```

- `amount` `"29.90"` gibi ondalık bir metindir (sayı da olur), `currency` ISO 4217 kodudur.
- Yuva tutarı para biriminin **en küçük birimiyle** saklar: 29,90 EUR için 2990, 1500 JPY için 1500. Panel ve
  API (gelir yanıtındaki `digits`) bunu geri çevirir.
- **Para birimleri asla çevrilmez ya da toplanmaz.** EUR ve USD ile satan bir site her para birimi için ayrı bir
  gelir rakamı görür.
- Tutarın ondalık basamağı para biriminin izin verdiğinden fazla olamaz: EUR için iki, JPY ve KRW için sıfır,
  KWD için üç. EUR'da `"29.999"` `400` ile reddedilir; aynı istekteki diğer olaylar da.
- Yalnızca özel ve yerleşik olaylar gelir taşır. Sayfa görüntülemede gelir reddedilir.

Geliri hedefler ve Olaylar görünümü raporlar; sayfa görüntüleme rakamları asla.

## Olaylar görünümü

**Analitik → Olaylar** her olay adını, seçilen aralıktaki ziyaretçi, ziyaret, olay ve geliriyle listeler. Bir
olayı seçip özelliklerini kırılımlayın: bir özellik seçin ya da herhangi bir özellik adını yazın; taşıdığı
değerleri sayılarıyla görün. Bir değeri seçerseniz yalnızca o değerli ziyaretler kalır. İzlenen özellikler
çip olarak görünür.

Anahtarınızın yazılı olduğu bir kod parçası için en alttaki **Olay gönderme** bağlantısını kullanın.

## Hedefler

Hedef, ziyaretçilerin yapmasını istediğiniz bir şeyi adlandırır. **Analitik → Hedefler** hedefleri dönüşümleriyle
listeler; hedefi sahipler ve yöneticiler yapar (**Yeni hedef**). Bir hedef şunlardan biridir:

| Tür | Ziyaretçi şunu yapınca ulaşılır |
|---|---|
| Sayfa görüntüleme | Yolu desene uyan bir sayfayı görüntüler. `*` yolun bir parçası, `**` istenen sayıda parça yerine geçer: `/thanks`, `/blog/**`. |
| Olay | Olayı gönderir; **Yalnızca şu özellikle** seçtiyseniz o özellik değere eşitken: `plan` değeri `pro` olan `Signup`. |
| Kaydırma derinliği | Yolu desene uyan bir sayfada en az belirli bir yüzdeye (1 ile 100) kadar kaydırır. |

Bir hedef, aralık için şunları raporlar:

| Rakam | Anlamı |
|---|---|
| **Dönüşümler** | Hedefe ulaşan ziyaretçiler, günlük, toplanmış. |
| **Dönüşüm oranı** | 100 ziyaretçi başına dönüşüm. |
| **Tamamlanmalar** | Hedefe kaç kez ulaşıldığı; bir ziyaretçi birkaç kez ulaşabilir. |
| **Gelir** | Hedefin olaylarının taşıdığı, para birimi başına; ödeme sayısı ve ortalamasıyla. Ayrı bir gelir hedefi yoktur: olay hedefi, olaylarının taşıdığı geliri gösterir. |

Eğilimini ve dönüşümlerin nereden geldiğini görmek için bir hedef seçin: kanallar, kaynaklar, giriş sayfaları,
ülkeler ve cihazlar.

Bir web sitesinin en fazla 100 hedefi olur; adlar benzersizdir ve en fazla 120 karakterdir.

**Hedefin hangi günden itibaren rakamı olur.** Yeni bir hedef yalnızca bugünden değil, ayrıntılı kayıtların
tutulduğu tüm günler boyunca (varsayılan 90) sayılır. Daha eski günlerde rakam yoktur ve hedef "Bu hedefin
rakamları şu günden itibaren var" der. Tanımını değiştirdiğiniz hedef o günler boyunca aynı şekilde yeniden
sayılır; daha eski günler önceki sayımlarını korur. Bir dönüşüm hunisinin adımı olan hedef, huniden çıkarılana
kadar silinemez. Hedef yapmak grafiklerinize bir not ekler.

## Dönüşüm hunileri

Dönüşüm hunisi, kaç ziyaretçinin bir hedef zincirinden sırayla geçtiğini ve nerede ayrıldığını gösterir.
**Analitik → Dönüşüm hunileri** onları listeler; sahipler ve yöneticiler iki ile sekiz hedeften, ziyaretçilerin
ulaştığı sırayla huni yapar (**Yeni huni**). Bir web sitesinde en fazla 20 huni olur; adlar benzersizdir.

Rapor her adım için, öncesindeki her adımdan sonra o adımı tamamlayan ziyaretçileri, önceki adıma göre kaybı,
ilk adımın ziyaretçilerine oranı ve önceki adımın ziyaretçilerine oranı gösterir.

Huni nasıl sayılır:

- **Tek ziyaret içinde.** Ziyaretçi 3. adım için ancak aynı ziyarette önce 1. ve 2. adıma ulaştıysa sayılır.
  Ziyaretçi yalnızca bir gün var olduğundan huni gece yarısını aşmaz; bir aralıktaki ziyaretçiler günlük
  toplanır.
- **Ayrıntılı kayıtlardan.** Huni her ziyaretin adımlarını gerektirir; bu yüzden yalnızca ayrıntılı kayıtların
  tutulduğu günlerde ([`YUVA_ANALYTICS_RAW_DAYS`](configuration.md#analitik)) çalışır. Daha erken başlayan aralık
  `422 needs_raw_window` yanıtı alır; panel sunabileceği ilk günden göstermeyi önerir.
- Bir rapor **en fazla 90 gün**dür; son 30 günde 10 milyondan fazla olay sayan bir web sitesinde 31 gün. Daha
  uzun aralık `422 range_too_large` yanıtı alır.
- Yanıtlar on dakika önbellekte tutulur.

## Segmentler

Filtreler her görünümü bazı ziyaretçilere daraltır: tek bir ülke, tek bir kanal, belirli bir olayı olan
ziyaretler. Bir filtre kümesini yeniden kullanmak için rakamların üstündeki **Segment**'i açıp **Filtreleri yeni
segment olarak kaydet**'i seçin. Sonra bir segment seçerseniz filtreleri (en fazla 10) görünüme uygulanır.
Segmentler web sitesini görenlerin hepsiyle paylaşılır ve adları benzersizdir; bir web sitesinde en fazla 100
olur. Web sitesini gören herkes segment yapabilir. Yazarı, sahipler ve yöneticiler onu değiştirebilir ya da
silebilir; segmenti silmek hiçbir rakamı silmez.

## Grafiklerde notlar

Not, grafiklerde bir günü ya da gün aralığını işaretler: bir lansman ya da kampanya gibi. Genel bakış
grafiğindeki **Not ekle** ile, en fazla 300 karakterlik bir metin ve ilk ile son günle eklenir. Web sitesini gören
herkes notu görür. Web sitesini gören herkes not ekleyebilir (web sitesi başına en fazla 1.000); yazarı, sahipler
ve yöneticiler düzenleyebilir ya da silebilir.

Yuva da kendi notlarını ekler: web sitesinin yeni anahtarı ve yaptığınız her hedef için. Bunları yalnızca sahipler
ve yöneticiler silebilir; kimse düzenleyemez.

## 404 sayfaları

Eksik sayfalar için gösterdiğiniz sayfada `notFound()` çağrısını bir kez yapın. Ziyaretçinin istediği adres için
`not_found` gönderir; ziyaretçi yalnızca bunu yaptıysa ziyaret yine hemen çıkma sayılır. Sonra `not_found`,
Olaylar görünümünde diğer olaylar gibi görünür ve bir hedef onu sayabilir.

**Next.js**, `app/not-found.tsx` içinde (`pages/404.tsx` de aynı şekilde), kök düzende `<YuvaAnalytics>` ile:

```tsx
"use client"

import { useEffect } from "react"
import { useYuvaAnalytics } from "useyuva/react"

export default function NotFound() {
  const { notFound } = useYuvaAnalytics()
  useEffect(() => notFound(), [notFound])
  return <h1>Sayfa bulunamadı</h1>
}
```

**Nuxt**, `error.vue` içinde, `nuxt.config`'te `analytics` ayarlıyken:

```vue
<script setup>
const props = defineProps({ error: Object })
const { notFound } = useYuvaAnalytics()
onMounted(() => props.error?.statusCode === 404 && notFound())
</script>

<template>
  <h1>{{ error?.statusCode === 404 ? "Sayfa bulunamadı" : "Bir şeyler ters gitti" }}</h1>
</template>
```

**Astro**, `src/pages/404.astro` içinde, betik etiketi düzeninizde dururken. Kuyruk, betik yüklenmiş olsun ya da
olmasın çalışır:

```astro
<Layout>
  <h1>Sayfa bulunamadı</h1>
  <script is:inline>
    ;(window.yuvaq = window.yuvaq || []).push(["notFound"])
  </script>
</Layout>
```

Statik bir sunucuda 404 sayfası ziyaretçinin istediği adreste sunulur; bu yüzden sayfa görüntüleme ve olay o
adresi taşır.

## API

Panel bu çağrıları kullanır. Okumak için `analytics:read` kapsamı gerekir; hedefleri ve dönüşüm hunilerini
yapmak ve değiştirmek `analytics:manage` ile bir sahip, yönetici ya da gelen kutusuyla sınırlı olmayan bir
anahtar gerektirir. Segmentleri ve notları web sitesini gören herkes `analytics:read` ile yazar.

| Çağrı | Yaptığı |
|---|---|
| `POST /client/v1/analytics/{key}` | Betiğin çağırdığı. Bir olay `{"n": "Signup", "u": "https://www.example.com/pricing", "p": {"plan": "pro"}, "$": {"a": "29.90", "c": "EUR"}}` biçimindedir. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/goals` | Hedefleri listeler ve yapar. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/goals/{goalId}` | Hedefi değiştirir ve siler. Bir huni adımı olduğu sürece `409 goal_in_use`. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/funnels` | Dönüşüm hunilerini listeler ve yapar. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/funnels/{funnelId}` | Huniyi değiştirir ve siler. |
| `GET /v1/analytics/properties/{propertyId}/funnels/{funnelId}/report` | Bir `period` ya da `from` ve `to` için adımlar. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/segments` | Segmentleri listeler ve kaydeder. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/segments/{segmentId}` | Segmenti yeniden adlandırır, değiştirir ve siler. |
| `GET`, `POST /v1/analytics/properties/{propertyId}/annotations` | Notları listeler (`from`, `to`) ve ekler. |
| `PATCH`, `DELETE /v1/analytics/properties/{propertyId}/annotations/{annotationId}` | Notu değiştirir ve siler. Yuva'nın kendi notları için `403 annotation_read_only`. |
| `POST /v1/analytics/properties/{propertyId}/query` | Rakamlar. `conversions`, `conversion_rate`, `completions` ve `revenue` için `goal_id` ekleyin; `event` ya da `prop` ile kırılımlayın (`prop` anahtarı adlandırır); segmenti uygulamak için `segment_id` ekleyin. |

Sınırlar `403` ile `goal_limit`, `funnel_limit`, `segment_limit` ya da `annotation_limit` yanıtı verir; başka bir
hedef, huni ya da segmentin aldığı ad `409` ile `goal_taken`, `funnel_taken` ya da `segment_taken` yanıtı verir.
Tüm alanlar [API sözleşmesinde](../../openapi/openapi.yaml).
