import fs from "node:fs"
import path from "node:path"
import ts from "typescript"
import { localized, prefix, type Locale } from "./routes"

export const REGISTRY_URL = "https://useyuva.com/r/"
export const SHADCN_DOCS = "https://ui.shadcn.com/docs/components/"
const source = path.resolve("../sdk/shadcn/registry/yuva")

export const shadcnRoot = (locale: Locale) => `${prefix(locale)}/shadcn/`
export const shadcnHref = (locale: Locale, rel = "") => `${shadcnRoot(locale)}${rel}`
export const shadcnPaths = (rel = "") => localized(`/shadcn/${rel}`)

export type Group = "blocks" | "components" | "hooks"
export type Row = { name: string; type: string; optional: boolean }
export type LabelRow = { key: string; value: string }
export type Table = { id: string; title: string; rows: Row[] }

type Spec = {
  props?: { id: string; file: string; interface: string }[]
  labels?: { file: string; interface: string; defaults: string; only?: RegExp }
  labelsFrom?: string
  usage: string
  example: string
}

const chatEnv = `server={process.env.NEXT_PUBLIC_YUVA_SERVER}
      channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL}`
const chatLabels = {
  file: "lib/yuva-chat-utils.ts",
  interface: "YuvaChatLabels",
  defaults: "defaultYuvaChatLabels",
}
const discussionLabels = {
  file: "lib/yuva-utils.ts",
  interface: "YuvaDiscussionsLabels",
  defaults: "defaultYuvaDiscussionsLabels",
}
const helpLabels = {
  file: "lib/yuva-help-utils.tsx",
  interface: "YuvaHelpLabels",
  defaults: "defaultYuvaHelpLabels",
}
const roadmapLabels = {
  file: "lib/yuva-roadmap-utils.ts",
  interface: "YuvaRoadmapLabels",
  defaults: "defaultYuvaRoadmapLabels",
}
const provider = (inner: string) => `import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"

export function Docs({ children }: { children: React.ReactNode }) {
  return (
    <YuvaDiscussionsProvider
      server={process.env.NEXT_PUBLIC_YUVA_SERVER}
      channel={process.env.NEXT_PUBLIC_YUVA_CHANNEL}
    >
      ${inner}
    </YuvaDiscussionsProvider>
  )
}`

