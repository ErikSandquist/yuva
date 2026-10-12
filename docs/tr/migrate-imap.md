# Herhangi bir IMAP sunucusundan taşıyın

IMAP konuşan herhangi bir sunucudaki posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını
oraya gönderin. Yuva posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## 1. Sunucuyu bulun

Kaynak adımında Yuva sunucuyu adresin alan adından arar (e-posta sunucularından, e-posta ayar dosyasından ve
IMAP kaydından). Bulamazsa "Diğer IMAP sunucusu"nu seçip şunları girin:

| Alan | Değer |
|---|---|
| IMAP sunucusu | Sağlayıcınızın yardım sayfalarındaki ya da e-posta uygulamanızın hesap ayarlarındaki sunucu adı. |
| Port | TLS ile 993 ya da STARTTLS ile 143. |
| Kullanıcı adı | Genelde tam adres; sağlayıcınız yalnızca `@` öncesini isteyebilir. |
| Şifre | Posta kutusunun şifresi, sağlayıcınız sunuyorsa bir uygulama şifresi. |

Sunucunun sertifikası sunucu adı için geçerli değilse ve sunucu sizinse deneme sertifikanın parmak izini
gösterir; kontrol ettikten sonra bu taşıma için ona güvenebilirsiniz. Özel ya da yerel bir ağdaki sunucular,
işletmeci izin vermedikçe reddedilir ([`YUVA_MIGRATION_ALLOW_PRIVATE`](configuration.md#yuvaya-taşıma)).

## 2. Neler okunur

Yuva gelen kutusunu, Gönderilenler'i ve Arşiv klasörlerini önce sunucunun gönderdiği özniteliklerle, sonra
adla bulur; kendi klasörlerinizle birlikte okur ve klasörleri etikete dönüştürür. Spam, çöp kutusu ve
taslaklar hiç okunmaz. Yuva tek bağlantıda ve saniyede 4 MB'ta kalır, sunucu isterse yavaşlar; bağlantı sınırı
olan bir sunucuda kopyalama sırasında e-posta uygulamalarınızı kapatmanız gerekir.

## 3. E-postaları Yuva'ya gönderin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın ve sağlayıcınızın
ayarlarından ya da web arayüzünden yönlendirmeyi kurun (posta kutusunda kopya tutun) ya da e-posta sunucunuzun
yöneticisinden posta kutusunu oraya yönlendirmesini isteyin. Sunucuda yönlendirme adresi yoksa ya da alan adı
yalnızca destek için kullanılıyorsa MX kayıtlarını değiştirin ([e-posta rehberi](email.md)). Sonra Yuva'da
"Deneme e-postası gönder"e basın ([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 4. Bittikten sonra

Yalnızca Yuva için bir şifre oluşturduysanız sağlayıcınızda kaldırın.
