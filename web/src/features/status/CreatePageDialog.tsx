import { Trans, useLingui } from "@lingui/react/macro"
import { LoaderIcon } from "lucide-react"
import { useState } from "react"

import { ErrorLine } from "@/components/common"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { useCreateStatusPage } from "@/features/status/queries"
import { Field } from "@/features/settings/ui"
import type { StatusPage } from "@/lib/api"

export function slugify(name: string) {
  return name
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/ı/g, "i")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48)
}

export const SLUG_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/

export function validSlug(slug: string) {
  return slug.length >= 3 && slug.length <= 48 && SLUG_RE.test(slug)
}

export function CreatePageDialog({
  inboxId,
  inboxName,
  onClose,
  onCreated,
}: {
  inboxId: string
  inboxName: string
  onClose: () => void
  onCreated: (p: StatusPage) => void
}) {
  const { t } = useLingui()
  const [name, setName] = useState(inboxName)
  const [slug, setSlug] = useState(slugify(inboxName))
  const [edited, setEdited] = useState(false)
  const create = useCreateStatusPage()
  const ok = name.trim() && validSlug(slug)
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="new-status-page-dialog">
        <form
          className="flex flex-col gap-5"
          onSubmit={(e) => {
            e.preventDefault()
            if (ok) create.mutate({ inbox_id: inboxId, name: name.trim(), slug }, { onSuccess: onCreated })
          }}
        >
          <DialogHeader>
            <DialogTitle>
              <Trans>Create status page</Trans>
            </DialogTitle>
            <DialogDescription>
              <Trans>
                A public page that tells your customers whether {inboxName} works. It stays unpublished until you
                publish it.
              </Trans>
            </DialogDescription>
          </DialogHeader>
          <Field label={<Trans>Name</Trans>} htmlFor="status-page-name">
            <Input
              id="status-page-name"
              value={name}
              maxLength={100}
              autoFocus
              required
              placeholder={t`Acme status`}
              onChange={(e) => {
                setName(e.target.value)
                if (!edited) setSlug(slugify(e.target.value))
              }}
              data-testid="new-status-page-name"
            />
          </Field>
          <Field
            label={<Trans>Address</Trans>}
            htmlFor="status-page-slug"
            hint={<Trans>3 to 48 letters, numbers and hyphens. It is unique on this server.</Trans>}
          >
            <Input
              id="status-page-slug"
              value={slug}
              maxLength={48}
              required
              spellCheck={false}
              autoCapitalize="none"
              onChange={(e) => {
                setEdited(true)
                setSlug(e.target.value.toLowerCase())
              }}
              data-testid="new-status-page-slug"
            />
          </Field>
          <ErrorLine error={create.error} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              <Trans>Cancel</Trans>
            </Button>
            <Button type="submit" disabled={!ok || create.isPending} data-testid="create-status-page">
              {create.isPending && <LoaderIcon className="animate-spin" />}
              <Trans>Create status page</Trans>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
