# Google Workspace'ten taşıyın

Bir Google Workspace posta kutusunun (kendi alan adınız, Gmail üzerinde) geçmişini Yuva'ya kopyalayın ve
yeni e-postalarını oraya gönderin. Yuva posta kutusunu yalnızca okur. Genel bakış
[Yuva'ya taşı](migrate.md) sayfasındadır; `@gmail.com` ile biten adres için [Gmail rehberini](migrate-gmail.md)
kullanın.

## 1. Başlamadan önce

Google, 14 Mart 2025'ten beri IMAP için düz şifre kabul etmez; Yuva bir uygulama şifresiyle bağlanır. Google
her iş ya da okul hesabında uygulama şifresi sunmaz; bu yüzden hesap için yöneticinizin yardımı gerekebilir:

- Bir yönetici IMAP'e izin verir: Yönetici konsolunda **Menü → Uygulamalar → Google Workspace → Gmail →
  Son kullanıcı erişimi**, **POP ve IMAP erişimi** altında "Tüm kullanıcılar için IMAP erişimini etkinleştir"i
  işaretleyin ve herhangi bir e-posta istemcisine izin verin (Yuva OAuth kullanmaz). Her kişi ayrıca kendi
  Gmail ayarlarında, **Yönlendirme ve POP/IMAP** altında IMAP'i açar. Google'ın sayfası:
  [Kullanıcılar için POP ve IMAP'i açma ya da kapatma](https://knowledge.workspace.google.com/admin/sync/turn-pop-and-imap-on-or-off-for-users).
- Hesapta 2 Adımlı Doğrulama açık olmalıdır. [Uygulama şifreleri](https://myaccount.google.com/apppasswords)
  yine de yoksa Google nedenler arasında iş ya da okul hesabını sayar; yöneticinize sorun. Hesap uygulama
  şifresi sunmuyorsa Yuva ona bağlanamaz. Google'ın sayfası:
  [Uygulama şifreleriyle oturum açma](https://support.google.com/accounts/answer/185833).

## 2. Uygulama şifresini oluşturun

[Uygulama şifreleri](https://myaccount.google.com/apppasswords)'ni açın, şifreye "Yuva" adını verip
oluşturun ve Yuva'nın Bağla adımına, kullanıcı adı olarak posta kutusu adresiyle yapıştırın.

## 3. Neler okunur, neler yavaşlatır

Yuva **Tüm postalar**'ı mesaj başına bir kez okur; Gmail etiketleri Yuva etiketi olur; Spam ve Çöp kutusu
hiç okunmaz. Google, IMAP indirmelerini kullanıcı başına günde 2.500 MB ile sınırlar ve sınıra ulaşılınca
erişimi 1 ile 24 saat arasında askıya alır. Yuva bunun altında, günde 1.500 MB'ta kalır; bu yüzden büyük bir
posta kutusu birkaç gün sürer, kendiliğinden duraklar ve ertesi gün devam eder. Google'ın sayfası:
[Gmail bant genişliği sınırları](https://knowledge.workspace.google.com/admin/gmail/gmail-bandwidth-limits).

## 4. E-postaları Yuva'ya gönderin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın. İki yol Google'da bir
kopya tutar:

- **Posta kutusunun sahibi yönlendirir**, [Gmail rehberindeki](migrate-gmail.md#4-posta-kutusunu-yuvaya-yönlendirin)
  gibi. Bir yönetici yönlendirmeyi sınırlamış olabilir.
- **Bir yönetici yönlendirir.** Yönetici konsolundaki Google **Yönlendirme** ayarı bir kullanıcının
  e-postalarının kopyasını başka bir adrese teslim eder; çift teslimatlı **Varsayılan yönlendirme** aynısını
  tüm kuruluş için yapar. Google'ın sayfası:
  [E-posta yönlendirme ve teslim seçenekleri](https://knowledge.workspace.google.com/admin/gmail/advanced/email-routing-and-delivery-options-for-google-workspace).

Yalnızca destek için kullanılan bir alan adında bunun yerine MX kayıtlarını Yuva'ya yöneltebilirsiniz
([e-posta rehberi](email.md)). Sonra Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 5. Bittikten sonra

Uygulama şifresini [Uygulama şifreleri](https://myaccount.google.com/apppasswords) sayfasından kaldırın.
