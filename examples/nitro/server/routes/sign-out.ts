import { defineHandler, deleteCookie } from "nitro/h3"

export default defineHandler((event) => {
  deleteCookie(event, "demo_user", { path: "/" })
  return Response.redirect(new URL("/", event.req.url), 303)
})
