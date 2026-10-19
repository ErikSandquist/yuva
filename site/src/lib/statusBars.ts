export type BarTone = "ok" | "warn" | "partial" | "maint" | "bad" | "off"

export const CELL = 8
export const GAP = 2
export const PITCH = CELL + GAP

const square = (x: number, y: number) => `M${x} ${y}h${CELL}v${CELL}h-${CELL}Z`

export function bars(marks: Record<number, BarTone>, count = 90, off = 0) {
  const paths: Record<BarTone, string> = { ok: "", warn: "", partial: "", maint: "", bad: "", off: "" }
  for (let i = 0; i < count; i++) paths[i < off ? "off" : (marks[i] ?? "ok")] += square(i * PITCH, 0)
  return paths
}

export type ChartKey = "solid" | "solidSlow" | "wash" | "washSlow" | "off"

export function chart(rows: number[], height = 10, slowFrom = 7) {
  const paths: Record<ChartKey, string> = { solid: "", solidSlow: "", wash: "", washSlow: "", off: "" }
  rows.forEach((row, x) => {
    const prev = x === 0 ? row : rows[x - 1]
    const lo = Math.min(row, prev)
    const hi = Math.max(row, prev)
    const slow = row >= slowFrom
    for (let y = 0; y < height; y++) {
      const key: ChartKey =
        y >= lo && y <= hi ? (slow ? "solidSlow" : "solid") : y < lo ? (slow ? "washSlow" : "wash") : "off"
      paths[key] += square(x * PITCH, (height - 1 - y) * PITCH)
    }
  })
  return paths
}
