# Microsoft 365'ten taşıyın

Bir Microsoft 365 (Exchange Online) posta kutusunun geçmişini Yuva'ya kopyalayın ve yeni e-postalarını oraya
gönderin. Yuva posta kutusunu yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır; Outlook.com,
Hotmail ve Live için [Outlook.com rehberini](migrate-outlook.md) kullanın.

## 1. Başlamadan önce

Yuva şifreyle ya da uygulama şifresiyle değil, **Microsoft ile oturum aç** ile bağlanır: Microsoft, IMAP için
Temel kimlik doğrulamayı (Basic authentication) tüm kiracılarda kapattı ve bir yönetici onu geri açamaz.
Microsoft'un sayfası:
[Deprecation of Basic authentication in Exchange Online](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/deprecation-of-basic-authentication-exchange-online).

Yuva yalnızca iki izin ister: posta kutusuna IMAP erişimi ve çevrimdışı erişim; böylece taşımanın sürdüğü
günler boyunca okumaya devam edebilir. Microsoft'un onay sayfası ilkini “IMAP aracılığıyla posta kutularına
okuma ve yazma erişimi” diye yazar, çünkü Microsoft'ta salt okunur bir IMAP izni yoktur. Yuva yine de yalnızca
okur: bağlantısı postayı silebilecek, taşıyabilecek ya da işaretleyebilecek her komutu reddeder
([bunun nasıl sağlandığı](migrate.md#yuva-posta-kutunuzla-neler-yapar-neler-yapmaz)). Yuva, bu iki izinden
fazlasını veren bir oturum açmayı reddeder.

Önce üç şeyi kontrol edin ya da yöneticinize sorun:

- **Posta kutusunda IMAP açık.** Varsayılan olarak açıktır ama bir yönetici posta kutusu başına kapatabilir:
  Exchange yönetim merkezinde **Recipients → Mailboxes** altında posta kutusunu seçin, **Manage email apps
  settings**'e gidin ve **IMAP**'i **Enabled** yapın. Microsoft'un sayfası:
  [Managing email apps for user mailboxes](https://learn.microsoft.com/en-us/exchange/recipients-in-exchange-online/manage-user-mailboxes/managing-email-apps-for-user-mailboxes).
- **Kuruluşunuz uygulamaya onay vermenize izin veriyor** (aşağıda).
- **Hiçbir Koşullu Erişim ilkesi oturum açmayı engellemiyor** (aşağıda).

## 2. Oturum açın

Bağla adımında **Microsoft ile oturum aç**'a basın, posta kutusunun kendi hesabıyla oturum açın ve erişime
izin verin. Tarayıcı başka bir Microsoft hesabını seçmekte ısrar ederse gizli pencere kullanın.

Oturum açma adı posta kutusu adresinden farklı olabilir (kullanıcı asıl adı çoğu zaman farklıdır): Microsoft
hesabın posta kutusunu açmasına izin verirse Yuva hesabı kabul eder; paylaşılan bir posta kutusuna tam
erişimi olan kişi de buna girer. Posta kutusunu açamayan bir hesabı hiçbir şey saklamadan reddeder.
Microsoft'un sayfası (paylaşılan posta kutusu durumuyla):
[Authenticate an IMAP connection using OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth).

Önce bir uygulama kaydetmeniz gerekip gerekmediği sunucuya bağlıdır:

- **Sunucunun kendi Microsoft uygulaması var.** Yapılacak başka bir şey yok: adım yalnızca düğmeyi gösterir.
- **Yok.** Adım, kendi kiracınızda [4. adımdaki](#4-uygulama-kaydedin-yalnızca-adım-isterse) gibi kaydettiğiniz
  bir uygulamanın **Application (client) ID** ve **İstemci gizli anahtarı** değerlerini ister.

## 3. Microsoft yönetici istediğinde

- **Onay.** Çalışanların bir uygulamaya kendilerinin izin verip veremeyeceği kuruluşun ayarıdır; bazıları
  kullanıcıların yalnızca doğrulanmış yayıncıların uygulamalarına onay vermesine izin verir ve Yuva'nın
  uygulaması yayıncı doğrulamalı değildir. O zaman Yuva **Kuruluşunuzun yöneticisinin Yuva'ya izin vermesi
  gerekiyor** başlığını ve **Microsoft 365 yöneticiniz için bağlantı**yı gösterir. Bağlantıyı bir yöneticiye
  gönderin (sayfa hazır bir mesajı kopyalayabilir). Yönetici bağlantıyı açıp onayladığında Yuva'ya döner ve
  siz yeniden **Microsoft ile oturum aç**'a basarsınız. Microsoft'un sayfaları:
  [Overview of user and admin consent](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/user-admin-consent-overview),
  [Grant tenant-wide admin consent](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/grant-admin-consent).
- **Koşullu Erişim.** Kuruluşunuzun bir ilkesi oturum açmayı engellerse Yuva bunu söyler. Yöneticinizden izin
  vermesini isteyin ya da bunun yerine posta kutusunu bir [dışa aktarım dosyasıyla](migrate-files.md) taşıyın.
- **IMAP kapalı.** Microsoft sizi oturum açtırıp posta kutusu belirteci reddederse posta kutusunda IMAP
  kapalıdır: bkz. 1. adım.

## 4. Uygulama kaydedin (yalnızca adım isterse)

Adım, kopyalayacağınız **Yönlendirme URL'si**nin yanında bu adımları listeler. Microsoft'un sayfası:
[Register an application](https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-register-app).

1. [Microsoft Entra yönetim merkezinde](https://entra.microsoft.com) **App registrations** bölümüne gidin ve
   **New registration**'ı seçin. Adını "Yuva" koyun.
2. **Supported account types** altında, kuruluşunuzun posta kutuları için **Single tenant only**'yi; emin
   değilseniz **Any Entra ID Tenant + Personal Microsoft accounts**'u seçin.
3. **Redirect URI** altında **Web**'i seçin ve Yuva'daki Yönlendirme URL'sini girin.
4. **Certificates & secrets** bölümünde bir client secret ekleyin ve yalnızca bir kez gösterilen **Value**
   değerini kopyalayın.
5. **API permissions** bölümünde bir izin ekleyin: **APIs my organization uses → Office 365 Exchange Online →
   Delegated permissions → IMAP.AccessAsUser.All**. Microsoft Graph'tan **offline_access** iznini de ekleyin.
6. Overview sayfasından **Application (client) ID** değerini kopyalayıp gizli anahtarla birlikte Yuva'ya girin.
   Tek kiracılı bir uygulamada **Directory (tenant) ID** değerini de girin (aksi hâlde isteğe bağlıdır); sonra
   **Microsoft ile oturum aç**'a basın.

Yuva gizli anahtarı girer girmez mühürler ve bir daha göstermez.

## 5. E-postaları Yuva'ya gönderin

Geçiş adımında kanalın [yönlendirme adresini](email.md#yönlendirme-adresi) kopyalayın. İki yol Microsoft 365'te
bir kopya tutar:

- **Bir yönetici yönlendirir.** Microsoft 365 yönetim merkezinde **Users → Active users** bölümünde kullanıcıyı
  seçin, **Mail** sekmesini açın, **Manage email forwarding**'e gidin, **Forward all emails sent to this
  mailbox**'ı seçin, adresi girin ve bir kopyanın saklanmasını seçin. Kuruluşun dışındaki bir adrese
  yönlendirme, bir giden spam ilkesiyle sınırlanmış olabilir. Microsoft'un sayfaları:
  [Configure email forwarding](https://learn.microsoft.com/en-us/microsoft-365/admin/email/configure-email-forwarding?view=o365-worldwide),
  [Control automatic external email forwarding](https://learn.microsoft.com/en-us/defender-office-365/outbound-spam-policies-external-email-forwarding).
- **Yalnızca destek için kullanılan bir alan adında** MX kayıtlarını Yuva'ya yöneltin
  ([e-posta rehberi](email.md)). O zaman e-postalar artık Microsoft 365'e ulaşmaz.

Sonra Yuva'da "Deneme e-postası gönder"e basın ([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## 6. Oturum açma çalışmayı bırakırsa

Microsoft Yuva'nın oturum açmasını kabul etmez olduğunda (şifre değişti, erişim kaldırıldı ya da çok uzun
süre kullanılmadı) Yuva **Yeniden oturum aç**'ı gösterir. Aynı hesapla oturum açın; taşıma kaldığı yerden devam
eder (**Taşımaya devam et**).

## 7. Bittikten sonra

Yuva taşıma bitince belirtecini siler ama onu Microsoft'ta iptal edemez. Yuva'nın erişimini kaldırın:

- **Siz:** [Uygulamalarım (My Apps)](https://myapps.microsoft.com) sayfasını açın, Yuva'yı seçin, **Manage your
  application**'a ve **Revoke Permissions**'a gidin. Microsoft'un sayfası:
  [Edit or revoke application permissions in the My Apps portal](https://support.microsoft.com/en-us/account-billing/edit-or-revoke-application-permissions-in-the-my-apps-portal-169be2b4-ee26-4338-aea8-d19bb2f329ee).
- **Bir yönetici,** herkes için ve bir yöneticinin verdiği izinler için: Microsoft Entra yönetim merkezinde
  **Enterprise apps** altında Yuva'yı seçip kaldırın.
- Yuva için bir uygulama kaydettiyseniz onu **App registrations** altından silin.

Yönlendirmeyi ya da MX'i olduğu gibi bırakın.
