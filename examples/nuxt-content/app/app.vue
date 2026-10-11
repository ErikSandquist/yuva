<script setup lang="ts">
const { data: navigation } = await useAsyncData("navigation", () => queryCollectionNavigation("content"))

useHead({
  htmlAttrs: { lang: "en" },
  link: [{ rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
  meta: [{ name: "theme-color", content: "#2563eb" }],
})
</script>

<template>
  <div class="site">
    <header class="top">
      <NuxtLink to="/" class="brand">
        <span class="logo" aria-hidden="true">A</span>
        Acme Docs
      </NuxtLink>
    </header>
    <nav class="pages" aria-label="Documentation">
      <NuxtLink v-for="item in navigation" :key="item.path" :to="item.path">{{ item.title }}</NuxtLink>
    </nav>
    <main>
      <NuxtPage />
    </main>
  </div>
</template>

<style>
:root {
  color-scheme: light dark;
  --bg: #ffffff;
  --fg: #1f2328;
  --muted: #59636e;
  --line: #d1d9e0;
  --soft: #f6f8fa;
  --accent: #2563eb;
  font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
  line-height: 1.6;
  -webkit-text-size-adjust: 100%;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0d1117;
    --fg: #e6edf3;
    --muted: #9198a1;
    --line: #30363d;
    --soft: #161b22;
    --accent: #4493f8;
  }
}
*,
*::before,
*::after {
  box-sizing: border-box;
}
body {
  margin: 0;
  background: var(--bg);
  color: var(--fg);
}
a {
  color: var(--accent);
}
.site {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  grid-template-areas: "top" "pages" "main";
  max-width: 72rem;
  margin: 0 auto;
}
.top {
  grid-area: top;
  display: flex;
  align-items: center;
  padding: 0.75rem 1rem;
  border-block-end: 1px solid var(--line);
}
.brand {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: inherit;
  font-weight: 700;
  text-decoration: none;
}
.logo {
  display: inline-grid;
  place-items: center;
  inline-size: 1.75rem;
  block-size: 1.75rem;
  border-radius: 0.5rem;
  background: var(--accent);
  color: #fff;
  font-size: 0.9rem;
}
.pages {
  grid-area: pages;
  display: flex;
  gap: 0.25rem;
  padding: 0.5rem 1rem;
  overflow-x: auto;
  border-block-end: 1px solid var(--line);
  scrollbar-width: none;
}
.pages a {
  flex-shrink: 0;
  padding: 0.35rem 0.75rem;
  border-radius: 999px;
  color: var(--muted);
  text-decoration: none;
  white-space: nowrap;
}
.pages a:hover {
  color: var(--fg);
  background: var(--soft);
}
.pages a.router-link-exact-active {
  color: var(--fg);
  background: var(--soft);
  font-weight: 600;
}
main {
  grid-area: main;
  min-inline-size: 0;
  padding: 1.5rem 1rem 3rem;
}
article {
  max-inline-size: 46rem;
  overflow-wrap: break-word;
}
h1 {
  margin-block: 0 1rem;
  font-size: 1.9rem;
  line-height: 1.25;
}
h2 {
  margin-block: 2rem 0.75rem;
  font-size: 1.35rem;
  line-height: 1.3;
}
h2 a,
h3 a {
  color: inherit;
  text-decoration: none;
}
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.875em;
}
:not(pre) > code {
  padding: 0.1em 0.35em;
  border-radius: 0.35rem;
  background: var(--soft);
  overflow-wrap: anywhere;
}
pre {
  overflow-x: auto;
  padding: 1rem;
  border: 1px solid var(--line);
  border-radius: 0.5rem;
  background: var(--soft);
}
table {
  display: block;
  max-inline-size: 100%;
  overflow-x: auto;
  border-collapse: collapse;
}
th,
td {
  padding: 0.4rem 0.75rem;
  border: 1px solid var(--line);
  text-align: start;
  vertical-align: top;
}
td {
  min-inline-size: 8rem;
}
th {
  background: var(--soft);
}
yuva-page-feedback,
yuva-discussions {
  margin-block: 2.5rem;
}
yuva-page-feedback {
  padding-block-start: 1.5rem;
  border-block-start: 1px solid var(--line);
}
@media (min-width: 768px) {
  .site {
    grid-template-columns: 13rem minmax(0, 1fr);
    grid-template-areas: "top top" "pages main";
  }
  .top {
    padding-inline: 1.5rem;
  }
  .pages {
    position: sticky;
    top: 0;
    align-self: start;
    flex-direction: column;
    padding: 1.5rem 1rem;
    border-block-end: 0;
    overflow: visible;
  }
  .pages a {
    border-radius: 0.5rem;
  }
  main {
    padding: 2rem 1.5rem 4rem;
  }
}
</style>
