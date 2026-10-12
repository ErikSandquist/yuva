# GMX ya da mail.com'dan taşıyın

Bir GMX ya da mail.com posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını oraya yönlendirin.
Yuva posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## 1. IMAP'i açın ve uygulama şifresi oluşturun

1. GMX'te (ya da mail.com'da) **Email** sekmesini açın, **Settings → POP3 & IMAP**'e tıklayın, **Enable
   access to this account via POP3 and IMAP**'i işaretleyip kaydedin. IMAP uzun süre kullanılmazsa GMX bunu
   kendiliğinden kapatır; kopyadan kısa süre önce açın. mail.com IMAP'i yalnızca Premium hesaplarda sunar.
2. Web sitesinde oturum açın, sağ üstteki profil simgesine, sonra **Security Options → Manage
   application-specific passwords → Create new application-specific password**'e tıklayın. Adını "Yuva"
   koyup devam edin. GMX şifreyi yalnızca bir kez gösterir.
3. Yuva'nın Bağla adımına yapıştırın; kullanıcı adı tam adresinizdir. İki faktörlü kimlik doğrulama
   kullanıyorsanız uygulama şifresi hesap şifrenizin yerini alır.

Sayfalar: [GMX: POP3 ve IMAP'i etkinleştirme](https://support.gmx.com/pop-imap/toggle.html),
[GMX: uygulamaya özel şifreler](https://support.gmx.com/security/2fa/application-specific-passwords.html) ve
[mail.com: POP3 ve IMAP'i etkinleştirme](https://support.mail.com/pop-imap/toggle.html).

## 2. Hangi sunucu

Yuva sunucuyu adresinizden seçer, 993 portunda TLS ile: GMX adresleri için `imap.gmx.net` (GMX'in Almanca
yardımı onu söyler; İngilizce yardımı Yuva'nın da bildiği `imap.gmx.com`'u söyler) ve mail.com adresleri için
`imap.mail.com` ([GMX sunucu bilgileri](https://hilfe.gmx.net/pop-imap/imap/imap-serverdaten.html),
[mail.com sunucu bilgileri](https://support.mail.com/premium/imap/server.html)).

## 3. Neler okunur

Yuva gelen kutusunu, Gönderilenler'i ve kendi klasörlerinizi okur; klasörleri etikete dönüştürür. Spam, çöp
kutusu ve taslaklar hiç okunmaz. Sunucu sayfaları bir indirme sınırı belirtmez; Yuva tek bağlantıda ve
saniyede 4 MB'ta kalır, sunucu isterse yavaşlar.

## 4. Posta kutusunu Yuva'ya yönlendirin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın.

1. GMX'te adınızın baş harflerine, sonra **E-Mail-Einstellungen**, **Filterregeln** ve **Filterregeln
   erstellen**'e tıklayın.
2. Koşulu (örneğin tüm yeni e-postalar), eylem olarak **Weiterleiten an**'ı seçip yönlendirme adresini girin.
3. Kaydedin. GMX yönlendirme adresine bir etkinleştirme bağlantısı gönderir. Bağlantı hedef gelen kutusuna bir
   konuşma olarak düşer; tıklayın, yönlendirme başlar.

GMX'in sayfası bir kopyanın GMX'te kalıp kalmadığını söylemez: ilk e-postaların eski posta kutusunda durduğunu
kontrol edin. GMX'in sayfası: [Weiterleitung einrichten](https://hilfe.gmx.net/email/kontakte-organisieren/weiterleitung-einrichten.html).
mail.com'da filtre kurallarının bir **Forward** görevi vardır ([mail.com: görevler](https://support.mail.com/email/filter-rules/tasks.html));
sayfası da bir kopyanın kalıp kalmadığını söylemez. Sonra Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 5. Bittikten sonra

"Yuva" uygulama şifresini **Manage application-specific passwords** altından silin. POP3 ve IMAP'i
kullanmıyorsanız tekrar kapatın.
