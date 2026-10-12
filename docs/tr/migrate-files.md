# Dışa aktarım dosyasından taşıyın

Dışa aktardığınız bir dosyadan bir posta kutusunun geçmişini Yuva'ya getirin; Yuva'nın hesaba hiçbir erişimi
olmaz. Bu yol çok büyük bir posta kutusu, IMAP'i olmayan bir program ya da Yuva'nın oturum açamadığı bir
hesap için uygundur. Genel bakış [Yuva'ya taşı](migrate.md) sayfasındadır.

**Ayarlar → Yuva'ya taşı → Yeni taşıma**'yı açın, e-postaların gönderildiği adresi yazın,
**Ya da bir dışa aktarım dosyası yükleyin**'i açın, **Google Takeout** ya da **Dışa aktarım dosyası**'nı
seçin ve dosyayı seçin. Sonra neyin getirileceğini seçin, tarayın ve kopyalayın; diğer taşımalardan farkı
yoktur. Dosya sunucuda denetlenir ve okunur; Yuva onu hiç değiştirmez.

## Yuva'nın okuduğu dosyalar

| Dosya | Notlar |
|---|---|
| Google Takeout `.mbox` | Önce Takeout arşivini zip'ten çıkarın ve Mail klasöründeki `.mbox` dosyasını yükleyin. Gmail'in etiketleri ve yazışma dizileri de gelir. |
| Bir `.mbox` dosyası | `mboxrd` (çoğu program bunu yazar) ya da "mbox seçenekleri" altından eski `mboxo`. |
| `.eml` dosyalarının `.zip`'i | Klasörleri etiket olur. |
| Bir Maildir'in `.zip`'i | Klasörleri etiket olur. |
| `.mbox` dosyalarının `.zip`'i | Her dosya bir etiket olur. `.mbox` dosyaları zip içinde sıkıştırılmadan saklanmış olmalıdır. |

Yuva `.tgz` ya da `.gz` dosyalarını, zip içindeki sıkıştırılmış `.mbox` dosyalarını ve Outlook'un `.pst`
dosyalarını okumaz. Arşivi zip'ten çıkarın ve içindeki `.mbox` dosyasını yükleyin.

## Sınırlar

| Sınır | Değer |
|---|---|
| Bir yükleme parçasının boyutu | 8 MiB |
| Bir dosyanın boyutu | 20 GiB; sunucunun [`YUVA_MIGRATION_UPLOAD_MAX_BYTES`](configuration.md#yuvaya-taşıma) ayarıyla belirlenir |
| Bir zip'teki girdi sayısı | 1.000.000 |
| Bir zip girdisinin sıkıştırması | en çok 100:1 |

[`YUVA_MIGRATION_MAX_MESSAGE_BYTES`](configuration.md#yuvaya-taşıma) değerinden büyük mesajlar, her
taşımada olduğu gibi atlanır. Taşıma başına bir dosya; başka bir dosya için yeni bir taşıma başlatın.

## Yükleme ve devam etme

Tarayıcı dosyayı 8 MiB'lık parçalara böler, birkaçını aynı anda gönderir ve başarısız olan parçayı yeniden
dener. Sayfayı açık tutun. Bağlantı kesilirse yükleme geri geldiğinde devam eder. Sayfa kapandıysa ya da
yenilendiyse taşımayı yeniden açın ve **aynı dosyayı yeniden seçin**: Yuva elindekini karşılaştırır ve yalnızca
eksik parçaları gönderir. Başka bir dosya karıştırılmaz; sayfa o dosyayı baştan göndermeden önce sorar.
Bütün parçalar ulaşınca Yuva dosyayı okur ve biçimini söyler.

## Dosya reddedilirse

Yuva bir zip'ten hiçbir şey okumadan önce onu denetler. Reddedilen bir dosyanın yüklenmiş parçaları silinir;
dosyayı düzeltip yeniden yükleyin.

| Neden | Anlamı |
|---|---|
| `unknown_format` | Ne mbox ne de zip. |
| `empty` | İçinde hiç mesaj yok. |
| `corrupt` | Zip okunamıyor; bozuk ya da yarım kalmış olabilir. Yeniden oluşturun. |
| `path_traversal` | Bir girdinin adında `..` ya da mutlak yol var. |
| `symlink` | Bir girdi sembolik bağ. Zip'i bağlar olmadan yeniden oluşturun. |
| `too_many_entries` | Zip'te 1.000.000'dan fazla girdi var. |
| `zip_bomb` | Bir girdi 100:1'den fazla sıkıştırılmış. |
| `entry_too_large` | Bir girdi, sunucunun tek dosya sınırından büyük. |
| `encrypted` | Bir girdinin şifresi var. |
| `unsupported_compression` | Bir girdi ne saklanmış ne de deflate ile sıkıştırılmış. |
| `compressed_mbox` | Zip içindeki bir `.mbox` sıkıştırılmış. Arşivi zip'ten çıkarın ve `.mbox` dosyasının kendisini yükleyin. |

Sunucunun sınırından büyük bir dosya gönderilmeden reddedilir. Sunucunun sınırı `0` ise dosya içe aktarımı
kapalıdır.

## Dosya silinir

Yüklenen parçalar, yani dosya, taşıma tamamlandığında, iptal edildiğinde ya da silindiğinde silinir. Yuva
yalnızca içe aktardığını tutar. Taşıma sayfası dosyanın ne zaman silindiğini söyler.

## Postanızdan dosya alın

Aşağıdaki her sayfa programın kendi yardımıdır.

- **Google Takeout.** [Google Takeout](https://takeout.google.com)'u açın, yalnızca Mail'i seçin, tek seferlik
  dışa aktarımı **Zip dosyaları** olarak seçip oluşturun. Arşiv hazır olunca Google size bir bağlantı
  e-postalar; bu birkaç dakikadan birkaç güne sürebilir, bağlantı yaklaşık 7 gün ve 5 indirme için geçerlidir.
  Arşivi zip'ten çıkarın ve Mail klasöründeki `.mbox` dosyasını yükleyin. Google'ın sayfası:
  [Google verilerinizi dışa aktarın](https://support.google.com/accounts/answer/3024190).
- **Apple Mail.** Posta kutularını seçin ve **Mailbox > Export Mailbox**'ı seçin. Mail her birini bir `.mbox`
  dosyası olarak yazar. Apple'ın sayfası:
  [Mail'de posta kutularını içe veya dışa aktarma](https://support.apple.com/guide/mail/import-or-export-mailboxes-mlhlp1030/mac).
- **Thunderbird.** Thunderbird her posta klasörünü profilinizde bir mbox dosyası olarak tutar. Mozilla'nın
  sayfası:
  [Profiller - Thunderbird kullanıcı verilerini nerede saklar](https://support.mozilla.org/en-US/kb/profiles-where-thunderbird-stores-user-data).
  Bir klasörün dosyasını yükleyin.
- **Outlook.** Outlook `.pst` olarak dışa aktarır; Yuva bunu okumaz. `.pst` dosyasını istediğiniz bir
  dönüştürme aracıyla `.mbox` ya da `.eml` dosyalarına çevirin, `.eml` dosyalarını bir `.zip` içine koyun ve
  onu yükleyin. Microsoft'un sayfası:
  [E-postayı bir Outlook .pst dosyasına dışa aktarma veya yedekleme](https://support.microsoft.com/en-us/office/export-or-backup-email-contacts-and-calendar-to-an-outlook-pst-file-14252b52-3075-4e9b-be4e-ff9ef1068f91).
- **Proton Mail.** Proton'un dışa aktarma aracı `.eml` dosyaları yazar. Onları bir `.zip` içine koyup
  yükleyin. Proton'un sayfası: [E-postaları dışa aktarma](https://proton.me/support/export-emails-import-export-app).

## Çok büyük Gmail posta kutuları

Bir Takeout dosyasıyla başlayın, sonra yeni e-postalar ve geçiş için posta kutusunu IMAP ile bağlayın:
[Gmail](migrate-gmail.md#çok-büyük-posta-kutusu) ve
[Google Workspace](migrate-google-workspace.md#çok-büyük-posta-kutusu) rehberlerine bakın.

## Arka uçtan

Gelen kutusu sınırı olmayan sahipler, yöneticiler ve `migrations:manage` kapsamı olan API anahtarları. Bir
taşımayı `file` ya da `takeout` ön ayarıyla başlatın, sonra:

| Uç nokta | Kullanım |
|---|---|
| `GET /v1/migrations/{migrationId}/upload` | Sunucudaki parçalar; yarıda kalan yüklemeye devam etmek için. |
| `POST /v1/migrations/{migrationId}/upload/parts/{part}` | `part` numaralı parçayı gönderir (0'dan başlar): en çok 8 MiB'lık bir `file` ve `sha256` değeri, `multipart/form-data` olarak. Parçalar herhangi bir sırada ve paralel gelebilir. |
| `POST /v1/migrations/{migrationId}/upload/complete` | Parçaları denetler ve dosyayı okur; taşıma `connected` olur. |

Ele alınmaya değer hatalar: `413 part_too_large`, `422 checksum_mismatch` (parça saklanmaz),
`422 hosted_limit` (dosya sunucunun sınırını aşıyor), `422 upload_incomplete` (`missing_parts` eksik ilk
parçaları adlandırır), `422 file_refused` (`reason` ve suçlu bir girdi varsa zip `entry`) ve
`409 migration_state`. Akışın geri kalanı [Yuva'ya taşı](migrate.md#arka-uçtan) sayfasındadır; tam şekiller
[API sözleşmesindedir](../../openapi/openapi.yaml).
