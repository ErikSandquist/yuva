# Uygulamalarda analitik

Swift ve Kotlin kütüphaneleri, iOS ve Android uygulamalarınızın ekranlarını, olaylarını ve gelirini web
sitenizle aynı analitik özelliğinde sayar; çerez, reklam kimliği ya da bir kişiyi bir açılıştan ötekine
izleyen hiçbir şey kullanmaz. Bu sayfa [Analitik](analytics.md) üzerine kuruludur; önce özelliği orada ekleyin.

## Platformu açın

Uygulamalar bir alan adı olmadan gönderir; bu yüzden bir özellik onları yalnızca açtığınız platformlar için
sayar.

1. Özelliğin **Ayarlar → Genel** bölümünde **Platformlar** altında **iOS**, **Android** ya da ikisini de
   işaretleyin.
2. Özelliğin anahtarını (`yuva_ak_…`) **Ayarlar → Kurulum** altından kopyalayın. Oradaki **Swift** ve **Kotlin**
   sekmelerinde aşağıdaki kodlar sunucunuza göre doldurulmuş olarak durur.

İşaretli olmayan bir platformdan gelen istek reddedilir (`403 platform_not_allowed`) ve
[Test olayları](#olayların-ulaştığını-denetleyin) altında **Reddedilen** olarak sayılır. Özellik oluşturulurken
panel ve API, yalnızca uygulamalar gönderse bile en az bir alan adı ister.

## Swift

iOS 17 ya da macOS 14 gerekir. Paketi Xcode'da (**File → Add Package Dependencies**) ya da `Package.swift`
içinde ekleyin ve `YuvaAnalytics` ürününü kullanın. `YuvaKit` gerekmez.

```swift
.package(url: "https://github.com/productdevbook/yuva", exact: "0.0.1"),
.product(name: "YuvaAnalytics", package: "yuva"),
```

Sunucunuzun sürümünü kullanın. Açılışta bir kez, ana aktörde yapılandırın:

```swift
import SwiftUI
import YuvaAnalytics

@main
struct MyApp: App {
    init() {
        YuvaAnalytics.configure(YuvaAnalyticsConfiguration(
            serverURL: URL(string: "https://support.example.com")!,
            key: "yuva_ak_xxxxxxxx"
        ))
    }

    var body: some Scene {
        WindowGroup {
            HomeView()
                .yuvaScreen("Home")
        }
    }
}
```

| Parametre | Anlamı |
|---|---|
| `serverURL` | Yuva sunucunuz. |
| `key` | Özelliğin anahtarı. |
| `appId` | Ekran adreslerinde uygulamayı adlandırır. Boş bırakılırsa bundle kimliğiniz. |
| `requireConsent` | Varsayılan `false`. Bkz. aşağıdaki İzin ve vazgeçme bölümü. |
| `debug` | Varsayılan `false`. Neyin kuyruğa alındığını, gönderildiğini ve atıldığını günlüğe yazar. |

### Ekranlar

`.yuvaScreen("Home")`, görünüm her göründüğünde bir ekran sayar. UIKit'te `screen`'i kendiniz çağırın:

```swift
override func viewDidAppear(_ animated: Bool) {
    super.viewDidAppear(animated)
    YuvaAnalytics.screen("Settings")
}
```

Ekran, `app://<uygulama kimliği>/<ekran>` adresinin bir sayfa görüntülemesidir; örneğin
`app://com.example.shop/Product%20Detail`. Uygulama kimliği alan adı yerine gösterilir ve özelliğin alan
adlarına karşı denetlenmez. Addaki baştaki ve sondaki eğik çizgiler atılır, kalanı yüzde kodlanır. Her
kullanıcı için aynı kalan adlar kullanın (`Product 4711` değil `Product`): rakamlar ekrana göre gruplanır.

### Olaylar ve gelir

```swift
YuvaAnalytics.track("Purchase", props: ["plan": "pro", "seats": 3], revenue: YuvaRevenue(amount: 9.99, currency: "EUR"))
YuvaAnalytics.track("Tapped help", interactive: false)
```

Bir olay, en son sayılan ekrana aittir; ilk ekrandan önce `app://<uygulama kimliği>/` adresine. Ad, özellik ve
gelir kuralları [Olaylar, hedefler ve dönüşüm hunileri](analytics-events.md) sayfasındakilerdir: değerleri metin,
sayı ya da boolean olan en fazla 30 özellik; `screen`, `pageview`, `engagement` ve `$` ya da `yuva_` ile
başlayan adlar ayrılmıştır. `interactive: false`, yalnızca bu olayı içeren bir ziyaretin hemen çıkma sayılmasını
sürdürür.

## Kotlin

Android 8.0 (API 26) gerekir. Kütüphane JitPack'te. Depoyu `settings.gradle.kts` içine ekleyin:

```kotlin
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
        maven("https://jitpack.io")
    }
}
```

ve bağımlılığı, sunucunuzun sürüm etiketiyle ekleyin:

```kotlin
implementation("com.github.productdevbook:yuva-analytics:v0.0.1")
```

Bir kez, `Application.onCreate` içinde yapılandırın:

```kotlin
import dev.yuva.analytics.YuvaAnalytics
import dev.yuva.analytics.YuvaAnalyticsConfiguration

class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        YuvaAnalytics.configure(
            this,
            YuvaAnalyticsConfiguration(serverUrl = "https://support.example.com", key = "yuva_ak_xxxxxxxx"),
        )
    }
}
```

`YuvaAnalyticsConfiguration` şunları alır: `serverUrl`, `key`, `appId` (boş bırakılırsa paket adınız),
`requireConsent` ve `debug`; anlamları yukarıdaki gibidir. Hata ayıklama iletileri Logcat'te `YuvaAnalytics`
etiketiyle görünür.

### Ekranlar

`TrackScreen`, adı değiştiğinde ya da bileşime girdiğinde bir Compose ekranı sayar. Navigation Compose ile ona
rota kalıbını verin; böylece `product/{id}` tek bir ekran olarak kalır:

```kotlin
import dev.yuva.analytics.TrackScreen

@Composable
fun AppNavigation(navController: NavHostController) {
    val entry by navController.currentBackStackEntryAsState()
    entry?.destination?.route?.let { TrackScreen(it) }
    NavHost(navController, startDestination = "home") { /* ... */ }
}
```

Compose dışında `YuvaAnalytics.screen("Settings")` çağırın. Ekran adresleri Swift'tekiyle aynı biçimde kurulur.

### Olaylar ve gelir

```kotlin
YuvaAnalytics.track(
    "Purchase",
    props = mapOf("plan" to "pro", "seats" to 3),
    revenue = YuvaRevenue(BigDecimal("9.99"), "EUR"),
)
YuvaAnalytics.track("Tapped help", interactive = false)
```

Metin, sayı ya da boolean olmayan bir özellik değeri metin olarak gönderilir. Kurallar Swift'tekilerle aynıdır.

Kütüphaneler, uygulamanızın kendi API'sine yaptığı çağrıların süresini de ölçebilir:
[iOS ve Android uygulamalarından API izleme](analytics-api-apps.md).

## İzin ve vazgeçme

İki kütüphane de aynı çalışır.

- **`requireConsent` olmadan**, kullanıcı vazgeçene kadar her şey sayılır. **Toplama**sı **Çerezsiz, herkes
  için** (varsayılan) olan bir özellik için doğrusu budur.
- **`requireConsent: true` ile**, siz `consent(true)` çağırana kadar hiçbir şey kuyruğa alınmaz; öncesindeki
  olaylar tutulmaz, atılır. Bundan sonra her olay kullanıcının iznini taşır; **Toplama**sı **Yalnızca izinle**
  olan bir özellik bunu ister. Seçim hatırlanmaz: kendiniz saklayın ve kullanıcının onay verdiği her
  açılışta `configure`'dan sonra yeniden `consent(true)` çağırın. `consent(false)` saymayı durdurur ve
  gönderilmeyi bekleyen her şeyi siler.
- **İzinle açılan özellikler.** `consent(true, persistent: true)`, `identify` ve `persistentId`, kabul eden kullanıcılara 13
  aylık bir kimlik verir ve ziyaretlerini bir kişiye bağlar: bkz. [İzinle açılan özellikler](analytics-consent.md#uygulamalar).
- **`optOut()`** saymayı kalıcı olarak durdurur: cihazda hatırlanır, gönderilmeyi bekleyen her şeyi siler ve
  `configure` sonraki her açılışta buna uyar. **`optIn()`** bunu geri alır, `isOptedOut` hangisinde olduğunu
  söyler. Bunu ayarlar ekranınızda sunun.
- **`flush()`** bekleyenleri şimdi gönderir. Kütüphaneler ayrıca bir olaydan birkaç saniye sonra, uygulama ön
  plana döndüğünde ve arka plana geçtiğinde de gönderir.

## Ne gönderilir, ne saklanır

Her istek şunları taşır: uygulamanın açılış için ürettiği ve yalnızca bellekte tuttuğu rastgele bir anahtar,
uygulamanın platformu (`ios` ya da `android`), sürümü, işletim sistemi sürümü, bir cihaz sınıfı (mobil, tablet ya
da masaüstü), ekran genişliği, dil ve olayların gerçekleştiği zamanlarla birlikte kendileri. Cihaz kimliği,
reklam kimliği ve çerez yoktur; kullanıcıyı sonradan tanımak için cihaza hiçbir şey yazılmaz. Sunucu,
bağlantının IP adresini gelir gelmez kısaltır, konum için kullanır ve saklamaz
([Analitik ve gizlilik](analytics-privacy.md)).

- **Bir açılış bir ziyarettir ve bir ziyaretçidir.** Anahtar her açılışta değişir; bu yüzden uygulamayı bir günde
  iki kez açan biri iki ziyaretçi olarak sayılır. Bir ziyareti web'deki aynı 30 dakikalık boşluk kuralı da
  bitirir. Bir uygulamanın ziyaretçilerini o günkü açılışlar olarak okuyun.
- **Swift kütüphanesi**, çalıştığı her Apple platformu için **iOS** bildirir; Mac uygulaması da dahil. Onun için
  **iOS**'u işaretleyin. Cihaz sınıfı orada `desktop`tur.

### Çevrimdışı kuyruk

Olaylar gönderilmeden önce uygulamanın özel depolamasına, yedeklerin dışında kalacak biçimde yazılır ve ancak
sunucu yanıt verdiğinde oradan çıkar.

| | |
|---|---|
| Boyut | En fazla 1.000 olay bekler. Aşıldığında en eskileri atılır. |
| Yaş | 72 saatten eski bir olay silinir, sunucu da onu atar (**Çok geç** olarak sayılır). |
| Zaman | Bir olay gerçekleştiği zamanı korur ve o günün altında saklanır. |
| Gruplar | İstek başına en fazla 100 olay ve 64 KiB. |
| Yeniden deneme | Ağ hatası ya da `5xx` yanıtı 5 saniye sonra, sonra 10, en çok 5 dakikaya kadar ikiye katlanarak yeniden denenir. `429` ve `503`, sunucunun `Retry-After` süresini bekler (yoksa bir dakika). Diğer her yanıt, `2xx` ya da `4xx`, grubu kuyruktan çıkarır. |

Uygulama kapandığında hâlâ kuyrukta olanlar, sonraki açılışta, onları üreten açılışın anahtarıyla gönderilir.

### Olayların ulaştığını denetleyin

Özelliğin **Ayarlar → Test olayları** sayfasını açın, sonra uygulamayı kullanın. Sayfa 5 saniyede bir yenilenir
ve açtığınız andan beri sunucunun neyi **Sayılan** olarak aldığını ve neyi attığını gösterir: **Çok geç**,
**Geçersiz**, **Reddedilen** (işaretli olmayan bir platform) ve **Diğer atılanlar**. Ekranlar **Canlı**
görünümünde bir dakika içinde belirir. **Cihazlar** görünümünde **Platform** ve **Uygulama sürümü** rakamları
kırılımlar ve onlara göre süzebilirsiniz.

## Mağazalar için yanıtlar

Bunlar, kütüphanenin kendi gönderdikleri içindir. Kendi özelliklerinizin ve uygulamanızın geri kalanının
topladıklarını siz ekleyin.

### App Store

Kütüphane, Xcode'un uygulamanızın gizlilik raporuna kattığı bir gizlilik manifestosu (`PrivacyInfo.xcprivacy`)
içerir. Manifesto izleme ve izleme alan adı bildirmez; iki tür toplanan veri bildirir, ikisi de
**kullanıcının kimliğine bağlı değil**, **izleme için kullanılmıyor** ve **analitik** için toplanıyor:
**Ürün etkileşimi** (ekranlar ve olaylar) ve **Yaklaşık konum** (sunucunun kısaltılmış IP adresinden bulduğu
yer). Ayrıca vazgeçmeyi hatırlamak için gereken `UserDefaults` erişimini `CA92.1` nedeniyle bildirir.

App Store Connect'te Ürün Etkileşimi ve Yaklaşık Konum için **Data Not Linked to You**, izleme için **No**
yanıtını verin. Tek kimlik, bir açılışın rastgele anahtarıdır.

### Google Play

Kütüphanenin manifestosu `AD_ID` iznini kaldırır; yani reklam kimliği eklemez. **Veri güvenliği** bölümünde:

| Veri türü | Toplanıyor | Paylaşılıyor | Notlar |
|---|---|---|---|
| Uygulama etkinliği → Uygulama etkileşimleri | Evet | Hayır | Ekranlar ve olaylar. Amaç: Analitik. |
| Konum → Yaklaşık konum | Evet | Hayır | Gelir gelmez IP adresinden bulunur; adres kısaltılır ve saklanmaz: geçici olarak işlenir. Amaç: Analitik. |

Veri yalnızca yapılandırdığınız Yuva sunucusuna gider. `serverURL` `https://` ile başlıyorsa aktarım sırasında
şifreli olduğunu belirtin.
