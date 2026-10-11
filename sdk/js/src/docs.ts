import { YuvaPageFeedbackElement, type PageFeedbackDetail, type PageRatingDetail } from "./docs/feedback"

export function defineDocs(): void {
  if (typeof customElements === "undefined") return
  if (!customElements.get("yuva-page-feedback")) customElements.define("yuva-page-feedback", YuvaPageFeedbackElement)
}

defineDocs()

export { YuvaPageFeedbackElement, type PageFeedbackDetail, type PageRatingDetail }
export type { IdentityTokenSource } from "./client/client"
export type { PageRating } from "./client/api"
