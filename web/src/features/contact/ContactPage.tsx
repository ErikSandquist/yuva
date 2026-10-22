import { useLingui } from "@lingui/react/macro"
import { useParams } from "react-router"

import { ContactDetail } from "@/features/contact/ContactDetail"
import { ContactsPage } from "@/features/contact/ContactsPage"
import { Page } from "@/features/settings/ui"
import { useIsPhone } from "@/hooks/use-media-query"

export function ContactPage() {
  const { t } = useLingui()
  const { contactId = "" } = useParams()
  const phone = useIsPhone()
  if (!phone) return <ContactsPage key="contacts" contactId={contactId} />
  return (
    <Page testId="contact-page" className="pt-4">
      <ContactDetail key={contactId} id={contactId} page back={{ to: "/contacts", label: t`Contacts` }} />
    </Page>
  )
}
