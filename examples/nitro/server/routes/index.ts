import { defineHandler, getCookie } from "nitro/h3"
import { useRuntimeConfig } from "nitro/runtime-config"

const escape = (value: string) => value.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)

export default defineHandler((event) => {
  const { server, channel } = useRuntimeConfig().yuva as { server: string; channel: string }
  const user = getCookie(event, "demo_user")
  const page = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Yuva on Nitro</title>
    <script src="${escape(server)}/yuva.js" defer></script>
  </head>
  <body>
    <p>${user ? `Signed in as ${escape(user)}. <a href="/sign-out">Sign out</a>` : `<a href="/sign-in">Sign in as Ada</a>`}</p>
    <yuva-chat channel="${escape(channel)}"></yuva-chat>
    <script type="module">
      await customElements.whenDefined("yuva-chat")
      document.querySelector("yuva-chat").setIdentityToken(async () => {
        const response = await fetch("/_yuva/identity", { credentials: "same-origin" })
        return response.ok ? (await response.json()).token : null
      })
    </script>
  </body>
</html>`
  return new Response(page, { headers: { "content-type": "text/html; charset=utf-8" } })
})
