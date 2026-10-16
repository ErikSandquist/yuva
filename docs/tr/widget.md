# Web widget'ı

`<yuva-chat>` web sitenize canlı sohbet, ya da kendi ürününüzün içine bir konuşma akışı ekler. Her
sayfada ve her framework'te çalışır.

## 1. Sohbet kanalı oluşturun

Panelde: Ayarlar → gelen kutusu → Müşteriler nereden yazıyor → Web sitesi sohbeti. Widget'ı
gösterebilecek her origin'i ekleyin, örneğin `https://www.example.com`, sonra kanalın genel
anahtarını (`yuva_pk_…`) kopyalayın. Anahtar gizli değildir.

## 2. Sayfaya ekleyin

```html
<script src="https://support.example.com/yuva.js" defer></script>
<yuva-chat channel="yuva_pk_xxxxxxxxxxxxxxxx"></yuva-chat>
```

Bu, köşede yüzen bir düğme gösterir. Bundler ile: `npm install useyuva`, `import "useyuva/chat"` ve
`server` niteliğini atayın. Nuxt'ta: `nuxt.config` içinde `modules: ["useyuva/nuxt"]`,
`yuva: { server, channel }` ve `<YuvaChat />`. Siteniz Content Security Policy gönderiyorsa Yuva
sunucunuza `script-src` ve `connect-src` içinde izin verin (`https://` ve `wss://`).

## 3. Oturum açmış kullanıcılar

Backend'iniz o anki kullanıcı için bir [kimlik token'ı](identity.md) imzalar; widget onu ister:

```js
document.querySelector("yuva-chat").setIdentityToken(async () => {
  const response = await fetch("/api/yuva-identity-token");
  return response.ok ? (await response.json()).token : null;
});
```

Anonim ziyaretçiler için `null` döndürün. Giriş yapıldıktan sonra yeniden çağırın, çıkış yapıldıktan
sonra `signOut()` çağırın.

## Widget neleri saklar

Widget oturumunu sayfanızın `localStorage` alanında tutar; böylece ziyaretçinin konuşması sayfa
yenilenince kaybolmaz. Her şey sayfanızın origin'ine bağlıdır.

| Anahtar | İçerik |
|---|---|
| `yuva:<channel>` | Oturum token'ı ve süresi, anonim ziyaretçi kimliği ve düğmenin durumu. |
| `yuva:reader:<server> <channel>` | Bir okuyucu dokümantasyon sayfasında oturum açtıktan sonra okuyucu token'ı. |

Token'lar rastgele metinlerdir; Yuva onları yalnızca özetlenmiş olarak saklar ve istediği an
sonlandırabilir. Depolama sizin origin'inize ait olduğundan sayfanıza enjekte edilen bir betik (XSS)
onları okuyabilir ve token'ın süresi dolana ya da sonlandırılana kadar o ziyaretçi gibi davranabilir.
Yapabilecekleriniz:

- **Oturumu kapatın.** `signOut()` oturumu sunucuda sonlandırır ve depolamadan siler; ziyaretçi kimliği
  ile düğme durumu kalır. `readerSignOut()` okuyucu token'ını sonlandırır ve anahtarını temizler.
  Kullanıcınız uygulamanızdan çıkarken ikisini de çağırın.
- **Oturum açmış kullanıcılar için kimlik token'ı kullanın.** Backend'inizin imzaladığı
  [kısa ömürlü bir token](identity.md) kullanıcının kim olduğunu kanıtlar; saklanan oturum tek güvence
  olmaz.
- **Content Security Policy gönderin** ve `script-src` değerini güvendiğiniz kaynaklarla sınırlayın;
  enjekte edilen betiklere karşı asıl korumadır.
- **Kendi arayüzünüzü yazıyorsanız token'ı `localStorage` dışında tutun.** `createYuvaClient()` bir
  `storage` seçeneği alır: `getItem` ve `setItem` olan herhangi bir nesne, örneğin `sessionStorage`'ı
  saran ya da değerleri bellekte tutan biri. `<yuva-chat>` öğesinde böyle bir seçenek yoktur ve her
  zaman `localStorage` kullanır; `localStorage` yoksa bellek kullanılır.

## Sorun giderme

- **Hiçbir şey görünmüyor**: sayfanın origin'ine izin verilmemiş (konsol adını yazar) ya da anahtar
  yanlış.
- **"Sign in to chat with us."**: kanal anonim ziyaretçilere kapalı ve token verilmemiş.

## Başvuru

### Nitelikler

| Nitelik | Anlamı |
|---|---|
| `channel` | Sohbet kanalının genel anahtarı. Zorunlu. |
| `server` | Yuva sunucusunun adresi. Varsayılan, `yuva.js`'nin yüklendiği yer. |
| `layout` | `launcher` (varsayılan) ya da sayfanızın içinde bir akış için `embedded`; ona bir yükseklik verin. |
| `locale` | `en`, `tr` ya da `de`. Varsayılan tarayıcının dili, o da yoksa İngilizce. |
| `dir` | `ltr` ya da `rtl`. Varsayılan, dilin yazım yönü. |
| `identity-token` | Bir token metni; genellikle `setIdentityToken` daha iyidir. |
| `open` | Varsa panel açıktır. |

### Metotlar ve olaylar

| Üye | Anlamı |
|---|---|
| `open()`, `close()`, `toggle()`, `isOpen` | Düğmenin açtığı panel. |
| `unread`, `yuva-unread` olayı | Henüz görülmemiş yanıtlar; olayda `detail.count` bulunur. |
| `setIdentityToken(fn)`, `signOut()` | Oturum açmış kullanıcılar, yukarıdaki gibi. |
| `rate(conversationId, "good" \| "bad", comment?)` | Kapalı bir konuşmayı değerlendirir; yapamadığında `409 already_rated` ya da `rating_unavailable`. |

### Kanal ayarları

| Ayar (API alanı) | Anlamı |
|---|---|
| İzin verilen kaynaklar (`allowed_origins`) | 1 ile 20 arası origin, birebir karşılaştırılır. |
| Anonim ziyaretçiler (`allow_anonymous`) | Kimlik token'ı olmayan ziyaretçiler sohbet edebilir. |
| E-posta adresi isteme (`ask_email_offline`) | Kimse müsait değilken adres ister. Okunmamış yanıtlar `YUVA_CHAT_EMAIL_DELAY` (varsayılan 5 dakika) sonra e-postayla gönderilir. Gelen kutusunda bir e-posta kanalı gerekir. |
| Karşılama mesajı (`greeting`) | İlk mesajdan önce gösterilir. |
| Düğme (`launcher`) | `position` ve `color` (`#rrggbb`). |

API üzerinden: sahip ya da yönetici olarak, `kind: "chat"` ve bir `chat` nesnesiyle
`POST /v1/inboxes/{inboxId}/channels`.
