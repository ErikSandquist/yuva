# İşletmeciler için Yuva'ya taşıma

[Yuva'ya taşı](migrate.md)'yı kendi sunucunuzda çalıştırmak için: yönlendirme alan adı, ayarlar, eski posta
kutusunun yalnızca okunduğuna güvenmek için neyin denetleneceği ve çok çalışma alanlı bir sunucunun
sınırları. Şimdilik yalnızca IMAP posta kutuları taşınabilir.

## Yönlendirme alan adı

Posta kutusunu taşımak onsuz da çalışır: ekipler MX kayıtlarını değiştirerek geçer. Bir yönlendirme alan adıyla
her e-posta kanalı ayrıca `<belirteç>@<yönlendirme alan adı>` biçiminde bir
[yönlendirme adresi](email.md#yönlendirme-adresi) alır; ekip eski posta kutusunu oraya yönlendirebilir.

1. Başka hiçbir posta sunucusunun işlemediği bir ad seçin, örneğin `forward.example.com`, ve sunucuda
   ayarlayın:

   ```sh
   YUVA_FORWARDING_DOMAIN=forward.example.com
   ```

2. Bu alan adının tüm e-postasını, bir kanalın kendi alan adıyla aynı biçimde Yuva'ya yönlendirin
   ([Gelen e-posta](email.md#gelen-e-posta) altındaki üç yol). Belirteç `@` işaretinden öncesidir, bu yüzden
   alan adının her adresi Yuva'ya ulaşmalı: [Cloudflare Email Worker](../../edge/README.md) ile alan adında
   Email Routing'i açın ve catch-all kuralını Worker'a yönlendirin. Worker'da yeni bir şey gerekmez: iletiyi
   her zamanki gibi `/ingress/email` için imzalar, kanalı Yuva belirteçten bulur. Bilinmeyen belirteçler
   reddedilir.
3. Yeniden başlatın, **Ayarlar → gelen kutusu → Müşteriler nereden yazıyor → E-posta**'yı açın ve kanalın adresini
   gösterdiğini kontrol edin.

Sızan bir yönlendirme adresi yenilenerek değiştirilebilir; eskisine gelen e-posta o andan sonra reddedilir.

## Ayarlar

Hepsi [yapılandırma başvurusunda](configuration.md#yuvaya-taşıma). Neyi sınırladıkları:

| Ayar | Sınırladığı |
|---|---|
| `YUVA_MIGRATION_SOURCES` | Ekiplerin başlatabileceği kaynaklar; başka kaynak istemek `422 hosted_limit` olur. |
| `YUVA_MIGRATION_WORKERS` | Süreç başına aynı anda kopyalanan taşıma sayısı. Kopyalamaması gereken süreçlerde `0` yapın. |
| `YUVA_MIGRATION_MAX_ACTIVE` | Bir çalışma alanının aynı anda taradığı, kopyaladığı ya da eşitlediği taşımalar. |
| `YUVA_MIGRATION_MAX_BYTES` | Bir çalışma alanının ayda aktardığı bayt; sınıra gelince kopya sonraki aya kadar bekler. |
| `YUVA_MIGRATION_MAX_MESSAGE_BYTES` | Tek bir ileti; daha büyükler atlanır ve sayılır. |
| `YUVA_MIGRATION_SYNC_MAX_DAYS` | Bir taşımanın kopyadan sonra eski posta kutusunu ne kadar okumaya devam edeceği; sonra durur ve kimlik bilgilerini siler. |
| `YUVA_MIGRATION_AUDIT_DAYS` | Bir taşımanın etkinliğinin bittikten sonra ne kadar tutulacağı. |
| `YUVA_FORWARDING_DOMAIN` | Kanalların yönlendirme adresi olup olmadığı. |
| `YUVA_MIGRATION_ALLOW_PRIVATE` | Özel ağlardaki posta sunucularına izin verilip verilmediği (aşağıda). |

Her bağlantı denemesi taşıma başına saatte 10, çalışma alanı başına saatte 30 ile sınırlıdır; art arda üç
başarısız oturum açma denemeleri 15 dakika durdurur, böylece sunucu bir sağlayıcıda şifre tahmin etmek için
kullanılamaz.

## Özel adresler

Yuva bir posta sunucusunun adını bir kez çözer ve adreslerden biri özel, loopback ya da yerel ağdaysa
reddeder; web kancalarında ve SMTP sunucularında olduğu gibi. Link-local ve bulut metadata adreslerine hiçbir
zaman izin verilmez. Kendi ağınızdaki bir sunucudan taşımak ya da Yuva'ya taşımayı yerel bir test sunucusunda
denemek için `YUVA_MIGRATION_ALLOW_PRIVATE=true` yapın; sunucu açılışta uyarı yazar. Paylaşılan bir
sunucuda kapalı bırakın.

## Eski posta kutusunun yalnızca okunduğunu denetleme

Söz üç katmana dayanır ([Yuva ne yapar](migrate.md#yuva-posta-kutunuzla-neler-yapar-neler-yapmaz)). Her birini
deponun bir kopyasında kendiniz denetleyebilirsiniz.

- **Koruma.** `api/` içinde `go test ./...`, `TestIMAPGuardRefusesWrites` (yazan her komut, her yazılışıyla,
  soketa tek bayt ulaşmadan reddedilir) ve `TestOnlyImapreadImportsClient` (IMAP istemcisini yalnızca
  `imapread` paketi, yalnızca salt okunur açışlarla ve `PEEK` ile kullanabilir) testlerini çalıştırır. Biri
  başarısız olursa derleme başarısız olur. Koruma çalışma anında bir posta kutusuna giden bir komutu
  durdurursa taşıma `source_write_refused` kaydeder.
- **Her şeyi kaydeden bir posta kutusu.** `deploy/compose.yaml`'ın `migrate-test` profili, test posta
  kutularıyla Dovecot çalıştırır ve her oturumu `<fixtures>/<user>/rawlog` içine yazar:

  ```sh
  cd api && go run ./internal/migrate/testdata/gen -out /tmp/yuva-migrate-fixtures
  cd ../deploy && docker compose --profile migrate-test up -d dovecot
  export YUVA_TEST_IMAP_URL=imaps://:migrate-test-only@127.0.0.1:59993
  ```

  Bir geliştirme sunucusundan test posta kutusunu taşıyın (`YUVA_MIGRATION_ALLOW_PRIVATE` açık), sonra
  sunucunun aldığı komutları listeleyin. Yalnızca `CAPABILITY`, `NOOP`, `LOGOUT`, `STARTTLS`, `LOGIN`,
  `AUTHENTICATE`, `LIST`, `STATUS`, `EXAMINE`, `UNSELECT`, `SEARCH` ve `FETCH` (`UID …` olarak da)
  görünebilir:

  ```sh
  cat /tmp/yuva-migrate-fixtures/<user>/rawlog/*.in | tr -d '\r' \
    | awk 'NF >= 3 { c = toupper($3); if (c == "UID") c = c " " toupper($4); print c }' | sort -u
  ```

- **Posta kutusunun kendisi.** `YUVA_TEST_IMAP_URL` ayarlıyken `go test ./internal/migrate/imapread -run
  TestDovecotSourceUnchanged`, bir test posta kutusunu taşır ve her iletiyi, bayrağı ve sayacı öncesiyle
  sonrasıyla karşılaştırır.

## Kimlik bilgileri ve denetim

Bir posta kutusunun kullanıcı adı ve şifresi girildiği anda `YUVA_MASTER_KEY` ile mühürlenir ve yalnızca bir iş
diliminde açılır. Hiçbir iş argümanında, olayda, günlükte ya da yanıtta görünmez. Yuva onları taşıma
tamamlandığında ya da iptal edildiğinde, `YUVA_MIGRATION_SYNC_MAX_DAYS` dolduğunda, değiştirildiklerinde ve
çalışma alanıyla birlikte siler. Ana anahtarı diğerleri gibi yedekleyin ([kurulum rehberi](install.md#ana-anahtar)):
onsuz duraklatılmış bir taşıma sürdürülemez.

Bir taşımadaki her eylem (kim, hangi istemciden, ne) bittikten sonra `YUVA_MIGRATION_AUDIT_DAYS` boyunca
etkinlik olarak tutulur ve saklama işi tarafından silinir. Günlükler taşımanın ve çalışma alanının kimliklerini,
durumları, hata kodlarını ve sayıları yazar; konu, adres ya da gövde yazmaz.

## Barındırılan sınırlar

Yuva'ya taşımayı birçok çalışma alanına sunan bir sunucuda, planlarınıza uyan sınırları ayarlayın. Bir sınırın
aşılması `422 hosted_limit` ile reddedilir ve sorunun `limit` alanı ayarı adlandırır:

| `limit` | Ne zaman |
|---|---|
| `YUVA_MIGRATION_SOURCES` | Çalışma alanı sunucunun sunmadığı bir kaynak seçti. |
| `YUVA_MIGRATION_MAX_ACTIVE` | Çalışma alanı izin verilen kadar taşıma çalıştırırken bir tane daha başlatıyor. |
| `YUVA_MIGRATION_MAX_BYTES` | Çalışma alanı bu ay izin verilen kadar bayt aktardı. Kopya bir `hosted_limit` hatası kaydeder, sonraki ayın birine (UTC) kadar bekler ve kendiliğinden sürer. |

## Kapasite

Bir taşıma tek bağlantıyla okur ve kaynağının bütçesi hızında kopyalar: Gmail ve Google Workspace için günde
1.500 MB, diğerleri için saniyede 4 MB. Büyük bir posta kutusunun ne kadar süreceğini sunucunuz değil yavaş
kaynaklar belirler; her süreç aynı anda `YUVA_MIGRATION_WORKERS` taşıma çalıştırır, bu yüzden aynı anda
taşınmasını beklediğiniz çalışma alanı sayısıyla artırın. Aktarılan e-posta canlı e-postayla aynı depolamayı
kullanır; "Özgün mesajları sakla" her iletinin kaynağını ayrıca saklar.
