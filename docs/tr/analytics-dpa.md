# Analitik için veri işleme

Bir Yuva sunucusunu sizin için başkası işletiyorsa (barındırılan hizmet), Analytics'i kullandığınızda o operatör
ziyaretçilerinizin verisini işler. Bu, GDPR'ın 28. maddesinin sizinle operatör arasında istediği sözleşmenin
taslağıdır. Avukatınızla uyarlayacağınız bir başlangıçtır; hukuki danışmanlık değildir ve imzalanmış bir sözleşme
değildir. Kendi sunucunuzda veri sorumlusu sizsiniz ve veriyi sizin için kimse işlemez; bunların hiçbirine
ihtiyacınız yok.

## Taraflar ve roller

| Taraf | Rol | Ne yapar |
|---|---|---|
| Çalışma alanı sahibi (siz) | Veri sorumlusu | Ziyaretçilerin neden ve nasıl sayıldığına karar verir: hangi web siteleri, hangi ayarlar, ne kadar süre. Ziyaretçileri bilgilendirir ([gizlilik bildirimi](analytics-privacy.md#gizlilik-bildiriminiz)). Taleplerini yanıtlar. |
| Sunucu operatörü | Veri işleyen | Sunucuyu işletir, sayfalarınızın gönderdiği olayları alır, ayarların dediği gibi saklar ve siler, onlarla başka bir şey yapmaz. |

## Neler işlenir

| Öğe | Ayrıntı |
|---|---|
| Konu | Çalışma alanınızın web siteleri için kitle ölçümü. |
| İlgili kişiler | Bu web sitelerinin ziyaretçileri. |
| Veri kategorileri | Kısaltılmış IP adresi (bellekte kullanılır, saklanmaz), tarayıcı ve cihaz türü, ziyaretçi özeti, sayfa host'u ve yolu, yönlendiren, `utm_*` parametreleri, ülke, bölge ve şehir (çocuklara yönelik web sitesinde yalnızca ülke), dil, ekran boyutu sınıfı. Tam liste [gizlilik sayfasında](analytics-privacy.md#ne-sakladığı). |
| Özel nitelikli veri | İstenmez. Sunucu e-posta adresi biçimindeki değerleri saklamaz. |
| Süre | Ayrıntılı kayıtlar varsayılan 90 gün (operatör ayarlar, 7 ila 400); günlük toplamlar varsayılan 25 ay (web sitesi başına `retention_months`); gerisi için sözleşmenin süresi. |

## Talimatlar

Veri işleyen, veriyi yalnızca sizin belgelenmiş talimatlarınıza göre işler (m. 28(3)(a)). Bir web sitesinin
ayarları bu talimatlardır: alan adları, kitle, toplama modu, sayılmayan sayfalar ve adresler, yol kuralları ve
saklama.

## Gizlilik ve güvenlik

- Veriye erişebilen personel gizlilik yükümlülüğü altındadır.
- Veri TLS üzerinden taşınır. Operatör, saklanan verileri ve yedekleri nasıl koruduğunu ve kimin hangi role göre
  eriştiğini anlatır.
- Operatör 32. maddenin istediği güvenlik önlemlerini alır (m. 28(3)(c)).

## Alt işleyenler

Yuva Analytics'in kendisi hiçbirini kullanmaz: konum araması ve tarayıcı ayrıştırma sunucunun içinde çalışır.
Operatörün altyapı sağlayıcıları (barındırma, depolama, yedekleme) veriyi onun adına işleyebilir. Operatör bunları
adlandırır ya da nasıl öğreneceğinizi söyler ve başka bir işleyeni hangi koşullarla devreye aldığını belirtir
(m. 28(3)(d)).

## Taleplere yardım

Varsayılan modda bir ziyaretçinin kayıtları bulunamaz, çünkü bir günün ötesinde hiçbir şey ziyaretçiyi tanımlamaz.
Veri işleyen, taleplere bu açıklamayla yanıt vermenize yardım eder. Veriyi sildiğinizde kalıcı olarak silinir.

## Silme ve iade

Verinizi istediğiniz zaman kendiniz alabilir ve silebilirsiniz:

- `POST /v1/analytics/properties/{propertyId}/query` çağrısı `?format=csv` ile rakamları CSV olarak verir.
- `POST /v1/analytics/properties/{propertyId}/delete-data` bir gün aralığını ya da `all: true` ile her şeyi siler;
  `DELETE /v1/analytics/properties/{propertyId}` web sitesini verisiyle birlikte kaldırır.
- Sözleşme bittiğinde operatör kalanı sizin seçiminize göre siler ya da iade eder ve yasa saklamayı gerektirmedikçe
  kopyaları siler. Yedekler operatörün belirttiği yedekleme süresi içinde yok olur.

## Denetimler

Operatör bu sözleşmeye uyduğunu göstermek için gereken bilgiyi verir ve sizin ya da belirlediğiniz bir denetçinin
yapacağı denetimlere izin verir (m. 28(3)(h)).

## Aktarımlar

Operatör sunucunun ve yedeklerinin hangi ülkede olduğunu belirtir ve yasanın istediği her yerde geçerli bir aktarım
mekanizmasına dayanır.

## Kaynak

[GDPR m. 28, veri işleyen](https://www.legislation.gov.uk/eur/2016/679/article/28) (UK GDPR metni; AB GDPR'sinde
aynı madde vardır).
