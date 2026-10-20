export const styles = `
.app { flex: 1; min-block-size: 0; display: flex; flex-direction: column; }
.app .page { flex: 1; }
.seg { display: grid; grid-auto-flow: column; grid-auto-columns: 1fr; border: 1px solid var(--yuva-strong); border-radius: 10px; overflow: hidden; }
.seg button { min-block-size: 44px; padding: 0 8px; border: 0; background: transparent; color: var(--yuva-muted); cursor: pointer; }
.seg button + button { border-inline-start: 1px solid var(--yuva-strong); }
.seg button[aria-pressed="true"] { background: var(--yuva-fg); color: var(--yuva-bg); }
.scores { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
.scores button { min-block-size: 48px; padding: 0; border: 1px solid var(--yuva-strong); border-radius: 10px; background: transparent; color: var(--yuva-muted); font-family: var(--yuva-mono); font-size: 15px; font-weight: 500; cursor: pointer; }
.scores button[aria-pressed="true"] { background: var(--yuva-wash); border-color: var(--yuva-brand); color: var(--yuva-brand); }
.ends { display: flex; justify-content: space-between; font-size: 12px; color: var(--yuva-faint); }
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
.hback { flex: none; padding: 6px 28px 0; }
.hback .link { display: inline-flex; align-items: center; gap: 4px; }
.hrows { display: flex; flex-direction: column; margin-block-end: -8px; }
.hrow { display: flex; align-items: center; gap: 12px; min-block-size: 56px; padding: 8px 0; border: 0; border-block-end: 1px solid var(--yuva-border); background: transparent; color: inherit; text-align: start; cursor: pointer; }
.hrow:last-child { border-block-end: 0; }
.hrow-text { flex: 1; min-inline-size: 0; display: flex; flex-direction: column; gap: 2px; }
.hrow-text b { font-weight: 500; }
.hrow-text span { font-size: 12px; color: var(--yuva-muted); display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.hrow > svg { flex: none; color: var(--yuva-faint); }
.hrow mark, .hrow-text mark { background: var(--yuva-wash); color: inherit; border-radius: 2px; }
.hrow:hover .hrow-text b { text-decoration: underline; }
.pills { display: flex; flex-wrap: wrap; gap: 8px; }
.pill { display: inline-flex; align-items: center; gap: 8px; min-block-size: 36px; padding: 0 12px; border: 1px solid var(--yuva-strong); border-radius: 999px; background: transparent; color: inherit; cursor: pointer; }
.pill:hover { background: var(--yuva-deep); }
.pill span { font-family: var(--yuva-mono); font-size: 11px; color: var(--yuva-faint); }
.ask { display: flex; align-items: center; gap: 12px; }
.ask-text { flex: 1; min-inline-size: 0; display: flex; flex-direction: column; }
.ask-text b { font-weight: 500; }
.ask-text span { font-size: 12px; color: var(--yuva-muted); }
.article-title { margin: 0; font-size: 22px; line-height: 1.2; font-weight: 600; letter-spacing: -0.01em; overflow-wrap: anywhere; }
.prose { overflow-wrap: anywhere; line-height: 1.55; }
.prose > :first-child { margin-block-start: 0; }
.prose > :last-child { margin-block-end: 0; }
.prose h1, .prose h2, .prose h3, .prose h4 { margin: 20px 0 8px; font-size: 16px; line-height: 1.3; font-weight: 600; }
.prose p, .prose ul, .prose ol, .prose pre, .prose blockquote, .prose table { margin: 0 0 12px; }
.prose ul, .prose ol { padding-inline-start: 20px; }
.prose a { color: var(--yuva-brand); }
.prose img { max-inline-size: 100%; block-size: auto; border-radius: 8px; }
.prose code { font-family: var(--yuva-mono); font-size: 13px; padding: 1px 4px; border-radius: 4px; background: var(--yuva-deep); }
.prose pre { padding: 10px; border-radius: 8px; background: var(--yuva-deep); overflow-x: auto; }
.prose pre code { padding: 0; background: none; }
.prose blockquote { padding-inline-start: 12px; border-inline-start: 2px solid var(--yuva-strong); color: var(--yuva-muted); }
.prose table { display: block; overflow-x: auto; border-collapse: collapse; }
.prose th, .prose td { padding: 4px 8px; border: 1px solid var(--yuva-border); text-align: start; }
`
