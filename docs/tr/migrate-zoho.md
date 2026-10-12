# Zoho Mail'den taşıyın

Bir Zoho Mail posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını oraya yönlendirin. Yuva
posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## 1. IMAP'e izin verin ve uygulama şifresi oluşturun

1. Zoho Mail'de **Settings → Mail Accounts**'u açın, birincil adresinize tıklayın, IMAP bölümündeki **IMAP
   Access**'i işaretleyip kaydedin.
2. Zoho hesabınızda iki faktörlü kimlik doğrulama açıksa Yuva'nın uygulamaya özel bir şifreye ihtiyacı var.
   Zoho Accounts'ta **Security → App passwords**'ü açın, **Generate New Password**'a tıklayın, adını "Yuva"
   koyup **Generate**'e tıklayın.
3. Şifreyi hemen kopyalayın: Zoho onu yalnızca bir kez gösterir. Yuva'nın Bağla adımına yapıştırın;
   kullanıcı adı tam Zoho Mail adresinizdir. İki faktörlü kimlik doğrulama yoksa hesap şifreniz çalışır.

Zoho'nun sayfaları: [IMAP ve SMTP yapılandırması](https://www.zoho.com/mail/help/imap-access.html) ve
[İki faktörlü kimlik doğrulama](https://www.zoho.com/mail/help/adminconsole/two-factor-authentication.html).

## 2. Hangi sunucu

Yuva sunucuyu adresinizden ve alan adının posta sunucularından bulur: kişisel adresler için
`imap.zoho.com`, kendi alan adınızdaki adresler için `imappro.zoho.com`; ikisi de 993 portunda TLS ile.
Diğer Zoho veri merkezlerindeki hesaplar aynı adların `.eu`, `.in`, `.com.au`, `.jp`, `.ca` ya da `.sa` ile
biten halini kullanır. Zoho'nun sayfası yalnızca `.com` sunucularını listeler, diğerleri için hesabınızdaki
sunucu ayrıntılarına yönlendirir; test bağlanamazsa bunları hesabınızdakilerle karşılaştırın.

## 3. Neler okunur

Yuva gelen kutusunu, Gönderilenler'i ve kendi klasörlerinizi okur; klasörleri etikete dönüştürür. Spam, çöp
kutusu ve taslaklar hiç okunmaz. Zoho IMAP'e yalnızca **Folder Settings** altında seçilen klasörleri sunar;
Seç adımında görünmeyen bir klasör orada gizlenmiştir. Sayfa bir indirme sınırı belirtmez; Yuva tek
bağlantıda ve saniyede 4 MB'ta kalır, Zoho isterse yavaşlar.

## 4. Posta kutusunu Yuva'ya yönlendirin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın.

1. Zoho Mail'de **Settings → Mail Accounts**'u açın, adresinizi seçin ve **Forwards** altına yönlendirme
   adresini girin.
2. Zoho yönlendirme adresine bir doğrulama kodu gönderir. Kod hedef gelen kutusuna bir konuşma olarak düşer;
   Settings → Mail Accounts'ta adresin yanındaki doğrula simgesine tıklayın, kodu girip onaylayın.
3. **Delete original email copy after forwarding**'i işaretlemeden bırakın, böylece eski posta kutusu
   eksiksiz kalır.

Yönlendirme Free plandaki yeni kullanıcılara sunulmaz ve Zoho art arda on başarısızlıktan sonra onu kapatır.
Zoho'nun sayfası: [E-posta yönlendirme](https://www.zoho.com/mail/help/email-forwarding.html).
Sonra Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 5. Bittikten sonra

"Yuva" şifresini Zoho Accounts'ta **Security → App passwords** altından silin.
