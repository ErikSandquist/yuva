# Yahoo Mail'den taşıyın

Bir Yahoo Mail posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını oraya yönlendirin. Yuva
posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## 1. Uygulama şifresi oluşturun

Yahoo, Yuva'yı yalnızca uygulama şifresiyle kabul eder.

1. Yahoo Hesap Güvenliği sayfanızı açın ve **External connections** altında **Create app password**'a
   tıklayın.
2. Adını "Yuva" koyup **Generate password**'e tıklayın.
3. Şifreyi kopyalayıp Yuva'nın Bağla adımına yapıştırın. Kullanıcı adı tam Yahoo adresinizdir. Hesap
   şifrenizi kullanmayın.

Yahoo, Yahoo'ya birkaç gündür art arda girdiğiniz bir tarayıcı ister, gizli pencere olmaz. Yahoo'nun
sayfaları: [3. taraf uygulama şifreleri oluşturma ve yönetme](https://help.yahoo.com/kb/SLN15241.html) ve
[IMAP sunucu ayarları](https://help.yahoo.com/kb/SLN4075.html).

## 2. Neler okunur

Yuva `imap.mail.yahoo.com` sunucusuna, 993 portunda TLS ile bağlanır. Gelen kutusunu, Gönderilenler'i,
Arşiv'i ve kendi klasörlerinizi okur; klasörleri etikete dönüştürür. Spam, çöp kutusu ve taslaklar hiç
okunmaz. Yahoo'nun sayfası bir indirme sınırı belirtmez; Yuva tek bağlantıda ve saniyede 4 MB'ta kalır,
Yahoo isterse yavaşlar.

## 3. Posta kutusunu Yuva'ya yönlendirin

Yahoo yalnızca ücretli hesaplarda (Yahoo Mail Plus) ve her ülkede değil e-posta yönlendirir. Geçiş
adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın.

1. Bilgisayarda Yahoo Mail'de **More options → Settings → Mailboxes**'ı açın ve birincil posta kutunuza
   tıklayın.
2. **Auto-forwarding** altına yönlendirme adresini girin ve **Verify**'a tıklayın.
3. Yahoo yönlendirme adresine bir onay e-postası gönderir. Hedef gelen kutusuna bir konuşma olarak düşer;
   açıp yönergeleri izleyin.

Yahoo'nun sayfası bir kopyanın Yahoo'da kalıp kalmadığını söylemez. Yahoo'nun sayfası:
[Otomatik e-posta yönlendirmeyi etkinleştirme](https://help.yahoo.com/kb/SLN36684.html). Ücretli hesabınız
yoksa gönderenlerden kanalın adresini kullanmalarını isteyin ya da sahibi olduğunuz bir alan adının MX
kayıtlarını değiştirin ([e-posta rehberi](email.md)). Sonra Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 4. Bittikten sonra

"Yuva" şifresini silin: Hesap Güvenliği sayfasında **External connections** altında uygulama şifresine
tıklayıp **Delete**'e basın.
