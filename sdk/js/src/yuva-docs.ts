import { scriptSrc } from "./docs/base"
import { defineDocs } from "./docs/define"
import { YuvaDiscussionsElement, type ThreadModule } from "./docs/discussions"

declare const YUVA_DOCS_THREAD: string

YuvaDiscussionsElement.loadThread = () =>
  import(/* @vite-ignore */ new URL(YUVA_DOCS_THREAD, scriptSrc || location.href).href) as Promise<ThreadModule>
defineDocs()
