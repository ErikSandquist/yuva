import { Trans, useLingui } from "@lingui/react/macro"

import { Kbd } from "@/components/common/Kbd"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"

export const SHORTCUTS = {
  next: "j",
  reply: "r",
  note: "n",
  attach: "f",
  snooze: "s",
  hand: "a",
  labels: "l",
  progress: "o",
  close: "e",
  leave: "L",
  history: "h",
  contact: "c",
  more: ".",
  spam: "!",
  quoted: "q",
  discardSuggestion: "X",
  copyDetails: "y",
  search: "/",
  help: "?",
  previous: "k",
  home: "g h",
  contacts: "g c",
  conversations: "g i",
  team: "g t",
  reports: "g r",
  assistants: "g a",
  mentions: "g m",
  feedback: "g f",
  find: "F",
  pin: "p",
  unread: "u",
  docs: "g d",
  broadcasts: "g b",
  analytics: "g n",
  status: "g s",
  discussion: "P",
} as const

export const DISCUSSION_KEYS = {
  next: "j",
  previous: "k",
  reply: "r",
  replyTo: "R",
  answer: "a",
  approve: "y",
  hide: "x",
  spam: "!",
  title: "t",
  lock: "l",
  page: "o",
} as const

export const BROADCAST_KEYS = {
  create: "c",
  design: "o",
  test: "t",
  send: "s",
  unschedule: "u",
  pause: "p",
  cancel: "X",
  report: "r",
  deliveries: "d",
} as const

export const STATUS_KEYS = {
  incidents: "1",
  components: "2",
  monitors: "3",
  settings: "4",
  addMonitor: "n",
  incident: "c",
  maintenance: "m",
  preview: "v",
  publishPage: "P",
  update: "r",
  publish: "p",
  resolve: "x",
  edit: "e",
  remove: "X",
} as const

export const DESIGNER_KEYS = {
  edit: "1",
  desktop: "2",
  phone: "3",
  dark: "4",
  add: "a",
  theme: "t",
  image: "i",
  versions: "v",
} as const

export function keyLabel(k: string): string[] {
  if (k.includes(" ")) return k.split(" ").flatMap(keyLabel)
  return /^[A-Z]$/.test(k) ? ["⇧", k] : [k.toUpperCase()]
}

const apple = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform)

export const mod = apple ? "⌘" : "Ctrl"

export const alt = apple ? "⌥" : "Alt"

