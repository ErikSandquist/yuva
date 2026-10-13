# İzinle açılan özellikler

Yuva Analytics varsayılan olarak bir ziyaretçiyi bir günden ötekine tanıyamaz ([Analitik ve gizlilik](analytics-privacy.md)).
İzinle açılan özellikler bunu, kabul eden ziyaretçiler için değiştirir: evet dedikleri anda siteniz ya da uygulamanız
cihazlarında rastgele bir kimlik tutar; siz de geri dönen ziyaretçileri, elde tutmayı, birkaç güne yayılan hunileri
ve oturum açmış kullanıcılar için kişi sayfasında son ziyaretlerini görürsünüz. Kabul etmeyen herkes eskisi gibi sayılır.

Bu sayfa [Analitik](analytics.md) ve onun [izin modu](analytics.md#i̇zin-modu) üzerine kuruludur. İzin aracını siz getirirsiniz;
Yuva hiçbir zaman bant göstermez.

## İzinle ne değişir

| `persistent` izniyle | Ne elde edersiniz |
|---|---|
| Cihazda rastgele bir kimlik | Betik ya da uygulama üretir, üretildiği andan itibaren en çok 13 ay saklanır ve her olayla gönderilir. Sunucu yalnızca, özelliğe ait bir sırla alınmış özetini saklar; bu yüzden aynı kimlik başka bir özellikte hiçbir şey ifade etmez. |
| Ziyaretçi bir kez sayılır | Bir ziyaretçi, günde bir kez yerine bir aralıkta, haftada ya da ayda bir kez sayılır. |
| Yeni ve geri dönen | **Ziyaretçi türü** boyutu (`visitor_type`): kimliğin ilk görüldüğü gün `new`, sonrası `returning`. Diğer herkes için boş. |
| Elde tutma | `GET /v1/analytics/properties/{propertyId}/retention`: gün, hafta ya da ay kohortları ve her birinden kaçının geri geldiği. |
| Günlere yayılan huniler | Hunideki `consented_visitors`, bu ziyaretçileri ziyaretler ve günler boyunca izler; her biri bir kez sayılır. |
| `identify` de açıksa | Kişinin son ziyaretleri: `GET /v1/contacts/{contactId}/visits` ve kişi sayfasında. |

Bu görünümlerde yalnızca izin veren ziyaretçiler vardır. Özelliği kullanmayan bir özellikte elde tutma kohortu olmaz.

### İzin olmadan hiçbir zaman olmayanlar

- Cihazda hiçbir şey saklanmaz. Tek istisna ziyaretçinin kendi vazgeçme tercihidir.
- Kimlik üretilmez, gönderilmez, okunmaz; her ziyaretçi, varsayılan moddaki gibi günün özetiyle sayılır.
- Hiçbir ziyaretçi bir kişiye ya da konuşmaya bağlanmaz: kişi sayfasında ziyaret listelenmez.
- Do Not Track ya da Global Privacy Control gönderen bir tarayıcı, izin aracı ne derse desin kimlik almaz.
- İzinsiz gönderilen, ya da sahte, süresi dolmuş veya daha önce kullanılmış imzalı bir token yok sayılır.

## Bir özellik için açın

İzinle açılan özellikler özelliğin `consent_features` ayarıdır ve iki değeri vardır:

| Değer | Ne yapar |
|---|---|
| `persistent` | Kabul eden ziyaretçilerin 13 aylık kimlik almasını sağlar. |
| `identify` | Kimliğin bir kişiye bağlanmasını sağlar. `persistent` ve bir gelen kutusuna bağlı özellik gerekir; tokenları o gelen kutusunun [kimlik sırrı](identity.md) imzalar. |

`analytics:manage` yetkili bir anahtarla ayarlayın:

```sh
curl -X PATCH -H "Authorization: Bearer $YUVA_API_KEY" -H "Content-Type: application/json" \
  https://support.example.com/v1/analytics/properties/$PROPERTY \
  -d '{"consent_features": ["persistent", "identify"]}'
```

Ayar olmadan `persistent` ve `identify` yok sayılır ve ziyaretçiler varsayılan modda sayılır.

**Çocuklara yönelik özellikler bunları kullanamaz.** **Kitle: Çocuk** olan bir özellik `consent_features` ayarını reddeder
(`400`): çocuğun cihazına kimlik verilmez ve hiçbir çocuk bir kişiye bağlanmaz.

Ziyaretçilerinize söyleyin. Gizlilik bildiriminize, izin veren bir ziyaretçinin cihazında en çok 13 ay kalan bir kimlik
aldığını, ne için kullanıldığını ve nasıl vazgeçileceğini ([vazgeçme](#vazgeçme-ve-silme)) ekleyin.

## İzin aracınızdan betiği çağırın

Betik cevabı saklamaz; bu yüzden ziyaretçinin güncel tercihiyle **her sayfa yüklemesinde** çağırın. Betiği
[izin modundaki](analytics.md#i̇zin-modu) gibi `data-consent="required"` ile kurun.

```js
window.yuva.consent({ analytics: true, persistent: true })
window.yuva.consent(false)
```

`consent({ analytics: true })` ziyaretçiyi kimliksiz sayar. `persistent: true` eklemek kimliği üretir. `consent(false)`
saymayı durdurur, kimliği cihazdan siler ve sunucuya kimliğin bir kişiye bağlantısını ve elde tutma günlerini unutmasını
söyler. Betik yüklenmeden önce çağrıyı kuyruğa alın:
`;(window.yuvaq = window.yuvaq || []).push(["consent", { analytics: true, persistent: true }])`.

İzinle açılan özellikler, bunları içeren bir betik ve uygulama sürümü gerektirir; sunucu bunları şimdiden kabul eder.

### Tarifler

Bunlar, aşağıdaki izin araçları için, belgelerinde adı geçen geri çağrıları kullanan örneklerdir. Aracınızın kendi
belgesine bakın; adlar ve kategoriler değişir. Önemli olan, `consent(...)` çağrısının her sayfa yüklemesinde ve tercih her
değiştiğinde ziyaretçinin seçimiyle yapılmasıdır.

**Herhangi bir araç.** Tek bir işlev yazın ve aracınızın yüklemede ve değişiklikte çağırdığı yerden çağırın:

```js
function syncYuva(analytics, persistent) {
  if (analytics) window.yuva.consent({ analytics: true, persistent })
  else window.yuva.consent(false)
}
```

**Cookiebot.** Cookiebot, her sayfa yüklemesinde ve tercih değiştiğinde `CookiebotOnConsentReady` olayını verir; **istatistik**
kategorisi `Cookiebot.consent.statistics` değeridir.

```js
window.addEventListener("CookiebotOnConsentReady", () => {
  const ok = Cookiebot.consent.statistics
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
})
```

**OneTrust.** `OptanonWrapper` yüklemede ve her değişiklikten sonra çalışır; `OnetrustActiveGroups` kabul edilen
kategori kimliklerini listeler, `C0002` **Performans**tır:

```js
function OptanonWrapper() {
  const ok = window.OnetrustActiveGroups.includes("C0002")
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
}
```

**Klaro.** Klaro yapılandırmasına bir hizmet ekleyin; `callback` işlevi seçimi yüklemede ve değişiklikte alır:

```js
services: [{
  name: "yuva",
  purposes: ["analytics"],
  callback: (consent) => {
    consent ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)
  },
}]
```

**CookieYes.** `document` üzerindeki `cookieyes_consent_update` olayı, tercih değiştiğinde kabul edilen kategorileri
`event.detail.accepted` içinde taşır; yüklemede `getCkyConsent()` yanıt verir:

```js
const apply = (ok) =>
  ok ? window.yuva.consent({ analytics: true, persistent: true }) : window.yuva.consent(false)

document.addEventListener("cookieyes_consent_update", (event) => {
  apply(event.detail.accepted.includes("analytics"))
})
apply(getCkyConsent().categories.analytics)
```

Aracınızda kimlik için ayrı bir anahtar varsa, onun cevabını `true` yerine `persistent` olarak verin.

## Ziyareti bir kişiye bağlayın

`identify` sunan bir özellikte oturum açmış kullanıcılar için betiğe, arka ucunuzun imzaladığı bir token verin; [widget](identity.md)
için olduğu gibi: aynı gelen kutusu, aynı sır, aynı alanlar; `exp` en çok 10 dakika sonrası ve yalnızca bir kez kullanılan bir
`jti`. Ziyaretçi oturum açtıktan sonra:

```js
const { token } = await fetch("/yuva-identity").then((r) => r.json())
window.yuva.identify(token)
```

`/yuva-identity` sizin kendi uç noktanızdır; [kimlik rehberinin](identity.md#2-o-anki-kullanıcı-için-token-imzalayın) anlattığı
uç nokta. Yuva tokenı bir sohbet oturumununki gibi denetler, `sub` için kişiyi bulur ya da oluşturur, kimliği ve ziyaretlerini
ona bağlar. Ziyaretler kişi sayfasında ve `GET /v1/contacts/{contactId}/visits` ucunda (`contacts:read` ve `analytics:read`
yetkileri) ayrıntılı kayıtlar saklandığı sürece görünür. Her oturum açışta `identify` çağırın; bir token iki kez kullanılamaz.
Hem analitiğe hem `persistent` özelliğine izin vermemiş bir ziyaretçi için hiçbir şey yapmaz.

Bir kişiyi silmek ona bağlı ziyaretleri de siler; iki kişiyi birleştirmek ziyaretleri taşır.

## Vazgeçme ve silme

İzin aracınızda vazgeçen bir ziyaretçi: geri çağrınız `consent(false)` çağırır. Kimlik cihazdan çıkar; sunucu kimliğin
bir kişiye bağlantısını ve elde tutma günlerini siler. Vazgeçme, tarayıcı Do Not Track gönderse bile kabul edilir.

Elinizdeki verilerin **silinmesini** isteyen bir kişi için: kimliği altında saklanan her şeyi `analytics:manage` yetkili bir
anahtarla silin:

```sh
curl -X DELETE -H "Authorization: Bearer $YUVA_API_KEY" \
  https://support.example.com/v1/analytics/properties/$PROPERTY/visitors/$PERSISTENT_ID
```

Kimliğin ham ziyaretlerini ve olaylarını, elde tutma günlerini ve kişi bağlantısını siler, tamamlanmış günleri onlar olmadan
yeniden toplar ve altında bir şey saklanmış olsun olmasın `204` yanıtlar. Kimlik 16 ile 64 arası harf, rakam, `-` ve `_`
karakterinden oluşur. Kişinin tarayıcısı bunu bilir: `window.yuva.persistentId()` onu verir; böylece sitenizdeki bir "Verilerimi
sil" düğmesi onu arka ucunuza gönderebilir, arka uç da çağrıyı anahtarınızla yapar. Yuva kimliğin yalnızca özetini saklar; bu
yüzden onu bir addan ya da e-posta adresinden bulamazsınız. Oturum açmış bir kullanıcının kişisini de silmek için bkz.
[Kullanıcıyı silme](identity.md#kullanıcıyı-silme).

## Sunucu olayları

İzin vermiş bir ziyaretçi için olay bildiren bir arka uç onu adıyla anabilir. Tarayıcının tuttuğu kimliği `persistent_id`
olarak, `consent: true` ile `POST /v1/analytics/properties/{propertyId}/events` ucuna gönderin:

```json
{
  "events": [{
    "name": "Signup",
    "url": "https://www.example.com/signup",
    "persistent_id": "0f8fad5b-d9cb-469f-a165-70867728950e",
    "consent": true
  }]
}
```

Olay o zaman, tarayıcının kendi olayları gibi ziyaretçinin günlerine dahil olur. `identity_token` de verilirse
(`identify` sunan bir özellikte) ziyaretçiyi tanıtır. `consent: true` olmadan kimlik yok sayılır. Diğer alanlar
[Sunucu olayları](analytics-server.md) sayfasındadır.

## Uygulamalar

Swift ve Kotlin kütüphaneleri betikle aynı üç çağrıyı alır: iki seçimli izin, vazgeçme için `consent(false)` ve arka
ucunuzun imzaladığı bir tokenla `identify`. Kimlik uygulamanın özel depolamasında en çok 13 ay durur; ne zaman soracağınıza
kendi izin ekranınız karar verir.

```swift
YuvaAnalytics.consent(analytics: true, persistent: true)
YuvaAnalytics.identify(token: token)
YuvaAnalytics.consent(false)
```

```kotlin
YuvaAnalytics.consent(analytics = true, persistent = true)
YuvaAnalytics.identify(token)
YuvaAnalytics.consent(false)
```

Bu çağrılar uygulama kütüphanelerinin bir sonraki sürümüyle gelir; adlar betikteki adlardır ve imzalar
[kütüphane sayfasında](analytics-apps.md) yer alacaktır. O zamana kadar `consent(true)` ve `consent(false)`,
[İzin ve vazgeçme](analytics-apps.md#i̇zin-ve-vazgeçme) bölümünde anlatıldığı gibi çalışır. Bunları her açılışta çağırın.

### Mağazalar için yanıtlar

Bunlar `persistent` açıkken ve kullanıcı kabul ettiğinde geçerlidir; açık değilse [uygulamalar sayfasındaki yanıtlar](analytics-apps.md#mağazalar-için-yanıtlar)
geçerlidir. Kimlik rastgeledir, kütüphane üretir, reklam kimliği değildir ve kimseyle paylaşılmaz.

**App Store.** Gizlilik raporuna ekleyin:

| Veri türü | Kullanıcıya bağlı | İzleme için kullanılıyor | Amaç |
|---|---|---|---|
| Identifiers → Device ID | Hayır | Hayır | Analitik |
| Identifiers → User ID, yalnızca `identify` çağırıyorsanız | Evet | Hayır | Analitik |

Yuva kimliği başka şirketlerin uygulama ya da sitelerinden gelen verilerle birleştirmez; bu yüzden Apple'ın anlamında
izleme değildir. Kendi kullanımınız aksini söylemiyorsa izleme için **No** yanıtını verin.

**Google Play.** **Veri güvenliği** bölümünde, `AD_ID` yine yok:

| Veri türü | Toplanıyor | Paylaşılıyor | Notlar |
|---|---|---|---|
| Cihaz veya diğer kimlikler | Evet | Hayır | Rastgele kimlik, isteğe bağlı (kullanıcı seçer). Amaç: Analitik. |
| Kişisel bilgiler → Kullanıcı kimlikleri, yalnızca `identify` çağırıyorsanız | Evet | Hayır | İsteğe bağlı. Amaç: Analitik. |

Kullanıcı reddedebildiği için ikisini de formda **isteğe bağlı** olarak işaretleyin ve verilerin istek üzerine silinebildiğini ekleyin.

## Başvuru

| Çağrı | Ne yapar |
|---|---|
| `PATCH /v1/analytics/properties/{propertyId}` | `consent_features` ayarını yapar. Yetki `analytics:manage`. |
| `GET /v1/analytics/properties/{propertyId}/retention` | İzin veren ziyaretçilerin kohortları (`grain`: `day`, `week`, `month`; `from`, `to`). Yetki `analytics:read`. |
| `DELETE /v1/analytics/properties/{propertyId}/visitors/{persistentId}` | Bir kimlik altında saklanan her şeyi siler. Yetki `analytics:manage`. |
| `GET /v1/contacts/{contactId}/visits` | Bağlı bir kişinin son ziyaretleri. |
| `POST /v1/analytics/properties/{propertyId}/events` | `persistent_id`, `consent` ve `identity_token` ile sunucu olayları. |

Elde tutma kohortları 30 güne ya da 12 hafta veya aya kadar gider ve her saat yeniden kurulur. Bir kimlik ilk görüldüğünden
13 ay sonra unutulur. Adres ve özellik başına dakikada 20 izin değişikliği kabul edilir.
