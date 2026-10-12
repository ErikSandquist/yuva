# Help Scout'tan taşıyın

Bir Help Scout hesabının konuşmalarını Yuva'ya kopyalayın ve destek adresini Yuva'ya yönlendirin. Yuva Help
Scout'u yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## Başlamadan önce

- Yuva için bir uygulama oluşturun. Help Scout'ta **Your Profile → My apps** yolunu açın ve **Create My
  App**'e tıklayın. Adını "Yuva" koyun ve Yuva'nın Bağlan adımında gösterdiği **Yönlendirme URL'si**ni
  redirection URL olarak girin. Oluşturun ve **App ID** ile **App Secret**'ı kopyalayın. Help Scout'un
  sayfası: [Authentication](https://developer.helpscout.com/mailbox-api/overview/authentication/).
- Bağlan adımında **App ID** ve **App Secret**'ı girin ve **Help Scout ile oturum aç**'a basın. Taşımak
  istediğiniz her posta kutusunu gören bir kullanıcı olarak oturum açın.

Help Scout'ta salt okunur izin yoktur, bu yüzden uygulama veri değiştirebilirdi. Onu salt okunur tutan Yuva'nın
bağlantısıdır: oturum açma dışında Help Scout'a okuma istekleri dışında hiçbir şey göndermez; bu yüzden hiçbir
konuşma, müşteri ya da ayar değişmez.

## Neler kopyalanır

| Help Scout | Yuva |
|---|---|
| Konuşmalar | Konuşmalar, tüm mesajlarıyla |
| Müşteri yazışmaları | Gelen mesajlar |
| Yanıtlar | Giden mesajlar |
| Sohbet ve telefon kayıtları | Mesajlar, yazana göre gelen ya da giden |
| Notlar | Yalnızca ekibinizin gördüğü notlar |
| Durum: active | Açık |
| Durum: pending | Beklemede |
| Durum: closed | Kapalı |
| Etiketler | Etiketler, eşlediğiniz gibi (ya da hiçbiri) |
| Kullanıcılar | Ekip üyeleri, eşlediğiniz gibi; diğerleri adıyla saklanır |
| Atanan kişi | Atanan kişi, kullanıcı eşlenmişse |
| Müşteriler | Kişiler, adı ve telefonuyla |
| Posta kutuları | Gelen kutuları, her posta kutusu eşlediğiniz gibi bir gelen kutusuna |
| Ekler | Mesajlardaki dosyalar |

Spam, taslak ve silinmiş konuşmalar hiç içe aktarılmaz. Konuşmalar özgün zamanlarını korur ve
[sessizce](migrate.md#geçmiş-nasıl-sessiz-kalır) gelir.

## Neler kopyalanmaz

İş akışları ve günlük satırları, kayıtlı yanıtlar, memnuniyet puanları, özel alanlar, Docs (bilgi tabanı),
Beacon ayarları ve raporlar. Yuva yalnızca yukarıdaki tabloda yazanı kopyalar.

## Özgün mesajlar

Bir e-posta yazışması için Yuva özgün mesajı okur; başlıklar, biçim ve ekler gönderildiği gibi gelir. Help
Scout özgün kaynakları yalnızca bir süre saklar; biri artık yoksa Yuva onun yerine Help Scout'un verisindeki
yazışma gövdesini kullanır ve eklerini ayrıca getirir. Sunucunun ek sınırından büyük bir dosya geride kalır.

## Sınırlar

Help Scout istekleri plana göre dakikada sınırlar ve bir hesabın tüm uygulamaları sınırı paylaşır. Yuva Help
Scout'un bildirdiği sınırın yarısında kalır ve Help Scout isterse yavaşlar. Bu yüzden büyük bir hesap biraz
sürer. Help Scout'un sayfası:
[Rate limiting](https://developer.helpscout.com/mailbox-api/overview/rate-limiting/).

## Geçiş

Bugün destek adresiniz Help Scout'a ulaşıyor; bunu onu barındıran e-posta sağlayıcısı ya da orada bir kural
yönlendiriyor. Geçiş adımında kanalın [yönlendirme adresi](email.md#yönlendirme-adresi)ni kopyalayın ve bu
yönlendirmeyi destek adresi Yuva'ya ulaşacak şekilde değiştirin. Bunu yapana kadar Yuva Help Scout'u iki
dakikada bir okur, böylece hiçbir konuşma kaybolmaz. Yuva'dan gönderilen yanıtlar kanalın adresinden çıkar.

Sonra **Deneme e-postası gönder**'e basın: ileti yeni yoldan Yuva'ya ulaştığında çalışıyor demektir. Sessiz
bir dönemden sonra ya da **Şimdi bitir**'e bastığınızda taşıma biter
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## Bittikten sonra

Yuva için oluşturduğunuz uygulamayı silin: Help Scout'ta **Your Profile → My apps**. Yuva taşıma bitince
kendi belirtecini siler ama Help Scout'ta iptal edemez; bu yüzden erişimini uygulamayı silmek sonlandırır.
Yönlendirmeyi olduğu gibi bırakın; isterseniz Help Scout'u arşiv olarak tutun.
