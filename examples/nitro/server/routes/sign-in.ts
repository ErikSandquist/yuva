import { defineHandler, setCookie } from "nitro/h3"

export default defineHandler((event) => {
  setCookie(event, "demo_user", "Ada", { httpOnly: true, sameSite: "lax", path: "/" })
  event.context.yuva?.track({ name: "Sign in" })
  return Response.redirect(new URL("/", event.req.url), 303)
})
