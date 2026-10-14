# Yuva Go SDK

Helpers for a host application's backend. MIT licensed.

```sh
go get github.com/productdevbook/yuva/sdk/go
```

## API client

`client` is a typed client for `/v1`, generated from the OpenAPI contract with
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen). Authenticate with an API key (or an
OAuth access token) as a bearer token:

```go
import "github.com/productdevbook/yuva/sdk/go/client"

c, err := client.NewClientWithResponses("https://yuva.example.com",
	client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+os.Getenv("YUVA_API_KEY"))
		return nil
	}))
if err != nil {
	return err
}
res, err := c.ListInboxesWithResponse(ctx, nil)
if err != nil {
	return err
}
if res.JSON200 == nil {
	return fmt.Errorf("yuva: %s: %s", res.Status(), res.Body)
}
for _, inbox := range res.JSON200.Items {
	fmt.Println(inbox.Id, inbox.Name)
}
```

Every operation has a `…WithResponse` method whose result holds the decoded body for each documented
status (`JSON200`, `ApplicationproblemJSON403`, …) next to the raw `Body` and `HTTPResponse`.
`go generate ./...` in `sdk/go` regenerates it after the contract changes.

## Identity tokens

Signed-in users of your app chat as themselves: your backend signs a short-lived identity token
with the inbox's identity secret, and the widget or mobile SDK exchanges it for a contact session at
`POST /client/v1/session`. The secret is shown once when the inbox is created or its secret is
rotated; keep it on the server.

```go
import "github.com/productdevbook/yuva/sdk/go/identity"

token, err := identity.Sign(os.Getenv("YUVA_IDENTITY_SECRET"), identity.Claims{
	Subject:       user.ID, // required: your user id
	Email:         user.Email,
	EmailVerified: user.EmailConfirmed, // only then does Yuva match the address to a contact
	Name:          user.Name,
	Locale:        "en",
	Attrs:         map[string]any{"plan": user.Plan},
})
// Hand the token to the widget or the mobile SDK.
```

Tokens are valid for 5 minutes by default (`TTL`, at most 10). Sign a new one whenever the page or
app starts a session.

## Webhooks

Yuva signs webhooks per [Standard Webhooks](https://www.standardwebhooks.com). Verify the raw body
with the endpoint's secret (shown once as `whsec_…` when the endpoint is created or its secret is
rotated) before you parse it:

```go
import "github.com/productdevbook/yuva/sdk/go/webhook"

func handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := webhook.Verify(os.Getenv("YUVA_WEBHOOK_SECRET"), r.Header, body, 5*time.Minute); err != nil {
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}
	// Ignore a webhook-id you have already handled; Yuva retries until it gets a 2xx.
	var event struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(body, &event)
	// message.created with data.contact.online == false: send your push notification.
	w.WriteHeader(http.StatusNoContent)
}
```

During a secret rotation Yuva signs with both the new and the old secret for 24 hours, so either
secret verifies.

## Server-side analytics

`analytics` reports events only your backend sees, such as a signup or a payment, to an analytics
property. It needs an API key with the `analytics:write` scope and the property's id. `Track`
forwards the visitor's address, user agent and client hints from their request (never stored by
Yuva), so the event counts as the same visitor as their browser that day; requests with `DNT: 1` or
`Sec-GPC: 1` send nothing. Events are queued and sent in batches of up to 100 with retries.

```go
import "github.com/productdevbook/yuva/sdk/go/analytics"

events, err := analytics.New(analytics.Config{
	BaseURL:    "https://yuva.example.com",
	APIKey:     os.Getenv("YUVA_API_KEY"),
	PropertyID: os.Getenv("YUVA_ANALYTICS_PROPERTY"),
	OnError:    func(err error) { slog.Warn("yuva analytics", "error", err) },
})
if err != nil {
	return err
}
defer events.Close(context.Background())

func signup(w http.ResponseWriter, r *http.Request) {
	// ...
	_ = events.Track(r, analytics.Event{
		Name:    "Signup",
		Props:   map[string]any{"plan": "pro"},
		Revenue: &analytics.Revenue{Amount: "29.90", Currency: "EUR"},
	})
}
```

Behind a proxy, set `ClientIP` to read the visitor's address the way your proxy reports it. Without
a request, `Send` takes an `Event` with your own `VisitorKey` (a session id, never an e-mail
address) instead of `IP` and `UserAgent`.

## API monitoring

For an analytics property of kind `api`, `Middleware` records every request your `net/http`
server answers: route pattern, method, status, duration and body sizes. It never sends a body,
header, query string or user agent. Wrap the `http.ServeMux` itself, so the pattern the mux
matched (`GET /users/{id}` becomes `/users/{id}`) is set on the request the middleware sees; a
request no pattern matched is sent with its path, and Yuva counts 404s and 405s without a pattern
as `(unmatched)`. Records go out in batches of up to 500, gzip-compressed, every 5 seconds through
the same queue and retries as events.

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", getUser)

monitor, err := analytics.New(analytics.Config{
	BaseURL:    "https://yuva.example.com",
	APIKey:     os.Getenv("YUVA_API_KEY"),
	PropertyID: os.Getenv("YUVA_API_PROPERTY"),
})
if err != nil {
	return err
}
defer monitor.Close(context.Background())

handler := monitor.Middleware(analytics.Monitor{
	Service:  "shop-api",
	Consumer: func(r *http.Request) string { return apiKeyName(r) },
})(mux)
http.ListenAndServe(":8080", handler)
```

`Route` names the pattern when your router is not a `ServeMux`. `SampleRate: 10` sends one in ten
successful requests (status 1 to 399), each standing for ten; errors and client errors are always
sent. `Country: true` also sends the caller's address so Yuva can count its country; it is
shortened on arrival and never stored. `SendRecord` queues a `Record` you build yourself.
