import { YuvaDiscussionsElement } from "./discussions"
import { YuvaPageFeedbackElement } from "./feedback"

export function defineDocs(): void {
  if (typeof customElements === "undefined") return
  if (!customElements.get("yuva-discussions")) customElements.define("yuva-discussions", YuvaDiscussionsElement)
  if (!customElements.get("yuva-page-feedback")) customElements.define("yuva-page-feedback", YuvaPageFeedbackElement)
}
