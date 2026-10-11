import { defineComponent, h, onMounted, ref, watch, type PropType } from "vue"
import { useRuntimeConfig } from "#imports"
import type { IdentityTokenSource, Layout } from "../../types"
import type { PageFeedbackDetail, PageQuestionDetail, PageRatingDetail } from "../../docs"

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

type Props = {
  channel?: string
  server?: string
  locale?: string
  identityToken?: string | IdentityTokenSource | null
} & Record<string, unknown>

function setup(
  tag: string,
  load: () => Promise<unknown>,
  props: Props,
  attributes: (props: Props) => Record<string, unknown>,
  listeners: Record<string, (event: Event) => void>
) {
  const config = useRuntimeConfig().public.yuva
  const element = ref<Tokened | null>(null)

  onMounted(() => {
    void load()
    watch(
      () => props.identityToken,
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

export const YuvaPageQuestions = defineComponent({
  name: "YuvaPageQuestions",
  props: docsProps,
  emits: {
    question: (_: PageQuestionDetail) => true,
  },
  setup(props, { emit, expose }) {
    const { element, render } = setup(
      "yuva-page-questions",
      () => import("../../docs"),
      props,
      (p) => ({ page: p.page, "page-title": p.pageTitle }),
      { onYuvaQuestion: detail<PageQuestionDetail>((d) => emit("question", d)) }
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
