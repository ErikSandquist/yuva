# iCloud Mail'den taşıyın

Bir iCloud Mail posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını oraya yönlendirin. Yuva
posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## 1. Uygulamaya özel parola oluşturun

iCloud Mail, Yuva'yı yalnızca uygulamaya özel bir parolayla kabul eder; bunun için Apple Hesabında iki
faktörlü kimlik doğrulama gerekir.

1. [account.apple.com](https://account.apple.com) adresinde oturum açın.
2. **Giriş ve Güvenlik** altında **Uygulamaya Özel Parolalar**'ı açın.
3. "Yuva" adlı yeni bir parola oluşturup ekrandaki adımları izleyin.
4. Yuva'nın Bağla adımına yapıştırın. Kullanıcı adı, iCloud adresinizin `@icloud.com` olmadan halidir.

Aynı anda en fazla 25 etkin uygulamaya özel parolanız olabilir. Apple'ın sayfası:
[Uygulamaya özel parolalarla oturum açma](https://support.apple.com/en-us/102654); sunucu ayarları
[iCloud Mail için posta sunucusu ayarları](https://support.apple.com/en-us/102525) sayfasındadır.

## 2. Neler okunur

Yuva gelen kutusunu, Gönderilenler'i, Arşiv'i ve kendi klasörlerinizi okur; klasörleri etikete dönüştürür.
İstenmeyen, çöp kutusu ve taslaklar hiç okunmaz. Apple bir indirme sınırı yayımlamaz; Yuva tek bağlantıda ve
saniyede 4 MB'ta kalır, iCloud isterse yavaşlar.

## 3. Posta kutusunu Yuva'ya yönlendirin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın.

1. [icloud.com/mail](https://www.icloud.com/mail) adresinde oturum açın, Posta kutuları listesinin üstündeki
   ayarlar düğmesine tıklayıp **Ayarlar**'ı seçin.
2. **Posta Yönlendirme**'yi seçin, "E-postalarımı şuraya yönlendir"i işaretleyip yönlendirme adresini girin.
3. "Yönlendirmeden sonra mesajları sil"i kapalı bırakın: iCloud'da tutulan kopyalar iCloud depolamanızı
   kullanır, ama eski posta kutusu eksiksiz kalır.

iCloud Mail'deki özel alan adına gelen e-postalar da yönlendirilir. Apple'ın sayfası:
[iCloud.com'daki Mail'de e-postayı otomatik yönlendirme](https://support.apple.com/guide/icloud/automatically-forward-email-mm6b1a3960/icloud).
Sonra Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 4. Bittikten sonra

"Yuva" parolasını account.apple.com'da **Uygulamaya Özel Parolalar** altından kaldırın ("Kaldır"). Apple
Hesabı parolasını değiştirmek tüm uygulamaya özel parolaları iptal eder.
