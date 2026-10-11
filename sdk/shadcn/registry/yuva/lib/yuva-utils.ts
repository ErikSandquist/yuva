import { ApiError } from "useyuva"

export interface YuvaDiscussionsLabels {
  heading: string
  sortNew: string
  sortTop: string
  sortUnanswered: string
  newDiscussion: string
  empty: string
  emptyUnanswered: string
  loadMore: string
  loadError: string
  retry: string
  back: string
  replies: (count: number) => string
  votes: (count: number) => string
  answered: string
  acceptedAnswer: string
  markAnswer: string
  unmarkAnswer: string
  locked: string
  lockedNote: string
  pending: string
  pendingNote: string
  upvote: string
  removeUpvote: string
  reply: string
  follow: string
  unfollow: string
  edit: string
  save: string
  cancel: string
  delete: string
  deleteConfirm: string
  edited: string
  deletedPost: string
  deletedAuthor: string
  anonymous: string
  team: string
  signIn: string
  signInHint: string
  signOut: string
  signedInAs: (name: string) => string
  titleLabel: string
  titlePlaceholder: string
  bodyLabel: string
  bodyPlaceholder: string
  replyPlaceholder: string
  write: string
  preview: string
  previewEmpty: string
  markdownHint: string
  startDiscussion: string
  postReply: string
  errorSignIn: string
  errorLocked: string
  errorEditWindow: string
  errorRateLimited: string
  errorPopupBlocked: string
  errorOff: string
  errorGeneric: string
  feedbackQuestion: string
  feedbackYes: string
  feedbackNo: string
  feedbackThanks: string
}

export const defaultYuvaDiscussionsLabels: YuvaDiscussionsLabels = {
  heading: "Discussions",
  sortNew: "New",
  sortTop: "Top",
  sortUnanswered: "Unanswered",
  newDiscussion: "New discussion",
  empty: "No discussions on this page yet. Start the first one.",
  emptyUnanswered: "Every discussion on this page has an answer.",
  loadMore: "Load more",
  loadError: "Discussions could not be loaded.",
  retry: "Try again",
  back: "All discussions",
  replies: (count) => (count === 1 ? "1 reply" : `${count} replies`),
  votes: (count) => (count === 1 ? "1 vote" : `${count} votes`),
  answered: "Answered",
  acceptedAnswer: "Accepted answer",
  markAnswer: "Mark as answer",
  unmarkAnswer: "Unmark as answer",
  locked: "Locked",
  lockedNote: "This discussion is locked. It takes no new replies.",
  pending: "Awaiting approval",
  pendingNote: "Only you can see this until a moderator approves it.",
  upvote: "Upvote",
  removeUpvote: "Remove upvote",
  reply: "Reply",
  follow: "Follow",
  unfollow: "Unfollow",
  edit: "Edit",
  save: "Save",
  cancel: "Cancel",
  delete: "Delete",
  deleteConfirm: "Delete this post?",
  edited: "edited",
  deletedPost: "This post was deleted.",
  deletedAuthor: "Deleted user",
  anonymous: "Anonymous",
  team: "Team",
  signIn: "Sign in",
  signInHint: "Sign in to start a discussion, reply and vote.",
  signOut: "Sign out",
  signedInAs: (name) => `Signed in as ${name}`,
  titleLabel: "Title",
  titlePlaceholder: "What is your question or idea?",
  bodyLabel: "Message",
  bodyPlaceholder: "Add the details. Markdown works.",
  replyPlaceholder: "Write a reply. Markdown works.",
  write: "Write",
  preview: "Preview",
  previewEmpty: "Nothing to preview.",
  markdownHint: "Markdown is supported.",
  startDiscussion: "Start discussion",
  postReply: "Reply",
  errorSignIn: "Sign in to post.",
  errorLocked: "This discussion is locked.",
  errorEditWindow: "Posts can be edited for 30 minutes after posting.",
  errorRateLimited: "Too many posts. Wait a minute and try again.",
  errorPopupBlocked: "Allow pop-ups for this site to sign in.",
  errorOff: "Discussions are turned off.",
  errorGeneric: "Something went wrong. Try again.",
  feedbackQuestion: "Was this page helpful?",
  feedbackYes: "Yes",
  feedbackNo: "No",
  feedbackThanks: "Thanks for your feedback.",
}

export function yuvaErrorMessage(error: unknown, labels: YuvaDiscussionsLabels): string | null {
  if (!(error instanceof ApiError)) return labels.errorGeneric
  switch (error.code) {
    case "sign_in_cancelled":
      return null
    case "sign_in_required":
      return labels.errorSignIn
    case "discussion_locked":
      return labels.errorLocked
    case "edit_window_closed":
      return labels.errorEditWindow
    case "rate_limited":
      return labels.errorRateLimited
    case "popup_blocked":
      return labels.errorPopupBlocked
    case "discussions_off":
      return labels.errorOff
    default:
      return labels.errorGeneric
  }
}

export const EDIT_WINDOW_MS = 30 * 60 * 1000

export function canEdit(post: { mine?: boolean; deleted?: boolean; created_at: string }): boolean {
  return !!post.mine && !post.deleted && Date.now() - Date.parse(post.created_at) < EDIT_WINDOW_MS
}

export function relativeTime(value: string, locale?: string): string {
  const seconds = Math.round((Date.parse(value) - Date.now()) / 1000)
  const format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ["year", 31_536_000],
    ["month", 2_592_000],
    ["week", 604_800],
    ["day", 86_400],
    ["hour", 3_600],
    ["minute", 60],
  ]
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit)
  }
  return format.format(0, "second")
}

export function currentPage(): { url: string; title: string } {
  if (typeof window === "undefined") return { url: "", title: "" }
  return { url: window.location.origin + window.location.pathname, title: document.title }
}

export const yuvaProseClassName =
  "text-sm leading-relaxed break-words text-foreground [&_a]:font-medium [&_a]:text-primary [&_a]:underline [&_a]:underline-offset-4 [&_blockquote]:border-s-2 [&_blockquote]:ps-3 [&_blockquote]:text-muted-foreground [&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-[0.85em] [&_h1]:text-base [&_h1]:font-semibold [&_h2]:text-base [&_h2]:font-semibold [&_h3]:font-semibold [&_h4]:font-semibold [&_hr]:border-border [&_li]:my-0.5 [&_ol]:list-decimal [&_ol]:ps-5 [&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-muted [&_pre]:p-3 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_table]:w-full [&_table]:text-start [&_td]:border [&_td]:border-border [&_td]:px-2 [&_td]:py-1 [&_th]:border [&_th]:border-border [&_th]:px-2 [&_th]:py-1 [&_ul]:list-disc [&_ul]:ps-5 [&>*]:my-2 [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
