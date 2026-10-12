# Gmail'den taşıyın

Bir Gmail posta kutusunun (`@gmail.com` ile biten adres) geçmişini Yuva'ya kopyalayın ve yeni e-postalarını
oraya yönlendirin. Yuva posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır;
iş ya da okul adresi için [Google Workspace rehberini](migrate-google-workspace.md) kullanın.

## 1. Uygulama şifresi oluşturun

Gmail, Yuva'yı yalnızca bir uygulama şifresiyle kabul eder: yalnızca Yuva için 16 karakterlik bir şifre.

1. Henüz açık değilse Google hesabında 2 Adımlı Doğrulama'yı açın.
2. Google hesabında [Uygulama şifreleri](https://myaccount.google.com/apppasswords)'ni açın, şifreye "Yuva"
   adını verip oluşturun.
3. Şifreyi kopyalayıp Yuva'nın Bağla adımına yapıştırın; kullanıcı adı Gmail adresidir.

2 Adımlı Doğrulama yalnızca güvenlik anahtarlarıyla kuruluysa, Gelişmiş Koruma açıksa ya da iş veya okul
hesabındaysanız bu seçeneği bulamazsınız. Google'ın sayfası:
[Uygulama şifreleriyle oturum açma](https://support.google.com/accounts/answer/185833).

## 2. Neler okunur

Yuva **Tüm postalar**'ı okur; böylece her mesaj bir kez kopyalanır. Gmail etiketleri Yuva etiketi olur.
Spam ve Çöp kutusu hiç okunmaz. Gmail'iniz olduğu gibi kalır.

## 3. Kopyayı yavaşlatanlar

Google, IMAP indirmelerini kullanıcı başına günde 2.500 MB ile sınırlar ve sınıra ulaşılınca erişimi 1 ile 24
saat arasında askıya alır. Yuva bunun altında, günde 1.500 MB'ta kalır; bu yüzden büyük bir posta kutusu
birkaç gün sürer. Kopyalama kendiliğinden duraklar ("Bekliyor: Gmail sınırı") ve ertesi gün devam eder; en
yeni konuşmalar önce gelir. Posta kutusunu eşitleyen diğer e-posta uygulamalarını kapatın, çünkü aynı payı
kullanırlar. Google'ın sayfası:
[Gmail bant genişliği sınırları](https://knowledge.workspace.google.com/admin/gmail/gmail-bandwidth-limits).

## 4. Posta kutusunu Yuva'ya yönlendirin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın.

1. Gmail'de Ayarlar › Tüm ayarları göster › Yönlendirme ve POP/IMAP'i açın.
2. "Yönlendirme adresi ekle"yi seçip yönlendirme adresini yapıştırın ve devam edin.
3. Google bu adrese bir onay gönderir. Yuva'nın Geçiş adımında görünür; kodunu Gmail'e girin ya da
   bağlantısını açın.
4. Gmail'de ayarları yenileyin, "Gelen postaların bir kopyasını şuraya yönlendir" ile bu adresi seçin,
   Gmail'in kopyasını gelen kutusunda tutun ve değişiklikleri kaydedin.

Gmail yalnızca yeni e-postaları yönlendirir, spam'i değil; geçmiş ilk adımlarda kopyalandı. Google'ın
sayfası: [Gmail iletilerini otomatik yönlendirme](https://support.google.com/mail/answer/10957).

Sonra denemek için Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 5. Bittikten sonra

Uygulama şifresini kaldırın: [Uygulama şifreleri](https://myaccount.google.com/apppasswords)'ni açın,
"Yuva"yı bulun ve "Kaldır"a tıklayın. Google hesabı şifresini değiştirmek de uygulama şifrelerini iptal eder.

## Çok büyük posta kutusu

Birkaç gigabaytlık bir posta kutusu, Gmail'in günlük sınırı yüzünden IMAP ile günler sürer. Bunun yerine bir Google Takeout
dosyasıyla başlayın: bağlantınızın izin verdiği hızda yüklenir ([nasıl](migrate-files.md)). Sonra posta
kutusunu yukarıdaki gibi IMAP ile bağlayın. Yuva yalnızca Takeout dosyasının en yeni mesajından sonraki
e-postaları okur, hiçbir mesajı iki kez içe aktarmaz ve geçişi her zamanki gibi yürütür. Panelde Bağla
adımı "Bunun yerine Takeout dosyası yükleyin" seçeneğini sunar; oradan başlayın.