export const specs: Record<string, Spec> = {
  "yuva-chat": {
    props: [
      { id: "props", file: "components/yuva-chat.tsx", interface: "YuvaChatProps" },
      { id: "email", file: "components/yuva-chat-email.tsx", interface: "YuvaChatEmailProps" },
    ],
    labels: chatLabels,
    usage: `import { YuvaChat } from "@/components/yuva-chat"

<YuvaChat server={server} channel={channel} />`,
    example: `import { YuvaChat } from "@/components/yuva-chat"

export function Support() {
  return (
    <YuvaChat
      ${chatEnv}
      className="h-[34rem]"
    />
  )
}`,
  },
  "yuva-chat-thread": {
    props: [
      {
        id: "props",
        file: "components/yuva-chat-thread.tsx",
        interface: "YuvaChatThreadProps",
      },
    ],
    labelsFrom: "yuva-chat",
    usage: `import { YuvaChatThread } from "@/components/yuva-chat-thread"

<YuvaChatThread chat={chat} />`,
    example: `import { YuvaChatThread } from "@/components/yuva-chat-thread"
import { useYuvaChat } from "@/hooks/use-yuva-chat"

export function Thread({ id }: { id: string }) {
  const chat = useYuvaChat({
    ${chatEnv.replaceAll("\n      ", "\n    ")},
  })
  const { loading, open } = chat

  React.useEffect(() => {
    if (!loading) void open(id)
  }, [loading, open, id])

  return (
    <div className="flex h-96 flex-col rounded-xl border">
      <YuvaChatThread chat={chat} />
    </div>
  )
}`,
  },
  "yuva-chat-composer": {
    props: [
      {
        id: "props",
        file: "components/yuva-chat-composer.tsx",
        interface: "YuvaChatComposerProps",
      },
    ],
    labelsFrom: "yuva-chat",
    usage: `import { YuvaChatComposer } from "@/components/yuva-chat-composer"

<YuvaChatComposer chat={chat} />`,
    example: `import { YuvaChatComposer } from "@/components/yuva-chat-composer"
import { useYuvaChat } from "@/hooks/use-yuva-chat"

export function Composer() {
  const chat = useYuvaChat({
    ${chatEnv.replaceAll("\n      ", "\n    ")},
  })
  return <YuvaChatComposer chat={chat} />
}`,
  },
  "yuva-chat-conversations": {
    props: [
      {
        id: "props",
        file: "components/yuva-chat-conversations.tsx",
        interface: "YuvaChatConversationsProps",
      },
    ],
    labelsFrom: "yuva-chat",
    usage: `import { YuvaChatConversations } from "@/components/yuva-chat-conversations"

<YuvaChatConversations chat={chat} onOpen={(id) => setView("thread")} />`,
    example: `import { YuvaChatConversations } from "@/components/yuva-chat-conversations"
import { useYuvaChat } from "@/hooks/use-yuva-chat"

export function Conversations() {
  const chat = useYuvaChat({
    ${chatEnv.replaceAll("\n      ", "\n    ")},
  })
  return <YuvaChatConversations chat={chat} />
}`,
  },
  "yuva-chat-email": {
    props: [
      {
        id: "props",
        file: "components/yuva-chat-email.tsx",
        interface: "YuvaChatEmailProps",
      },
    ],
    labelsFrom: "yuva-chat",
    usage: `import { YuvaChatEmail } from "@/components/yuva-chat-email"

<YuvaChatEmail chat={chat} />`,
    example: `import { YuvaChatEmail } from "@/components/yuva-chat-email"
import { useYuvaChat } from "@/hooks/use-yuva-chat"

export function EmailCard() {
  const chat = useYuvaChat({
    ${chatEnv.replaceAll("\n      ", "\n    ")},
  })
  return <YuvaChatEmail chat={chat} />
}`,
  },
  "use-yuva-chat": {
    props: [
      { id: "options", file: "hooks/use-yuva-chat.tsx", interface: "UseYuvaChatOptions" },
      { id: "returns", file: "hooks/use-yuva-chat.tsx", interface: "YuvaChatState" },
    ],
    usage: `import { useYuvaChat } from "@/hooks/use-yuva-chat"

const chat = useYuvaChat({ server, channel })`,
    example: `import { useYuvaChat } from "@/hooks/use-yuva-chat"

export function Inbox() {
  const chat = useYuvaChat({
    ${chatEnv.replaceAll("\n      ", "\n    ")},
  })

  return (
    <ul>
      {chat.conversations.map((conversation) => (
        <li key={conversation.id}>
          <button onClick={() => void chat.open(conversation.id)}>{conversation.subject}</button>
        </li>
      ))}
    </ul>
  )
}`,
  },
  "yuva-discussions": {
    props: [
      {
        id: "props",
        file: "components/yuva-discussions.tsx",
        interface: "YuvaDiscussionsProps",
      },
    ],
    labels: discussionLabels,
    usage: `import { YuvaDiscussions } from "@/components/yuva-discussions"

<YuvaDiscussions server={server} channel={channel} />`,
    example: `import { YuvaDiscussions } from "@/components/yuva-discussions"

export function PageDiscussions({ title }: { title: string }) {
  return (
    <YuvaDiscussions
      ${chatEnv}
      pageTitle={title}
    />
  )
}`,
  },
  "yuva-discussion-list": {
    props: [
      {
        id: "props",
        file: "components/yuva-discussion-list.tsx",
        interface: "YuvaDiscussionListProps",
      },
    ],
    labelsFrom: "yuva-discussions",
    usage: `import { YuvaDiscussionList } from "@/components/yuva-discussion-list"

<YuvaDiscussionList page={page} onSelect={(discussion) => setId(discussion.id)} />`,
    example: provider(
      `<YuvaDiscussionList page="/docs/getting-started" onSelect={(discussion) => console.log(discussion.id)} />`
    ).replace(
      "import { YuvaDiscussionsProvider }",
      'import { YuvaDiscussionList } from "@/components/yuva-discussion-list"\nimport { YuvaDiscussionsProvider }'
    ),
  },
  "yuva-discussion-thread": {
    props: [
      {
        id: "props",
        file: "components/yuva-discussion-thread.tsx",
        interface: "YuvaDiscussionThreadProps",
      },
    ],
    labelsFrom: "yuva-discussions",
    usage: `import { YuvaDiscussionThread } from "@/components/yuva-discussion-thread"

<YuvaDiscussionThread id={id} onBack={() => setId(null)} />`,
    example: provider(`<YuvaDiscussionThread id={id} />`).replace(
      "import { YuvaDiscussionsProvider }",
      'import { YuvaDiscussionThread } from "@/components/yuva-discussion-thread"\nimport { YuvaDiscussionsProvider }'
    ),
  },
  "yuva-discussion-composer": {
    props: [
      {
        id: "composer",
        file: "components/yuva-discussion-composer.tsx",
        interface: "YuvaDiscussionComposerProps",
      },
      {
        id: "sign-in",
        file: "components/yuva-discussion-composer.tsx",
        interface: "YuvaSignInProps",
      },
    ],
    labelsFrom: "yuva-discussions",
    usage: `import { YuvaDiscussionComposer } from "@/components/yuva-discussion-composer"

<YuvaDiscussionComposer withTitle onSubmit={async ({ title, body }) => {}} />`,
    example: provider(
      `<YuvaDiscussionComposer withTitle onSubmit={async ({ title, body }) => save(title, body)} />`
    ).replace(
      "import { YuvaDiscussionsProvider }",
      'import { YuvaDiscussionComposer } from "@/components/yuva-discussion-composer"\nimport { YuvaDiscussionsProvider }'
    ),
  },
  "yuva-page-feedback": {
    props: [
      {
        id: "props",
        file: "components/yuva-page-feedback.tsx",
        interface: "YuvaPageFeedbackProps",
      },
    ],
    labels: { ...discussionLabels, only: /^feedback/ },
    usage: `import { YuvaPageFeedback } from "@/components/yuva-page-feedback"

<YuvaPageFeedback />`,
    example: provider(`<YuvaPageFeedback pageTitle="Getting started" />`).replace(
      "import { YuvaDiscussionsProvider }",
      'import { YuvaPageFeedback } from "@/components/yuva-page-feedback"\nimport { YuvaDiscussionsProvider }'
    ),
  },
  "use-yuva-discussions": {
    props: [
      {
        id: "provider",
        file: "hooks/use-yuva-discussions.tsx",
        interface: "YuvaDiscussionsProviderProps",
      },
      { id: "list", file: "hooks/use-yuva-discussions.tsx", interface: "DiscussionsResult" },
      { id: "one", file: "hooks/use-yuva-discussions.tsx", interface: "DiscussionResult" },
      { id: "reader", file: "hooks/use-yuva-discussions.tsx", interface: "YuvaReaderState" },
    ],
    labels: discussionLabels,
    usage: `import { useDiscussions, YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"

const { discussions, loading } = useDiscussions(page, "top")`,
    example: `import { useDiscussions, YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"

function Titles() {
  const { discussions, loading } = useDiscussions("/docs/getting-started", "top")
  if (loading) return null
  return (
    <ul>
      {discussions.map((discussion) => (
        <li key={discussion.id}>{discussion.title}</li>
      ))}
    </ul>
  )
}

export function Docs() {
  return (
    <YuvaDiscussionsProvider
      ${chatEnv}
    >
      <Titles />
    </YuvaDiscussionsProvider>
  )
}`,
  },
  "yuva-subscribe": {
    props: [{ id: "props", file: "components/yuva-subscribe.tsx", interface: "YuvaSubscribeProps" }],
    labels: {
      file: "components/yuva-subscribe.tsx",
      interface: "YuvaSubscribeLabels",
      defaults: "defaultYuvaSubscribeLabels",
    },
    usage: `import { YuvaSubscribe } from "@/components/yuva-subscribe"

<YuvaSubscribe server={server} channel={channel} />`,
    example: `import { YuvaSubscribe } from "@/components/yuva-subscribe"

export function Newsletter() {
  return (
    <YuvaSubscribe
      ${chatEnv}
      name="optional"
      privacyUrl="/privacy"
    />
  )
}`,
  },
  "use-yuva-subscribe": {
    props: [
      { id: "options", file: "hooks/use-yuva-subscribe.tsx", interface: "UseYuvaSubscribeOptions" },
      { id: "returns", file: "hooks/use-yuva-subscribe.tsx", interface: "YuvaSubscribeState" },
    ],
    usage: `import { useYuvaSubscribe } from "@/hooks/use-yuva-subscribe"

const { lists, states, subscribe, setSubscribed } = useYuvaSubscribe({ server, channel })`,
    example: `import { useYuvaSubscribe } from "@/hooks/use-yuva-subscribe"

export function Lists() {
  const { lists, states, setSubscribed } = useYuvaSubscribe({
    ${chatEnv.replaceAll("\n      ", "\n    ")},
  })

  return (
    <ul>
      {lists.map((list) => (
        <li key={list.id}>
          {list.name}
          <button onClick={() => void setSubscribed(list.id, states[list.id] !== "subscribed", "I agree")}>
            {states[list.id] === "subscribed" ? "Unsubscribe" : "Subscribe"}
          </button>
        </li>
      ))}
    </ul>
  )
}`,
  },
  "yuva-help": {
    props: [{ id: "props", file: "components/yuva-help.tsx", interface: "YuvaHelpProps" }],
    labels: helpLabels,
    usage: `import { YuvaHelp } from "@/components/yuva-help"

<YuvaHelp server={server} channel={channel} page="/billing" onAsk={() => chat.open()} />`,
    example: `import { YuvaHelp } from "@/components/yuva-help"

export function HelpCenter() {
  return (
    <YuvaHelp
      ${chatEnv}
      page={window.location.pathname}
      onAsk={(articleId) => openChat(articleId)}
    />
  )
}`,
  },
  "yuva-help-search": {
    props: [{ id: "props", file: "components/yuva-help-search.tsx", interface: "YuvaHelpSearchProps" }],
    labelsFrom: "yuva-help",
    usage: `import { YuvaHelpSearch } from "@/components/yuva-help-search"

<YuvaHelpSearch server={server} channel={channel} onSelect={(article) => setId(article.id)} />`,
    example: `import { YuvaHelpSearch } from "@/components/yuva-help-search"

export function Search() {
  return (
    <YuvaHelpSearch
      ${chatEnv}
      onSelect={(article, query) => console.log(article.title, query)}
    />
  )
}`,
  },
  "yuva-help-article": {
    props: [{ id: "props", file: "components/yuva-help-article.tsx", interface: "YuvaHelpArticleProps" }],
    labelsFrom: "yuva-help",
    usage: `import { YuvaHelpArticle } from "@/components/yuva-help-article"

<YuvaHelpArticle server={server} channel={channel} id={articleId} onAsk={(id) => chat.open(id)} />`,
    example: `import { YuvaHelpArticle } from "@/components/yuva-help-article"

export function Article({ id }: { id: string }) {
  return (
    <YuvaHelpArticle
      ${chatEnv}
      id={id}
      onAsk={(articleId) => openChat(articleId)}
    />
  )
}`,
  },
  "use-yuva-help": {
    props: [
      { id: "options", file: "hooks/use-yuva-help.tsx", interface: "UseYuvaHelpOptions" },
      { id: "returns", file: "hooks/use-yuva-help.tsx", interface: "YuvaHelpState" },
      { id: "search", file: "hooks/use-yuva-help.tsx", interface: "YuvaHelpSearchState" },
      { id: "article", file: "hooks/use-yuva-help.tsx", interface: "YuvaHelpArticleState" },
    ],
    labels: helpLabels,
    usage: `import { useYuvaHelp, useYuvaHelpSearch } from "@/hooks/use-yuva-help"

const help = useYuvaHelp({ server, channel })
const { results } = useYuvaHelpSearch(help, query)`,
    example: `import { useYuvaHelp, useYuvaHelpSearch } from "@/hooks/use-yuva-help"

export function Results({ query }: { query: string }) {
  const help = useYuvaHelp({
    ${chatEnv.replaceAll("\n      ", "\n    ")},
  })
  const { results } = useYuvaHelpSearch(help, query)

  return (
    <ul>
      {results?.items.map((article) => (
        <li key={article.id}>{article.title}</li>
      ))}
    </ul>
  )
}`,
  },
  "yuva-roadmap": {
    props: [
      { id: "props", file: "components/yuva-roadmap.tsx", interface: "YuvaRoadmapProps" },
      { id: "list", file: "components/yuva-roadmap.tsx", interface: "YuvaRoadmapListProps" },
    ],
    labels: roadmapLabels,
    usage: `import { YuvaRoadmap } from "@/components/yuva-roadmap"

<YuvaRoadmap server={server} channel={channel} />`,
    example: `import { YuvaRoadmap } from "@/components/yuva-roadmap"

export function Roadmap() {
  return (
    <YuvaRoadmap
      ${chatEnv}
    />
  )
}`,
  },
  "yuva-idea": {
    props: [{ id: "props", file: "components/yuva-idea.tsx", interface: "YuvaIdeaProps" }],
    labelsFrom: "yuva-roadmap",
    usage: `import { YuvaIdea } from "@/components/yuva-idea"

<YuvaIdea id={ideaId} onBack={() => setIdea(null)} />`,
    example: `import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"
import { YuvaIdea } from "@/components/yuva-idea"

export function Idea({ id }: { id: string }) {
  return (
    <YuvaDiscussionsProvider
      ${chatEnv}
    >
      <YuvaIdea id={id} />
    </YuvaDiscussionsProvider>
  )
}`,
  },
  "yuva-idea-suggest": {
    props: [{ id: "props", file: "components/yuva-idea-suggest.tsx", interface: "YuvaIdeaSuggestProps" }],
    labelsFrom: "yuva-roadmap",
    usage: `import { YuvaIdeaSuggest } from "@/components/yuva-idea-suggest"

<YuvaIdeaSuggest onSuggested={(idea) => setIdea(idea.id)} />`,
    example: `import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"
import { YuvaIdeaSuggest } from "@/components/yuva-idea-suggest"

export function Suggest() {
  return (
    <YuvaDiscussionsProvider
      ${chatEnv}
    >
      <YuvaIdeaSuggest onSuggested={(idea) => console.log(idea.title)} />
    </YuvaDiscussionsProvider>
  )
}`,
  },
  "use-yuva-roadmap": {
    props: [
      { id: "board", file: "hooks/use-yuva-roadmap.tsx", interface: "RoadmapResult" },
      { id: "ideas", file: "hooks/use-yuva-roadmap.tsx", interface: "IdeasResult" },
    ],
    labelsFrom: "yuva-roadmap",
    usage: `import { useIdeas, useRoadmap } from "@/hooks/use-yuva-roadmap"

const { board } = useRoadmap()
const { ideas, vote } = useIdeas("planned", "top")`,
    example: `import { YuvaDiscussionsProvider } from "@/hooks/use-yuva-discussions"
import { useIdeas } from "@/hooks/use-yuva-roadmap"

function Planned() {
  const { ideas, loading } = useIdeas("planned", "top")
  if (loading) return null
  return (
    <ul>
      {ideas.map((idea) => (
        <li key={idea.id}>
          {idea.title} ({idea.votes})
        </li>
      ))}
    </ul>
  )
}

export function Roadmap() {
  return (
    <YuvaDiscussionsProvider
      ${chatEnv}
    >
      <Planned />
    </YuvaDiscussionsProvider>
  )
}`,
  },
}

