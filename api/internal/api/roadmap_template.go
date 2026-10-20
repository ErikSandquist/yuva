package api

import "html/template"

const roadmapStyle = `
.skip{position:absolute;inset-inline-start:8px;top:-60px;z-index:9;background:var(--ink);color:var(--paper);padding:8px 14px;border-radius:8px}
.skip:focus{top:8px}
ol.crumbs,ul.plain{margin:0;padding:0}
.crumbs{display:flex;flex-wrap:wrap;gap:4px 8px;font-size:13px;color:var(--soft)}
.crumbs li{list-style:none;display:inline-flex;gap:8px}
.crumbs li+li::before{content:"/";color:var(--cross)}
.langs{display:flex;flex-wrap:wrap;gap:4px 14px;font-size:13px}
.langs a[aria-current]{font-weight:600;color:var(--ink)}
.cols{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(240px,100%),1fr));gap:1px;background:var(--rule)}
.cols>section{background:var(--paper);min-width:0;display:flex;flex-direction:column}
.cols h2{margin:0;padding:14px 20px;border-bottom:1px solid var(--rule);display:flex;justify-content:space-between;gap:8px;font-size:13px;font-weight:600}
.cols h2 a{font-weight:400;font-size:12px}
.card{display:flex;flex-direction:column;gap:6px;padding:16px 20px;border-bottom:1px solid var(--rule);min-width:0}
.card b{font-weight:600;font-size:15px;overflow-wrap:anywhere}
.card p{margin:0;color:var(--soft);font-size:13px;overflow-wrap:anywhere}
a.card:hover{text-decoration:none;background:var(--deep)}
a.card:hover b{text-decoration:underline;text-underline-offset:3px}
.facts{display:flex;flex-wrap:wrap;gap:4px 12px;font-size:12px;color:var(--soft)}
.stagebadge{display:inline-block;border:1px solid var(--strong);border-radius:999px;padding:2px 10px;font-size:12px}
.prose{padding:32px 28px;overflow-wrap:anywhere}
.prose>*+*{margin-top:14px}
.prose p,.prose li{font-size:16px;line-height:1.65}
.prose ul,.prose ol{padding-inline-start:24px}
.prose a{color:var(--brand);text-decoration:underline;text-underline-offset:3px}
.prose pre{margin:0;padding:14px 16px;overflow:auto;background:var(--deep);border:1px solid var(--rule);border-radius:8px}
.prose code{font-size:.9em}
.prose blockquote{margin-inline:0;padding:4px 16px;border-inline-start:3px solid var(--strong);color:var(--soft)}
.reply{padding:16px 28px;border-top:1px solid var(--rule);background:var(--paper)}
.reply header{display:flex;gap:10px;align-items:center;font-size:13px;color:var(--soft)}
.reply .prose{padding:8px 0 0}
.avatar{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;border-radius:50%;background:var(--deep);border:1px solid var(--rule);font-size:11px;font-weight:600;color:var(--ink)}
.hist{padding:16px 28px;display:flex;flex-direction:column;gap:6px;font-size:14px;background:var(--paper)}
.act{display:flex;flex-wrap:wrap;gap:8px;margin-top:6px}
.act .btn.alt{background:var(--paper);color:var(--ink);border:1px solid var(--strong)}
@media (max-width:700px){.card,.cols h2,.reply,.hist,.prose{padding-inline:16px}}
`

// The Vote and Suggest buttons are hidden until the script shows them, so the page reads complete without JavaScript.
var roadmapScript = `var L=document.documentElement.lang;` +
	`document.querySelectorAll("time[data-local]").forEach(function(t){var d=new Date(t.getAttribute("datetime"));if(!isNaN(d))t.textContent=d.toLocaleDateString(L,{dateStyle:"medium"})});` +
	`var c=document.querySelector("yuva-chat");if(c)document.querySelectorAll("button[data-roadmap]").forEach(function(b){b.hidden=false;b.addEventListener("click",function(){` +
	`if(typeof c.openApp==="function")c.openApp("roadmap",b.dataset.idea?{idea:b.dataset.idea}:{})})})`

var roadmapScriptHash = publicScriptHash(roadmapScript)

