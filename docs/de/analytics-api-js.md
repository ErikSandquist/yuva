# API-Monitoring in JavaScript

Das Paket `useyuva` meldet jeden Request, den Ihre Node-, Bun- oder Deno-API beantwortet, mit einer Middleware und ohne
Code pro Route. Der Router jedes Frameworks sagt, welche Route gepasst hat, sodass `/users/42` als `/users/{id}`
gezählt wird. Sie brauchen eine API-Property und einen Schlüssel mit dem Scope `analytics:write`: siehe
[Eine API-Property anlegen](analytics-api.md#eine-api-property-anlegen). Behalten Sie den Schlüssel auf dem Server.

```sh
npm install useyuva
```

Legen Sie einen Monitor an und geben Sie ihn der Middleware Ihres Frameworks. Nötig sind Node 22 oder neuer, Bun, Deno
oder eine Worker-Laufzeit; verwendet wird nur `fetch`.

```ts
// monitor.ts
import { createMonitor } from "useyuva/server"

export const monitor = createMonitor({
  server: "https://support.example.com",
  apiKey: process.env.YUVA_API_KEY!,
  propertyId: process.env.YUVA_API_PROPERTY!,
  service: "shop-api",
})
```

| Option | Standard | |
|---|---|---|
| `server`, `apiKey`, `propertyId` | erforderlich | Ihre Yuva-Adresse, der Schlüssel und die ID der API-Property. |
| `environment` | `production` | Unterscheidet `production`, `staging` und Ähnliches. |
| `service`, `release` | keiner | Der Service, der den Request beantwortet hat, und sein Release. Das Release wird akzeptiert, aber noch nicht angezeigt. |
| `sampleRate` | jeder Request | `10` sendet jeden zehnten erfolgreichen Request (Status 1 bis 399), jeder steht für zehn. Fehler, Client-Fehler und Status `0` werden immer gesendet. |
| `flushInterval` | 5000 | Millisekunden zwischen Batches. Ein voller Batch von 500 geht sofort. |
| `queueSize` | 50.000 | So viele Records warten höchstens; weitere werden verworfen und erreichen `onError`. |
| `maxRetries`, `minBackoff` | 5, 1000 | Wiederholungen eines Batches nach Netzwerkfehler, `429` oder `5xx` und die erste Wartezeit in Millisekunden, bei jeder Wiederholung verdoppelt. Ein längeres `Retry-After` gilt. |
| `onError` | keiner | Wird mit einem `Error` aufgerufen: ein `MonitorRecordError` für einen Record, den Yuva als ungültig ausgelassen hat, oder der Fehler eines Batches. |
| `fetch` | globales `fetch` | Sendet die Requests. |

Die Records gehen gzip-komprimiert hinaus. Jede Middleware nimmt außerdem diese Funktionen, jede wird mit dem Request des
Frameworks aufgerufen:

| Option | |
|---|---|
| `consumer(request)` | Wer aufgerufen hat: der Name eines API-Schlüssels oder eine Client-ID. Nie die Adresse einer Person; ein Wert in Form einer E-Mail-Adresse wird als `(scrubbed)` gespeichert. |
| `apiVersion(request)` | Die API-Version, die der Request nutzte. |
| `clientIP(request)` | Die Adresse des Aufrufers, damit Yuva das Land des Requests zählen kann. Sie wird beim Eingang gekürzt und nie gespeichert. Ohne sie wird keine Adresse gesendet. |

Eine Middleware erfasst die Methode, die gepasste Route, den Status (`0`, wenn keine Antwort gesendet wurde), die Dauer,
die Body-Größen aus `Content-Length` und die Startzeit. Nie einen Body, Header, Query-String oder eine Adresse (außer
dem, was `clientIP` liefert). Ein Request, auf den keine Route passte, wird mit seinem Pfad gesendet, und Yuva zählt
ein solches `404` oder `405` als `(unmatched)`; siehe [Routen](analytics-api.md#routen).

## Eine Routensyntax

Wie auch immer der Router die Route schreibt, sie wird gleich gespeichert: `:id` und `[id]` werden zu `{id}`, ein
Catch-all wird zu `{path...}`. Dieselbe API in jedem Framework, in [Go](analytics-api-go.md) oder mit
[OpenTelemetry](analytics-api-otel.md) zeigt dieselben Routen.

## Wann Records gesendet werden

- **Serverless-Plattformen** (Cloudflare Workers, Vercel, Lambda und Ähnliche): Die Middleware übergibt nach jeder
  Antwort einen Flush an das `waitUntil` des Requests, nichts wartet auf das Intervall.
- **Dauerhaft laufende Server**: Records werden nach `flushInterval` gebündelt und beim Schließen des Servers gesendet.
  Rufen Sie `await monitor.close()` auf, bevor der Prozess endet, sonst gehen wartende Records verloren. srvx bietet
  `waitUntil` auch auf Node, Bun und Deno, aber dort bündelt die Queue trotzdem nach Intervall.

`monitor.record(record)` erfasst einen Request wie die Middleware, `monitor.send(record)` stellt einen unverändert in die
Queue.

## srvx

```ts
import { serve } from "srvx"
import { yuvaMonitor } from "useyuva/srvx"

serve({ plugins: [yuvaMonitor(monitor, { route: (request) => routeOf(request) })], fetch: app.fetch })
```

srvx hat keinen Router; die Route ist das, was `route(request)` liefert, sonst die, die eine h3-App in den Kontext des
Requests gelegt hat. `server.close()` sendet die wartenden Records.

## h3

```ts
import { H3 } from "h3"
import { defineYuvaMonitor } from "useyuva/h3"

const app = new H3().use(defineYuvaMonitor({ monitor })).get("/users/:id", getUser)
```

Registrieren Sie sie als Erstes, vor anderen Middleware.

## Nitro und Nuxt

Beide lesen den Schlüssel aus der Server-Laufzeitkonfiguration, nie aus der öffentlichen, sodass er den Browser nicht
erreicht. Geben Sie die Yuva-Adresse und die beiden Werte an:

| | Nuxt | Nitro |
|---|---|---|
| Adresse | `yuva.server` oder `NUXT_YUVA_SERVER` | `NITRO_YUVA_SERVER` |
| Schlüssel | `NUXT_YUVA_MONITOR_API_KEY` | `NITRO_YUVA_MONITOR_API_KEY` |
| Property | `NUXT_YUVA_MONITOR_PROPERTY_ID` | `NITRO_YUVA_MONITOR_PROPERTY_ID` |

In Nuxt fügen Sie das Modul hinzu und setzen die Adresse in `nuxt.config`:

```ts
export default defineNuxtConfig({
  modules: ["useyuva/nuxt"],
  yuva: { server: "https://support.example.com", monitor: { service: "shop-api" } },
})
```

In Nitro nehmen Sie `yuva` aus `useyuva/nitro` in `modules` auf: `yuva({ server, monitor: { service: "shop-api" } })`.
`monitor` nimmt `apiKey`, `propertyId`, `environment`, `service`, `release`, `sampleRate` und `flushInterval`; Schlüssel
und Property-ID können stattdessen aus den Variablen kommen. Das Monitoring ist an, wenn beide zur Build-Zeit bekannt
sind, aus der Konfiguration oder der Umgebung; ein Wert, der nur in der Build-Umgebung steht, wird nicht in den Build
geschrieben, setzen Sie ihn also auch zur Laufzeit. Die Routen kommen vom Router von Nitro. Requests für öffentliche
Dateien (Public Assets) und in Nuxt für `/_nuxt/` (das Verzeichnis der Build-Assets) werden nicht erfasst, damit
Asset-Pfade nie zu Routen werden. Records gehen über `waitUntil`, wo die Plattform eines hat, und beim Schließen von
Nitro hinaus. Es läuft auf Nitro v2 und v3.

## Hono

```ts
import { Hono } from "hono"
import { yuvaMonitor } from "useyuva/hono"

const app = new Hono().use(yuvaMonitor(monitor)).get("/users/:id", getUser)
```

Die Route ist der Pfad des Handlers, der geantwortet hat, Sub-Apps eingeschlossen. Auf Cloudflare Workers gehen die
Records über `executionCtx.waitUntil` hinaus, mit `@hono/node-server` beim Schließen des Servers.

## Express

```ts
import { yuvaMonitor } from "useyuva/express"

app.use(yuvaMonitor(monitor))
app.get("/users/:id", getUser)
```

Die Route ist der Basispfad des Routers zusammen mit dem Pfad der Route. Eine Verbindung, die vor der Antwort
geschlossen wurde, ist Status `0`. Das `close` des HTTP-Servers sendet die wartenden Records.

## Fastify

```ts
import { yuvaMonitor } from "useyuva/fastify"

await app.register(yuvaMonitor(monitor))
app.get("/users/:id", getUser)
```

Jede Route der App wird erfasst, Plugins mit Präfix eingeschlossen. `app.close()` sendet die wartenden Records.

## Next.js-Route-Handler

Ein Route-Handler kennt sein Muster zur Laufzeit nicht, deshalb geben Sie es als zweites Argument an, geschrieben wie den
Ordner: `[id]` und `[...slug]` werden zu `{id}` und `{slug...}`, Routengruppen und Slots entfallen.

```ts
// app/users/[id]/route.ts
import { withYuvaMonitor } from "useyuva/next"
import { monitor } from "@/monitor"

export const GET = withYuvaMonitor(monitor, "/users/[id]", async (request, { params }) => {
  return Response.json(await findUser((await params).id))
})
```

`notFound()` und `redirect()` zählen mit ihrem Status. Gibt die Plattform Next.js ein `waitUntil` (Vercel), geht der Flush
über `after()`; `next start` bündelt nach Intervall.

## Die Zahlen lesen

Records erscheinen innerhalb von etwa 15 Sekunden in einer Abfrage. Eine störende Route ausblenden, Routenregeln und die
Abfrage stehen unter [API-Monitoring](analytics-api.md#routen-auflisten-und-ausblenden). Ohne Framework senden Sie
Records selbst mit [curl](analytics-api.md#requests-mit-curl-senden) oder `monitor.send`.
