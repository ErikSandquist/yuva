# Yapılandırma

Yuva ayarlarını başlarken ortam değişkenlerinden okur. Hatalı bir değer, değişkenin adını veren bir
mesajla sunucuyu durdurur.

Çalışma alanı, gelen kutusu ya da kanal ayarları burada değil; panelde, API'de ve
[operatör komutlarında](operations.md#operatör-komutları) yapılır.

## Değer atama

1. Değişkeni `compose.yaml` dosyasının yanındaki `.env` dosyasına ekleyin:

   ```sh
   YUVA_CHAT_EMAIL_DELAY=10m
   ```

2. Sunucuyu yeniden başlatın:

   ```sh
   docker compose up -d
   ```

Boş bir değişken, `YUVA_METRICS_ADDR` dışında, atanmamış sayılır ve varsayılanı alır. Listeler
virgülle ayrılır. Boolean değerler `true`/`false`, `1`/`0` ve `t`/`f` kabul eder.

## Zorunlu

| Değişken | Anlamı |
|---|---|
| `YUVA_DATABASE_URL` | Postgres 16+ adresi, ör. `postgres://yuva:secret@db:5432/yuva?sslmode=disable`. |
| `YUVA_MASTER_KEY` | base64 ile 32 rastgele bayt (`openssl rand -base64 32`). Saklanan gizli bilgileri şifreler. Bkz. [Ana anahtar](install.md#ana-anahtar). |
| `YUVA_PUBLIC_URL` | Kişilerin ve uygulamaların kullandığı adres, ör. `https://support.example.com`, alan adının kökünde. Varsayılan `http://localhost:8080`, yalnızca yerel denemeler için. |

## Sunucu

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_LISTEN_ADDR` | `:8080` | Metrikler dışındaki her şey için HTTP adresi. |
| `YUVA_METRICS_ADDR` | `:9090` | Prometheus adresi (`GET /metrics`). Dışarı açmayın. `off` ya da boş değer kapatır. |
| `YUVA_CLIENT_IP_HEADER` | atanmamış | Vekil sunucunuzun istemci IP'sini yazdığı başlık, ör. `X-Real-IP`; hız sınırları için. `X-Forwarded-For` gibi bir listede, `YUVA_TRUSTED_PROXIES` içinde olmayan en sağdaki adres sayılır. |
| `YUVA_TRUSTED_PROXIES` | atanmamış | Vekil sunucularınızın adresleri ya da CIDR aralıkları, ör. `10.0.0.0/8`. Atanırsa istemci IP başlığı yalnızca onlardan okunur. Analitik toplayıcı ziyaretçinin `X-Forwarded-For` halkasını yalnızca bu adreslerden okur ve bunlardan biri olmayan en sağdaki girdiyi alır; atanmamışsa bağlanan adresi kullanır. |
| `YUVA_COOKIE_SECURE` | `YUVA_PUBLIC_URL` `https` ise `true` | Oturum çerezini `Secure` olarak işaretler. |
| `YUVA_MCP` | `on` | `off`, MCP uç noktası `/mcp`'yi kapatır (`404` döner). OAuth açık kalır. |
| `YUVA_PANEL` | `on` | `off`, panelin ve OAuth onay sayfasının sunulmasını durdurur; API anahtarları çalışmaya devam eder. |
| `YUVA_PANEL_ANALYTICS_KEY` | boş | Bu sunucuda, alan adı panelinki olan bir [Analitik](analytics.md) web sitesinin herkese açık anahtarı (`yuva_ak_…`). Panel kendi ekranlarını orada sayar; kayıt kimlikleri `:id` olarak gider (`/conversations/:id`), hiçbir konuşma ya da kişi kimliği gönderilmez. |
| `YUVA_WIDGET` | `on` | `off`, `/yuva.js`, `/yuva-chat.js`, `/yuva-docs.js` ve `/yuva-analytics.js` dosyalarının sunulmasını durdurur; `/client/v1` kalır. |

## Giriş ve geçiş anahtarları

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_WEBAUTHN_RP_ID` | `YUVA_PUBLIC_URL` alan adı | Geçiş anahtarı relying party id'si. Değiştirmek mevcut geçiş anahtarlarını kullanılamaz yapar. |
| `YUVA_WEBAUTHN_RP_NAME` | `Yuva` | Tarayıcıların yeni geçiş anahtarı için gösterdiği ad. |
| `YUVA_WEBAUTHN_ORIGINS` | `YUVA_PUBLIC_URL` origin'i | Geçiş anahtarları ve panelin gerçek zamanlı soketi için izin verilen origin'ler. |
| `YUVA_SIGNUP` | `invite` | `invite`: giriş kodu yalnızca mevcut kişilere ve davet edilmiş adreslere gider; çalışma alanları `yuva bootstrap` ile açılır. `open`: herkes kodla giriş yapıp panelde çalışma alanı oluşturabilir. |
| `YUVA_SIGNUP_WORKSPACES_PER_PERSON` | `3` | `open` kayıtta bir kişinin oluşturup silmediği çalışma alanı sayısı; fazlası `403` alır. |
| `YUVA_SIGNUP_WORKSPACES_PER_IP` | `5` | `open` kayıtta bir IP adresinden (IPv6 /64) 24 saatte oluşturulan çalışma alanı sayısı; fazlası `429` alır. |
| `YUVA_OPERATOR_EMAIL` | boş | Yeni biri kodla kaydolduğunda (okuyucular ayrıca işaretlenir) ya da çalışma alanı oluşturduğunda e-posta alan adresler, virgülle ayrılmış: adres, çalışma alanı adı, zaman ve ülke (Cloudflare arkasında, `YUVA_CLIENT_IP_HEADER=CF-Connecting-IP` ile) ya da IP adresi. En fazla 5 dakikada bir e-posta gider, öncekinden bu yana olanların hepsini listeler. İngilizce, sunucunun SMTP'si üzerinden. Aynı e-posta bülten [onay isteklerini, askıya almaları ve otomatik duraklatmaları](operations.md#barındırılan-sunucuda-bültenler) de listeler. Boş ise kapalı. |

## Sunucu e-postası

Giriş kodları, davetler ve bildirimler. E-posta kanallarının [kendi SMTP'si](email.md#giden-smtp)
vardır.

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_SMTP_HOST` | atanmamış | SMTP sunucusu. Atanmamışsa e-postalar loga yazılır. |
| `YUVA_SMTP_PORT` | `tls` ile `465`, yoksa `587` | SMTP portu. |
| `YUVA_SMTP_TLS` | `starttls` | `starttls`, `tls` (doğrudan TLS) ya da `none`. |
| `YUVA_SMTP_USERNAME` | atanmamış | SMTP kullanıcı adı. |
| `YUVA_SMTP_PASSWORD` | atanmamış | SMTP parolası. |
| `YUVA_SMTP_FROM` | atanmamış | Gönderen, ör. `Yuva <yuva@example.com>`. `YUVA_SMTP_HOST` ile birlikte zorunlu. |
| `YUVA_SMTP_ALLOW_PRIVATE` | `false` | E-posta kanallarının loopback ve özel ağ adreslerindeki SMTP sunucularını (ör. Mailpit) kullanmasına izin verir. |

## Ekler ve depolama

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_STORAGE` | `local` | `local` (bir dizin) ya da `s3` (S3, R2, MinIO, …). |
| `YUVA_STORAGE_DIR` | `data/attachments` | `local` için dizin; Docker imajında `/data/attachments`. |
| `YUVA_S3_ENDPOINT` | atanmamış | Uç nokta adresi, ör. `https://s3.eu-central-1.amazonaws.com`. |
| `YUVA_S3_REGION` | `auto` | Bölge. |
| `YUVA_S3_BUCKET` | atanmamış | Bucket. |
| `YUVA_S3_ACCESS_KEY_ID` | atanmamış | Erişim anahtarı id'si. |
| `YUVA_S3_SECRET_ACCESS_KEY` | atanmamış | Gizli erişim anahtarı. |
| `YUVA_S3_PATH_STYLE` | `false` | Path-style adresler; MinIO genelde bunu ister. |
| `YUVA_ATTACHMENT_MAX_BYTES` | `26214400` (25 MiB) | Bayt cinsinden en büyük ek boyutu. |
| `YUVA_ATTACHMENT_TYPES` | aşağıda | İzin verilen içerik türleri; `image/*` gibi joker karakterler çalışır. |
| `YUVA_EMAIL_IMAGES_MAX_BYTES` | `524288000` (500 MiB) | Bir çalışma alanının [e-posta tasarımı](designs.md) görsellerinin bayt cinsinden toplam boyutu; `0` sınırsız demektir. Aşan yüklemeler reddedilir (`409 image_quota`). |

Varsayılan türler: `image/png,image/jpeg,image/gif,image/webp,image/heic,application/pdf,text/plain,text/csv,application/zip,application/json,video/mp4,video/quicktime,audio/mpeg,audio/mp4`.

## Gelen e-posta

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_INGRESS_SECRET` | atanmamış | `/ingress/email` isteklerini imzalar; Email Worker aynı değeri kullanır. Atanmamışsa bu uç nokta tüm e-postaları reddeder. |
| `YUVA_INGRESS_ACCEPT_V1` | `false` | Eski bir Email Worker'ı yeniden deploy edene kadar eski `v1` imzasını da kabul eder. |
| `YUVA_INGRESS_AUTHSERV_ID` | atanmamış | E-postayı alan sunucunuzun authserv-id'si, ör. `mx.cloudflare.net`. DMARC sonuçlarının kullanılması için gerekir. |
| `YUVA_INGRESS_MAX_CONCURRENT` | `8` | `/ingress/email`'in süreç başına aynı anda işlediği mesaj sayısı; fazlası `503` alır. |
| `YUVA_SES_TOPIC_ARNS` | atanmamış | `/ingress/ses`'in SES geri dönme (bounce) ve şikâyet bildirimlerini kabul ettiği SNS topic ARN'leri. |
| `YUVA_EMAIL_SENDER_HOURLY_CAP` | `500` | Bir göndericinin bir çalışma alanına saatte gönderebileceği e-posta sayısı; fazlası reddedilir (`429`). |

## Yuva'ya taşıma

[Bir posta kutusunu Yuva'ya taşımak](migrate.md) için. Çalışma alanını sınırlayan ayarlar barındırılan
sunucular içindir; birini aşan bir istek adı belirtilerek `422 hosted_limit` ile reddedilir.

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_FORWARDING_DOMAIN` | ayarsız | `forward.example.com` gibi, e-postalarını kanalların alan adları gibi Yuva'ya yönelttiğiniz bir alan adı ([Gelen e-posta](email.md#gelen-e-posta)). O zaman her e-posta kanalının bu alan adında bir [yönlendirme adresi](email.md#yönlendirme-adresi) `<belirteç>@` olur. Ayarsızsa yönlendirme adresi yoktur ve posta kutusu taşıma e-postaları yalnızca MX ile geçirir. |
| `YUVA_MIGRATION_SOURCES` | yapılmış olan hepsi | Ekiplerin taşıyabileceği kaynakların virgülle ayrılmış listesi: `imap`, `file`, `zendesk`, `helpscout`. Şimdilik IMAP üzerinden posta kutuları ve dışa aktarım dosyaları yapılmıştır. Boşsa taşıma kapalıdır. |
| `YUVA_MIGRATION_WORKERS` | `2` | İşlem başına aynı anda kopyalanan taşıma sayısı; `0` bu işlemde hiç çalıştırmaz. |
| `YUVA_MIGRATION_MAX_ACTIVE` | `0` | Bir çalışma alanının aynı anda taradığı, kopyaladığı ya da eşitlediği en fazla taşıma sayısı; `0` sınırsız. |
| `YUVA_MIGRATION_MAX_BYTES` | `0` | Bir çalışma alanının ayda aktarabileceği en fazla bayt; `0` sınırsız. |
| `YUVA_MIGRATION_MAX_MESSAGE_BYTES` | `52428800` (50 MiB) | Aktarılan en büyük mesaj; daha büyükleri atlanır. |
| `YUVA_MIGRATION_UPLOAD_MAX_BYTES` | `21474836480` (20 GiB) | Bir taşımanın kabul ettiği en büyük [dışa aktarım dosyası](migrate-files.md); daha büyüğü `422 hosted_limit` ile reddedilir. `0` dosya içe aktarımını kapatır. |
| `YUVA_MIGRATION_SYNC_MAX_DAYS` | `30` | Bir taşımanın kopyadan sonra eski posta kutusunu okumaya devam ettiği gün sayısı; sonra durur ve erişim bilgileri silinir. 1 ile 365 arası. |
| `YUVA_MIGRATION_AUDIT_DAYS` | `365` | Bir taşımanın etkinliğinin bittikten sonra saklandığı gün sayısı. 1 ile 36500 arası. |
| `YUVA_MICROSOFT_CLIENT_ID`, `YUVA_MICROSOFT_CLIENT_SECRET` | ayarsız | "Microsoft ile oturum aç" için sunucunun kendi Microsoft Entra uygulaması ([nasıl kaydedilir](migrate-operators.md#microsoft-uygulamasını-kaydetme)). İkisini birlikte ayarlayın ya da hiçbirini. Ayarsızsa ekip, Bağla adımında kendi uygulamasını kaydeder; Outlook.com ve Microsoft 365 yalnızca sunucunun [`YUVA_PUBLIC_URL`](#sunucu) değeri bir HTTPS adresiyse (ya da `localhost` ise) sunulur, çünkü Microsoft yalnızca onu yönlendirme URL'si olarak kabul eder. |
| `YUVA_MICROSOFT_TENANT` | `common` | Sunucunun Microsoft uygulamasının yetkilisi: her iş, okul ya da kişisel hesabı kabul eden bir uygulama için `common`; bunlarla sınırlı bir uygulama için `organizations`, `consumers`, bir kiracı kimliği ya da doğrulanmış bir alan adı. |
| `YUVA_MIGRATION_ALLOW_PRIVATE` | `false` | Taşımanın özel ya da yerel ağdaki e-posta sunucularına da bağlanmasına izin verir (geliştirme, kendi ağınızdaki sunucular). |

## Bültenler

[Bülten gönderme](broadcasts.md). Listeler ve formları, bu ayarlar ne derse desin çalışır.

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_BROADCASTS` | `YUVA_SIGNUP=open` ise `approval`, değilse `on` | `on`: her çalışma alanı gönderir. `approval`: kayıt yoluyla oluşturulan bir çalışma alanı, operatör onaylayana kadar gönderemez ve her çalışma alanı bir [ısınma sınırına](broadcasts.md#barındırılan-sunucularda-onay) tabidir. `off`: bülten oluşturmak, değiştirmek ve göndermek `403 broadcasts_disabled` yanıtı verir. |
| `YUVA_BROADCAST_WORKERS` | `2` | Bir sürecin aynı anda çalıştırdığı bülten gönderim işleri; her biri bir gönderen kanal içindir. Destek yanıtlarının kendi kuyruğu vardır ve yavaşlamaz. En az 1. |
| `YUVA_BROADCAST_DAILY_RECIPIENTS` | `0` (sınır yok) | Bir çalışma alanının bültenlerinin UTC günü başına en fazla kaç alıcısı olabileceği. Sınıra ulaşan gönderim ertesi gün devam eder. Operatör çalışma alanı başına daha küçük bir sınır koyabilir ([Operasyon](operations.md#barındırılan-sunucuda-bültenler)). |
| `YUVA_IMPORT_MAX_ROWS` | `100000` | [Abone içe aktarırken](importing.md) bir CSV dosyasının sahip olabileceği en çok veri satırı. 1 ile 100000 arası; daha yüksek bir değer sunucunun başlamasını engeller. Bu değeri aşan dosya başarısız olur ve hiçbir şey eklenmez. |
| `YUVA_MAIL_LINK_URL` | `YUVA_PUBLIC_URL` | Liste ve bülten e-postalarındaki abonelikten çıkma, onay, tıklama ve görsel bağlantılarının işaret ettiği kaynak (şema ve alan adı, yol yok), örn. `https://links.example.com`. Bu sunucuya ulaşmalıdır. O alan adı yalnızca bu bağlantılara (`/m/`, `/s/`, `/c/`, `/i/`) yanıt verir, geri kalan her şey için `404` döner; panel ve API ana alan adınızda kalır. |

## Analitik

[Web sitelerinizi sayma](analytics.md).

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_ANALYTICS` | `on` | `off` analitiği kaldırır: uç noktaları, toplama uç noktası ve `/yuva-analytics.js` `404` döner. |
| `YUVA_ANALYTICS_BUFFER` | `50000` | Bir sürecin yazmadan önce bellekte tuttuğu olay sayısı (saniyede bir yazılır). Dolu bir arabellek `503` döner. En az 100. |
| `YUVA_ANALYTICS_VISIT_CACHE` | `200000` | Bir sürecin sürmekte olan ziyaretleri yenilerinden ayırmak için bellekte tuttuğu ziyaret sayısı. En az 1000. |
| `YUVA_ANALYTICS_SALT_GRACE` | `2h` | Bir günün tuzunun, geç gelen olaylar için gün bittikten sonra ne kadar yaşadığı. En çok `6h`. |
| `YUVA_ANALYTICS_MONTHLY_EVENTS` | `YUVA_SIGNUP=open` ise `1000000`, değilse `0` | Bir çalışma alanının UTC ayı başına sayabileceği olay; fazlası ay bitene kadar atılır. `0` sınırsız demektir. [`yuva workspace analytics-limit`](operations.md#operatör-komutları) bunu çalışma alanı başına değiştirir. |
| `YUVA_ANALYTICS_PROPERTY_EVENTS_PER_MINUTE` | `60000` | Bir web sitesinin dakikada gönderebileceği olay; fazlası atılır. |
| `YUVA_ANALYTICS_PROPERTIES_PER_WORKSPACE` | `50` | Bir çalışma alanının sahip olabileceği web sitesi; fazlası reddedilir (`403 property_limit`). |
| `YUVA_ANALYTICS_RAW_DAYS` | `90` | Ayrıntılı ziyaret ve olayların saklandığı gün sayısı, 7 ile 400 arası. Daha eski günlerde yalnızca günlük toplamlar kalır. |
| `YUVA_ANALYTICS_WORKERS` | `2` | Bir sürecin aynı anda çalıştırdığı analitik işleri (günlük toplamlar, temizlik); `0` bu süreçte hiç çalıştırmaz. |
| `YUVA_ANALYTICS_ROLLUP_TOP` | `1000` | Bir günün toplamlarında her dökümde tutulan değer sayısı; kalanı `(other)` olarak toplanır. 10 ile 100000 arası. |
| `YUVA_GEOIP` | `dbip` | Konumların nereden geldiği. `dbip`: sunucunun her ay indirdiği ücretsiz DB-IP Lite şehir veritabanı. `file`: kendi MMDB şehir veritabanınız. `off`: konum veritabanı yok. |
| `YUVA_GEOIP_DIR` | `data/geoip` | İndirilen `dbip` veritabanının dizini. Yeniden başlatmalarda kalan bir birime koyun; compose kurulumunda örneğin `/data/geoip`. |
| `YUVA_GEOIP_FILE` | atanmamış | MMDB dosyasının yolu; `YUVA_GEOIP=file` ile zorunlu. |

`dbip` ile veritabanı [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) lisanslıdır ve sunucunuz
konumların gösterildiği her yerde ona atıf yapmalıdır. Panel bunu yapar (“IP Geolocation by DB-IP”,
`https://db-ip.com` bağlantısıyla); rakamları başka bir yerde gösterirseniz orada da belirtin. Metin ve
bağlantı, `GET /v1/analytics/properties/{propertyId}/health` yanıtının `geoip.attribution` ve
`geoip.attribution_url` alanlarındadır.

Komutlar, [Operatör komutları](operations.md#operatör-komutları) bölümündeki gibi çalıştırılır:

| Komut | Ne yapar |
|---|---|
| `analytics property create --workspace <ws> --name <name> --domain <host>... [--inbox <id\|slug>] [--platform web\|ios\|android]... [--timezone <zone>]` | Bir web sitesi oluşturur, kimliğini ve genel anahtarını sekmeyle ayırarak yazdırır. |
| `analytics property list --workspace <ws>` | Web sitelerini anahtarları, alan adları ve bugünkü ziyaretçileriyle, ayrıca ayın olaylarını sınırıyla birlikte listeler. |
| `analytics geoip-update` | Bu ayın DB-IP Lite veritabanını, orada değilse `YUVA_GEOIP_DIR` içine indirir ve verilmesi gereken atfı yazdırır. `file` ile dosyanızı denetler. |
| `workspace analytics-limit --workspace <ws> <events\|0>` | Çalışma alanının aylık olay sayısını belirler; `0` sınırı kaldırır. |

## Sohbet

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_CHAT_EMAIL_DELAY` | `5m` | Sohbetten ayrılmış bir kişiye, okunmamış bir yanıtın ne kadar süre sonra e-postayla gönderileceği. En az `1s`. |
| `YUVA_DISCUSSION_EMAIL_DELAY` | `10m` | Bir takipçiye ve tartışma başlığına bu sürede en çok bir e-posta gider; yeni yanıtları ve kabul edilen yanıtı listeler. En az `1s`. |
| `YUVA_ANONYMOUS_CONTACTS_PER_HOUR` | `20` | IP adresi (IPv6 /64), kanal ve saat başına yeni anonim ziyaretçi sayısı; fazlası `429` alır. |

## Webhook'lar

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_WEBHOOK_ALLOW_PRIVATE` | `false` | Webhook'ların loopback ve özel ağ adreslerine ulaşmasına izin verir. Yalnızca geliştirme için. |

## Web Push

| Değişken | Varsayılan | Anlamı |
|---|---|---|
| `YUVA_VAPID_PUBLIC_KEY` | atanmamış | VAPID genel anahtarı. İki anahtarı birlikte atayın ya da hiçbirini; `yuva vapid-keys` bir çift üretir. |
| `YUVA_VAPID_PRIVATE_KEY` | atanmamış | VAPID özel anahtarı. |
| `YUVA_VAPID_SUBJECT` | `mailto:` + `YUVA_SMTP_FROM` adresi, yoksa `https` ise `YUVA_PUBLIC_URL` | Push servisleri için iletişim adresi: `mailto:…` ya da `https://…`. |

## Derleme

Sunucunun bildirdiği sürüm (`GET /v1/version`), derleme sırasında Docker build argümanı `VERSION`
ile belirlenir. Ortam değişkeni değildir.
