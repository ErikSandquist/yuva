# Sunucu tarafı olaylar

Bazı şeyleri yalnızca arka ucunuz görür: oluşturulan bir hesap, geçen bir ödeme, yenilenen bir abonelik.
Sunucunuz bunları bir analitik özelliğine gönderebilir; ziyaretçinin adresini ve tarayıcısını taşıyan bir olay,
o ziyaretçinin web sitenizdeki ziyaretine katılır. Bu sayfa [Analitik](analytics.md) ve
[Olaylar, hedefler ve dönüşüm hunileri](analytics-events.md) sayfalarındaki olay kuralları üzerine kuruludur.

## Anahtar oluşturun

Sunucu olayları bir API anahtarı ister; web sitesinin herkese açık anahtarı (`yuva_ak_…`) bunlar için çalışmaz.
Sahipler ve yöneticiler **Ayarlar → API anahtarları** altında `analytics:write` kapsamlı bir anahtar oluşturur
ve onu sunucuda tutar. Gelen kutularıyla sınırlı bir anahtar, o gelen kutularının özelliklerine ulaşır. Bir
üyenin oturumu reddedilir (`403 forbidden`).

Özelliğin kimliği de gerekir. **Ayarlar → Kurulum** altındaki **Go** ve **curl** sekmeleri aşağıdaki kodları
onunla doldurulmuş olarak verir. Arka ucunuzdan gelen olaylar için özelliğin ayarlarında bir platform
işaretlemeniz gerekmez.

## Olay gönderme

```sh
curl https://support.example.com/v1/analytics/properties/PROPERTY_ID/events \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "events": [{
      "name": "Signup",
      "url": "https://www.example.com/signup",
      "ip": "203.0.113.7",
      "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
      "props": { "plan": "pro" }
    }]
  }'
```

```json
{ "accepted": 1, "errors": [] }
```

`POST /v1/analytics/properties/{propertyId}/events` 1 ile 100 arasında olay alır ve `202` yanıtlar.

