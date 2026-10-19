package api

import (
	"html/template"
	"strings"
)

var homeFavicon = []byte(strings.Replace(strings.Replace(string(yuvaMark), `<svg class="mark" `, `<svg xmlns="http://www.w3.org/2000/svg" `, 1), ` width="16" height="16" aria-hidden="true"`, ` width="64" height="64"`, 1))

var homeGlobe = template.HTML(`<svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.4" aria-hidden="true"><circle cx="8" cy="8" r="6.3"/><path d="M1.7 8h12.6M8 1.7c1.8 1.8 2.6 3.9 2.6 6.3S9.8 12.5 8 14.3M8 1.7C6.2 3.5 5.4 5.6 5.4 8s.8 4.5 2.6 6.3"/></svg>`)

const homeStyle = `
:root{--code:#0b0b0f}
@media (prefers-color-scheme:dark){:root{--deep:#121216;--code:#18181b}}
h3{margin:0}
.skip:focus{position:static;width:auto;height:auto;clip:auto;padding:8px 14px;display:inline-block}
.brandmark .mark,.brandmark img{width:24px;height:24px;max-width:none}
.nav-h{display:flex;flex-wrap:wrap;align-items:center;gap:8px 24px;font-size:14px;color:var(--soft)}
.nav-h .ink{color:var(--ink)}
.hbtn{display:inline-block;background:var(--brand);color:#fff;padding:12px 18px;font-weight:500;text-decoration:none}
.hbtn.out{background:none;color:var(--ink);border:1px solid var(--strong);padding:11px 18px}
.hbtn.sm{background:var(--ink);color:var(--paper);padding:8px 14px;font-size:14px}
a.hbtn:hover{text-decoration:none;opacity:.9}
.lp{position:relative}
.lp summary{list-style:none;cursor:pointer;display:inline-flex;align-items:center;gap:6px;min-height:36px;padding:6px 10px;border:1px solid var(--strong);color:var(--soft)}
.lp summary::-webkit-details-marker{display:none}
.lp ul{position:absolute;inset-inline-end:0;top:calc(100% + 4px);z-index:5;margin:0;padding:6px;list-style:none;background:var(--paper);border:1px solid var(--strong);max-height:60vh;overflow:auto;min-width:210px}
.lp li a{display:block;padding:6px 10px;white-space:nowrap}
.lp li a[aria-current]{font-weight:600;color:var(--brand)}
.h-hero{display:grid;grid-template-columns:minmax(0,1.15fr) minmax(0,1fr);gap:1px;background:var(--rule)}
.h-hero.solo{grid-template-columns:minmax(0,1fr)}
.h-hero>*{background:var(--paper);min-width:0}
.h-hero .hero{padding:72px 40px 64px;gap:24px}
.h-hero .hero .lead{font-size:18px;max-width:440px}
.h-hero.solo .hero .lead{max-width:560px}
.h-hero .hero h1{max-width:none}
.ctas{display:flex;flex-wrap:wrap;gap:12px;align-items:center}
.h-aside{background:var(--deep)!important;padding:40px;display:flex;flex-direction:column;justify-content:center}
.hcard{background:var(--paper);border:1px solid var(--rule)}
.hcard>*{border-bottom:1px solid var(--rule)}
.hcard>:last-child{border-bottom:0}
.hc-top{display:flex;justify-content:space-between;align-items:center;gap:8px;padding:12px 16px;font-size:11px;letter-spacing:.6px;color:var(--cross)}
.hc-top a{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.hc-live{display:inline-flex;align-items:center;gap:6px;color:var(--ok-deep);text-transform:uppercase}
.hc-state{padding:20px 16px 16px;display:flex;flex-direction:column;gap:6px}
.hc-state strong{font-size:22px;font-weight:600;letter-spacing:-.6px}
.hc-row{padding:10px 16px;display:grid;grid-template-columns:96px minmax(0,1fr) 52px;gap:12px;align-items:center;font-size:13px}
.hc-row.nobars{grid-template-columns:minmax(0,1fr) auto}
.hc-row b{font-weight:500;overflow-wrap:anywhere}
.hc-row .mono{font-size:11px;color:var(--cross);text-align:end}
.hc-row .strip{height:12px}
.hc-foot{display:flex;justify-content:space-between;padding:10px 16px;font-size:11px;color:var(--cross)}
.fact{padding:24px 24px 22px;display:flex;flex-direction:column;gap:4px;min-width:0}
.fact b{font-size:26px;font-weight:600;letter-spacing:-.8px;overflow-wrap:anywhere}
.fact span{font-size:13px;color:var(--soft)}
.sechead{padding:20px 40px;border-bottom:1px solid var(--rule);font-size:12px;font-weight:400;margin:0}
.feat{padding:28px 28px 32px;display:flex;flex-direction:column;gap:10px}
.feat h3{font-size:17px;font-weight:600;letter-spacing:-.3px}
.feat p,.pad p{font-size:14px;color:var(--soft)}
.feat .n{font-size:11px;color:var(--brand)}
.tl{padding:32px 40px;display:flex;flex-direction:column}
.step{display:grid;grid-template-columns:72px minmax(0,1fr);gap:16px;padding:14px 0;border-top:1px solid var(--rule)}
.step:first-child{border-top:0}
.step .mono{font-size:12px;color:var(--cross)}
.step b{font-weight:600;display:inline-flex;align-items:center;gap:8px}
.step small{display:block;font-size:14px;color:var(--soft)}
.illus{background:var(--deep)!important;padding:32px 40px;display:flex;flex-direction:column;gap:16px}
.illus .row{display:flex;flex-wrap:wrap;justify-content:space-between;gap:4px 12px;font-size:14px}
.chart .mk{stroke:var(--strong);stroke-width:.5;stroke-dasharray:1 1;fill:none}
.chart .base{stroke:var(--rule);stroke-width:.5;fill:none}
.reach{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(120px,100%),1fr));gap:1px;background:var(--rule);border:1px solid var(--rule);margin-top:8px}
.reach span{background:var(--paper);padding:14px;display:flex;flex-direction:column;gap:2px;min-width:0}
.reach b{font-size:20px;font-weight:600;letter-spacing:-.5px}
.reach small{font-size:12px;color:var(--soft)}
.bandbox{padding:40px;display:flex;flex-direction:column;gap:20px}
.bandbox h2{font-size:32px;font-weight:600;letter-spacing:-1px;max-width:560px}
.site{border:1px solid var(--rule)}
.site-top{padding:18px 24px;display:flex;flex-wrap:wrap;justify-content:space-between;gap:8px 20px;font-size:14px;color:var(--soft)}
.site-top b{color:var(--ink);font-weight:700}
.site-top span{display:flex;gap:20px}
.fbar{margin:0 24px 24px;border:1px solid var(--rule);display:grid;grid-template-columns:minmax(0,1fr) 300px auto;gap:1px;background:var(--rule)}
.fbar>*{background:var(--paper);display:flex;align-items:center;gap:10px;min-width:0}
.fbar .msg{height:48px;padding:0 18px;font-size:14px;white-space:nowrap;overflow:hidden}
.fbar .msg span:last-child{color:var(--soft);overflow:hidden;text-overflow:ellipsis}
.fbar .up{padding:0 18px;gap:12px}
.fbar .up .strip{flex:1;height:10px;min-width:0}
.fbar .up small{font-size:10px;color:var(--cross)}
.fbar .pw{padding:0 18px;font-size:12px;color:var(--cross);white-space:nowrap}
.fbar .pw img{width:14px;height:14px}
.snippet{display:block;font-size:13px;background:var(--code);color:#fafafa;border:0;border-radius:0;padding:16px 20px;white-space:pre;overflow-x:auto;word-break:normal}
.two{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(380px,100%),1fr));gap:1px;background:var(--rule)}
.two>*{background:var(--paper);min-width:0}
.two .copy{padding:40px;display:flex;flex-direction:column;gap:14px}
.two .copy h2{font-size:28px;font-weight:600;letter-spacing:-.8px}
.two .copy p{font-size:15px;color:var(--soft)}
.convs{background:var(--deep)!important;padding:32px 40px;display:flex;flex-direction:column}
.conv{background:var(--paper);border:1px solid var(--rule);margin-top:-1px;padding:10px 14px;display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px;font-size:13px}
.conv .mono{font-size:11px;color:var(--cross);text-transform:uppercase}
.conv .soft{color:var(--soft)}
.reply{margin-top:12px;align-self:flex-start;background:var(--ink);color:var(--paper);padding:8px 14px;font-size:13px;font-weight:500}
.faq details{border-bottom:1px solid var(--rule)}
.faq summary{list-style:none;cursor:pointer;padding:18px 40px;display:flex;justify-content:space-between;gap:16px;font-weight:500}
.faq summary::-webkit-details-marker{display:none}
.faq summary::after{content:"+";color:var(--cross);font-family:"Geist Mono",ui-monospace,monospace}
.faq details[open] summary::after{content:"−"}
.faq p{padding:0 40px 20px;font-size:15px;color:var(--soft);max-width:720px}
.cta{padding:64px 40px;display:flex;flex-wrap:wrap;justify-content:space-between;align-items:center;gap:24px;background:var(--deep)}
.cta h2{font-size:34px;font-weight:600;letter-spacing:-1.2px;max-width:520px}
.foot .mark-l{display:inline-flex;align-items:center;gap:8px}
@media (max-width:860px){.h-hero{grid-template-columns:minmax(0,1fr)}.fbar{grid-template-columns:minmax(0,1fr)}.fbar .msg{height:auto;min-height:48px;padding:10px 18px;white-space:normal}.site-top{padding-inline:16px}.fbar{margin-inline:16px}}
@media (max-width:700px){.h-hero .hero,.h-aside,.tl,.illus,.bandbox,.two .copy,.convs,.cta,.sechead{padding-inline:16px}.h-hero .hero{padding-block:48px 40px}.faq summary{padding-inline:16px}.faq p{padding-inline:16px}.hc-row{grid-template-columns:72px minmax(0,1fr) 48px}.cta h2,.bandbox h2{font-size:26px}.feat{padding-inline:16px}.fact{padding-inline:16px}}
`

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="{{.Lang}}" dir="{{.Dir}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.W.Title}}</title>
<meta name="description" content="{{.W.Description}}">
<link rel="canonical" href="{{.Canonical}}">
{{range .Alternates}}<link rel="alternate" hreflang="{{.Hreflang}}" href="{{.Href}}">
{{end}}<meta name="theme-color" content="#ffffff" media="(prefers-color-scheme: light)"><meta name="theme-color" content="#09090b" media="(prefers-color-scheme: dark)">
<link rel="icon" href="{{.Favicon}}" type="image/svg+xml">
<meta property="og:type" content="website"><meta property="og:site_name" content="{{.W.SiteName}}"><meta property="og:title" content="{{.W.Title}}"><meta property="og:description" content="{{.W.Description}}"><meta property="og:url" content="{{.Canonical}}">
<meta property="og:locale" content="{{.OGLocale}}">{{range .OGAlternates}}<meta property="og:locale:alternate" content="{{.}}">{{end}}
<meta property="og:image" content="{{.OGImage}}"><meta property="og:image:type" content="image/png"><meta property="og:image:width" content="1200"><meta property="og:image:height" content="630"><meta property="og:image:alt" content="{{.W.SiteName}}: {{.W.H1}}">
<meta name="twitter:card" content="summary_large_image"><meta name="twitter:title" content="{{.W.Title}}"><meta name="twitter:description" content="{{.W.Description}}"><meta name="twitter:image" content="{{.OGImage}}"><meta name="twitter:image:alt" content="{{.W.SiteName}}: {{.W.H1}}">
<link rel="preload" href="{{.FontSans}}" as="font" type="font/woff2" crossorigin>
<style>
@font-face{font-family:Geist;src:url("{{.FontSans}}") format("woff2");font-weight:100 900;font-display:swap}
@font-face{font-family:"Geist Mono";src:url("{{.FontMono}}") format("woff2");font-weight:100 900;font-display:swap}
{{.Style}}
</style>
{{.LD}}</head><body>
{{define "x"}}<svg class="x s" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg><svg class="x e" viewBox="0 0 11 11" aria-hidden="true"><path d="M5.5 0v11M0 5.5h11" stroke="currentColor"/></svg>{{end}}
<a class="sr skip" href="#main">{{.W.SkipToContent}}</a>
<header class="band"><div class="col bar">{{template "x"}}
<a class="brandmark" href="{{.HomePath}}"><img src="{{.Favicon}}" alt="" width="24" height="24"><span class="name">Yuva</span><span class="label">{{.W.Status}}</span></a>
<nav class="nav-h" aria-label="{{.W.NavLabel}}"><a href="#features">{{.W.NavFeatures}}</a><a href="#footer-band">{{.W.NavEmbed}}</a><a href="#faq">{{.W.NavFAQ}}</a><a href="{{.DocsURL}}">{{.W.NavDocs}}</a>
<a class="ink" href="{{.SignInURL}}">{{.W.SignIn}}</a><a class="hbtn sm" href="{{.CreateURL}}">{{.W.Create}}</a>
<details class="lp"><summary lang="{{.Lang}}">{{.Globe}}<span class="sr">{{.W.Language}}: </span>{{.LocaleName}}</summary><ul>{{range .Locales}}<li><a href="{{.Href}}" hreflang="{{.Code}}" lang="{{.Code}}"{{if .On}} aria-current="true"{{end}}>{{.Name}}</a></li>{{end}}</ul></details>
</nav></div></header>
<main id="main">
<section class="band" aria-labelledby="hero-h"><div class="col">{{template "x"}}<div class="h-hero{{if not .Card}} solo{{end}}">
<div class="hero"><span class="kick accent">{{.W.Kicker}}</span><h1 id="hero-h">{{.W.H1}}</h1><p class="lead">{{.W.Lead}}</p>
<div class="ctas"><a class="hbtn" href="{{.CreateURL}}">{{.W.CtaCreate}}</a>{{with .Card}}<a class="hbtn out" href="{{.URL}}">{{$.W.CtaLive}} <span class="fl">→</span></a>{{end}}</div>
<span class="sm soft">{{.W.Note}}</span></div>
{{with .Card}}<div class="h-aside"><div class="hcard">
<div class="hc-top mono"><a href="{{.URL}}">{{.Host}}</a><span class="hc-live"><i class="q q-operational"></i>{{$.W.CardLive}}</span></div>
<div class="hc-state"><span class="kick k-{{.Class}}"><span><i class="q q-{{.Class}}"></i>{{.Kicker}}</span></span><strong>{{.Title}}</strong></div>
{{range .Rows}}<div class="hc-row{{if not .Bars}} nobars{{end}}"><b>{{.Name}}</b>{{if .Bars}}{{.Bars}}<span class="mono">{{.Uptime}}</span>{{else}}<span class="st k-{{.Class}}"><i class="q q-{{.Class}}"></i>{{.State}}</span>{{end}}</div>{{end}}
{{if .HasBars}}<div class="hc-foot mono"><span>{{.Oldest}}</span><span>{{$.W.CardToday}}</span></div>{{end}}
</div></div>{{end}}
</div></div></section>
<div class="band"><div class="col">{{template "x"}}<ul class="grid g4" style="list-style:none;margin:0;padding:0">{{range .Facts}}<li class="fact"><b>{{.N}}</b><span>{{.Label}}</span></li>{{end}}</ul></div></div>
<section class="band" id="features" aria-labelledby="features-h"><div class="col">{{template "x"}}
<h2 class="sechead label" id="features-h">01 — {{.W.FeaturesHead}}</h2>
<div class="grid">{{range .Features}}<div class="feat"><span class="mono n">{{.N}}</span><h3>{{.Title}}</h3><p>{{.Text}}</p></div>{{end}}</div></div></section>
<section class="band" aria-labelledby="breaks-h"><div class="col">{{template "x"}}
<h2 class="sechead label" id="breaks-h">02 — {{.W.BreaksHead}}</h2>
<div class="grid" style="grid-template-columns:repeat(auto-fit,minmax(min(380px,100%),1fr))">
<div class="tl">{{range .Steps}}<div class="step"><span class="mono">{{.Time}}</span><span><b><i class="q q-{{.Class}}"></i>{{.Title}}</b><small>{{.Text}}</small></span></div>{{end}}</div>
<div class="illus"><span class="row"><b>{{.W.ResponseTitle}}</b><span class="soft">{{.W.ResponsePeak}} · {{.W.SampleData}}</span></span>
<svg class="chart" viewBox="0 0 192 48" shape-rendering="crispEdges" role="img" aria-label="{{.W.ResponseAlt}}"><path class="base" d="M0 44H192"/><path class="mk" d="M60 0V48M132 0V48"/><path class="area" d="{{.ChartArea}}"/><path class="line" d="{{.ChartLine}}"/></svg>
<div class="scale mono"><span>19:35</span><span>20:05 {{.W.Opened}}</span><span>20:52 {{.W.Resolved}}</span><span>21:20</span></div>
<div class="reach">{{range .Reach}}<span><b>{{.N}}</b><small>{{.Label}}</small></span>{{end}}</div></div>
</div></div></section>
<section class="band" id="footer-band" aria-labelledby="band-h"><div class="col">{{template "x"}}
<div class="head"><span class="label">03 — {{.W.BandHead}}</span></div>
<div class="bandbox"><h2 id="band-h">{{.W.BandH2}}</h2>
<div class="site" role="img" aria-label="{{.W.BandAlt}}"><div class="site-top"><b>Northwind</b><span><span>{{.W.BandNavA}}</span><span>{{.W.BandNavB}}</span><span>{{.W.BandNavC}}</span><span>© 2026</span></span></div>
<div class="fbar"><span class="msg"><i class="q q-operational"></i><b>{{.W.BandOperational}}</b><span>{{.W.BandSub}}</span></span><span class="up"><small class="mono">90 D</small>{{.Band}}<b class="sm">99.98%</b></span><span class="pw"><img src="{{.Favicon}}" alt="" width="14" height="14">{{.W.BandPowered}}</span></div></div>
<code class="snippet" tabindex="0" dir="ltr" aria-label="{{.W.SnippetLabel}}">{{.Snippet}}</code></div></div></section>
<section class="band" aria-labelledby="conv-h"><div class="col two">{{template "x"}}
<div class="copy"><span class="label">04 — {{.W.ConvHead}}</span><h2 id="conv-h">{{.W.ConvH2}}</h2><p>{{.W.ConvText}}</p></div>
<div class="convs">{{range .Convs}}<span class="conv"><span><b>{{.Who}}</b> <span class="soft">{{.Text}}</span></span><span class="mono">{{.Via}}</span></span>{{end}}
<span class="conv"><span><b>{{.W.ConvMore}}</b> <span class="soft">{{.W.ConvMoreText}}</span></span><span></span></span>
<span class="reply">{{.W.ConvReply}}</span></div></div></section>
<section class="band faq" id="faq" aria-labelledby="faq-h"><div class="col">{{template "x"}}
<h2 class="sechead label" id="faq-h">05 — {{.W.FAQHead}}</h2>
{{range $i, $q := .W.FAQ}}<details{{if eq $i 0}} open{{end}}><summary>{{$q.Q}}</summary><p>{{$q.A}}</p></details>{{end}}</div></section>
<section class="band" aria-labelledby="cta-h"><div class="col cta">{{template "x"}}<h2 id="cta-h">{{.W.CtaH2}}</h2>
<div class="ctas"><a class="hbtn" href="{{.CreateURL}}">{{.W.CtaCreate}}</a><a class="hbtn out" href="{{.GuideURL}}">{{.W.CtaGuide}}</a></div></div></section>
</main>
<footer><div class="col foot"><span class="mark-l"><img src="{{.Favicon}}" alt="" width="16" height="16">{{.W.FooterTag}}</span>
<nav class="links" aria-label="Yuva"><a href="{{.SiteURL}}">{{.W.LinkSite}}</a><a href="{{.GuideURL}}">{{.W.LinkPages}}</a><a href="{{.DocsURL}}">{{.W.NavDocs}}</a><a href="{{.GitHubURL}}" rel="noopener">{{.W.LinkGitHub}}</a></nav></div></footer>
</body></html>`))
