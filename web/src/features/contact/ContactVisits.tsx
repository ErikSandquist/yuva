import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { Link } from "react-router"

import { formatDateTime, formatDuration, formatRelative } from "@/components/common/text"
import { Button } from "@/components/ui/button"
import type { useContactVisits } from "@/features/contact/queries"
import type { AnalyticsContactVisit } from "@/lib/api"

function Visit({ v }: { v: AnalyticsContactVisit }) {
  const { t, i18n } = useLingui()
  const seconds = Math.max(0, (Date.parse(v.ended_at) - Date.parse(v.started_at)) / 1000)
  const pages = v.pages.slice(0, 3)
  const rest = v.pages.length - pages.length
  const views = v.pageviews
  const source = v.source || v.utm_campaign || t`Direct`
  return (
    <li className="flex flex-col gap-1 py-2.5 first:pt-0 last:pb-0" data-testid="contact-visit">
      <span className="flex items-baseline gap-2">
        <Link
          to={`/analytics/${v.property_id}`}
          className="min-w-0 flex-1 truncate text-body font-medium underline-offset-4 hover:underline"
        >
          {v.property_name}
        </Link>
        <time
          dateTime={v.started_at}
          title={formatDateTime(v.started_at, i18n.locale)}
          className="shrink-0 text-caption text-faint"
        >
          {formatRelative(v.started_at, i18n.locale)}
        </time>
      </span>
      <ul className="flex min-w-0 flex-col text-small text-muted-foreground" dir="ltr">
        {pages.map((p) => (
          <li key={p.path} className="flex min-w-0 gap-1.5">
            <span className="min-w-0 truncate" title={p.path}>
              {p.path}
            </span>
            {p.views > 1 && <span className="shrink-0 text-faint tabular-nums">×{p.views}</span>}
          </li>
        ))}
      </ul>
      {rest > 0 && (
        <span className="text-caption text-faint">
          <Plural value={rest} one="and # more page" other="and # more pages" />
        </span>
      )}
      <span className="flex flex-wrap gap-x-1.5 text-caption text-faint">
        <span className="min-w-0 truncate">{source}</span>
        <span aria-hidden>·</span>
        <span>
          <Plural value={views} one="# view" other="# views" />
        </span>
        {seconds >= 1 && (
          <>
            <span aria-hidden>·</span>
            <span>{formatDuration(seconds, i18n.locale)}</span>
          </>
        )}
      </span>
    </li>
  )
}

export function ContactVisits({ visits }: { visits: ReturnType<typeof useContactVisits> }) {
  const items = visits.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <div className="flex flex-col gap-2.5" data-testid="contact-visits">
      <ul className="flex flex-col divide-y">
        {items.map((v) => (
          <Visit key={`${v.property_id}${v.started_at}`} v={v} />
        ))}
      </ul>
      {visits.hasNextPage && (
        <Button
          variant="ghost"
          size="sm"
          className="self-start"
          onClick={() => void visits.fetchNextPage()}
          disabled={visits.isFetchingNextPage}
        >
          <Trans>Older visits</Trans>
        </Button>
      )}
      <p className="text-caption text-faint">
        <Trans>Only visits made after they agreed to a persistent id and signed in.</Trans>
      </p>
    </div>
  )
}
