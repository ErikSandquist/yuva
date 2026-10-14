# iOS ve Android uygulamalarından API izleme

Swift ve Kotlin kütüphaneleri, uygulamanızın kendi API'nize yaptığı çağrıların süresini ölçüp sunucunuzun bildirdiklerinin
yanına bir API özelliğine ekleyebilir. API'yi kullanıcılarınızın gördüğü gibi görürsünüz: süreye kendi ağları dahildir
ve yanıt alamayan çağrılar da sayılır. Bir API özelliği gerekir: bkz.
[API özelliği oluşturun](analytics-api.md#api-özelliği-oluşturun). Bu sayfa
[Uygulamalarda analitik](analytics-apps.md) sayfasının üzerine kurulur; siz açana kadar kapalıdır.

## Uygulamaya izin verin

1. API özelliğinin herkese açık anahtarını (`yuva_ak_…`) `GET /v1/analytics/properties` yanıtındaki `public_key`
   alanından alın. Bir web sitesi özelliğinin anahtarı gibi herkese açıktır ve yalnızca burada anlatılan çağrılar için
   çalışır.
2. Platformu özellikte açın (`analytics:manage` kapsamı, sahipler ve yöneticiler). `platforms` listesinde ne `ios`
   ne `android` olan bir API özelliğinin anahtarı reddedilir (`403`); listede olmayan bir platformun uygulaması da
   reddedilir (`403 platform_not_allowed`).

```sh
curl -X PATCH https://support.example.com/v1/analytics/properties/PROPERTY_ID \
  -H "Authorization: Bearer $YUVA_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "platforms": ["ios", "android"] }'
```

`YuvaAnalyticsConfiguration` yine uygulamanın kendi analitik özelliğinin anahtarını alır;
[Uygulamalarda analitik](analytics-apps.md) sayfasındaki gibi kurun.

## Swift

Kütüphaneyi yapılandırırken `requests` verin ve API'nizle konuşan URL oturumuna bir `YuvaRequestDelegate` verin:

```swift
YuvaAnalytics.configure(
    YuvaAnalyticsConfiguration(
        serverURL: URL(string: "https://support.example.com")!,
        key: "yuva_ak_xxxxxxxx",
        requests: YuvaRequestMonitoring(
            key: "yuva_ak_yyyyyyyy",
            hosts: ["api.example.com"],
            routes: ["/users/{id}", "/users/me", "/orders/{id}/items/{item}"],
            apiVersion: "v2"
        )
    ))

let api = URLSession(configuration: .default, delegate: YuvaRequestDelegate(), delegateQueue: nil)
```

Zaten bir delegate'i olan oturum, bunun yerine `urlSession(_:task:didFinishCollecting:)` içinden
`YuvaAnalytics.recordRequest(task, metrics: metrics)` çağırır. Tek bir görev `task.delegate = YuvaRequestDelegate()`
alabilir. Süre, getirmenin başından yanıtın sonuna kadar sürer.

## Kotlin

Kütüphane OkHttp getirmez; uygulamanızdakini kullanır. `requests` verin ve interceptor'ı API'nizle konuşan istemciye
ekleyin:

```kotlin
YuvaAnalytics.configure(
    this,
    YuvaAnalyticsConfiguration(
        serverUrl = "https://support.example.com",
        key = "yuva_ak_xxxxxxxx",
        requests = YuvaRequestMonitoring(
            key = "yuva_ak_yyyyyyyy",
            hosts = listOf("api.example.com"),
            routes = listOf("/users/{id}", "/users/me", "/orders/{id}/items/{item}"),
            apiVersion = "v2",
        ),
    ),
)

val api = OkHttpClient.Builder().addInterceptor(YuvaRequestInterceptor()).build()
```

Süre, yanıt gövdesi sonuna kadar okunana ya da kapatılana kadar sürer; bu yüzden her gövdeyi okuyun ya da kapatın.
İptal edilen çağrılar kaydedilmez.

## Seçenekler

`YuvaRequestMonitoring` iki kütüphanede aynı parametreleri alır.

| Parametre | Varsayılan | Anlamı |
|---|---|---|
| `key` | | API özelliğinin herkese açık anahtarı. |
| `hosts` | | API'nizin ana makine adları. Yalnızca bunlara yapılan çağrılar kaydedilir; port yok sayılır. |
| `routes` | yok | API'nizin kalıpları, örneğin `/users/{id}`. `{name}` bir yol segmentiyle eşleşir. |
| `route` | yok | Bir isteğin rotasını kendisi adlandıran işlev; yanıtı `routes` üzerinde önceliklidir. |
| `sendUnmatchedPaths` | `false` | Hiçbir rotayla eşleşmeyen çağrının yolunu gönderir; kalıbı sunucu çıkarır. |
| `apiVersion` | yok | Uygulamanın çağırdığı API sürümü; API sürümü kırılımı içindir. |
| `environment` | `production` | `production`, `staging` gibi ortamları ayırır. |

Bir çağrının rotası `route` işlevinin yanıtıdır; yoksa `routes` içinden en belirli olanıdır (en çok sabit segmenti
olan). Hiçbir rotaya denk gelmeyen çağrı atlanır; `sendUnmatchedPaths` açıksa yolu, sorgusuz, sunucuya gider ve sunucu
[Rotalar](analytics-api.md#rotalar) altında anlatıldığı gibi kimlikleri `{id}` yapar. Yol göndermek yerine
bildiğiniz rotaları listeleyin: böylece cihazdan kodunuzun adlandırmadığı hiçbir şey çıkmaz.

## Ne kaydedilir

Çağrı başına: yöntem, rota, durum (yanıt gelmediğinde `0`, örneğin zaman aşımı ya da ağ yok), süre, istek ve yanıt
gövdelerinin boyutu ve çağrının başladığı an. URL, sorgu, başlıklar ve gövdeler asla. Kayıtlar tüketici, hizmet ya da
adres taşımaz; ülkeleri bağlantının adresinden bulunur, sunucu bu adresi kısaltır ve hemen atar. Sunucu kayıtlarıyla
aynı görünümlerde ve `dataset: api` sorgularında, uygulamanın platformu ve sürümüyle görünürler.

- **İzin ve vazgeçme** uygulamanın analitiğini izler. `requireConsent` ile `consent(true)` öncesinde hiçbir şey
  kaydedilmez ve her kayıt kullanıcının iznini taşır; **Toplama**sı **Yalnızca izinle** olan bir özellik bunu ister.
  `optOut()` sonrasında hiçbir şey kaydedilmez ve bekleyenler silinir. `Sec-GPC: 1` ya da `DNT: 1` taşıyan istek
  kabul edilir ve hiçbir şey saymaz. Bkz. [İzin ve vazgeçme](analytics-apps.md#i̇zin-ve-vazgeçme).
- **Kuyruk.** Kayıtlar, sunucu yanıtlayana kadar uygulamanın özel depolamasında bekler: en çok 1.000, hiçbiri 72
  saatten eski değil, 100 kayıt ve 64 KiB'lık gruplar halinde, [çevrimdışı kuyruğun](analytics-apps.md#çevrimdışı-kuyruk)
  yeniden denemeleriyle. `YuvaAnalytics.queuedRequests()` kaç kaydın beklediğini döndürür (Swift'te `await`
  edin; Kotlin'de `suspend` bir işlevdir).
- **Hız.** Tek bir adres bir özelliğe dakikada 600 kayıt gönderebilir; fazlasına `429` yanıtlanır.

Kütüphaneler sizin yerinize `POST /client/v1/analytics/{key}/requests` adresine gönderir. Siz çağırmazsınız.

## Mağazalar için yanıtlar

İstek ölçümü açıkken bunu [Uygulamalarda analitik](analytics-apps.md#mağazalar-için-yanıtlar) sayfasındaki yanıtlara
ekleyin. Kayıtlar kimlik taşımaz.

### App Store

Kütüphanenin gizlilik manifestosu **Performance Data** türünü zaten bildirir: kullanıcının kimliğine bağlı değil,
izleme için kullanılmıyor, **Analytics** ve **App Functionality** amaçlı. App Store Connect'te **Performance Data**
için de **Data Not Linked to You** yanıtını verin.

### Google Play

**Data safety** altında şunu ekleyin:

| Veri türü | Toplanıyor | Paylaşılıyor | Notlar |
|---|---|---|---|
| Uygulama bilgisi ve performans → Tanılama | Evet | Hayır | Amaçlar: Analitik ve Uygulama işlevselliği. Geçici olarak işlenmez, çünkü sunucu toplu sayıları saklar. `requireConsent` açıksa kullanıcılar için isteğe bağlıdır. |