var roadmapTemplate = template.Must(template.New("roadmap").Parse(`<!doctype html>
<html lang="{{.Lang}}" dir="{{.Dir}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
{{if .NoIndex}}<meta name="robots" content="noindex">
{{end}}<meta property="og:site_name" content="{{.Name}}"><meta property="og:type" content="{{.OGType}}"><meta property="og:title" content="{{.OGTitle}}"><meta property="og:description" content="{{.Description}}"><meta property="og:url" content="{{.Canonical}}"><meta property="og:locale" content="{{.OGLocale}}">
<meta name="twitter:card" content="summary">
{{.JSONLD}}
<link rel="preload" href="{{.FontSans}}" as="font" type="font/woff2" crossorigin>
<style>
@font-face{font-family:Geist;src:url("{{.FontSans}}") format("woff2");font-weight:100 900;font-display:swap}
@font-face{font-family:"Geist Mono";src:url("{{.FontMono}}") format("woff2");font-weight:100 900;font-display:swap}
` + statusStyle + roadmapStyle + `{{if .Accent}}:root{--brand:{{.Accent}}}{{end}}
</style></head><body{{if ne .View "home"}} class="sub"{{end}}>
{{define "x"}}<svg class="x s" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg><svg class="x e" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg>{{end}}
{{define "crumbs"}}{{if .Crumbs}}<ol class="crumbs">{{range .Crumbs}}<li>{{if .URL}}<a href="{{.URL}}">{{.Name}}</a>{{else}}<span>{{.Name}}</span>{{end}}</li>{{end}}</ol>{{end}}{{end}}
{{define "card"}}<a class="card" href="{{.URL}}"><b>{{.Title}}</b>{{if .Excerpt}}<p>{{.Excerpt}}</p>{{end}}<span class="facts"><span>{{.Votes}}</span><span>{{.Comments}}</span>{{if .Date.ISO}}<span>{{.DateLabel}} <time datetime="{{.Date.ISO}}" data-local>{{.Date.Text}}</time></span>{{end}}</span></a>{{end}}
<a class="skip" href="#main">{{.W.Skip}}</a>
<header class="band"><div class="col bar">{{template "x"}}
<a class="brandmark" href="{{.Home}}">{{if .Logo}}<img src="{{.Logo}}" alt="">{{end}}<span class="name">{{.Name}}</span><span class="label">{{.W.Roadmap}}</span></a>
<nav class="top"></nav></div></header>
<main id="main">
{{if eq .View "home"}}
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide"><span class="label">{{.W.Roadmap}}</span><h1>{{.H1}}</h1>{{if .Lead}}<p class="lead">{{.Lead}}</p>{{end}}
{{if .Chat}}{{if .Suggest}}<div class="act"><button class="btn" type="button" data-roadmap hidden>{{.W.Suggest}}</button></div>{{end}}{{end}}</div>
</div></section>
<section class="band"><div class="col">{{template "x"}}<div class="cols">
{{range .Columns}}<section><h2><span>{{.Name}}</span>{{if .More}}<a href="{{.More}}">{{.MoreLabel}}</a>{{end}}</h2>
{{range .Cards}}{{template "card" .}}{{else}}<div class="quiet">{{$.W.Empty}}</div>{{end}}</section>{{end}}
</div></div></section>
{{else if eq .View "stage"}}
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide">{{template "crumbs" .}}<h1>{{.H1}}</h1></div>
</div></section>
<section class="band"><div class="col">{{template "x"}}<div class="cols">{{range .Columns}}<section>{{range .Cards}}{{template "card" .}}{{else}}<div class="quiet">{{$.W.Empty}}</div>{{end}}</section>{{end}}</div></div></section>
{{else}}
<section class="band"><div class="col grid">{{template "x"}}
<div class="hero wide">{{template "crumbs" .}}<span class="stagebadge">{{.Idea.StageName}}</span><h1>{{.H1}}</h1>
<span class="facts"><span>{{.Idea.Votes}}</span><span>{{.Idea.Comments}}</span></span>
{{if .Chat}}<div class="act"><button class="btn" type="button" data-roadmap data-idea="{{.Idea.ID}}" hidden>{{.W.Vote}}</button></div>{{end}}</div>
</div></section>
<section class="band"><div class="col">{{template "x"}}
<article><div class="prose">{{.Idea.Body}}</div></article>
{{if .Idea.History}}<div class="hist"><span class="label">{{.W.History}}</span>{{range .Idea.History}}<span>{{.Label}} <time datetime="{{.At.ISO}}" data-local>{{.At.Text}}</time></span>{{end}}</div>{{end}}
</div></section>
<section class="band"><div class="col">{{template "x"}}<div class="head"><span class="label">{{.W.Comments}}</span></div>
{{range .Idea.Replies}}<div class="reply"><header><span class="avatar" aria-hidden="true">{{.Initials}}</span><b>{{.Name}}</b><time datetime="{{.At.ISO}}" data-local>{{.At.Text}}</time></header><div class="prose">{{.Body}}</div></div>{{else}}<div class="quiet">{{.W.NoComments}}</div>{{end}}
</div></section>
{{end}}
</main>
{{with .Chat}}<script defer src="{{.Script}}"></script><yuva-chat channel="{{.Key}}" server="{{.Server}}"></yuva-chat>{{end}}
<footer><div class="col foot"><span>{{if gt (len .Langs) 1}}<nav class="langs" aria-label="{{.SW.Language}}">{{range .Langs}}<a href="{{.Href}}" hreflang="{{.Code}}" lang="{{.Code}}"{{if .On}} aria-current="true"{{end}}>{{.Label}}</a>{{end}}</nav>{{end}}</span>
<span class="links"><a class="powered" href="{{.YuvaSite}}" rel="noopener">{{.Mark}}{{.SW.Powered}}</a></span></div></footer>
<script>{{.Script}}</script>
</body></html>`))
