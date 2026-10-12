# Fastmail'den taşıyın

Bir Fastmail posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını oraya yönlendirin. Yuva
posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## 1. Uygulama şifresi oluşturun

Fastmail, Yuva'yı yalnızca uygulama şifresiyle kabul eder; Basic planlarda IMAP erişimi olmadığı için
uygulama şifresi de yoktur.

1. Fastmail'de **Settings → Privacy & Security**'yi açın, **Connected apps & API tokens** altında uygulama
   şifrelerini yönetip yenisini oluşturun. İstenirse şifrenizi onaylayın.
2. Adını "Yuva" koyun, erişimi **Mail, Contacts & Calendars** olarak bırakın ve şifreyi oluşturun.
3. Şifreyi kopyalayıp Yuva'nın Bağla adımına yapıştırın. Kullanıcı adı tam Fastmail adresinizdir.

Fastmail'in sayfaları: [Uygulama şifreleri](https://www.fastmail.help/hc/en-us/articles/360058752854-App-passwords)
ve [Sunucu adları ve portlar](https://www.fastmail.help/hc/en-us/articles/1500000278342-Server-names-and-ports).

## 2. Neler okunur

Yuva `imap.fastmail.com` sunucusuna, 993 portunda TLS ile bağlanır. Gelen kutusunu, Gönderilenler'i,
Arşiv'i ve kendi klasörlerinizi okur; klasörleri etikete dönüştürür. Spam, çöp kutusu ve taslaklar hiç
okunmaz. Fastmail'in sayfası bir indirme sınırı belirtmez; Yuva tek bağlantıda ve saniyede 4 MB'ta kalır,
Fastmail isterse yavaşlar.

## 3. Posta kutusunu Yuva'ya yönlendirin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın.

1. Fastmail'de **Settings → Filters & Rules**'u açın ve **Create rule**'a tıklayın.
2. Koşul olarak **All messages**'ı, eylem olarak **Send a copy to**'yu seçip yönlendirme adresini girin.
3. Kuralı kaydedin. Eski posta kutusu kopyasını tutar.

Discard kuralına takılan ya da spam diye süzülen e-postalar yönlendirilmez; Fastmail'de kalır ve Yuva
orayı okumaz. Fastmail'in sayfası: [E-posta yönlendirmeyi kurma](https://www.fastmail.help/hc/en-us/articles/360058753434-Set-up-mail-forwarding).
Sonra Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 4. Bittikten sonra

**Settings → Privacy & Security**'de uygulama şifrelerini açıp "Yuva"yı kaldırın. Kaldırma geri alınamaz.
