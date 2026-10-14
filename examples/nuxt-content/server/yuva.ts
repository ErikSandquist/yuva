import { defineYuvaServer } from "useyuva/nitro/runtime"

export default defineYuvaServer({
  user: (event) => ({
    subject: "demo-reader",
    name: "Demo Reader",
    locale: getHeader(event, "accept-language")?.slice(0, 2),
  }),
})
