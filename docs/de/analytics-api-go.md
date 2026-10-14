# API-Monitoring in Go

Das Go-Paket umschließt Ihren `net/http`-Handler und meldet jeden Request, den er beantwortet, ohne Code pro Route. Sie
brauchen eine API-Property und einen Schlüssel mit dem Scope `analytics:write`: siehe
[Eine API-Property anlegen](analytics-api.md#eine-api-property-anlegen).

```sh
go get github.com/productdevbook/yuva/sdk/go/analytics
```

`Middleware` umschließt einen `net/http`-Handler und meldet jeden Request, den er beantwortet. Umschließen Sie den
`http.ServeMux` selbst, damit das vom Mux gefundene Muster bekannt ist: `GET /users/{id}` wird als Route
`/users/{id}` erfasst.

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

Records gehen alle 5 Sekunden in gzip-Batches von bis zu 500 hinaus, mit den Wiederholungen der
[serverseitigen Events](analytics-server.md#go). Rufen Sie `Close` auf, bevor der Prozess endet, sonst gehen die
wartenden Records verloren.

| `Monitor` | Standard | |
|---|---|---|
| `Route` | das Muster des Mux | Nennt die Route eines Requests, wenn Ihr Router kein `ServeMux` ist. |
| `Consumer` | keiner | Wer aufgerufen hat: der Name eines API-Schlüssels oder eine Client-ID. Nie die Adresse einer Person; ein Wert in E-Mail-Form wird als `(scrubbed)` gespeichert. |
| `APIVersion` | keine | Die API-Version, die der Request nutzte. |
| `Environment` | `production` | Unterscheidet `production`, `staging` und dergleichen. |
| `Service` | keiner | Der Service, der den Request beantwortet hat. |
| `Release` | keines | Das Release des Service. Es wird angenommen, aber noch nicht angezeigt. |
| `SampleRate` | jeder Request | `10` sendet jeden zehnten erfolgreichen Request (Status 1 bis 399), jeder steht für zehn. Fehler und Client-Fehler werden immer gesendet. |
| `Country` | aus | Sendet die Adresse des Aufrufers (`Config.ClientIP`), damit Yuva das Land des Requests zählen kann. Sie wird beim Eingang gekürzt und nie gespeichert. |

Ein Request, auf den kein Muster passte, wird mit seinem Pfad gesendet. Ein Handler, der in Panik gerät, wird mit
dem Status erfasst, den er geschrieben hat, oder mit `0`, wenn er keinen schrieb.

Routen, was erfasst wird und wie Sie die Zahlen lesen, ist für jeden Stack gleich: [API-Monitoring](analytics-api.md). Für
Node, Bun und Deno siehe [API-Monitoring in JavaScript](analytics-api-js.md), für jede andere Sprache
[API-Monitoring mit OpenTelemetry](analytics-api-otel.md).
