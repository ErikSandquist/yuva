import { getCookie } from "nitro/h3"
import { defineYuvaServer } from "useyuva/nitro/runtime"

export default defineYuvaServer({
  user(event) {
    const name = getCookie(event, "demo_user")
    return name ? { subject: `demo-${name.toLowerCase()}`, name } : null
  },
  webhooks: {
    "message.created": (payload) => {
      console.log("message.created", payload.conversation_id, payload.data.id)
    },
  },
})
