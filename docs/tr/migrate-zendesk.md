# Zendesk'ten taşıyın

Bir Zendesk Support hesabının destek taleplerini Yuva'ya kopyalayın ve destek adresini Yuva'ya yönlendirin.
Yuva Zendesk'i yalnızca okur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

## Başlamadan önce

- Zendesk yöneticisi olmanız gerekir: her talebi yalnızca yönetici okuyabilir. Planın API erişimi içermesi
  gerekir.
- Yuva için bir OAuth istemcisi oluşturun. Admin Center'da **Apps and integrations → APIs → OAuth clients**
  yolunu açın ve **Add OAuth client**'a tıklayın. Adını "Yuva" koyun, istemci türü olarak **Confidential**
  seçin, kapsamı `read` yapın ve Yuva'nın Bağlan adımında gösterdiği **Yönlendirme URL'si**ni girin.
  Kaydedin ve istemci kimliğini ve gizli anahtarı hemen kopyalayın: Zendesk gizli anahtarı yalnızca bir kez
  tam olarak gösterir. Zendesk'in sayfası:
  [Using OAuth authentication with your application](https://support.zendesk.com/hc/en-us/articles/4408845965210-Using-OAuth-authentication-with-your-application).
- Bağlan adımında **Alt alan adı**nı (`acme.zendesk.com` için `acme`), **İstemci kimliği**ni ve **İstemci
  gizli anahtarı**nı girin ve **Zendesk ile oturum aç**'a basın. Yönetici olarak oturum açın ve erişime izin
  verin.

Yuva yalnızca `read` kapsamını ister ve daha fazlasını veren oturum açmayı reddeder. Oturum açma dışında
bağlantısı Zendesk'e okuma istekleri dışında hiçbir şey göndermez; bu yüzden hiçbir talep, kullanıcı ya da
ayar değişmez.

## Neler kopyalanır

| Zendesk | Yuva |
|---|---|
| Talepler | Konuşmalar, tüm mesajlarıyla |
| Herkese açık yorumlar | Mesajlar: talep sahibinden gelen, temsilciden giden |
| Özel yorumlar | Yalnızca ekibinizin gördüğü notlar |
| Durum: new, open | Açık |
| Durum: pending, on-hold | Beklemede |
| Durum: solved, closed | Kapalı |
| Etiketler | Etiketler, eşlediğiniz gibi (ya da hiçbiri) |
| Temsilciler ve yöneticiler | Ekip üyeleri, eşlediğiniz gibi; diğerleri adıyla saklanır |
| Atanan kişi | Atanan kişi, temsilci eşlenmişse |
| Talep sahipleri | Kişiler |
| Markalar | Gelen kutuları, her marka eşlediğiniz gibi bir gelen kutusuna |
| Ekler | Mesajlardaki dosyalar ([aşağıda](#ekler)) |

Spam ve silinmiş talepler hiç içe aktarılmaz. Konuşmalar özgün zamanlarını korur ve
[sessizce](migrate.md#geçmiş-nasıl-sessiz-kalır) gelir.

## Neler kopyalanmaz

Makrolar, otomasyonlar, tetikleyiciler ve görünümler; memnuniyet puanları; Yardım Merkezi (makaleler ve
topluluk); özel talep ve kullanıcı alanları; kuruluşlar ve gruplar; raporlar. Yuva yalnızca yukarıdaki
tabloda yazanı kopyalar.

## Ekler

Yuva her yorumun dosyalarını getirir. Zendesk bazılarını dış sunucularda tutar; bunlar Yuva'nın belirteci
olmadan getirilir, çünkü belirteç yalnızca Zendesk hesabınıza ve Zendesk'in kendi dosya sunucusuna gider.
Sunucunun ek sınırından büyük bir dosya ya da Zendesk'te artık olmayan bir dosya geride kalır; mesaj onsuz
kopyalanır.

## Sınırlar

Zendesk istekleri plana göre dakikada sınırlar; talepleri listeleyen artımlı dışa aktarım dakikada 10
isteğe izin verir. Yuva bunun yarısında kalır: dakikada en fazla 5 dışa aktarım isteği ve geri kalan her şey
için Zendesk'in hesap için bildirdiği sınırın yarısı. Bu yüzden büyük bir hesap biraz sürer ve Zendesk
isterse kopyalama kendiliğinden yavaşlar. Zendesk'in sayfaları:
[Incremental exports](https://developer.zendesk.com/api-reference/ticketing/ticket-management/incremental_exports/),
[Usage limits](https://developer.zendesk.com/api-reference/ticketing/account-configuration/usage_limits/).

## Geçiş

Bugün destek adresiniz Zendesk'e ulaşıyor; bunu onu barındıran e-posta sağlayıcısı ya da orada bir kural
yönlendiriyor. Geçiş adımında kanalın [yönlendirme adresi](email.md#yönlendirme-adresi)ni kopyalayın ve bu
yönlendirmeyi destek adresi Yuva'ya ulaşacak şekilde değiştirin. Bunu yapana kadar Yuva Zendesk'i iki
dakikada bir okur, böylece hiçbir talep kaybolmaz. Yuva'dan gönderilen yanıtlar kanalın adresinden çıkar.

Sonra **Deneme e-postası gönder**'e basın: ileti yeni yoldan Yuva'ya ulaştığında çalışıyor demektir. Sessiz
bir dönemden sonra ya da **Şimdi bitir**'e bastığınızda taşıma biter
([nasıl çalışır](migrate.md#deneme-e-postası-sessiz-dönem-ve-bitirme)).

## Bittikten sonra

Yuva için oluşturduğunuz OAuth istemcisini silin: Admin Center'da **Apps and integrations → APIs → OAuth
clients**. Yuva taşıma bitince kendi belirtecini siler ama Zendesk'te iptal edemez; bu yüzden erişimini
istemciyi silmek sonlandırır. Yönlendirmeyi olduğu gibi bırakın; isterseniz Zendesk'i arşiv olarak tutun.
