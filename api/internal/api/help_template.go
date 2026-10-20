package api

import "html/template"

const helpStyle = `
h3,h4{margin:0}
.skip{position:absolute;inset-inline-start:8px;top:-60px;z-index:9;background:var(--ink);color:var(--paper);padding:8px 14px;border-radius:8px}
.skip:focus{top:8px}
.search{display:flex;flex-wrap:wrap;gap:8px;max-width:560px;margin-top:8px}
.search input{flex:1 1 220px;min-width:0;font:15px Geist,ui-sans-serif,system-ui,sans-serif;color:var(--ink);background:var(--paper);border:1px solid var(--strong);border-radius:8px;padding:10px 14px;min-height:44px}
.search input:focus-visible{outline:2px solid var(--brand);outline-offset:2px}
.crumbs{display:flex;flex-wrap:wrap;gap:4px 8px;font-size:13px;color:var(--soft)}
.crumbs li{list-style:none;display:inline-flex;gap:8px}
.crumbs li+li::before{content:"/";color:var(--cross)}
ol.crumbs,ul.plain{margin:0;padding:0}
ul.plain li{list-style:none}
.langs{display:flex;flex-wrap:wrap;gap:4px 14px;font-size:13px}
.langs a[aria-current]{font-weight:600;color:var(--ink)}
.entry{display:flex;flex-direction:column;gap:4px;padding:20px 28px;background:var(--paper);min-width:0}
.entry b{font-weight:600;font-size:16px;overflow-wrap:anywhere}
.entry span{color:var(--soft);font-size:14px;overflow-wrap:anywhere}
a.entry:hover{text-decoration:none;background:var(--deep)}
a.entry:hover b{text-decoration:underline;text-underline-offset:3px}
.subs{display:flex;flex-direction:column;gap:2px;margin-top:6px;font-size:13px}
mark{background:var(--wash);color:inherit;border-radius:3px;padding:0 2px}
.article{display:grid;grid-template-columns:minmax(0,1fr) 240px;gap:1px;background:var(--rule)}
.article>*{background:var(--paper);min-width:0}
.article.solo{grid-template-columns:minmax(0,1fr)}
.toc{padding:28px;display:flex;flex-direction:column;gap:8px;font-size:13px;align-self:start;position:sticky;top:0}
.toc a{color:var(--soft);overflow-wrap:anywhere}
.toc a.sub{padding-inline-start:12px}
.prose{padding:40px 40px 32px;overflow-wrap:anywhere}
.prose>*+*{margin-top:16px}
.prose h2{font-size:24px;font-weight:600;letter-spacing:-.5px;margin-top:32px}
.prose h3{font-size:19px;font-weight:600;letter-spacing:-.3px;margin-top:24px}
.prose h4{font-size:16px;font-weight:600;margin-top:20px}
.prose p,.prose li{font-size:16px;line-height:1.65}
.prose ul,.prose ol{padding-inline-start:24px}
.prose li+li{margin-top:6px}
.prose a{color:var(--brand);text-decoration:underline;text-underline-offset:3px}
.prose img{max-width:100%;height:auto;border-radius:8px;border:1px solid var(--rule)}
.prose blockquote{margin-inline:0;padding:4px 16px;border-inline-start:3px solid var(--strong);color:var(--soft)}
.prose code{display:inline;font-size:.9em;padding:2px 5px;word-break:normal;white-space:normal}
.prose pre{margin:0;padding:14px 16px;overflow:auto;background:var(--deep);border:1px solid var(--rule);border-radius:8px}
.prose pre code{display:block;padding:0;border:0;background:none;font-size:13px;white-space:pre;word-break:normal;overflow-wrap:normal}
.prose table{border-collapse:collapse;display:block;overflow-x:auto;max-width:100%}
.prose th,.prose td{border:1px solid var(--rule);padding:8px 12px;text-align:start}
.prose hr{border:0;border-top:1px solid var(--rule);margin-block:28px}
.meta{padding:0 40px 8px;font-size:13px;color:var(--soft)}
.helpful{padding:24px 40px;display:flex;flex-wrap:wrap;align-items:center;gap:10px 16px;border-top:1px solid var(--rule)}
.helpful form{display:flex;flex-wrap:wrap;align-items:center;gap:10px}
.helpful .btn.alt{background:var(--paper);color:var(--ink);border:1px solid var(--strong)}
.chat{padding:28px;display:flex;flex-direction:column;gap:6px;align-items:flex-start}
.chat strong{font-size:18px;font-weight:600;letter-spacing:-.3px}
@media (max-width:860px){.article{grid-template-columns:minmax(0,1fr)}.toc{position:static;order:-1}}
@media (max-width:700px){.prose,.meta,.helpful{padding-inline:16px}.entry{padding-inline:16px}.chat{padding-inline:16px}}
`

