import { Trans, useLingui } from "@lingui/react/macro"
import { Link } from "react-router"

import { ErrorLine } from "@/components/common"
import { ExportButtons } from "@/components/common/ExportButtons"
import { formatDate } from "@/components/common/text"
import { Skeleton } from "@/components/ui/skeleton"
import { listLink, useContactSubscriptions } from "@/features/broadcasts/queries"
import { StateTag } from "@/features/broadcasts/SubscriberSheet"
import { api } from "@/lib/api"
import { saveDownload } from "@/lib/download"
import { useSession } from "@/lib/session"

export function ContactSubscriptions({ contactId }: { contactId: string }) {
  const { i18n } = useLingui()
  const { canManage } = useSession()
  const subs = useContactSubscriptions(contactId)
  const items = subs.data ?? []
  return (
    <div className="flex flex-col gap-3" data-testid="contact-subscriptions">
      {subs.isPending ? (
        <Skeleton className="h-10 w-full" />
      ) : subs.error ? (
        <ErrorLine error={subs.error} className="text-small" />
      ) : items.length === 0 ? (
        <p className="text-body text-faint">
          <Trans>Not on any mailing list.</Trans>
        </p>
      ) : (
        <ul className="flex flex-col divide-y">
          {items.map((s) => (
            <li key={s.id} className="flex flex-col gap-0.5 py-2 first:pt-0" data-testid="contact-subscription">
              <span className="flex items-center gap-2">
                <Link
                  to={listLink(s.list.id)}
                  className="min-w-0 flex-1 truncate text-body font-medium underline-offset-4 hover:underline"
                >
                  {s.list.name}
                </Link>
                <StateTag state={s.state} />
              </span>
              <span className="flex flex-wrap gap-x-2 text-caption text-faint">
                <span className="min-w-0 truncate" dir="ltr">
                  {s.email}
                </span>
                <span aria-hidden>·</span>
                <time dateTime={s.updated_at}>{formatDate(s.updated_at, i18n.locale)}</time>
              </span>
            </li>
          ))}
        </ul>
      )}
      {canManage && items.length > 0 && (
        <>
          <p className="text-caption text-faint">
            <Trans>Their consent records, for a request about what you hold on them:</Trans>
          </p>
          <ExportButtons
            testId="contact-consent"
            run={(format) =>
              saveDownload(
                api.GET("/v1/contacts/{contactId}/consent", {
                  params: { path: { contactId }, query: { format } },
                  parseAs: "blob",
                }),
                `consent-${contactId}.${format}`
              )
            }
          />
        </>
      )}
    </div>
  )
}
