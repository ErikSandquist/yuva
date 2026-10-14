# API monitoring in Go

The Go package wraps your `net/http` handler and reports every request it answers, with no code per route. You need
an API property and a key with the scope `analytics:write`: see [Make an API property](analytics-api.md#make-an-api-property).

```sh
go get github.com/productdevbook/yuva/sdk/go/analytics
```

`Middleware` wraps a `net/http` handler and reports every request it answers. Wrap the `http.ServeMux` itself, so
the pattern the mux matched is known: `GET /users/{id}` is recorded as the route `/users/{id}`.

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

Records go out in gzip batches of up to 500 every 5 seconds, with the retries of [server-side events](analytics-server.md#go).
Call `Close` before the process exits, or the records still waiting are lost.

| `Monitor` | Default | |
|---|---|---|
| `Route` | the mux's pattern | Names the route of a request when your router is not a `ServeMux`. |
| `Consumer` | none | Who called: an API key's name or a client id. Never a person's address; a value shaped like an e-mail address is stored as `(scrubbed)`. |
| `APIVersion` | none | The API version the request used. |
| `Environment` | `production` | Tells apart `production`, `staging` and the like. |
| `Service` | none | The service that served the request. |
| `Release` | none | The service's release. It is accepted but not shown yet. |
| `SampleRate` | every request | `10` sends one in ten successful requests (status 1 to 399), each standing for ten. Errors and client errors are always sent. |
| `Country` | off | Sends the caller's address (`Config.ClientIP`) so Yuva can count the request's country. It is shortened on arrival and never stored. |

A request no pattern matched is sent with its path. A handler that panics is recorded with the status it wrote,
or `0` when it wrote none.

Routes, what is recorded and how to read the numbers are the same for every stack: [API monitoring](analytics-api.md). For
Node, Bun and Deno see [API monitoring in JavaScript](analytics-api-js.md), for any other language
[API monitoring with OpenTelemetry](analytics-api-otel.md).