const autoSpecs = new Map<string, Spec>()

function specFor(name: string, file: string): Spec {
  const known = specs[name]
  if (known) return known
  let auto = autoSpecs.get(name)
  if (!auto) {
    const sf = parse(file)
    const exported = (node: ts.Node) =>
      !!ts.getModifiers(node as ts.HasModifiers)?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword)
    const interfaces = sf.statements.filter(
      (st): st is ts.InterfaceDeclaration =>
        ts.isInterfaceDeclaration(st) && exported(st) && /(Props|Options|State|Result)$/.test(st.name.text)
    )
    const functions = sf.statements.filter(
      (st): st is ts.FunctionDeclaration => ts.isFunctionDeclaration(st) && exported(st) && !!st.name
    )
    const main = functions[0]?.name?.text ?? name
    const alias = `@/${file.replace(/\.tsx?$/, "")}`
    const hook = main.startsWith("use")
    const usage = `import { ${main} } from "${alias}"\n\n${hook ? `const value = ${main}()` : `<${main} />`}`
    auto = {
      props: interfaces.map((i) => ({ id: i.name, file, interface: i.name })),
      usage,
      example: usage,
    }
    autoSpecs.set(name, auto)
  }
  return auto
}

export type Item = {
  name: string
  group: Group
  title: string
  description: string
  dependencies: string[]
  shadcn: string[]
  yuva: string[]
  files: string[]
  usage: string
  example: string
}

