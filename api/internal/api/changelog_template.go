package api

import "html/template"

const changelogStyle = `
.skip{position:absolute;inset-inline-start:8px;top:-60px;z-index:9;background:var(--ink);color:var(--paper);padding:8px 14px;border-radius:8px}
.skip:focus{top:8px}
ol.crumbs{margin:0;padding:0}
.crumbs{display:flex;flex-wrap:wrap;gap:4px 8px;font-size:13px;color:var(--soft)}
.crumbs li{list-style:none;display:inline-flex;gap:8px}
.crumbs li+li::before{content:"/";color:var(--cross)}
.langs{display:flex;flex-wrap:wrap;gap:4px 14px;font-size:13px}
.langs a[aria-current]{font-weight:600;color:var(--ink)}
.filters{display:flex;flex-wrap:wrap;gap:8px;margin-top:18px}
.filters a{border:1px solid var(--strong);border-radius:999px;padding:3px 12px;font-size:13px;color:var(--ink)}
.filters a[aria-current]{background:var(--ink);color:var(--paper);text-decoration:none}
.rel{display:grid;grid-template-columns:minmax(120px,200px) minmax(0,1fr);gap:8px 32px;padding:28px 0;border-bottom:1px solid var(--rule)}
.rel .when{font-size:13px;color:var(--soft)}
.rel h2,.rel h1{margin:0 0 10px;font-size:22px;line-height:1.25;overflow-wrap:anywhere}
.rel h2 a{color:inherit}
.chips{display:flex;flex-wrap:wrap;gap:6px;margin:0 0 12px;padding:0}
.chips li{list-style:none;border:1px solid var(--strong);border-radius:999px;padding:1px 10px;font-size:12px}
.chips .new{border-color:#16a34a;color:#15803d}
.chips .improved{border-color:#2563eb;color:#1d4ed8}
.chips .fixed{border-color:#ca8a04;color:#a16207}
.rel img.cover{display:block;max-width:100%;height:auto;border-radius:8px;border:1px solid var(--rule);margin:0 0 16px}
.prose{overflow-wrap:anywhere}
.prose>*+*{margin-top:14px}
.prose p,.prose li{font-size:16px;line-height:1.65}
.prose ul,.prose ol{padding-inline-start:24px}
.prose img{max-width:100%;height:auto;border-radius:6px}
.prose a{color:var(--brand);text-decoration:underline;text-underline-offset:3px}
.prose pre{margin:0;padding:14px 16px;overflow:auto;background:var(--deep);border:1px solid var(--rule);border-radius:8px}
.prose code{font-size:.9em}
.prose blockquote{margin-inline:0;padding:4px 16px;border-inline-start:3px solid var(--strong);color:var(--soft)}
.prose table{border-collapse:collapse;display:block;overflow:auto}
.prose td,.prose th{border:1px solid var(--rule);padding:4px 10px}
.more{padding:24px 0}
@media (max-width:700px){.rel{grid-template-columns:minmax(0,1fr)}}
`

var changelogScript = `var L=document.documentElement.lang;` +
	`document.querySelectorAll("time[data-local]").forEach(function(t){var d=new Date(t.getAttribute("datetime"));if(!isNaN(d))t.textContent=d.toLocaleDateString(L,{dateStyle:"medium"})})`

var changelogScriptHash = publicScriptHash(changelogScript)