export function ShortcutSheet({
  open,
  onOpenChange,
  home,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  home?: boolean
}) {
  const { t } = useLingui()
  const k = keyLabel
  const groups: { title: string; items: [string[], string][] }[] = [
    {
      title: t`Answering`,
      items: [
        [k(SHORTCUTS.reply), t`Write a reply`],
        [k(SHORTCUTS.note), t`Write a note for the team`],
        [[mod, "↵"], t`Send and close`],
        [["⇧", mod, "↵"], t`Send only`],
        [["↵"], t`Send or add the note, in a chat or app conversation (⇧↵ for a new line)`],
        [["Tab"], t`Use the suggested reply`],
        [k(SHORTCUTS.discardSuggestion), t`Discard the suggested reply`],
        [k(SHORTCUTS.attach), t`Attach files`],
        [["/"], t`Canned replies, in the reply box`],
        [k(SHORTCUTS.leave), t`Leave it to the teammate who is typing`],
      ],
    },
    {
      title: t`This conversation`,
      items: [
        [k(SHORTCUTS.hand), t`Assign or hand to a teammate`],
        [k(SHORTCUTS.snooze), t`Snooze`],
        [k(SHORTCUTS.labels), t`Labels`],
        [k(SHORTCUTS.progress), t`Set the feedback progress`],
        [k(SHORTCUTS.close), t`Close or reopen`],
        [k(SHORTCUTS.contact), t`Contact details`],
        [k(SHORTCUTS.history), t`Other conversations`],
        [k(SHORTCUTS.more), t`Priority, inbox, spam and more`],
        [k(SHORTCUTS.spam), t`Mark or unmark as spam`],
        [k(SHORTCUTS.quoted), t`Show or hide quoted text`],
        [k(SHORTCUTS.copyDetails), t`Copy the feedback details`],
        [k(SHORTCUTS.find), t`Search in this conversation`],
        [k(SHORTCUTS.pin), t`Pin to the top of your list, or unpin`],
        [k(SHORTCUTS.unread), t`Mark as unread and go back to the list`],
        [k(SHORTCUTS.discussion), t`Turn into a public discussion`],
      ],
    },
    {
      title: t`Conversation list`,
      items: [
        [["↑", "↓"], t`Move through the list`],
        [["↵"], t`Open the conversation`],
        [[k(SHORTCUTS.next)[0], k(SHORTCUTS.previous)[0]], t`Open the next or previous conversation`],
        [[SHORTCUTS.search], t`Search conversations`],
      ],
    },
    {
      title: t`Discussion`,
      items: [
        [["↑", "↓"], t`Move through the posts`],
        [k(DISCUSSION_KEYS.reply), t`Reply to the discussion`],
        [k(DISCUSSION_KEYS.replyTo), t`Reply to the selected post`],
        [k(DISCUSSION_KEYS.answer), t`Mark the selected reply as the answer, or clear it`],
        [k(DISCUSSION_KEYS.approve), t`Approve or show the selected post`],
        [k(DISCUSSION_KEYS.hide), t`Hide the selected post`],
        [k(DISCUSSION_KEYS.spam), t`Mark the selected post as spam`],
        [k(DISCUSSION_KEYS.title), t`Edit the title`],
        [k(DISCUSSION_KEYS.lock), t`Lock or unlock`],
        [k(DISCUSSION_KEYS.page), t`Open on its page`],
        [[mod, "↵"], t`Send the reply`],
      ],
    },
    {
      title: t`Discussions and approvals`,
      items: [
        [["↑", "↓"], t`Move through the list`],
        [["↵"], t`Open the discussion`],
        [k(DISCUSSION_KEYS.approve), t`Approve the selected post`],
        [k(DISCUSSION_KEYS.hide), t`Hide the selected post`],
        [k(DISCUSSION_KEYS.spam), t`Mark the selected post as spam`],
      ],
    },
    {
      title: t`Broadcasts`,
      items: [
        [["↑", "↓"], t`Move through the broadcasts`],
        [["↵"], t`Open the broadcast`],
        [k(BROADCAST_KEYS.create), t`New broadcast`],
        [k(BROADCAST_KEYS.design), t`Edit the design`],
        [k(BROADCAST_KEYS.test), t`Send a test`],
        [k(BROADCAST_KEYS.send), t`Review, then send or schedule`],
        [k(BROADCAST_KEYS.unschedule), t`Back to draft`],
        [k(BROADCAST_KEYS.pause), t`Pause or resume sending`],
        [k(BROADCAST_KEYS.cancel), t`Cancel the broadcast`],
        [k(BROADCAST_KEYS.report), t`Report`],
        [k(BROADCAST_KEYS.deliveries), t`Recipients`],
      ],
    },
    {
      title: t`Status`,
      items: [
        [["↑", "↓"], t`Move through the status pages`],
        [["↵"], t`Open the status page`],
        [k(STATUS_KEYS.incidents), t`Incidents`],
        [k(STATUS_KEYS.components), t`Components`],
        [k(STATUS_KEYS.monitors), t`Monitors`],
        [k(STATUS_KEYS.settings), t`Page settings`],
        [k(STATUS_KEYS.addMonitor), t`Add a monitor`],
        [k(STATUS_KEYS.incident), t`New incident or status page`],
        [k(STATUS_KEYS.maintenance), t`Plan maintenance`],
        [k(STATUS_KEYS.preview), t`Preview the page`],
        [k(STATUS_KEYS.publishPage), t`Publish or unpublish the page`],
        [k(STATUS_KEYS.update), t`Post an update`],
        [k(STATUS_KEYS.publish), t`Publish the draft`],
        [k(STATUS_KEYS.resolve), t`Resolve or complete`],
        [k(STATUS_KEYS.edit), t`Edit the incident`],
        [k(STATUS_KEYS.remove), t`Delete the draft`],
      ],
    },
    {
      title: t`E-mail designer`,
      items: [
        [[mod, "Z"], t`Undo`],
        [["⇧", mod, "Z"], t`Redo`],
        [[mod, "S"], t`Save now and keep a version`],
        [["↑", "↓"], t`Select the previous or next block`],
        [[alt, "↑", "↓"], t`Move the selected block`],
        [[mod, "D"], t`Duplicate the selected block`],
        [["⌫"], t`Delete the selected block`],
        [["Esc"], t`Leave the text, then the block`],
        [k(DESIGNER_KEYS.add), t`Add a block`],
        [k(DESIGNER_KEYS.theme), t`Colours, fonts and width`],
        [k(DESIGNER_KEYS.image), t`Choose the selected block’s image`],
        [k(DESIGNER_KEYS.versions), t`Versions`],
        [
          [DESIGNER_KEYS.edit, DESIGNER_KEYS.desktop, DESIGNER_KEYS.phone, DESIGNER_KEYS.dark],
          t`Edit, or preview on a desktop, a phone or in dark mode`,
        ],
      ],
    },
    {
      title: t`Everywhere`,
      items: [
        ...(home ? [[k(SHORTCUTS.home), t`Home`] as [string[], string]] : []),
        [k(SHORTCUTS.conversations), t`Conversations`],
        [k(SHORTCUTS.mentions), t`Mentions`],
        [k(SHORTCUTS.feedback), t`Feedback`],
        [k(SHORTCUTS.contacts), t`Contacts`],
        [k(SHORTCUTS.team), t`Team`],
        [k(SHORTCUTS.reports), t`Reports`],
        [k(SHORTCUTS.docs), t`Docs`],
        [k(SHORTCUTS.broadcasts), t`Broadcasts`],
        [k(SHORTCUTS.status), t`Status`],
        [k(SHORTCUTS.analytics), t`Analytics`],
        [k(SHORTCUTS.assistants), t`Assistants`],
        [[mod, "K"], t`Everything`],
        [[mod, ","], t`Settings`],
        [[SHORTCUTS.help], t`This list`],
        [["Esc"], t`Close a panel or leave a field`],
      ],
    },
  ]
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85svh] overflow-y-auto sm:max-w-2xl" data-testid="shortcut-sheet">
        <DialogHeader>
          <DialogTitle>
            <Trans>Keyboard shortcuts</Trans>
          </DialogTitle>
        </DialogHeader>
        <div className="grid gap-x-8 gap-y-6 sm:grid-cols-2">
          {groups.map((g) => (
            <section key={g.title} className="flex flex-col">
              <h3 className="mb-1 text-small font-medium text-faint">{g.title}</h3>
              <ul className="flex flex-col">
                {g.items.map(([ks, label]) => (
                  <li key={label} className="flex items-center justify-between gap-3 border-t py-1.5 text-body">
                    <span>{label}</span>
                    <span className="flex shrink-0 gap-1">
                      {ks.map((key) => (
                        <Kbd key={key}>{key}</Kbd>
                      ))}
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}
