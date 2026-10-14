import { notFound } from "next/navigation"
import { withYuvaMonitor } from "../../../../../src/next"
import { createMonitor } from "../../../../../src/server"

const monitor = createMonitor({
  server: process.env.YUVA_TEST_SERVER!,
  apiKey: "yuva_key",
  propertyId: "prop-1",
  flushInterval: 50,
})

export const GET = withYuvaMonitor(
  monitor,
  "/users/[id]",
  async (_request: Request, context: { params: Promise<{ id: string }> }) => {
    const { id } = await context.params
    if (id === "missing") notFound()
    return Response.json({ id })
  },
  { consumer: (request) => request.headers.get("x-client") }
)