| Alan | |
|---|---|
| `name` | Zorunlu, 1 ile 120 karakter: `pageview`, yerleşik bir olay ya da kendi olayınızın adı. Kurallar [olaylarınkiyle](analytics-events.md#olay-gönderme) aynıdır; `engagement` yalnızca tarayıcılar içindir. |
| `url` | Zorunlu, en fazla 2.000 karakter. Özelliğin alan adlarından birindeki bir sayfa ya da iOS veya Android'li bir özellik için bir uygulama ekranı, `app://<uygulama kimliği>/<ekran>`. Yalnızca alan adı ve yol saklanır. |
| `referrer` | Sayfanın yönlendireni; bir ziyaret başlatan sayfa görüntülemesi için. |
| `timestamp` | Olayın gerçekleştiği an, RFC 3339. Boş bırakılırsa şimdi. |
| `props` | En fazla 30 metin, sayı ya da boolean değer; [özellik kuralları](analytics-events.md#özellikler-prop) geçerlidir. |
| `revenue` | `{"a": "29.90", "c": "EUR"}`; bkz. [Gelir](analytics-events.md#gelir). |
| `interactive` | `false`, özel bir olayı etkileşimsiz yapar. |
| `ip`, `user_agent`, `sec_ch_ua_platform`, `sec_ch_ua_mobile`, `accept_language` | Ziyaretçinin isteği, [iletilmiş olarak](#ziyaretçinin-ziyaretine-katılma). |
| `visitor_key` | `ip` ve `user_agent` yerine ziyaretçi için kendi anahtarınız. |
| `dnt`, `gpc` | Ziyaretçinin isteğinde `DNT: 1` ya da `Sec-GPC: 1` vardı. Olay saklanmaz. |
| `consent` | Ziyaretçi analitiğe izin verdi. |

## Ziyaretçinin ziyaretine katılma

Sunucunuzun kendi adresini değil, ziyaretçiden aldığınızı iletin. Yuva `ip`'yi gelir gelmez kısaltır, `user_agent`
ile web sitesine gelen tarayıcının kendi isteğini nasıl karmalıyorsa öyle karmalar ve ikisini de unutur.
Olay böylece ziyaretçinin o günkü ziyaretine ait olur: kayıt, dönüşüm hunisinde ona götüren sayfa görüntülemelerinin
ardından görünür. İstekte varsa `sec_ch_ua_platform` ve `sec_ch_ua_mobile` (`Sec-CH-UA-*` başlıkları, geldiği
gibi) ile `accept_language`'ı da ekleyin. Adres konum için de kullanılır.

Elinizde tarayıcı isteği olmayan bir ziyaretçi için, örneğin bir webhook ya da arka plan işi için, bunun yerine
bir `visitor_key` verin: onlar için kendi anahtarınız, örneğin bir oturum kimliği; **asla bir e-posta adresi
değil**. Günün tuzu ile karmalanır, hiç saklanmaz ve tarayıcıdan ayrı, kendi başına bir ziyaretçi oluşturur.
Onunla `ip` yalnızca ziyaretçinin yerini, `user_agent` yalnızca cihazı betimler.

Bir olayın başlattığı ziyaretin platformu **Sunucu**dur.

## İzin ve Do Not Track

Göndermeden önce ziyaretçinin seçimlerine uyun:

- İsteğinde `DNT: 1` ya da `Sec-GPC: 1` olan bir ziyaretçi için hiç gönderim yapılmamalıdır. Olayı yine de
  `dnt: true` ya da `gpc: true` ile gönderirseniz Yuva onu alır ama saklamaz.
- **Toplama**sı **Yalnızca izinle** olan bir özellik, bir olayı yalnızca `consent: true` ile saklar.

## Ne döner

İstek geçerliyse, olaylara ne olduğundan bağımsız olarak yanıt `202`'dir.

- **`errors`**, geçersiz diye dışarıda bırakılan olayları, her biri `events` içindeki `index`'i ve nedenini
  söyleyen bir `detail` ile listeler. Diğerleri alınır.
- **Alınır ama saklanmaz**, **Ayarlar → Durum** altında sayılır: `dnt` ya da `gpc` olan olaylar; izin
  özelliğinde izinsiz olaylar; botlar, hariç tutulan adresler ve yollar; adres başına, özellik başına ya da
  aylık sınırı aşan olaylar; 72 saatten eski olaylar (**Çok geç**).
- **İsteğin tamamı reddedilir**: JSON olmayan ya da yukarıdaki biçimi bozan bir gövde (`400`), eksik ya da yanlış
  anahtar (`401`), `analytics:write` kapsamı olmayan bir anahtar ya da bir üyenin oturumu (`403`) ve bilinmeyen
  bir özellik (`404`) için. Meşgul bir sunucu `Retry-After` ile `503` yanıtlar: o süreden sonra isteği yeniden
  gönderin.

**Ayarlar → Test olayları**, siz denerken sunucunun neyi saydığını ve neyi attığını gösterir.

## Go

```sh
go get github.com/productdevbook/yuva/sdk/go/analytics
```

```go
import "github.com/productdevbook/yuva/sdk/go/analytics"

yuva, err := analytics.New(analytics.Config{
	BaseURL:    "https://support.example.com",
	APIKey:     os.Getenv("YUVA_API_KEY"),
	PropertyID: "PROPERTY_ID",
})
if err != nil {
	log.Fatal(err)
}
defer yuva.Close(context.Background())

// in a handler: the visitor's address and browser come from r
yuva.Track(r, analytics.Event{
	Name:  "Signup",
	Props: map[string]any{"plan": "pro"},
})
```

`Track`, `r`'nin adresini, `User-Agent`, `Sec-CH-UA-Platform`, `Sec-CH-UA-Mobile` ve `Accept-Language` başlıklarını
iletir; olay `URL` ve yönlendireni boş bırakırsa bunları `r`'den alır. Bu URL özelliğin alan adlarından birinde
olmalıdır; bu yüzden bir vekil sunucunun ya da iç bir ana makine adının arkasında `URL`'yi kendiniz verin.
`DNT: 1` ya da `Sec-GPC: 1` olan bir istek hiçbir şey göndermez ve `Track` `nil` döner. Ziyaretçi onay
verdiyse olayda `Consent: true` verin. `VisitorKey` verildiğinde `Track` ne adresi ne de kullanıcı aracısını
iletir; yer ve cihaz isterseniz olaya `IP` ve `UserAgent` da yazın.

`Send`, olayı olduğu gibi kuyruğa alır; isteği olmayan bir arka plan işi için.

| `Config` | Varsayılan | |
|---|---|---|
| `HTTPClient` | `http.DefaultClient` | İstekleri gönderir. |
| `FlushInterval` | 5 saniye | Kuyruktaki olayların ne sıklıkla gönderildiği. 100'lük dolu bir grup hemen gider. |
| `QueueSize` | 10.000 | En çok bekleyen olay; fazlası `ErrQueueFull` ile atılır. |
| `MaxRetries` | 5 | Ağ hatası, `429` ya da `5xx` sonrası bir grubun yeniden deneme sayısı; eksi değer: hiç. |
| `MinBackoff` | 1 saniye | İlk yeniden denemenin gecikmesi, bir dakikaya kadar ikiye katlanır. Daha uzun bir `Retry-After` geçerli olur. |
| `ClientIP` | `r.RemoteAddr` ana makinesi | `Track` için ziyaretçinin adresi. Bir vekil sunucunun arkasında onun bildirdiği adresi döndürün. |
| `OnError` | yok | Yuva'nın geçersiz diye dışarıda bıraktığı olay için bir `*EventError`, reddettiği (`400`, `401`, `403`, `404`) ya da her yeniden denemeden sonra da başarısız olan bir grup için bir `*BatchError` ve `ErrQueueFull` ile çağrılır. |

Olaylar arka planda gruplar halinde gönderilir. `Flush(ctx)` kuyruktakileri şimdi gönderir, `Close(ctx)` onları
gönderir ve istemciyi durdurur: süreç çıkmadan önce çağırın, yoksa bekleyen olaylar kaybolur.
