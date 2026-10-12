# Teslim edilebilirlik

Bültenlerinizin gelen kutusuna ulaşması çoğunlukla gönderen alan adının doğru kurulmasına ve insanların
e-postanızı spam olarak işaretlememesine bağlıdır. Yuva, bir e-posta kanalının kurulumunu ona deneme e-postası
göndererek sınayan bir denetim sunar; bu sayfa sonucun nasıl okunacağını ve bulunanların nasıl düzeltileceğini
anlatır.

[Bülten](broadcasts.md) gönderen bir SMTP hesabı olan bir [e-posta kanalına](email.md) ve kanalın gönderdiği
alan adının DNS'ine erişime ihtiyacınız var.

## 1. Denetimi çalıştırın

Panelde: Ayarlar, gelen kutusu, Müşteriler nereden yazıyor; e-posta kanalını açın, sonra Bülten gönderimi
bölümündeki "Teslim edilebilirliği denetle ve DKIM kur". "Denetim yap"a basın.

Yuva, kanalın bülten hesabı üzerinden kanalın kendi adresine, bir bültenin taşıdığı başlıklarla bir deneme
e-postası gönderir; alan sunucunun onu nasıl değerlendirdiğini okur ve DNS kayıtlarınıza bakar. Sonuç bir dakika
içinde, en geç 10 dakika içinde görünür. Kanal başına saatte altı denetim yapabilirsiniz. Deneme e-postası hiçbir
zaman bir konuşma olmaz.

İlk bültenden önce, sonra da DNS ya da SMTP hesabında her değişiklikten sonra çalıştırın. Bir kanal denetlenene
kadar bülten gözden geçirmesi uyarır.

### Sonucu okumak

