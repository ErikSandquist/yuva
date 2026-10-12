# Outlook.com'dan taşıyın

Bir Outlook.com, Hotmail, Live ya da MSN posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını
oraya gönderin. Yuva posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır; iş
ya da okul posta kutusu için [Microsoft 365 rehberini](migrate-microsoft365.md) kullanın.

## 1. Başlamadan önce

Yuva şifreyle ya da uygulama şifresiyle değil, **Microsoft ile oturum aç** ile bağlanır: Microsoft'un
Outlook.com IMAP ayarları bağlanma yolu olarak OAuth 2.0'ı gösterir. Microsoft'un sayfaları:
[POP, IMAP and SMTP settings for Outlook.com](https://support.microsoft.com/en-us/outlook/pop-imap-and-smtp-settings-for-outlook-com),
[Authenticate an IMAP connection using OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth).

Yuva yalnızca iki izin ister: posta kutusuna IMAP erişimi ve çevrimdışı erişim; böylece taşımanın sürdüğü
günler boyunca okumaya devam edebilir. Microsoft'un onay sayfası ilkini “IMAP aracılığıyla posta kutularına
okuma ve yazma erişimi” diye yazar, çünkü Microsoft'ta salt okunur bir IMAP izni yoktur. Yuva yine de yalnızca
okur: bağlantısı postayı silebilecek, taşıyabilecek ya da işaretleyebilecek her komutu reddeder
([bunun nasıl sağlandığı](migrate.md#yuva-posta-kutunuzla-neler-yapar-neler-yapmaz)). Yuva, bu iki izinden
fazlasını veren bir oturum açmayı reddeder.

## 2. Oturum açın

Bağla adımında **Microsoft ile oturum aç**'a basın, posta kutusunun kendi hesabıyla oturum açın ve erişime
izin verin. Tarayıcı başka bir Microsoft hesabını seçmekte ısrar ederse gizli pencere kullanın.

Önce bir uygulama kaydetmeniz gerekip gerekmediği sunucuya bağlıdır:

- **Sunucunun kendi Microsoft uygulaması var.** Yapılacak başka bir şey yok: adım yalnızca düğmeyi gösterir.
  Barındırılan sunucuların çoğunda vardır.
- **Yok.** Adım, kendi Microsoft hesabınızda aşağıdaki gibi kaydettiğiniz bir uygulamanın **Application
  (client) ID** ve **İstemci gizli anahtarı** değerlerini ister. İşletmeciniz size bir tane vermeyi de
  önerebilir: bkz. [İşletmeciler için Yuva'ya taşıma](migrate-operators.md#microsoft-uygulamasını-kaydetme).

## 3. Uygulama kaydedin (yalnızca adım isterse)

Adım, kopyalayacağınız **Yönlendirme URL'si**nin yanında bu adımları listeler. Microsoft'un sayfası:
[Register an application](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app).
Kayıt için bir Azure hesabı gerekir; ücretsiz olan yeter.

1. [Microsoft Entra yönetim merkezinde](https://entra.microsoft.com) **App registrations** bölümüne gidin ve
   **New registration**'ı seçin. Adını "Yuva" koyun.
2. **Supported account types** altında **Personal accounts only**'yi seçin.
3. **Redirect URI** altında **Web**'i seçin ve Yuva'daki Yönlendirme URL'sini girin.
4. **Certificates & secrets** bölümünde bir client secret ekleyin ve yalnızca bir kez gösterilen **Value**
   değerini kopyalayın.
5. **API permissions** bölümünde bir izin ekleyin: **APIs my organization uses → Office 365 Exchange Online →
   Delegated permissions → IMAP.AccessAsUser.All**. Microsoft Graph'tan **offline_access** iznini de ekleyin.
6. Overview sayfasından **Application (client) ID** değerini kopyalayın, gizli anahtarla birlikte Yuva'ya
   girin ve **Microsoft ile oturum aç**'a basın. **Directory (tenant) ID** alanını boş bırakın.

Yuva gizli anahtarı girer girmez mühürler ve bir daha göstermez.

## 4. E-postaları Yuva'ya gönderin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın. Outlook.com'da
**Ayarlar → Posta → Yönlendirme** yolunu açın (İngilizce arayüzde **Settings → Mail → Forwarding**),
**Enable forwarding**'i seçin, adresi girin, **Keep a copy of forwarded messages**'ı işaretleyin ve kaydedin.
Microsoft yönlendirmeyi açmadan önce iki adımlı doğrulama ister. Microsoft'un sayfası:
[Turn on or off automatic forwarding in Outlook.com](https://support.microsoft.com/en-us/office/turn-on-or-off-automatic-forwarding-in-outlook-com-6246987c-6c8f-4144-b255-14fc07007dad).

Yalnızca destek için kullanılan bir alan adında bunun yerine MX kayıtlarını Yuva'ya yöneltebilirsiniz
([e-posta rehberi](email.md)). Sonra Yuva'da "Deneme e-postası gönder"e basın
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 5. Oturum açma çalışmayı bırakırsa

Microsoft Yuva'nın oturum açmasını kabul etmez olduğunda (şifre değişti, erişim kaldırıldı ya da çok uzun
süre kullanılmadı) Yuva **Yeniden oturum aç**'ı gösterir. Aynı hesapla oturum açın; taşıma kaldığı yerden devam
eder (**Taşımaya devam et**).

## 6. Bittikten sonra

Yuva taşıma bitince belirtecini siler ama onu Microsoft'ta iptal edemez. Yuva'nın erişimini kaldırın:
Microsoft hesabınızda [Gizlilik → Uygulamalar ve hizmetler](https://account.microsoft.com/privacy/app-access)
bölümünü açın, Yuva'yı seçin ve izinlerini kaldırın. Yuva için bir uygulama kaydettiyseniz onu da Entra
yönetim merkezinde **App registrations** altından silin. Yönlendirmeyi olduğu gibi bırakın.
