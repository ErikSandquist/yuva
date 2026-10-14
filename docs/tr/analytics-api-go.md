# Go'da API izleme

Go paketi `net/http` işleyicinizi sarar ve yanıtladığı her isteği rota başına kod yazmadan bildirir. Bir API özelliği ve
`analytics:write` kapsamlı bir anahtar gerekir: bkz. [API özelliği oluşturun](analytics-api.md#api-özelliği-oluşturun).

```sh
go get github.com/productdevbook/yuva/sdk/go/analytics
```

`Middleware` bir `net/http` işleyicisini sarar ve yanıtladığı her isteği bildirir. Mux'ın eşleştirdiği kalıbın
bilinmesi için `http.ServeMux`'un kendisini sarın: `GET /users/{id}`, `/users/{id}` rotası olarak kaydedilir.

```go
import "github.com/productdevbook/yuva/sdk/go/analytics"

mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", getUser)

monitor, err := analytics.New(analytics.Config{
	BaseURL:    "https://support.example.com",
	APIKey:     os.Getenv("YUVA_API_KEY"),
	PropertyID: os.Getenv("YUVA_API_PROPERTY"),
})
if err != nil {
	log.Fatal(err)
}
defer monitor.Close(context.Background())

handler := monitor.Middleware(analytics.Monitor{
	Service:  "shop-api",
	Consumer: func(r *http.Request) string { return apiKeyName(r) },
})(mux)
log.Fatal(http.ListenAndServe(":8080", handler))
```

Kayıtlar, [sunucu tarafı olayların](analytics-server.md#go) yeniden denemeleriyle, 5 saniyede bir en çok 500'lük
gzip gruplar halinde gider. Süreç çıkmadan önce `Close` çağırın; yoksa bekleyen kayıtlar kaybolur.

| `Monitor` | Varsayılan | |
|---|---|---|
| `Route` | mux'ın kalıbı | Yönlendiriciniz `ServeMux` değilse bir isteğin rotasını verir. |
| `Consumer` | yok | Kimin çağırdığı: bir API anahtarının adı ya da istemci kimliği. Hiçbir zaman bir kişinin adresi olmaz; e-posta biçimindeki değer `(scrubbed)` olarak saklanır. |
| `APIVersion` | yok | İsteğin kullandığı API sürümü. |
| `Environment` | `production` | `production`, `staging` gibi ortamları ayırır. |
| `Service` | yok | İsteği karşılayan servis. |
| `Release` | yok | Servisin sürümü. Kabul edilir, henüz gösterilmez. |
| `SampleRate` | her istek | `10`, başarılı isteklerin (durum 1-399) ondan birini gönderir; her biri on isteği temsil eder. Hatalar ve istemci hataları her zaman gönderilir. |
| `Country` | kapalı | Yuva'nın isteğin ülkesini sayabilmesi için arayanın adresini (`Config.ClientIP`) gönderir. Varışta kısaltılır ve hiç saklanmaz. |

Hiçbir kalıbın eşleşmediği istek yoluyla gönderilir. Panik eden bir işleyici yazdığı durumla kaydedilir; hiçbir şey
yazmadıysa `0` ile.

Rotalar, nelerin kaydedildiği ve sayıların nasıl okunacağı her yığında aynıdır: [API izleme](analytics-api.md). Node, Bun
ve Deno için bkz. [JavaScript'te API izleme](analytics-api-js.md), diğer tüm diller için
[OpenTelemetry ile API izleme](analytics-api-otel.md).
