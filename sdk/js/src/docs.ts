import { YuvaDiscussionsElement, type DiscussionDetailEvent, type DiscussionPostDetail } from "./docs/discussions"
import { YuvaPageFeedbackElement, type PageFeedbackDetail, type PageRatingDetail } from "./docs/feedback"

export function defineDocs(): void {
  if (typeof customElements === "undefined") return
  if (!customElements.get("yuva-discussions")) customElements.define("yuva-discussions", YuvaDiscussionsElement)
  if (!customElements.get("yuva-page-feedback")) customElements.define("yuva-page-feedback", YuvaPageFeedbackElement)
}

defineDocs()

export {
  YuvaDiscussionsElement,
  YuvaPageFeedbackElement,
  type DiscussionDetailEvent,
  type DiscussionPostDetail,
  type PageFeedbackDetail,
  type PageRatingDetail,
}
export type { IdentityTokenSource } from "./client/client"
export type { Discussion, DiscussionPost, PageRating, ReaderProfile } from "./client/api"
export type { ReaderSession } from "./client/reader"
