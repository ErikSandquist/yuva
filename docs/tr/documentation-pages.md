# Dokümantasyon sayfaları

Dokümantasyon sitenize "Bu sayfa işinize yaradı mı?" sorusunu ve herkese açık bir tartışma alanı
ekleyin. Sayfalarınız olduğu yerde kalır; iki web bileşeni değerlendirmeleri ve gizli geri
bildirimleri Yuva'nın gelen kutusuna gönderir ve sayfanın tartışmalarını gösterir: GitHub
Discussions'taki gibi yanıtları, oyları ve kabul edilmiş bir yanıtı olan başlıklar.

## 1. Bir sohbet kanalı

[Web widget'ı](widget.md#1-sohbet-kanalı-oluşturun) için olduğu gibi kurulmuş bir sohbet kanalı
kullanın; tek bir kanal ikisine de hizmet edebilir.

- Dokümantasyon sitesinin origin'ini kanalın **izin verilen kaynaklarına** ekleyin
  (`https://docs.example.com`, ayrıca `http://localhost:3000` gibi geliştirme sunucunuz). Başka bir
  origin'de öğeler boş kalır.
- Gelen kutusuna bir [e-posta kanalı](email.md) ekleyin: geri bildirimlere verilen yanıtlar
  okuyuculara e-postayla ulaşır.
- Tartışmaların nasıl işleyeceğini kanalın `discussions` ayarıyla seçin: `moderated` (varsayılan:
  bir okurun gönderileri, içlerinden biri onaylanana kadar ekibinizi bekler), `open` (hemen
  yayımlanır) ya da `off` (tartışma yok; öğe bunu söyler).

## 2. Öğeleri ekleyin

### Düz HTML

```html
<script src="https://support.example.com/yuva-docs.js" defer></script>

<yuva-page-feedback channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-page-feedback>
<yuva-discussions channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-discussions>
```

Bunları her doküman sayfasının paylaştığı şablona, makalenin arkasına koyun. `/yuva-docs.js`
dosyasını Yuva sunucunuz sunar.

npm'de `useyuva/docs`, `useyuva/react` ve `useyuva/nuxt` olarak bulunurlar (`useyuva` 0.0.7 ve sonrası).

### Nuxt Content

Çalıştırılabilir örnek: [`examples/nuxt-content`](../../examples/nuxt-content)
([nasıl çalıştırılır](../../examples/README.md#nuxt-content)). Modülü ekleyin:

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ["@nuxt/content", "useyuva/nuxt"],
  yuva: { server: "https://support.example.com", channel: "yuva_pk_xxxxxxxxxxxxxxxx" },
});
```

```vue
<!-- app/pages/[...slug].vue, <ContentRenderer> sonrasına -->
<YuvaPageFeedback :page-title="page.title" />
<YuvaDiscussions :page-title="page.title" />
```

`NUXT_PUBLIC_YUVA_SERVER` ve `NUXT_PUBLIC_YUVA_CHANNEL`, çalışma anında `yuva` değerlerinin yerine
geçer. Bileşenler sunucuda render edilir ve öğeleri tarayıcıda yükler; bu yüzden `nuxt generate` de
çalışır. Olaylar `@rating`, `@feedback`, `@discussion`, `@post` ve `@reader`'dır. Yeni bir sitede `nuxt build` "Nuxt Content
requires better-sqlite3" hatasıyla durur: paketi kurun ya da örnekteki gibi
`content.experimental.sqliteConnector: "native"` atayın. Sayfa bir `content` koleksiyonunu sorgular;
onu örneğin [`content.config.ts`](../../examples/nuxt-content/content.config.ts) dosyasındaki gibi
tanımlayın.

### Fumadocs

Çalıştırılabilir örnek: [`examples/fumadocs`](../../examples/fumadocs)
([nasıl çalıştırılır](../../examples/README.md#fumadocs)). Doküman sayfası bir server component
olduğu için React bileşenlerini bir client component içine sarın:

```tsx
// components/yuva.tsx
"use client";
import { YuvaDiscussions, YuvaPageFeedback } from "useyuva/react";

const channel = process.env.NEXT_PUBLIC_YUVA_CHANNEL!;
const server = process.env.NEXT_PUBLIC_YUVA_SERVER;

export const PageDiscussions = ({ title }: { title: string }) =>
  <YuvaDiscussions channel={channel} server={server} pageTitle={title} />;
export const PageFeedback = ({ title }: { title: string }) =>
  <YuvaPageFeedback channel={channel} server={server} pageTitle={title} />;
```

```tsx
// app/docs/[[...slug]]/page.tsx, <DocsBody> sonrasına
<PageFeedback title={page.data.title} />
<PageDiscussions title={page.data.title} />
```

Fumadocs'un kendi `<Feedback>` bileşenini eklediyseniz onu `<PageFeedback />` ile değiştirin ya da
tutup yalnızca `<PageDiscussions />` ekleyin. `useyuva` `sdk/js`'ten yol üzerinden bağlıysa Next,
React'in ikinci bir kopyasını yükleyebilir; örnekteki `next.config.mjs` gibi `turbopack.root`
atayın.

### shadcn/ui

[shadcn/ui](https://ui.shadcn.com) üzerine kurulu, tartışmaları kendi kodu olarak, kendi tasarım
sisteminde isteyen React siteleri için. Çalıştırılabilir örnek:
[`examples/shadcn-next`](../../examples/shadcn-next)
([nasıl çalıştırılır](../../examples/README.md#shadcn-next)).

```sh
npx shadcn@latest add https://useyuva.com/r/yuva-discussions.json https://useyuva.com/r/yuva-page-feedback.json
```

Bu, bileşenleri projenize kopyalar (`components/yuva-*.tsx`, `hooks/use-yuva-discussions.tsx`,
`lib/yuva-utils.ts`), kullandıkları shadcn/ui bileşenlerini ekler (button, card, input, tabs,
textarea, avatar, badge, separator, skeleton) ve `useyuva` ile `lucide-react`'i kurar. Daha küçük
öğeler `use-yuva-discussions` (yalnızca provider ve hook'lar), `yuva-discussion-list`,
`yuva-discussion-thread` ve `yuva-discussion-composer`; hepsi
`https://useyuva.com/r/registry.json`'da listelenir.

```tsx
// app/layout.tsx
import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions";

<YuvaDiscussionsProvider
  server={process.env.NEXT_PUBLIC_YUVA_SERVER}
  channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL}
>
  {children}
</YuvaDiscussionsProvider>
```

```tsx
// makalenin altında
<YuvaPageFeedback />
<YuvaDiscussions />
```

`YuvaDiscussions`; `page` ve `pageTitle` (varsayılan olarak sorgu ve parça olmadan URL ile
`document.title`), `defaultSort`, `labels` ve `className` alır; çevresinde provider yoksa `server`
ve `channel`'ı kendisine verin. Provider ayrıca `identityToken` (öğelerdeki gibi bir fonksiyon),
tarihler için `locale` ve kendi yaptığınız bir `createYuvaClient()`'ı paylaşmak için `client` alır.

- **Özelleştirmek** için dosyaları düzenleyin: artık sizin. Renkler temanızın değişkenlerinden
  gelir (`primary`, `muted`, `border`, `destructive` …), bu yüzden karanlık mod temanızı izler.
  Gönderiler, sunucunun `body_html`'ini `lib/yuva-utils.ts`'teki `yuvaProseClassName` sınıflarıyla
  gösterir; yazma alanının önizlemesi React öğelerinden oluşur, yayımlanan ise sunucunun ürettiğidir.
- **Kendi düzeniniz** için bileşenlerin arkasındaki hook'lar: `useDiscussions(page, sort)`,
  `useDiscussion(id)`, `useYuvaReader()` (`signIn()`'i bir tıklamadan çağırın),
  `useDiscussionMode()` ve `usePageRating(page)`.
- **Etiketler** varsayılan olarak İngilizcedir (`defaultYuvaDiscussionsLabels`). Değiştirmek ya da
  çevirmek için `labels`'ı provider'a ya da tek bir bileşene verin; sayılar fonksiyondur. Türkçe:

```tsx
import type { YuvaDiscussionsLabels } from "@/lib/yuva-utils";

const tr: Partial<YuvaDiscussionsLabels> = {
  heading: "Tartışmalar",
  sortNew: "Yeni",
  sortTop: "En çok oy alan",
  sortUnanswered: "Yanıtsız",
  newDiscussion: "Yeni tartışma",
  empty: "Bu sayfada henüz tartışma yok. İlkini siz başlatın.",
  replies: (count) => `${count} yanıt`,
  votes: (count) => `${count} oy`,
  answered: "Yanıtlandı",
  acceptedAnswer: "Kabul edilen yanıt",
  reply: "Yanıtla",
  signIn: "Giriş yap",
  signInHint: "Tartışma başlatmak, yanıtlamak ve oy vermek için giriş yapın.",
  write: "Yaz",
  preview: "Önizleme",
  startDiscussion: "Tartışmayı başlat",
  feedbackQuestion: "Bu sayfa işinize yaradı mı?",
  feedbackYes: "Evet",
  feedbackNo: "Hayır",
};

<YuvaDiscussionsProvider server={server} channel={channel} labels={tr} locale="tr">
```

Bileşenler mantıksal boşluklar kullanır (`ps-`, `border-s`, `text-start`), bu yüzden `dir="rtl"`
altında da çalışır. Öğeler gibi `useyuva` 0.0.7 ya da sonrasını isterler.

### Diğer site üreticileri

Betiği (ya da `useyuva/docs`'u) yükleyin ve iki öğeyi de makalenin altına koyun. `useyuva/docs` ve
`useyuva/react` sunucu tarafı render sırasında içe aktarılabilir; öğeleri yalnızca tarayıcıda
kaydederler. Starlight 0.42, VitePress 1.6 ve Docusaurus 3.10 ile denetlendi.

- **Starlight**: [`Footer`'ı override edin](https://starlight.astro.build/guides/overriding-components/): varsayılan `Footer`'ı, ardından iki öğeyi ve
  `<script>import "useyuva/docs";</script>`'i render edin. Astro özel öğeler için ayar istemez.
- **VitePress**: [`Layout`'u sarın](https://vitepress.dev/guide/extending-default-theme#layout-slots) ve iki öğeyi `doc-after` slot'una koyun;
  `.vitepress/theme/index.js` içinde `useyuva/docs`'u içe aktarın ve yapılandırmada `vue: { template: { compilerOptions: { isCustomElement: (tag) => tag.startsWith("yuva-") } } }` ayarlayın.
- **Docusaurus**: `DocItem/Footer`'ı [sarın](https://docusaurus.io/docs/swizzling#wrapping) (`npm run swizzle @docusaurus/theme-classic DocItem/Footer -- --wrap --danger`;
  CLI onu güvensiz sayar) ve ardından `useyuva/react`'tan `YuvaPageFeedback` ile `YuvaDiscussions`'ı render edin.
- **MkDocs, Hugo, Jekyll ve diğerleri**: [düz HTML](#düz-html) gibi.

## Okuyucular ve ekibiniz ne görür

- **Okuyucular** bir sayfayı Evet ya da Hayır ile değerlendirir (anonim bir sayaç), ardından geri
  bildirim yazabilir; geri bildirim gizli kalır ve yanıt için isteğe bağlı bir e-posta adresi alır.
- **Herkes** sayfanın tartışmalarını giriş yapmadan okur: Yeni, En çok oy alan ya da Yanıtsız
  olarak sıralanan başlıklar; her birinde açılış gönderisi, en üste sabitlenmiş kabul edilen yanıt,
  yanıtlar (yanıtlara bir düzey yanıt), oylar ve "Yanıtlandı" rozeti. Yazarlar yalnızca adları ve
  baş harfleriyle görünür; ekibinizin gönderilerinde "Ekip" rozeti vardır.
- **Katılmak** giriş ister. **Giriş yap**, Yuva sunucunuzda küçük bir pencere açar; okur orada
  e-posta koduyla ya da passkey ile giriş yapar. Hesap bu Yuva sunucusunu kullanan her sitede
  geçerlidir ve pencere sayfaya bu kanal için 24 saat geçerli bir token verir. Okurlar başlık açar,
  yanıt yazar, oy verir, bir başlığı takip eder (biri yanıt yazdığında ya da bir yanıt kabul
  edildiğinde e-posta), kendi başlıklarında bir yanıtı kabul eder, gönderilerini 30 dakika boyunca
  düzenler ve istedikleri zaman siler. Gönderiler önizlemeli Markdown'dır; sayfa, sunucunun bundan
  ürettiği, betik ve ham HTML içermeyen HTML'i gösterir. **Çıkış yap** yalnızca bu sitedeki
  token'ı sonlandırır.
- **Moderasyon**: `moderated` bir kanalda okurun ilk gönderileri kendisine "Onay bekliyor" olarak
  görünür ve ekibinizden biri birini onaylayana kadar diğer herkesten gizlenir; ondan sonra hemen
  yayımlanır. Bir gönderiyi spam olarak işaretlemek onu gizler ve o yazarın gönderilerini yeniden
  beklemeye alır. Kilitli bir başlık okurlardan yeni yanıt, oy ya da yanıt kabulü almaz.
- **Ekibiniz** geri bildirimleri gelen kutusunda `feedback` konuşmaları olarak alır, tartışmalarla
  panelde çalışır: site ve sayfaya göre başlıklar, onay bekleyen gönderilerin kuyruğu, ekip olarak
  yanıt yazma, yanıt kabul etme, başlıkları kilitleme ve yeniden adlandırma. Gizli bir konuşma,
  ekibinizin yazdığı metinle herkese açık bir başlığa dönüşebilir; konuşmanın başka hiçbir yeri
  yayımlanmaz. **Dokümanlar** görünümü sayfaları 7, 30 ya da 90 gündeki değerlendirmeleri, geri
  bildirimleri, açık ve yanıtsız başlıkları ve bekleyen gönderileriyle listeler.

## Bilmekte fayda var

- **Bir sayfa, sorgu ve fragment olmadan URL'sidir.** Dile göre ayrılan URL'ler
  (`/en/docs/install`, `/de/docs/install`) kendi değerlendirmeleri olan ayrı sayfalardır; birlikte
  saymak için `page` olarak tek bir URL verin ve `locale`'i sayfanın diline atayın.
- **Kendi girişiniz**: sitenizde hesaplar varsa öğelere `setIdentityToken` ile (React'te
  `identityToken` prop'u) bir [kimlik token'ı](identity.md) verin. Geri bildirim formunda e-posta
  alanı olmaz, yanıtlar token'daki adrese gider ve `<yuva-discussions>` giriş penceresi olmadan o
  kullanıcı olarak yazar. Kanalda **Anonim ziyaretçiler** açık değilse geri bildirim token ister;
  değerlendirmeler asla istemez.
- **Giriş penceresi**: tarayıcılar onu yalnızca bir tıklamayla açar; öğe de öyle açar.
  `Cross-Origin-Opener-Policy: same-origin` gönderen sayfalar pencereyle bağlantıyı keser; oralarda
  `same-origin-allow-popups` kullanın.
- **Content Security Policy**: Yuva sunucusuna `script-src` (`/yuva-docs.js` için) ve `connect-src`
  içinde izin verin.
- **Öğeler boş kalıyor**: origin'e izin verilmemiş ya da anahtar yanlış; tarayıcı konsoluna bakın.
- **Bir başlığa bağlantılar**: takipçilere giden e-postalar ve okurun hesap sayfası
  `<sayfa>#yuva-discussion-<id>` adresine bağlanır. Öğe o başlığı açar ve ona kaydırır, bir
  `hashchange` olduğunda da; listeye dönmek fragment'ı adresten kaldırır.
- **Önceki sorular**: yayımlanmış yanıtlar, sayfalarında, onları yayımlayan üyenin yanıtıyla
  tartışmalara dönüştü.

## Başvuru

### Nitelikler

| Nitelik | Anlamı |
|---|---|
| `channel` | Sohbet kanalının genel anahtarı (`yuva_pk_…`). Zorunlu. |
| `server` | Yuva sunucusunun adresi. Varsayılan, `yuva-docs.js`'nin yüklendiği origin. |
| `locale` | [47 dilden](widget.md#diller) biri; örneğin `en`, `tr`, `de`, `pt-BR` ya da `ar`. Varsayılan tarayıcının dili; Yuva'da olmayan bir dil İngilizceye döner. |
| `dir` | `ltr` ya da `rtl`. Varsayılan, dilin yazım yönü. |
| `identity-token` | İmzalı bir kimlik token'ı. Genellikle `setIdentityToken` daha iyidir. |
| `page` | Sayfanın URL'si. Varsayılan o anki URL; istemci tarafı gezinme izlenir. |
| `page-title` | Sayfanın başlığı. Varsayılan `document.title`. |

React sarmalayıcıları aynılarını prop olarak alır (`channel`, `server`, `locale`, `dir`, `page`,
`pageTitle`, `identityToken`), ayrıca `className` ve `style`.

### Özellikler ve metotlar

| Öğe | Üye | Anlamı |
|---|---|---|
| ikisi de | `channel`, `server`, `locale`, `page`, `pageTitle` | Özellik olarak nitelikler. |
| ikisi de | `setIdentityToken(fn)` | Token, oturum açmamış okuyucu için `null` döndüren bir fonksiyon. |
| `yuva-page-feedback` | `rating` | Bu tarayıcıda bu sayfa için `"up"`, `"down"` ya da `null`. |
| `yuva-page-feedback` | `rate("up" \| "down")` | Evet ya da Hayır'a basmakla aynı. |
| `yuva-discussions` | `discussions` | Listelenen başlıklar. |
| `yuva-discussions` | `reader` | Giriş yapmış okur (`{ token, expires_at, reader: { name, initials } }`) ya da `null`. |
| `yuva-discussions` | `reload()` | Listeyi yeniden getirir. |
| `yuva-discussions` | `open(id)` | Bir başlığı gösterir. |
| `yuva-discussions` | `signIn()`, `signOut()` | Düğmelerle aynı; `signIn()`'i bir tıklamadan çağırın. |

### Olaylar

Shadow DOM'dan dışarı kabarcıklanırlar (bubble), bu yüzden `document` üzerinde dinleyebilirsiniz.

| Olay | Öğe | `detail` | React prop'u |
|---|---|---|---|
| `yuva-rating` | `yuva-page-feedback` | `{ page, rating, previous }` | `onRating` |
| `yuva-feedback` | `yuva-page-feedback` | `{ page, rating, conversation_id }` | `onFeedback` |
| `yuva-discussion` | `yuva-discussions` | Okur bir başlık açtığında `{ page, discussion_id, state }` | `onDiscussion` |
| `yuva-post` | `yuva-discussions` | Okur yanıt yazdığında `{ page, discussion_id, post_id, state }` | `onPost` |
| `yuva-reader` | `yuva-discussions` | Girişten sonra `{ name, initials }`, çıkıştan sonra `null` | `onReader` |

### CSS değişkenleri

Öğeler `color` ve `font` değerlerini sayfadan alır ve koyu temaya uyar. Vurgu rengi kanalın düğme
renginden gelir; kendi renginizi kullanmak için `!important` ile atayın.

| Değişken | Kullanıldığı yer |
|---|---|
| `--yuva-accent` | Düğmeler, seçili değerlendirme ve sıralama, oylar, odak halkaları, gönderilerdeki bağlantılar, ekip avatarları. |
| `--yuva-on-accent` | Vurgu rengi üzerindeki metin. |
| `--yuva-border` | Düğmelerin, alanların, başlıkların ve gönderilerin kenarlıkları. |
| `--yuva-soft` | Seçili değerlendirmenin arka planı, üzerine gelinen başlıklar, gönderilerdeki kod. |
| `--yuva-danger` | Hata mesajları. |

### Uç noktalar

| Uç nokta | Kullanıldığı yer |
|---|---|
| `POST /client/v1/channels/{channel_key}/page-ratings` | Bir değerlendirmeyi sayar. |
| `POST /client/v1/feedback` | `page_url`, `page_title`, `rating` ile geri bildirim gönderir. |
| `GET /client/v1/channels/{channel_key}/discussions?page=&sort=` | Bir sayfanın başlıkları (token olmadan bir dakika önbelleğe alınır). |
| `GET /client/v1/channels/{channel_key}/discussions/{discussionId}` | Yanıtlarıyla bir başlık. |
| `POST /client/v1/discussions`, `POST /client/v1/discussions/{discussionId}/posts` | Başlık açar, yanıt yazar. |
| `PATCH`, `DELETE /client/v1/discussion-posts/{postId}`, `PUT`, `DELETE …/vote` | Düzenler, siler, oy verir. |
| `PUT`, `DELETE /client/v1/discussions/{discussionId}/answer`, `…/follow` | Yanıt kabul eder, takip eder. |
| `POST /v1/account/reader-tokens`, `DELETE /client/v1/reader-token` | Giriş penceresinin token'ı, çıkış. |
| `GET /v1/discussions`, `GET /v1/discussion-posts?state=pending` ve `/v1/discussions*`'ın geri kalanı | Ekibinizin görünümü ve moderasyon. |
| `POST /v1/conversations/{conversationId}/discussion` | Bir konuşmayı herkese açık bir başlığa dönüştürür. |
| `GET /v1/docs/sites`, `GET /v1/docs/pages`, `GET /v1/docs/summary`, `GET /v1/docs/page` | Dokümanlar görünümü. |
| `GET /v1/conversations?page=&kind=feedback` | Bir sayfanın geri bildirimleri. |

Ayrıntılar: [API sözleşmesi](../../openapi/openapi.yaml).
