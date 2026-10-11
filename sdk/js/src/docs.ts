import { defineDocs } from "./docs/define"
import { YuvaDiscussionsElement, type DiscussionDetailEvent, type DiscussionPostDetail } from "./docs/discussions"
import { YuvaPageFeedbackElement, type PageFeedbackDetail, type PageRatingDetail } from "./docs/feedback"

YuvaDiscussionsElement.loadThread = () => import("./docs/thread")
defineDocs()

export {
  defineDocs,
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
