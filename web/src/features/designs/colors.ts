import type { EmailThemeColors } from "@/lib/api"
import { contrast, darkBackground, darkBorder, formatColor, parseColor, readableOn, type Rgb } from "@/lib/emailDark"

const WHITE: Rgb = { r: 1, g: 1, b: 1, a: 1 }
const INK: Rgb = { r: 0.07, g: 0.07, b: 0.09, a: 1 }

export const COLOR_RE = /^#[0-9a-fA-F]{6}$/

export function ratio(a: string, b: string) {
  const x = parseColor(a)
  const y = parseColor(b)
  return x && y ? contrast(x, y) : 1
}

export function textOn(bg: string, wanted?: string) {
  if (wanted && ratio(wanted, bg) >= 4.5) return wanted
  const c = parseColor(bg)
  if (!c) return "#ffffff"
  return contrast(WHITE, c) >= contrast(INK, c) ? "#ffffff" : formatColor(INK)
}

export function derivedDark(light: EmailThemeColors): EmailThemeColors {
  const p = (v: string) => parseColor(v) ?? WHITE
  const card = darkBackground(p(light.card))
  const page = darkBackground(p(light.page))
  const on = (v: string) => formatColor(readableOn(p(v), card))
  return {
    page: formatColor(page),
    card: formatColor(card),
    text: on(light.text),
    muted: on(light.muted),
    accent: on(light.accent),
    link: on(light.link),
    border: formatColor(darkBorder(p(light.border), p(light.card), card)),
  }
}
