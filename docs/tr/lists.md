# E-posta listeleri ve abonelik formları

Bir e-posta listesi bir gelen kutusuna aittir ve sizden haber almayı kabul eden kişileri tutar.
Ziyaretçiler sitenizdeki bir formla katılır, arka uç zaten izni olan kişileri ekler, her değişiklik de
dışa aktarabileceğiniz bir izin kaydına yazılır.

Bir listenin onay ve karşılama e-postaları, gelen kutusunun [e-posta kanalı](email.md) üzerinden, o
kanalın adresi ve SMTP hesabıyla gider. E-posta kanalı olmayan bir liste için Yuva hiçbir şey göndermez.

## 1. Gönderen bilgilerini girin

Pazarlama içeren e-posta, kimin gönderdiğini ve ona nasıl ulaşılacağını belirtmelidir. Panelde: Ayarlar →
gelen kutusu → Gönderen bilgileri. Yasal unvanı, posta adresini, sicil bilgisini (MERSİS numarası,
ticaret sicil ya da vergi numarası) ve bir iletişim bilgisini (e-posta adresi, telefon ya da URL)
yazın. Yuva bunları gelen kutusunun liste e-postalarının alt bilgisine koyar.

API ile: `sender` nesnesiyle `PATCH /v1/inboxes/{inboxId}`.

## 2. Liste oluşturun

Panelde: Bültenler, sonra Yeni liste. Gelen kutusunu ve listenin e-postalarını gönderecek e-posta
kanalını seçin.

| Ayar (API alanı) | Anlamı |
|---|---|
| Ad (`name`) | En çok 100 karakter, gelen kutusunda benzersiz. |
| Açıklama (`description`) | En çok 500 karakter; formlarda ve tercih sayfasında gösterilir. |
| Çift onay (`double_opt_in`) | Varsayılan açık. Formdan gelen aboneler abone olmadan önce e-postayla onaylar. |
| Herkese açık (`public`) | Varsayılan açık. Formlarda ve tercih sayfasında sunulur. Özel listeleri yalnızca API ve ekibiniz doldurur. |
| Ticari (`commercial`) | Varsayılan açık. Pazarlama içeriğini işaretler; izin kayıtlarının dışa aktarımında yer alır. |
| Karşılama (`welcome`) | Varsayılan açık. Biri abone olunca abonelikten çıkma bağlantılı bir makbuz gönderir. |
| Gönderen kanal (`channel_id`) | Aynı gelen kutusunda SMTP hesabı olan bir e-posta kanalı. Yoksa formlar hiçbir şey göndermez ve abonelikler `pending` kalır. |

API ile: `inbox_id` ve `name` ile `POST /v1/lists`.

## 3. Formu sitenize ekleyin