| Satır | Ne söyler |
|---|---|
| SPF | E-postayı gönderen sunucunun SPF kaydına göre buna izinli olup olmadığını. Ayrıca denetlenen alan adının gönderen alan adınız olup olmadığını da söyler. SPF yalnızca sağlayıcınızın alan adı için geçiyorsa DMARC yalnızca DKIM'e dayanır; DKIM geçtiği sürece sorun yoktur. |
| DKIM | Bir imzanın geçip geçmediğini, hangi alan adı için olduğunu ve bu alan adının gönderen alan adınız olup olmadığını. İmzanın Gmail ve Yahoo'nun istediği `List-Unsubscribe` ve `List-Unsubscribe-Post` başlıklarını kapsayıp kapsamadığını da söyler. |
| DMARC | DMARC'ın geçip geçmediğini ve ilkenizi: `none` (alıcılar yalnızca rapor verir), `quarantine` (başarısız e-posta spama gider) ya da `reject`. DMARC kaydı yoksa bir kayıt eklemeniz söylenir. |
| MX | Kanal adresinin alan adının posta sunucusu olup olmadığını. Bültene gelen yanıtlar oraya ulaşır. |
| Yuva DKIM | Yalnızca kanalın [kendi anahtarı](#4-yuva-dkim-ile-imzalayın) varsa: DNS'in onunla yanıt verip vermediği. |

Bülten gözden geçirmesi aynı sonucu kullanır. Kanal hiç denetlenmediyse, denetim bitmediyse, DMARC yoksa, DKIM
gönderen alan adınızla eşleşmiyorsa ya da abonelikten çıkma başlıkları imzalı değilse uyarır. Hiçbir DKIM imzası
geçmediyse göndermeyi engeller.

Denetim bir sonuç okuyamazsa şunlardan birini görürsünüz:

| Mesaj | Ne yapmalı |
|---|---|
| 10 dakika içinde geri gelmedi (`timeout`) | Yalnızca DNS denetlendi. Kanalın adresine giden e-postanın Yuva'ya ulaştığından (yönlendirme ya da MX) emin olun, sonra yeniden denetleyin. |
| Güvenilen bir sunucudan sonuç olmadan ulaştı (`unverifiable`) | Sunucunun yöneticisi Yuva'nın güvendiği alan sunucuyu ([`YUVA_INGRESS_AUTHSERV_ID`](configuration.md#gelen-e-posta)) tanımlamamış; bu yüzden SPF, DKIM ve DMARC okunamadı. Yalnızca DNS denetlendi. |
| Gönderilemedi (`failed`) | SMTP ayarlarını denetleyin; SMTP sunucusunun yanıtı gösterilir. |

## 2. Gönderen alan adını kurun

Bu kayıtlar kanalın From adresinin alan adına eklenir. SPF, DKIM ve DMARC geçmeli; DMARC için SPF ya da DKIM'in
kendi alan adınız için olması (hizalı) gerekir.

| Kayıt | Ne eklenir |
|---|---|
| SPF | Ad başına tek bir TXT kaydı; sağlayıcınızı içerir. |
| DKIM | Sağlayıcınızın verdiği kayıtlar ya da [Yuva DKIM](#4-yuva-dkim-ile-imzalayın). |
| DMARC | `_dmarc.example.com TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"`. Raporlar doğru görününce `p=quarantine` ilkesine geçin. |

Kayıtların yayılması zaman alır. Sağlayıcınız doğrulandıklarını söyleyince denetimi yeniden çalıştırın.

### Amazon SES

1. SES konsolunda gönderen alan adınız için Easy DKIM ile bir alan adı kimliği oluşturun. SES üç CNAME kaydı
   gösterir; bunları ekleyin. Doğrulama 72 saate kadar sürebilir. Easy DKIM sizin alan adınızla,
   abonelikten çıkma başlıkları dahil imzalar; bu yüzden SES kanalı buna dayanır ve
   [Yuva DKIM](#4-yuva-dkim-ile-imzalayın) kullanmaz.
2. SPF'nin kendi alan adınız için de geçmesi adına özel bir MAIL FROM alan adı kurun: kimliğin başka hiçbir şey için
   kullanmadığınız bir alt alan adını seçin (örneğin `bounce.example.com`); üzerine konsolun gösterdiği
   değerlerle bir MX kaydı (`10 feedback-smtp.<bölge>.amazonses.com`) ve bir TXT kaydı
   (`"v=spf1 include:amazonses.com ~all"`) ekleyin. Bu adda tek bir MX kaydı olmalıdır.
3. Yeni bir SES hesabı sandbox'tadır: yalnızca doğrulanmış adreslere, günde 200 ileti ve saniyede bir iletiyle
   gönderebilir. İlk bültenden önce SES konsolunda (Account dashboard) üretim erişimi isteyin. Sandbox durumu
   bölgeye özeldir.
4. Konsolda SMTP settings, "Create SMTP credentials". Gösterilen kullanıcı adı ve parola AWS erişim anahtarlarınız
   değildir ve tek bir bölge içindir: aynı bölgenin SMTP uç noktasını kullanın. Gösterilince kaydedin; bir daha
   görüntülenemez.
5. Kanalda "Bültenler için ayrı bir hesap kullan"ı açın ve uç noktayı, portu, şifrelemeyi ve bu bilgileri girin.
6. Geri dönenler ve şikâyetler için [Geri dönen e-postalar](email.md#geri-dönen-e-postalar) bölümünde anlatıldığı
   gibi SNS'i bağlayın.

### Postmark

1. Postmark'ta gönderen alan adınızı ekleyin ve gösterdiği DKIM ile Return-Path kayıtlarını ekleyin.
2. Sunucunun Settings sayfasında SMTP'yi açın. Kullanıcı adı ve parola, sunucunun API belirtecidir (ya da bir SMTP
   belirtecinin erişim anahtarı ve gizli anahtarı).
3. Kanalda bülten akışı için `smtp-broadcasts.postmarkapp.com` sunucusunu kullanın (`smtp.postmarkapp.com`
   işlem e-postalarınındır); port 587, 2525 ya da 25, STARTTLS ile.

### Başka bir SMTP aktarıcısı

1. Sağlayıcınızın panelinde gönderen alan adınızı ekleyin. Size SMTP sunucusunu, portu, şifrelemeyi, kullanıcı adı
   ve parolayı, DKIM kayıtlarını ve SPF kaydınıza ekleyeceğiniz `include:` değerini verir.
2. Ad başına tek SPF kaydı bırakın: `include:` değerini var olan kayda ekleyin.
3. Sağlayıcı kendi alan adınızla imzalayamıyorsa [Yuva DKIM](#4-yuva-dkim-ile-imzalayın) kullanın.
4. SMTP bilgilerini kanalda "Bültenler için ayrı bir hesap kullan" altına girin. Ayrı hesap, pazarlama
   e-postalarına gelen şikâyetleri destek yanıtlarınızdan uzak tutar.

## 3. Yeniden denetleyin

Denetimi çalıştırın. SPF ve DKIM'in geçmesini, DKIM'in kendi alan adınız için olmasını ve abonelikten çıkma
başlıklarını kapsamasını, bir de DMARC kaydı olmasını hedefleyin. Başlamak için `p=none` ilkeli bir DMARC kaydı
yeterlidir.

## 4. Yuva DKIM ile imzalayın

Denetim DKIM'in yalnızca sağlayıcınızın alan adı için geçtiğini ya da imzanın abonelikten çıkma başlıklarını
kapsamadığını söylüyorsa ya da sağlayıcınız kendi alan adınızla hiç imzalayamıyorsa kullanın. Yuva o zaman kanalın
bültenlerini, deneme, onay ve hoş geldin e-postalarını abonelikten çıkma başlıklarını da kapsayacak şekilde
kendisi imzalar. Sağlayıcınızın imzası yanında kalır; destek yanıtları değişmez.

Yuva DKIM, Amazon SES dışındaki aktarıcılar içindir. SES, Yuva imzaladıktan sonra `From`, `To`, `Reply-To` ve
`Date` başlıklarını değiştirir; bu, ondan önce eklenen her imzayı bozar. SES ayrıca her e-postayı doğrulanmış
alan adınızla kendisi imzalar. Bu yüzden Yuva, bültenleri SES üzerinden giden bir kanalda kendi DKIM'ini açmayı
reddeder ve daha önce açılmış bir anahtar olsa bile SES üzerinden imzalamaz; kanalın gönderim sayfası nedenini
gösterir.

1. Kanalın gönderim sayfasını açın (yukarıda) ve Yuva DKIM'i bulun. Bir seçici seçin (küçük harf, rakam ve tire;
   varsayılan `yuva`) ve "Anahtar oluştur"a basın. Anahtar RSA 2048'dir; özel anahtar sunucudan hiç çıkmaz.
2. Gösterilen TXT kaydını gösterilen adda ekleyin (ad, gönderen alan adınızla biter). DNS sağlayıcınız bir metni
   255 karakterle sınırlıyorsa değeri tek bir kaydın birkaç metnine bölün.
3. Birkaç dakika bekleyin (bazı sağlayıcılarda daha uzun sürer), sonra "DNS'i denetle ve imzalamaya başla"ya
   basın. Bunu kayıt yayımlanınca bir sahip ya da yönetici yapar.
4. Denetimi yeniden çalıştırın. Yuva DKIM satırı geçmeli.

"İmzalamayı durdur" hemen kapatır; "Anahtarı sil" anahtarı kaldırır, ardından DNS kaydını silebilirsiniz. Yeni bir
anahtar yeni bir kayıt ister. Kanalın gönderen alan adını değiştirmek, yeni alan adı için yeniden açana kadar
imzalamayı kapatır.

## 5. Gmail ve Yahoo

Gmail ve Yahoo toplu e-posta gönderenler için kurallar koyar. Gmail bunları Gmail adreslerine günde 5.000 iletiden
itibaren uygular; Yahoo'nun yönergeleri bir sayı vermez.

- **Kimlik doğrulayın.** SPF ve DKIM ile bir DMARC kaydı (`none` ilkesine izin verilir). From adresindeki alan adı
  SPF ya da DKIM alan adıyla hizalanmalıdır. Gmail en az 1.024 bit DKIM anahtarı ister, 2.048'i önerir.
- **Tek tıkla abonelikten çıkma.** `List-Unsubscribe` başlığı, `List-Unsubscribe-Post: List-Unsubscribe=One-Click`
  ([RFC 8058](https://www.rfc-editor.org/rfc/rfc8058)) ve e-postada görünür bir abonelikten çıkma bağlantısı.
  Yahoo, istekleri iki gün içinde yerine getirmenizi söyler. Yuva her bültene iki başlığı ve bağlantıyı ekler ve
  hemen abonelikten çıkarır.
- **Spam şikâyetlerini düşük tutun.** %0,3'ün altında; Google %0,1'in altını önerir. Yuva bir bülteni %0,3'te
  duraklatır ([Yuva'nın yaptığı duraklatmalar](broadcasts.md#yuvanın-yaptığı-duraklatmalar)).
- **TLS ve gönderen sunucular için geçerli DNS.** Gmail TLS bağlantısı ve gönderen IP'ler için ileri ve ters DNS
  ister. Bu SMTP sağlayıcınızın işidir.

Kaynaklar: [Google'ın gönderen yönergeleri](https://support.google.com/a/answer/81126) ve
[Yahoo'nun gönderen en iyi uygulamaları](https://senders.yahooinc.com/best-practices/). İkisi de değişir;
kurulum yaparken okuyun.

### Google Postmaster Tools

[Postmaster Tools](https://support.google.com/mail/answer/9981691), Gmail'in e-postanızda gördüklerini gösterir:
kişisel Gmail adreslerine giden e-posta için spam oranı, itibar, kimlik doğrulama ve teslim hataları. Bir Google
hesabıyla girin, e-postanızın imzalandığı ya da gönderildiği alan adını (DKIM `d=` alan adı ya da SPF/Return-Path
alan adı) ekleyin ve Google'ın gösterdiği TXT ya da CNAME kaydıyla doğrulayın. Alan adı doğrulanana kadar hiçbir
şey görünmez; az e-posta giden günlerde veri gösterilmez.

## 6. Yavaş başlayın

Yeni bir gönderen alan adının itibarı yoktur ve büyük bir ilk gönderim tam da spam gibi görünür.

- **Barındırılan sunucular.** Yeni bir çalışma alanının [onay istemesi](broadcasts.md#barındırılan-sunucularda-onay)
  gerekebilir, sonra kendiliğinden ısınır: ilk gün 500 alıcı, her temiz günden sonra iki katı (en az 100 e-posta
  gönderilmiş, geri dönme %3'ün, şikâyet %0,1'in altında). Kötü bir gün bir katlamayı geri alır. Operatör başka
  bir sınır koyabilir.
- **Kendi sunucunuz.** Koyduğunuzdan başka sınır yoktur. Çalışma alanı başına günlük alıcıyı sınırlamak için
  [`YUVA_BROADCAST_DAILY_RECIPIENTS`](configuration.md#bültenler) değerini ayarlayın (sınıra ulaşan gönderim
  ertesi gün sürer) ve kanalın hızını sağlayıcınızın izin verdiğinin altında tutun. Sınırı ilk haftalarda kendiniz
  yükseltin.
- **Her iki durumda:** önce en ilgili abonelerinize gönderin, listeyi temiz tutun ([içe aktarma](importing.md)
  e-posta alamayacak adresleri dışarıda bırakır) ve sonrakini göndermeden önce ilk bültenin geri dönenlerine ve
  şikâyetlerine bakın. Ayrılmış bir IP adresi için sağlayıcınızın ısınma önerisini izleyin.

## Arka uçtan

| Uç nokta | Kullanım |
|---|---|
| `POST /v1/channels/{channelId}/deliverability-check` | Denetim başlatır (`202`). Kanal başına saatte en fazla altı (`429 rate_limited`); kanal gönderemiyorsa `409 email_channel_required`. |
| `GET /v1/channels/{channelId}/deliverability` | Son denetim: `state`, `spf`, `dkim`, `dmarc`, `mx`, `yuva_dkim`. Hiç denetlenmediyse `404`. |
| `POST /v1/channels/{channelId}/dkim-key` | Bir `selector` ile anahtar oluşturur; yanıtta eklenecek `dns_name` ve `dns_value` vardır. Zaten varsa `409 dkim_exists`. |
| `PATCH /v1/channels/{channelId}` | `email.dkim.enabled: true`, DNS yanıt verince imzalamayı açar (`409 dkim_dns_missing`); `false` kapatır. |
| `DELETE /v1/channels/{channelId}/dkim-key` | Anahtarı kaldırır. |

`state`: `queued`, `sent` (deneme e-postası gelene kadar), `received`, `unverifiable`, `timeout` ya da `failed`.
Oluşturmak ve denetlemek `inboxes:manage`, okumak `inboxes:read` ister. Tam şekiller
[API sözleşmesinde](../../openapi/openapi.yaml).