var changelogPageTemplate = template.Must(template.New("changelog").Parse(`<!doctype html>
<html lang="{{.Lang}}" dir="{{.Dir}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
{{range .Alternates}}<link rel="alternate" hreflang="{{.Hreflang}}" href="{{.Href}}">
{{end}}<link rel="alternate" type="application/atom+xml" href="{{.FeedURL}}" title="{{.W.Feed}}">
{{if .NoIndex}}<meta name="robots" content="noindex">
{{end}}<meta property="og:site_name" content="{{.Name}}"><meta property="og:type" content="{{.OGType}}"><meta property="og:title" content="{{.OGTitle}}"><meta property="og:description" content="{{.Description}}"><meta property="og:url" content="{{.Canonical}}"><meta property="og:locale" content="{{.OGLocale}}">
{{if .OGImage}}<meta property="og:image" content="{{.OGImage}}"><meta name="twitter:card" content="summary_large_image">{{else}}<meta name="twitter:card" content="summary">{{end}}
{{.JSONLD}}
<link rel="preload" href="{{.FontSans}}" as="font" type="font/woff2" crossorigin>
<style>
@font-face{font-family:Geist;src:url("{{.FontSans}}") format("woff2");font-weight:100 900;font-display:swap}
@font-face{font-family:"Geist Mono";src:url("{{.FontMono}}") format("woff2");font-weight:100 900;font-display:swap}
` + statusStyle + changelogStyle + `{{if .Accent}}:root{--brand:{{.Accent}}}{{end}}
</style></head><body{{if ne .View "home"}} class="sub"{{end}}>
{{define "x"}}<svg class="x s" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg><svg class="x e" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg>{{end}}
{{define "chips"}}<ul class="chips" aria-label="{{.Caption}}">{{range .Labels}}<li class="{{.Code}}">{{.Name}}</li>{{end}}</ul>{{end}}
<a class="skip" href="#main">{{.W.Skip}}</a>
<header class="band"><div class="col bar">{{template "x"}}
<a class="brandmark" href="{{.Home}}">{{if .Logo}}<img src="{{.Logo}}" alt="">{{end}}<span class="name">{{.Name}}</span><span class="label">{{.W.Title}}</span></a>
<nav class="top"></nav></div></header>
<main id="main">
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide">{{if .Crumbs}}<ol class="crumbs">{{range .Crumbs}}<li>{{if .URL}}<a href="{{.URL}}">{{.Name}}</a>{{else}}<span>{{.Name}}</span>{{end}}</li>{{end}}</ol>{{else}}<span class="label">{{.W.Title}}</span>{{end}}
{{if eq .View "home"}}<h1>{{.H1}}</h1>{{if .Lead}}<p class="lead">{{.Lead}}</p>{{end}}
<nav class="filters" aria-label="{{.W.Filter}}">{{range .Filters}}<a href="{{.Href}}"{{if .On}} aria-current="true"{{end}}>{{.Name}}</a>{{end}}</nav>{{end}}</div>
</div></section>
<section class="band"><div class="col">{{template "x"}}
{{range .Items}}<article class="rel"><div class="when"><time datetime="{{.At.ISO}}" data-local>{{.At.Text}}</time></div>
<div>{{if $.Single}}<h1>{{.Title}}</h1>{{else}}<h2><a href="{{.URL}}">{{.Title}}</a></h2>{{end}}
{{template "chips" .}}{{if .Image}}<img class="cover" src="{{.Image}}" width="{{.ImageW}}" height="{{.ImageH}}" alt="">{{end}}
<div class="prose">{{.Body}}</div></div></article>
{{else}}<div class="quiet">{{.W.Empty}}</div>{{end}}
{{if .Older}}<p class="more"><a href="{{.Older}}" rel="next">{{.W.Older}}</a></p>{{end}}
{{if .Single}}<p class="more"><a href="{{.Home}}">{{.W.Back}}</a></p>{{end}}
</div></section>
</main>
<footer><div class="col foot"><span>{{if gt (len .Langs) 1}}<nav class="langs" aria-label="{{.SW.Language}}">{{range .Langs}}<a href="{{.Href}}" hreflang="{{.Code}}" lang="{{.Code}}"{{if .On}} aria-current="true"{{end}}>{{.Label}}</a>{{end}}</nav>{{end}}</span>
<span class="links"><a href="{{.FeedURL}}">{{.W.Feed}}</a><a class="powered" href="{{.YuvaSite}}" rel="noopener">{{.Mark}}{{.SW.Powered}}</a></span></div></footer>
<script>{{.Script}}</script>
</body></html>`))
