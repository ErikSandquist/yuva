# Yol haritası panosu

Yol haritası panosu müşterilerinize ne geliştirdiğinizi gösterir ve neye ihtiyaç duyduklarını söylemelerini
sağlar: aşamalara ayrılmış fikirler, oylar ve okuyuculardan gelen öneriler. Bir gelen kutusuna aittir; bu
yüzden müşterilerinizin zaten gönderdiği geri bildirimler, yeniden yanıtlanmak yerine bir fikre bağlanabilir.
Fikir ilerlediğinde bağlı her müşteri durumu görür; fikir yayınlandığında tek bir yanıt hepsine kendi
kanalından ulaşır.

Pano ekibiniz için geri bildirimleri sınıflandırma aracı olarak özel kalabilir ya da herkese açık olabilir:
Yuva sunucunuzda zorunlu JavaScript olmadan düz HTML bir sayfa ve [web widget'ında](widget.md) bir Yol haritası
uygulaması. Her gelen kutusunun en fazla bir panosu olur.

## 1. Pano oluşturun

Panelde: kenar çubuğunda Yol haritası (kısayol `g p`), sonra gelen kutusunda Pano oluştur. Panoları sahipler
ve yöneticiler oluşturur ve değiştirir; fikirlerle gelen kutusuna erişimi olan temsilciler çalışır.

| Ayar (API alanı) | Anlamı |
|---|---|
| Ad (`name`) | En fazla 100 karakter. |
| Açıklama (`description`) | En fazla 1.000 karakter, adın altında gösterilir. |
| Adres (`slug`) | 3 ile 48 karakter: `a-z`, `0-9` ve araya giren tireler. Sunucuda benzersiz; bazı sözcükler ayrılmıştır. |
| Sohbet kanalı (`channel_id`) | Aynı gelen kutusunun bir [sohbet kanalı](widget.md). Herkese açık sayfa onun sohbetini gömer, ekibinizin başlattığı fikirler ona kaydedilir. Zorunludur; pano varken silinemez. |
| Herkese açık (`public`) | Varsayılan olarak kapalı. Özel bir pano ekibiniz dışında herkese `404` verir. |
| Öneriler (`suggestions`) | `moderated` (varsayılan), `open` ya da `off`; bkz. [Öneriler ve oylar](#öneriler-ve-oylar). |
| Analitik (`analytics_property_id`) | Panoyla birlikte saklanır. |

Sayfanın logosu ve vurgu rengi gelen kutusunun markalamasıdır. Adresi değiştirirseniz eskisi 90 gün boyunca
panoya ayrılmış kalır ve altındaki herkese açık adres yenisine yönlendirir. Herkese açık seçeneğini kapatmak
panoyu hemen web'den kaldırır ve widget Yol haritası kutucuğunu kaybeder.

API ile: `inbox_id`, `channel_id`, `name` ve `slug` ile `POST /v1/roadmap-boards`, sonra
`{"public": true}` ile `PATCH /v1/roadmap-boards/{boardId}`. `GET /v1/roadmap-boards/{boardId}/preview`,
pano özel olsa bile bir sayfayı ziyaretçilerin göreceği gibi döndürür. `DELETE` panoyu fikirleri, oyları,
yanıtları ve aşama geçmişiyle siler; geri bildirimler ve onlara daha önce atanan ilerleme kalır.

## 2. Fikirler ve aşamalar

Fikir, bir başlığı (3 ile 200 karakter) ve Markdown gövdesi (en fazla 20.000) olan bir konudur ve beş
aşamadan birindedir:

| Aşama | Anlamı | Bağlı geri bildirimlere atanan ilerleme |
|---|---|---|
| `ideas` | Toplandı, karar verilmedi. Yeni fikirler buradan başlar. | yok |
| `planned` | Geliştireceksiniz. | `planned` |
| `in_progress` | Geliştiriyorsunuz. | `in_progress` |
| `shipped` | Bitti. Yeni oy almaz. | `done` |
| `closed` | Geliştirmeyeceksiniz. Yeni oy almaz ve herkese açık listelerde görünmez. | `wont_do` |

Ekibiniz fikirleri panelde ya da bir geri bildirimden başlatır (bkz. [Geri bildirimi bağlayın](#3-geri-bildirimi-bağlayın)).
Gelen kutusuna erişimi olan temsilciler, sahipler ve yöneticiler aşamayı taşır: panoda bir kartı sürükleyin ya
da aşama menüsünü kullanın (`m`; önceki ve sonraki aşama için `[` ve `]`). Panel önce kaç bağlı geri bildirimin
hangi ilerlemeyi alacağını söyler. Bir yorum ekleyebilirsiniz; fikir üzerinde herkese açık yanıtınız olarak
yayınlanır. Her taşıma fikrin aşama geçmişinde tutulur; herkese açık sayfanın "Planlandığı tarih" ve
"Yayınlandığı tarih" bilgileri de buradan gelir. Fikri bulunduğu aşamaya taşımak hiçbir şeyi değiştirmez.

API ile: `stage` ve isteğe bağlı `comment` ile `POST /v1/discussions/{discussionId}/stage` (yorum için üye
oturumu gerekir) ve `GET /v1/discussions/{discussionId}/stage-changes`. Fikirler `idea` türünde tartışmalardır:
`GET /v1/discussions?kind=idea` ile listeleyin; `board_id`, `stage`, `sort` (`votes`, `new`, `activity`) ve `q`
ile süzün; `POST /v1/discussions` ve `kind: idea` ile birini başlatın.

## 3. Geri bildirimi bağlayın

Bir geri bildirim konuşmasını açın ve Bir fikre bağla'yı seçin (`b`): gelen kutusunun panosunda arayın ya da
yazdığınız başlık ve metinle yeni bir fikir başlatıp konuşmayı ona bağlamak için Bu geri bildirimden yeni
fikir'i seçin. Geri bildirimden hiçbir şey herkese açık olmaz. Geri bildirim, fikri bir etiket olarak gösterir.

- Yalnızca `feedback` konuşmaları bağlanır ve yalnızca aynı gelen kutusunun panosundaki bir fikre.
- Bir fikrin çok sayıda bağlı geri bildirimi olur; bir geri bildirimin en fazla bir fikri olur. Yeniden bağlamak
  bağlantıyı değiştirir. Bir fikir en fazla 5.000 bağlantı alır.
- Bir konuşmayı başka bir gelen kutusuna taşımak, onun fikirle bağlantısını koparır.

Panodaki her kart, fikrin arkasında ne kadar geri bildirim olduğunu gösterir. API ile:
`PUT` ve `DELETE /v1/discussions/{discussionId}/conversations/{conversationId}`;
`GET /v1/conversations?idea_id=` bir fikrin geri bildirimlerini, `idea=none` ise fikri olmayanları listeler.

### İlerleme aşamayı izler

Bir fikir `planned`, `in_progress`, `shipped` ya da `closed` aşamasına taşındığında Yuva, bağlı her geri
bildirimin ilerlemesini yukarıdaki tabloya göre ayarlar. Her konuşma bir zaman çizelgesi olayı alır ("via idea …"),
kişi yeni ilerleme satırını görür ve [webhook'lar](webhooks.md) `conversation.updated` gönderir. Zaten o
ilerlemede olan geri bildirimler atlanır. Daha yeni bir taşıma, hâlâ çalışan eskisine her zaman üstün gelir.
`ideas` aşamasına geri taşıma hiçbir ilerlemeyi değiştirmez. Geri bildirimi `ideas` aşamasını geçmiş bir fikre
bağlamak ilerlemesini hemen ayarlar; bağlantıyı kaldırmak onu olduğu gibi bırakır. Bir geri bildirimin
ilerlemesini yine elle ayarlayabilirsiniz; fikrin bir sonraki taşımasına kadar geçerli kalır.

### Bağlılara yanıt ver

Bir fikir yayınlandığında panel hemen Bağlılara yanıt ver'i önerir. Bu,
[durum sayfalarındakiyle](status-pages.md#7-konuşmaları-bir-olaya-bağlayın) aynı işlevdir: bir kez yazılan tek
bir yanıt, açık olan her bağlı konuşmaya sizin adınıza ve her konuşmanın kendi kanalından (e-posta, sohbet ya da
uygulama) gider ve konuşmaları kapatabilir. Gönderildikten sonra geri alınamaz; panel her konuşma için
gönderildi, atlandı ya da başarısız oldu bilgisini bildirir. API ile:
`POST /v1/discussions/{discussionId}/linked-replies` `202` döner ve
`GET /v1/discussions/{discussionId}/linked-replies/{replyId}` sonucu bildirir. Bir anahtarın `roadmap:write`,
`conversations:read` ve `messages:write` (kapatmak için ayrıca `conversations:write`) yetkisi olmalı ve
çalışma alanı botların göndermesine izin vermelidir ([Botlar gönderebilir](headless.md#botlar-gönderebilir)).

## Öneriler ve oylar

Okuyucular fikirlere oy verir ve yenilerini önerir. Bunun için giriş gerekir: **Giriş yap**, Yuva sunucunuzda
küçük bir pencere açar; okuyucu orada e-posta koduyla ya da passkey ile giriş yapar, tıpkı
[dokümantasyon sayfalarında](documentation-pages.md#okuyucular-ve-ekibiniz-ne-görür) olduğu gibi. Ürününüzde
hesaplar varsa widget'a bir [kimlik token'ı](identity.md) verin; kullanıcılarınız hemen oy verir ve öneri
yazar. Anonim ziyaretçiler yalnızca okur. Oy vermek fikri takip etmeyi de başlatır: fikir `planned`,
`in_progress`, `shipped` ya da `closed` aşamasına geçtiğinde ya da yanıt aldığında takipçiler bir e-posta özeti alır.

| Mod | Okuyucunun önerisi |
|---|---|
| `moderated` (varsayılan) | Çalışma alanındaki yazılarından biri onaylanana kadar onay bekler. Kendisi "Onay bekliyor" görür. Kimlik token'lı kullanıcılar ve ekibiniz hemen yayınlar. |
| `open` | Hemen yayınlanır; spam işaretlediğiniz yazarlar hariç. |
| `off` | Öneri yok. Oylar ve yanıtlar yine çalışır. |

Bekleyen fikirler Yol haritası ekranındaki şeritte görünür; sahipler ve yöneticiler onları onaylar ya da gizler
(`state` ile `PATCH /v1/discussion-posts/{postId}`). Kuyruk `GET /v1/discussion-posts?state=pending` adresindedir.
Bir panoda en fazla 200 bekleyen fikir olur (`429 suggestions_full`). Özel bir panodaki fikirler yazarlarına
e-posta göndermez.

## Herkese açık sayfalar

Herkese açık bir pano sunucunuzda `/roadmap/{slug}` adresindedir:

| Adres | Sayfa |
|---|---|
| `/roadmap/{slug}` | Fikirler, Planlandı, Üzerinde çalışılıyor ve Yayınlandı (son 90 gün; tümü `?stage=shipped` adresinde) sütunları, oya göre sıralı. Her kartta başlık, özet, oy ve yorum sayısı var. |
| `/roadmap/{slug}/ideas/{id}` | Aşama geçmişi ve yayınlanmış yanıtlarıyla bir fikir. Yazarlar yalnızca ad ve baş harflerle görünür. |

Sayfa JavaScript olmadan da eksiksiz okunur. JavaScript ile Oy ver ve Fikir öner düğmeleri widget'ın Yol haritası
uygulamasını o fikirde açar. Özel bir pano, bilinmeyen bir adres ve bekleyen ya da gizli bir fikir düz `404`
verir. Sayfalar canonical bağlantı, Open Graph etiketleri ve yapılandırılmış veri taşır, bir dakika önbelleğe
alınır (oy sayısı o kadar geriden gelebilir) ve adres başına dakikada 120 istek yanıtlar. Sayfa sözcükleri
İngilizce, Türkçe ve Almanca vardır; sayfa gelen kutusunun dilini kullanır, `?lang=` başka bir dil seçer.

## Web widget'ında

Gelen kutusunun herkese açık bir panosu olduğunda [widget'ın](widget.md#ziyaretçi-neler-görür) ana ekranında bir
Yol haritası kutucuğu belirir. Uygulama aşamaya göre sekmeleri, en çok oy alan fikirleri, metni, oyları ve
yanıtlarıyla bir fikri ve başlık yazılırken benzer fikirleri listeleyen Fikir öner'i gösterir. Sayfanız onu
açabilir: `document.querySelector("yuva-chat").openApp("roadmap", { idea: "<fikir id>" })`.

Kendi arayüzünüz için [shadcn bileşenleri](https://useyuva.com/shadcn) `use-yuva-roadmap`, `yuva-roadmap`,
`yuva-idea` ve `yuva-idea-suggest`, [headless istemcinin](headless.md)
`GET /client/v1/channels/{channel_key}/roadmap` uç noktasını ve `kind=idea` ile tartışma uç noktalarını kullanır.

## Bir asistandan

[MCP](mcp.md#yol-haritasını-bir-asistanla-kullanın) ile bir asistan fikirleri listeler, geri bildirimleri onlara
bağlar ve yalnızca `roadmap:stage` yetkisini adıyla verdiğinizde, kaç geri bildirimin değişeceğini onayladıktan
sonra bir aşamayı taşır.

## API, webhook'lar ve yetkiler

Hepsi `/v1/roadmap-boards` ve tartışma uç noktaları altındadır; [API başvurusunda](headless.md) etiketi
`roadmap`'tir. [Webhook'lar](webhooks.md), `discussion.created`, `discussion.post_created` ve
`discussion.updated` yanında `discussion`, `from`, `to` ve `linked` ile `discussion.stage_changed` gönderir.
Gelen kutularıyla sınırlı bir anahtar yalnızca o gelen kutularının panolarını görür; başka her pano ya da fikir
`404` verir.

| Yetki | İzin verdiği |
|---|---|
| `roadmap:read` | Panolar, aşama geçmişi, bağlılara yanıtın sonucu. Yetki adı verilmeden oluşturulan anahtarlara verilir. |
| `roadmap:write` | Pano oluşturma, değiştirme ve silme, geri bildirimleri fikre bağlama ve bağı kaldırma, bağlılara yanıt. Yetki adı verilmeden oluşturulan anahtarlara verilir. |
| `roadmap:stage` | Bir fikrin aşamasını taşıma. Hiçbir yetkiye dahil değildir: adıyla verin. |

Fikirleri okumak ve bağlamak ayrıca `conversations:read` ister. Var olan anahtarlar bu yetkilerden hiçbirini
kazanmaz. `YUVA_ROADMAP=off` panoları tümüyle kaldırır ([Yapılandırma](configuration.md#yol-haritası-panosu)).
