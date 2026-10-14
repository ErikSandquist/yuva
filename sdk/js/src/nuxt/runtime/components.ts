import { defineComponent, h, onMounted, ref, watch, type PropType } from "vue"
import { useRuntimeConfig } from "#imports"
import type { IdentityTokenSource, Layout } from "../../types"
import type { DiscussionDetailEvent, DiscussionPostDetail, PageFeedbackDetail, PageRatingDetail } from "../../docs"
import type { ReaderProfile } from "../../client/api"
import type { SubscribeDetail, SubscribeErrorDetail } from "../../subscribe"

type Tokened = HTMLElement & { setIdentityToken(source: IdentityTokenSource | null): void }

const shared = {
  channel: String,
  server: String,
  locale: String,
  dir: String as PropType<"ltr" | "rtl">,
  identityToken: [String, Function] as PropType<string | IdentityTokenSource | null>,
}

const docsProps = { ...shared, page: String, pageTitle: String }

const chatProps = { ...shared, layout: String as PropType<Layout>, open: Boolean }

const subscribeProps = {
  ...shared,
  lists: Array as PropType<string[]>,
  layout: String as PropType<"stacked" | "inline">,
  name: String as PropType<"off" | "optional" | "required">,
  buttonText: String,
  consent: String,
  privacyUrl: String,
}

type Props = {
  channel?: string
  server?: string
  locale?: string
  identityToken?: string | IdentityTokenSource | null
} & Record<string, unknown>

function identityRoute(path: string): IdentityTokenSource {
  return async () => {
    const response = await fetch(path, { credentials: "same-origin" })
    return response.ok ? ((await response.json()) as { token?: string }).token : null
  }
}

function setup(
  tag: string,
  load: () => Promise<unknown>,
  props: Props,
  attributes: (props: Props) => Record<string, unknown>,
  listeners: Record<string, (event: Event) => void>
) {
  const config = useRuntimeConfig().public.yuva
  const element = ref<Tokened | null>(null)
  const route = config?.identity ? identityRoute(config.identity) : undefined

  onMounted(() => {
    void load()
    watch(
      () => (props.identityToken === undefined ? route : props.identityToken),
      (source) => {
        const target = element.value
        if (!target || typeof source !== "function") return
        void customElements.whenDefined(tag).then(() => target.setIdentityToken(source))
      },
      { immediate: true }
    )
  })

  return {
    element,
    render: () => {
      const values: Record<string, unknown> = {
        channel: props.channel || config?.channel || undefined,
        server: props.server || config?.server || undefined,
        locale: props.locale || config?.locale || undefined,
        "identity-token": typeof props.identityToken === "string" ? props.identityToken : undefined,
        dir: props.dir,
        ...attributes(props),
      }
      const vnode: Record<string, unknown> = { ref: element, ...listeners }
      for (const [name, value] of Object.entries(values))
        vnode[`^${name}`] = value === "" || value == null ? undefined : value
      return h(tag, vnode)
    },
  }
}

const detail =
  <T>(emit: (detail: T) => void) =>
  (event: Event) =>
    emit((event as CustomEvent<T>).detail)

export const YuvaPageFeedback = defineComponent({
  name: "YuvaPageFeedback",
  props: docsProps,
  emits: {
    rating: (_: PageRatingDetail) => true,
    feedback: (_: PageFeedbackDetail) => true,
  },
  setup(props, { emit, expose }) {
    const { element, render } = setup(
      "yuva-page-feedback",
      () => import("../../docs"),
      props,
      (p) => ({ page: p.page, "page-title": p.pageTitle }),
      {
        onYuvaRating: detail<PageRatingDetail>((d) => emit("rating", d)),
        onYuvaFeedback: detail<PageFeedbackDetail>((d) => emit("feedback", d)),
      }
    )
    expose({ element })
    return render
  },
})

export const YuvaDiscussions = defineComponent({
  name: "YuvaDiscussions",
  props: docsProps,
  emits: {
    discussion: (_: DiscussionDetailEvent) => true,
    post: (_: DiscussionPostDetail) => true,
    reader: (_: ReaderProfile | null) => true,
  },
  setup(props, { emit, expose }) {
    const { element, render } = setup(
      "yuva-discussions",
      () => import("../../docs"),
      props,
      (p) => ({ page: p.page, "page-title": p.pageTitle }),
      {
        onYuvaDiscussion: detail<DiscussionDetailEvent>((d) => emit("discussion", d)),
        onYuvaPost: detail<DiscussionPostDetail>((d) => emit("post", d)),
        onYuvaReader: detail<ReaderProfile | null>((d) => emit("reader", d)),
      }
    )
    expose({ element })
    return render
  },
})

export const YuvaChat = defineComponent({
  name: "YuvaChat",
  props: chatProps,
  emits: {
    unread: (_: { count: number }) => true,
  },
  setup(props, { emit, expose }) {
    const { element, render } = setup(
      "yuva-chat",
      () => import("../../chat"),
      props,
      (p) => ({ layout: p.layout, open: p.open || undefined }),
      { onYuvaUnread: detail<{ count: number }>((d) => emit("unread", d)) }
    )
    expose({ element })
    return render
  },
})

export const YuvaSubscribe = defineComponent({
  name: "YuvaSubscribe",
  props: subscribeProps,
  emits: {
    subscribe: (_: SubscribeDetail) => true,
    error: (_: SubscribeErrorDetail) => true,
  },
  setup(props, { emit, expose }) {
    const { element, render } = setup(
      "yuva-subscribe",
      () => import("../../subscribe"),
      props,
      (p) => ({
        lists: (p.lists as string[] | undefined)?.join(","),
        layout: p.layout,
        name: p.name,
        "button-text": p.buttonText,
        consent: p.consent,
        "privacy-url": p.privacyUrl,
      }),
      {
        onYuvaSubscribe: detail<SubscribeDetail>((d) => emit("subscribe", d)),
        onYuvaSubscribeError: detail<SubscribeErrorDetail>((d) => emit("error", d)),
      }
    )
    expose({ element })
    return render
  },
})
