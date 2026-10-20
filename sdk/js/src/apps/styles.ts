export const styles = `
.app { flex: 1; min-block-size: 0; display: flex; flex-direction: column; }
.app .page { flex: 1; }
.seg { display: grid; grid-auto-flow: column; grid-auto-columns: 1fr; border: 1px solid var(--yuva-strong); border-radius: 10px; overflow: hidden; }
.seg button { min-block-size: 44px; padding: 0 8px; border: 0; background: transparent; color: var(--yuva-muted); cursor: pointer; }
.seg button + button { border-inline-start: 1px solid var(--yuva-strong); }
.seg button[aria-pressed="true"] { background: var(--yuva-fg); color: var(--yuva-bg); }
.attach { display: flex; align-items: center; gap: 4px; }
.attach .chips { padding: 0; flex: 1; }
.rowbtns { display: flex; gap: 8px; align-self: stretch; }
.rowbtns > * { flex: 1; }
.stat { display: flex; align-items: center; gap: 12px; }
.stat-mark { display: grid; place-items: center; flex: none; inline-size: 40px; block-size: 40px; border-radius: 50%; background: color-mix(in srgb, var(--yuva-ok) 14%, var(--yuva-bg)); color: var(--yuva-ok); }
.stat-mark.warn { background: color-mix(in srgb, var(--yuva-warn) 16%, var(--yuva-bg)); color: var(--yuva-warn); }
.stat-mark.bad { background: color-mix(in srgb, var(--yuva-danger) 14%, var(--yuva-bg)); color: var(--yuva-danger); }
.stat-mark.maint { background: color-mix(in srgb, var(--yuva-info) 14%, var(--yuva-bg)); color: var(--yuva-info); }
.stat b { display: block; font-size: 16px; font-weight: 600; }
.stat span { font-family: var(--yuva-mono); font-size: 11px; letter-spacing: 0.08em; text-transform: uppercase; color: var(--yuva-faint); }
.comp { display: flex; flex-direction: column; gap: 6px; }
.comp-top { display: flex; justify-content: space-between; gap: 8px; }
.comp-top span:last-child { font-family: var(--yuva-mono); font-size: 12px; color: var(--yuva-muted); }
.cells { display: flex; gap: 2px; block-size: 22px; }
.cells i { flex: 1; min-inline-size: 1px; border-radius: 1px; background: var(--yuva-ok); }
.cells i.none { background: var(--yuva-border); }
.cells i.warn { background: var(--yuva-warn); }
.cells i.bad { background: var(--yuva-danger); }
.cells i.maint { background: var(--yuva-info); }
.inc { display: flex; flex-direction: column; gap: 2px; padding-block: 8px; border-block-end: 1px solid var(--yuva-border); }
.inc:last-of-type { border-block-end: 0; }
.inc a { color: inherit; font-weight: 500; text-decoration: none; }
.inc a:hover { text-decoration: underline; }
.inc span { font-size: 12px; color: var(--yuva-faint); }
.lists { display: flex; flex-direction: column; gap: 8px; }
.check { display: flex; gap: 10px; align-items: flex-start; min-block-size: 28px; font-size: 13px; color: var(--yuva-muted); }
.check input { flex: none; inline-size: 18px; block-size: 18px; margin: 1px 0 0; accent-color: var(--yuva-accent); }
.check b { display: block; font-weight: 500; color: var(--yuva-fg); }
.form { display: flex; flex-direction: column; gap: 12px; }
.form .row2 { display: flex; gap: 8px; }
.form .row2 .field { flex: 1; min-inline-size: 0; }
.done-card { display: flex; flex-direction: column; align-items: center; gap: 8px; text-align: center; padding: 8px 0; }
.done-card svg { inline-size: 28px; block-size: 28px; color: var(--yuva-ok); }
.done-card .rowbtns { inline-size: 100%; margin-block-start: 8px; }
.note { margin: 0; font-size: 12px; color: var(--yuva-faint); }
`