var helpTemplate = template.Must(template.New("help").Parse(`<!doctype html>
<html lang="{{.Lang}}" dir="{{.Dir}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
{{range .Alternates}}<link rel="alternate" hreflang="{{.Hreflang}}" href="{{.Href}}">
{{end}}{{if .NoIndex}}<meta name="robots" content="noindex">
{{end}}<meta property="og:site_name" content="{{.Name}}"><meta property="og:type" content="{{.OGType}}"><meta property="og:title" content="{{.OGTitle}}"><meta property="og:description" content="{{.Description}}"><meta property="og:url" content="{{.Canonical}}"><meta property="og:locale" content="{{.OGLocale}}">
<meta name="twitter:card" content="summary">
{{.JSONLD}}
<link rel="preload" href="{{.FontSans}}" as="font" type="font/woff2" crossorigin>
<style>
@font-face{font-family:Geist;src:url("{{.FontSans}}") format("woff2");font-weight:100 900;font-display:swap}
@font-face{font-family:"Geist Mono";src:url("{{.FontMono}}") format("woff2");font-weight:100 900;font-display:swap}
` + statusStyle + helpStyle + `{{if .Accent}}:root{--brand:{{.Accent}};--wash:color-mix(in srgb,{{.Accent}} 12%,var(--paper))}{{end}}
</style></head><body{{if ne .View "home"}} class="sub"{{end}}>
{{define "x"}}<svg class="x s" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg><svg class="x e" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg>{{end}}
{{define "searchform"}}<form class="search" role="search" method="get" action="{{.SearchURL}}"><label class="sr" for="q">{{.W.SearchLabel}}</label><input id="q" type="search" name="q" value="{{.Query}}" placeholder="{{.W.SearchPlaceholder}}" maxlength="200" autocomplete="off"><button class="btn" type="submit">{{.W.SearchGo}}</button></form>{{end}}
{{define "items"}}{{range .}}<a class="entry" href="{{.URL}}"><b>{{if .TitleHTML}}{{.TitleHTML}}{{else}}{{.Title}}{{end}}</b>{{if .Excerpt}}<span>{{if .ExcerptHTML}}{{.ExcerptHTML}}{{else}}{{.Excerpt}}{{end}}</span>{{end}}{{if .Collection}}<span class="label">{{.Collection}}</span>{{end}}</a>{{end}}{{end}}
<a class="skip" href="#main">{{.W.Skip}}</a>
<header class="band"><div class="col bar">{{template "x"}}
<a class="brandmark" href="{{.Home}}">{{if .Logo}}<img src="{{.Logo}}" alt="">{{end}}<span class="name">{{.Name}}</span><span class="label">{{.W.Help}}</span></a>
<nav class="top">{{if .Site}}<a href="{{.Site}}" rel="noopener">{{.SW.Site}}</a>{{end}}</nav></div></header>
<main id="main">
{{if eq .View "home"}}
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide"><span class="label">{{.W.Help}}</span><h1>{{.H1}}</h1>{{if .Lead}}<p class="lead">{{.Lead}}</p>{{end}}{{template "searchform" .}}</div>
</div></section>
{{if .Collections}}<section class="band"><div class="col">{{template "x"}}<div class="head"><span class="label">01 — {{.W.Collections}}</span></div>
<div class="grid">{{range .Collections}}<div class="entry"><a href="{{.URL}}"><b>{{.Name}}</b></a>{{if .Description}}<span>{{.Description}}</span>{{end}}<span class="label">{{.Count}}</span>{{if .Subs}}<span class="subs">{{range .Subs}}<a href="{{.URL}}">{{.Name}} <span class="soft">· {{.Count}}</span></a>{{end}}</span>{{end}}</div>{{end}}</div>
</div></section>{{end}}
{{if .Popular}}<section class="band"><div class="col">{{template "x"}}<div class="head"><span class="label">02 — {{.W.Popular}}</span></div><div class="rows">{{template "items" .Popular}}</div></div></section>{{end}}
{{if .Items}}<section class="band"><div class="col">{{template "x"}}<div class="head"><span class="label">{{.W.Other}}</span></div><div class="rows">{{template "items" .Items}}</div></div></section>{{end}}
{{else if eq .View "collection"}}
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide">{{template "crumbs" .}}<h1>{{.H1}}</h1>{{if .Lead}}<p class="lead">{{.Lead}}</p>{{end}}{{template "searchform" .}}</div>
</div></section>
{{if .Collections}}<section class="band"><div class="col">{{template "x"}}<div class="head"><span class="label">{{.W.SubTopics}}</span></div>
<div class="grid">{{range .Collections}}<a class="entry" href="{{.URL}}"><b>{{.Name}}</b>{{if .Description}}<span>{{.Description}}</span>{{end}}<span class="label">{{.Count}}</span></a>{{end}}</div>
</div></section>{{end}}
<section class="band"><div class="col">{{template "x"}}<div class="head"><span class="label">{{.W.ArticlesIn}}</span></div><div class="rows">{{template "items" .Items}}</div></div></section>
{{else if eq .View "search"}}
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide">{{template "crumbs" .}}<h1>{{.H1}}</h1>{{template "searchform" .}}{{if .Partial}}<p class="soft">{{.W.Partial}}</p>{{end}}</div>
</div></section>
<section class="band"><div class="col">{{template "x"}}
{{if .Items}}<div class="rows">{{template "items" .Items}}</div>{{else}}<div class="quiet">{{if .Query}}{{.W.NoResults}}{{else}}{{.W.SearchHint}}{{end}}</div>{{end}}
</div></section>
{{else if eq .View "article"}}
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide">{{template "crumbs" .}}<h1>{{.H1}}</h1></div>
</div></section>
<section class="band"><div class="col">{{template "x"}}
<div class="article{{if not .TOC}} solo{{end}}">
<article><div class="prose">{{.Body}}</div>
{{if .Updated.ISO}}<p class="meta">{{.W.UpdatedOn}}: <time datetime="{{.Updated.ISO}}" data-local>{{.Updated.Text}}</time></p>{{end}}
{{with .Helpful}}<div class="helpful" id="helpful">
{{if .State}}<span>{{$.W.Thanks}}</span>
<form method="post" action="{{.Action}}" data-helpful data-thanks="{{$.W.Thanks}}"><input type="hidden" name="previous" value="{{.State}}">{{if eq .State "up"}}<button class="btn alt" type="submit" name="rating" value="down">{{$.W.Change}}: {{$.W.No}}</button>{{else}}<button class="btn alt" type="submit" name="rating" value="up">{{$.W.Change}}: {{$.W.Yes}}</button>{{end}}</form>
{{else}}<span>{{$.W.WasHelpful}}</span>
<form method="post" action="{{.Action}}" data-helpful data-thanks="{{$.W.Thanks}}"><button class="btn" type="submit" name="rating" value="up">{{$.W.Yes}}</button><button class="btn alt" type="submit" name="rating" value="down">{{$.W.No}}</button></form>{{end}}
</div>{{end}}
</article>
{{if .TOC}}<nav class="toc" aria-label="{{.W.Contents}}"><span class="label">{{.W.Contents}}</span>{{range .TOC}}<a href="#{{.ID}}"{{if .Sub}} class="sub"{{end}}>{{.Text}}</a>{{end}}</nav>{{end}}
</div></div></section>
{{if .Related}}<section class="band"><div class="col">{{template "x"}}<div class="head"><span class="label">{{.W.Related}}</span></div><div class="rows">{{template "items" .Related}}</div></div></section>{{end}}
{{end}}
{{define "crumbs"}}{{if .Crumbs}}<ol class="crumbs">{{range .Crumbs}}<li>{{if .URL}}<a href="{{.URL}}">{{.Name}}</a>{{else}}<span>{{.Name}}</span>{{end}}</li>{{end}}</ol>{{end}}{{end}}
</main>
{{with .Chat}}<section class="band"><div class="col chat">{{template "x"}}<strong>{{$.W.StillNeed}}</strong><span class="soft">{{$.W.StillNeedText}}</span></div></section>
<script defer src="{{.Script}}"></script><yuva-chat channel="{{.Key}}" server="{{.Server}}"{{if .Article}} help-article="{{.Article}}"{{end}}></yuva-chat>{{end}}
<footer><div class="col foot"><span>{{if gt (len .Langs) 1}}<nav class="langs" aria-label="{{.SW.Language}}">{{range .Langs}}<a href="{{.Href}}" hreflang="{{.Code}}" lang="{{.Code}}"{{if .On}} aria-current="true"{{end}}>{{.Label}}</a>{{end}}</nav>{{end}}</span>
<span class="links"><a class="powered" href="{{.YuvaSite}}" rel="noopener">{{.Mark}}{{.SW.Powered}}</a></span></div></footer>
<script>{{.Script}}</script>
</body></html>`))