Form, aynı gelen kutusunun bir [sohbet kanalını](widget.md#1-sohbet-kanalı-oluşturun) kullanır:
kanalın herkese açık anahtarı ve izinli origin'leri formun nerede görünebileceğini belirler. Panelde
listenin sayfasındaki Form sekmesi bu kod parçalarını sizin için doldurur.

```html
<script src="https://support.example.com/yuva-subscribe.js" defer></script>
<yuva-subscribe
  channel="yuva_pk_xxxxxxxxxxxxxxxx"
  lists="LIST-ID"
  privacy-url="https://www.example.com/privacy"
></yuva-subscribe>
```

`lists` alanını boş bırakırsanız gelen kutusunun tüm herkese açık listeleri, her biri için bir kutuyla
sunulur. Öğe bir e-posta adresi ister ve işaretlenmemiş bir izin kutusu gösterir. Ziyaretçi kutuyu
işaretlemeden abone olamaz; gördüğü cümle de kanıt olarak saklanır.

Paketleyici kullanan bir projede `npm install useyuva` çalıştırın ve şunlardan birini kullanın:

| Yığın | Kod |
|---|---|
| Herhangi bir paketleyici | `import "useyuva/subscribe"`, sonra yukarıdaki öğe ve bir `server` özniteliği. |
| React | `import { YuvaSubscribe } from "useyuva/react"` ve `<YuvaSubscribe channel server lists={["LIST-ID"]} />`. |
| Nuxt | `nuxt.config` içinde `modules: ["useyuva/nuxt"]` ve `yuva: { server, channel }`, sonra `<YuvaSubscribe :lists="['LIST-ID']" />`. |
| shadcn/ui | `npx shadcn@latest add https://useyuva.com/r/yuva-subscribe.json`, sonra `<YuvaSubscribe server channel lists={["LIST-ID"]} />`. |

Siteniz bir Content Security Policy gönderiyorsa Yuva sunucunuza `script-src` ve `connect-src` içinde
izin verin. Oturum açmış kullanıcılar için öğeye `setIdentityToken` ile bir [kimlik belirteci](identity.md)
verin: e-posta alanı yerine her liste için bir anahtar gösterir ve kullanıcının adresi doğrulanmışsa
hemen abone eder.

## 4. Aboneler ne görür

1. Formu doldururlar. Adres yeni, abone, abonelikten çıkmış ya da bilinmiyor olsun yanıt hep aynıdır;
   böylece form kimin listenizde olduğunu kimseye söylemez.
2. Çift onayda `/s/…` bağlantılı bir e-posta alırlar. Sayfayı açmak hiçbir şeyi değiştirmez; onaylayan,
   sayfadaki düğmedir. Ardından liste karşılama gönderiyorsa karşılama e-postası gelir.
3. Her liste e-postasında `/m/…` bağlantısı vardır; kişi buradan bir listeden ya da her şeyden çıkar
   veya listeleri seçer. Bu sayfayı açmak da hiçbir şeyi değiştirmez. Tek tıkla abonelikten çıkmayı
   (RFC 8058) destekleyen posta programları listeyi tek bir istekle çıkarır.

Sayfalar ve e-postalar İngilizce, Türkçe ve Almanca gelir. Abonelikten çıkan kişiye yalnızca kendisi
yeniden abone olabilir: ekibiniz ve anahtarlarınız olamaz. Her şeyden çıkmış, bir e-postayı spam diye
bildirmiş ya da itiraz ettikten sonra silinmiş bir adres reddedilir (`409 opted_out`). Bekleyen
abonelikler 30 gün sonra, abonelikten çıkanlar üç yıl sonra silinir.

## 5. Arka uçtan abone ekleyin

Bir dosya dolusu kişi için [Abone içe aktarma](importing.md) sayfasına bakın. Başka bir yerde, örneğin kayıt
sırasında, kabul etmiş kişiler için kullanın. Kişinin nasıl kabul ettiğini
siz bildirirsiniz; Yuva bunu kanıt olarak saklar. Anahtarın `lists:write` kapsamı olmalıdır.

```sh
curl -X POST https://support.example.com/v1/lists/LIST-ID/subscriptions \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Idempotency-Key: signup-1234" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "ada@example.com",
    "name": "Ada",
    "consent": {
      "confirmed": true,
      "obtained_at": "2026-10-01T09:30:00Z",
      "text": "Ürün haberlerini e-postayla göndermenizi kabul ediyorum.",
      "page_url": "https://www.example.com/signup",
      "reference": "order-1234"
    }
  }'
```

`confirmed: true` ile abonelik hemen `subscribed` olur ve karşılama e-postası gider. `confirmed: false`
ile `pending` olur ve kişiye önce bir onay e-postası gider; kabul ettiğinden emin değilseniz bunu
kullanın. `201` yeni bir abonelik, `200` listede zaten olan bir adrestir.

## 6. İzin kayıtlarını dışa aktarın

Listeyi açın, sonra Dışa aktar'ı seçin ya da `GET /v1/lists/{listId}/consent-export` çağırın
(`format=csv` veya `json`; daha yeni olaylar için `since`; Türkiye'deki göndericiler aşağıdaki Türkiye: İYS bölümüne bakın). Her satır bir olaydır: adres, durum, zaman,
kaynak, IP adresi, sayfa ve kişinin gördüğü sözler. `GET /v1/contacts/{contactId}/consent` bir kişinin
tüm listelerdeki kayıtlarını verir; GDPR m. 15 ya da KVKK m. 11 kapsamındaki erişim talepleri içindir.
`=`, `+`, `-` ya da `@` ile başlayan hücrelerin başına işaret konur, böylece e-tablolar bunları formül
olarak çalıştırmaz.

## Türkiye: İYS

Yuva İYS'ye bağlanmaz. İYS'ye kendiniz yükleyeceğiniz bir dosya verir; orada gönderici olarak kaydolmak
da size aittir.

Listeyi açın, sonra Dışa aktar'ı seçin ve **İYS için indir**'e basın ya da
`GET /v1/lists/{listId}/consent-export?format=iys` çağırın. Yalnızca o tarihten sonra izin verenleri ya da
ayrılanları almak için "Başlangıç (isteğe bağlı)" alanına tarih girin ya da `since` verin. Dosya, abone başına
bir satır içeren bir CSV'dir:

| Sütun | Değer |
|---|---|
| `recipient` | E-posta adresi. |
| `type` | `EPOSTA`. |
| `source` | Form ya da tercih sayfasından gelen abonelikler için `HS_WEB`, diğerleri için `HS_EORTAM`. |
| `status` | Abone olanlar için `ONAY`, ayrılanlar için `RET`. |
| `consentDate` | Kişinin izin verdiği ya da ayrıldığı zaman, Türkiye saatiyle (`YYYY-MM-DD HH:MM:SS`). |
| `recipientType` | `BIREYSEL`. |

Henüz onaylamamış, onay bekleyen abonelikler dosyaya alınmaz. Sütunlar, bildiğimiz kadarıyla İYS'nin izin
alanlarını izler. Yüklemeden önce İYS hesabınızın istediğiyle karşılaştırın ve yalnızca ticari e-posta
gönderen listeleri kaydedin. Bu sayfa hukuki tavsiye değildir.

## Sorun giderme

- **Formda liste yok**: liste özel ya da arşivlenmiş, veya `lists` kanalın gelen kutusunda herkese açık
  olmayan bir kimliği gösteriyor.
- **Form hata gösteriyor**: sayfanın origin'i sohbet kanalının izinli origin'lerinden biri değil.
- **Onay e-postası gelmedi**: listenin gönderen kanalı yok (panel bunu işaretler) ya da adres bugün zaten
  3 onay aldı.
- **`409 email_channel_required`**: kanal, listenin gelen kutusunda SMTP hesabı olan bir e-posta kanalı
  değil.

## Başvuru

### `<yuva-subscribe>` öznitelikleri

| Öznitelik | Anlamı |
|---|---|
| `channel` | Sohbet kanalının herkese açık anahtarı. Zorunlu. |
| `server` | Yuva sunucu URL'si. Varsayılan: `yuva-subscribe.js`'in geldiği yer. |
| `lists` | Virgülle ayrılmış liste kimlikleri. Boş: tüm herkese açık listeler. |
| `layout` | `stacked` (varsayılan) ya da `inline`. |
| `name` | `off` (varsayılan), `optional` ya da `required`: ad alanı ekler. |
| `button-text` | Düğmenin etiketi. |
| `consent` | Kendi izin cümleniz. Gösterildiği gibi saklanır. |
| `privacy-url` | Kutunun yanında gösterilen gizlilik bildirimi bağlantısı. |
| `locale`, `dir` | `en` ya da `tr`; `ltr` ya da `rtl`. |
| `identity-token` | Bir belirteç dizesi; genelde `setIdentityToken` daha iyidir. |

`--yuva-accent`, `--yuva-on-accent`, `--yuva-radius`, `--yuva-font` ve
`::part(field|button|consent|status)` ile biçimlendirin.

### Olaylar

| Olay | `detail` |
|---|---|
| `yuva-subscribe` | `{ lists, confirmation }`: sunulan kimlikler ve bir onay e-postası beklenip beklenmediği. |
| `yuva-subscribe-error` | `{ code }`. |

React ve shadcn: `onSubscribe`, `onError`. Nuxt: `@subscribe`, `@error`.

### Uç noktalar

| Uç nokta | Kullanım |
|---|---|
| `GET`, `POST /v1/lists` | Listeleri sıralar ve oluşturur (`archived=true` arşivlenenleri sıralar). |
| `GET`, `PATCH`, `DELETE /v1/lists/{listId}` | Listeyi okur, değiştirir, arşivler (`archived: true`) ya da siler. |
| `GET /v1/lists/{listId}/growth` | Günlük yeni, onaylanan ve ayrılan sayısı; `days=30` ya da `90`. |
| `GET`, `POST /v1/lists/{listId}/subscriptions` | Aboneleri sıralar (`state`, `q`) ve ekler. |
| `GET`, `PATCH`, `DELETE /v1/lists/{listId}/subscriptions/{subscriptionId}` | İzin olaylarıyla okur, abonelikten çıkarır (`state: unsubscribed`), siler. |
| `POST /v1/lists/{listId}/subscriptions/{subscriptionId}/confirmation` | Onay e-postasını yeniden gönderir. |
| `GET /v1/lists/{listId}/consent-export` | Bir listenin izin kayıtları. |
| `GET /v1/contacts/{contactId}/subscriptions` | Bir kişinin abonelikleri. |
| `GET /v1/contacts/{contactId}/consent` | Bir kişinin izin kayıtları. |
| `GET /client/v1/channels/{channel_key}/lists` | Form için herkese açık listeler ve bir form belirteci. |
| `POST /client/v1/channels/{channel_key}/subscriptions` | Formdan abone olur. |
| `GET /client/v1/subscriptions`, `PUT`, `DELETE /client/v1/subscriptions/{listId}` | Oturum açmış kullanıcının listeleri, abone olma, çıkma. |

İstek ve yanıt biçimlerinin tamamı [API sözleşmesinde](../../openapi/openapi.yaml) yer alır. Kendi
formlarınız için `useyuva`, [headless istemcide](headless.md) `lists()`, `subscribe()`,
`subscriptions()`, `subscribeList()` ve `unsubscribeList()` sunar.

### Kapsamlar ve olaylar

| Ad | Anlamı |
|---|---|
| `lists:read` | Listeleri, abonelikleri, büyümeyi ve izin dışa aktarımlarını okur. |
| `lists:write` | Liste oluşturur, değiştirir, siler; abone ekler, abonelikten çıkarır, siler; onayları yeniden gönderir. |
| `subscription.created` | [Webhook](webhooks.md) olayı: biri herhangi bir durumda abone oldu. |
| `subscription.updated` | Webhook olayı: durum değişti; eski durum `data.previous_state` içindedir. |

Bu kapsamlar var olmadan önce oluşturulan anahtarlara verilmez.

### Sınırlar

| Sınır | Değer |
|---|---|
| Bir formda sunulan liste | 10 |
| IP adresi başına form isteği | Saatte 10 |
| Kanal başına form isteği | Saatte 600 |
| Adres başına onay e-postası | Günde 3; liste başına 10 dakikada 1 |
| Kanal başına onay e-postası | Günde 1.000 |
| Panelden ya da API'den yeniden gönderilen onay | Abonelik başına günde 3, toplamda 10 |
| Form belirteci | Verildikten 2 saniye ile 2 saat arasında geçerli |