type RegistryItem = {
  name: string
  type: string
  title: string
  description: string
  dependencies?: string[]
  registryDependencies?: string[]
  files: { path: string }[]
}

const groupOf = (type: string): Group =>
  type === "registry:block" ? "blocks" : type === "registry:hook" ? "hooks" : "components"
export const groupOrder: Group[] = ["blocks", "components", "hooks"]

const order = Object.keys(specs)
let cachedItems: Item[] | undefined

export function items(): Item[] {
  cachedItems ??= (
    JSON.parse(fs.readFileSync(path.resolve("../sdk/shadcn/registry.json"), "utf8")).items as RegistryItem[]
  )
    .map((entry) => {
      const spec = specFor(entry.name, entry.files[0]!.path.replace(/^registry\/yuva\//, ""))
      const deps = entry.registryDependencies ?? []
      return {
        name: entry.name,
        group: groupOf(entry.type),
        title: entry.title,
        description: entry.description,
        dependencies: entry.dependencies ?? [],
        shadcn: deps.filter((d) => !d.startsWith("http")),
        yuva: deps.filter((d) => d.startsWith(REGISTRY_URL)).map((d) => d.slice(REGISTRY_URL.length, -".json".length)),
        files: entry.files.map((f) => f.path.replace(/^registry\/yuva\//, "")),
        usage: spec.usage,
        example: spec.example,
      }
    })
    .map((item, index) => ({ item, key: order.includes(item.name) ? order.indexOf(item.name) : 1000 + index }))
    .sort((a, b) => a.key - b.key)
    .map(({ item }) => item)
  return cachedItems
}

export const byGroup = (group: Group) => items().filter((i) => i.group === group)
export const installCommands = (name: string) => {
  const url = `${REGISTRY_URL}${name}.json`
  return [
    { id: "npm", command: `npx shadcn@latest add ${url}` },
    { id: "pnpm", command: `pnpm dlx shadcn@latest add ${url}` },
    { id: "yarn", command: `yarn dlx shadcn@latest add ${url}` },
    { id: "bun", command: `bunx --bun shadcn@latest add ${url}` },
  ]
}

const parsed = new Map<string, ts.SourceFile>()
function parse(file: string) {
  let sf = parsed.get(file)
  if (!sf) {
    const full = path.join(source, file)
    sf = ts.createSourceFile(full, fs.readFileSync(full, "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
    parsed.set(file, sf)
  }
  return sf
}

function interfaceOf(file: string, name: string) {
  const found = parse(file).statements.find(
    (s): s is ts.InterfaceDeclaration => ts.isInterfaceDeclaration(s) && s.name.text === name
  )
  if (!found) throw new Error(`shadcn: ${name} not found in ${file}`)
  return found
}

const squash = (text: string) => text.replace(/\s+/g, " ").trim()

export function propTables(name: string): Table[] {
  return (specs[name]?.props ?? autoSpecs.get(name)?.props ?? []).map(({ id, file, interface: iface }) => ({
    id,
    title: iface,
    rows: interfaceOf(file, iface)
      .members.filter(
        (m): m is ts.PropertySignature | ts.MethodSignature => ts.isPropertySignature(m) || ts.isMethodSignature(m)
      )
      .map((m) => {
        const key = m.name.getText()
        const type = ts.isMethodSignature(m)
          ? `${m.typeParameters ? `<${m.typeParameters.map((t) => t.getText()).join(", ")}>` : ""}(${m.parameters.map((p) => p.getText()).join(", ")}) => ${m.type?.getText() ?? "void"}`
          : (m.type?.getText() ?? "unknown")
        return { name: key, type: squash(type), optional: !!m.questionToken }
      }),
  }))
}

export function labelRows(name: string): { owner: string; rows: LabelRow[] } | null {
  const spec = specs[name]
  const owner = spec?.labels ? name : spec?.labelsFrom
  const labels = owner ? specs[owner]?.labels : undefined
  if (!owner || !labels) return null
  const decl = parse(labels.file)
    .statements.flatMap((s) => (ts.isVariableStatement(s) ? s.declarationList.declarations : []))
    .find((d) => d.name.getText() === labels.defaults)
  const object = decl?.initializer
  if (!object || !ts.isObjectLiteralExpression(object)) throw new Error(`shadcn: ${labels.defaults} not found`)
  const rows = object.properties
    .filter((p): p is ts.PropertyAssignment => ts.isPropertyAssignment(p))
    .map((p) => ({ key: p.name.getText(), value: squash(p.initializer.getText()) }))
    .filter((r) => !labels.only || labels.only.test(r.key))
  return { owner, rows }
}

export const chatEmailExample = specs["yuva-chat-email"]!.example
