import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, toast, useConfirm } from "@/components/common"
import { ExportButtons } from "@/components/common/ExportButtons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ListFields, listValues, type ListValues } from "@/features/broadcasts/ListForm"
import { useDeleteList, useSetList } from "@/features/broadcasts/queries"
import { Card, FormActions, FormCard, Section } from "@/features/settings/ui"
import { api, unwrap, type MailingList, type MailingListUpdate } from "@/lib/api"
import { saveDownload } from "@/lib/download"
import { useSession } from "@/lib/session"

export function ListSettings({ list }: { list: MailingList }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const setList = useSetList()
  const [values, setValues] = useState<ListValues>(() => listValues(list))
  const [confirm, confirmDialog] = useConfirm()
  const save = useMutation({
    mutationFn: () => {
      const body: MailingListUpdate = {
        name: values.name.trim(),
        description: values.description.trim(),
        double_opt_in: values.double_opt_in,
        public: values.public,
        commercial: values.commercial,
        welcome: values.welcome,
        channel_id: values.channel_id || null,
      }
      return unwrap(api.PATCH("/v1/lists/{listId}", { params: { path: { listId: list.id } }, body }))
    },
    onSuccess: (l) => {
      setList(l)
      setValues(listValues(l))
      toast(t`Saved`)
    },
  })
  const archive = useMutation({
    mutationFn: (archived: boolean) =>
      unwrap(api.PATCH("/v1/lists/{listId}", { params: { path: { listId: list.id } }, body: { archived } })),
    onSuccess: (l) => {
      setList(l)
      toast(l.archived_at ? t`List archived` : t`List restored`)
    },
  })
  const remove = useDeleteList()
  const people = list.counts.subscribed + list.counts.pending + list.counts.unsubscribed
  return (
    <>
      <Section title={<Trans>Settings</Trans>}>
        <FormCard
          onSubmit={() => values.name.trim() && save.mutate()}
          footer={canManage && <FormActions pending={save.isPending} error={save.error} />}
        >
          <fieldset disabled={!canManage} className="flex min-w-0 flex-col gap-5" data-testid="list-settings">
            <ListFields
              values={values}
              onChange={(patch) => setValues((v) => ({ ...v, ...patch }))}
              error={save.error}
            />
          </fieldset>
        </FormCard>
      </Section>
      {canManage && (
        <Section
          title={<Trans>Archive</Trans>}
          description={
            <Trans>
              An archived list keeps its subscribers and records, but forms and the preference page stop offering it.
            </Trans>
          }
        >
          <Card>
            <div className="flex flex-wrap items-center gap-3">
              <p className="min-w-0 flex-1 text-body">
                {list.archived_at ? <Trans>This list is archived.</Trans> : <Trans>Stop taking new subscribers.</Trans>}
              </p>
              <Button
                variant="outline"
                size="sm"
                disabled={archive.isPending}
                onClick={() => archive.mutate(!list.archived_at)}
                data-testid="list-archive"
              >
                {list.archived_at ? <Trans>Restore</Trans> : <Trans>Archive</Trans>}
              </Button>
            </div>
            <ErrorLine error={archive.error} />
          </Card>
        </Section>
      )}
      {canManage && (
        <Section title={<Trans>Delete</Trans>}>
          <Card tone="danger">
            <div className="flex flex-wrap items-center gap-3">
              <p className="min-w-0 flex-1 text-body">
                <Trans>
                  Deletes the list, its subscriptions and their consent records. Download the consent export first if
                  you may need to show it later.
                </Trans>
              </p>
              <Button
                variant="destructive"
                size="sm"
                disabled={remove.isPending}
                onClick={() =>
                  confirm({
                    title: <Trans>Delete {list.name}?</Trans>,
                    description: (
                      <>
                        <Plural
                          value={people}
                          one="Its # subscription and its consent record are deleted."
                          other="Its # subscriptions and their consent records are deleted."
                        />{" "}
                        <Trans>
                          People who had unsubscribed are remembered as opted out, so nobody on the team can add them
                          again.
                        </Trans>
                      </>
                    ),
                    confirm: <Trans>Delete list</Trans>,
                    run: () =>
                      remove.mutate(list.id, {
                        onSuccess: () => {
                          toast(t`List deleted`)
                          navigate("/broadcasts")
                        },
                      }),
                  })
                }
                data-testid="list-delete"
              >
                <Trans>Delete list</Trans>
              </Button>
            </div>
            <ErrorLine error={remove.error} />
          </Card>
        </Section>
      )}
      {confirmDialog}
    </>
  )
}

export function ListExport({ list }: { list: MailingList }) {
  const [since, setSince] = useState("")
  return (
    <Section
      title={<Trans>Consent export</Trans>}
      description={
        <Trans>
          One row per event, oldest first: who agreed or left, when, how, from which page and IP address, and the words
          they saw. Keep it as proof of consent (GDPR Art. 7, KVKK), to answer a person who asks what you hold, or to
          register consents with İYS yourself. Spreadsheet formulas in the CSV are made harmless.
        </Trans>
      }
    >
      <Card>
        <div className="flex flex-col gap-2">
          <label htmlFor="export-since" className="text-body font-medium">
            <Trans>From (optional)</Trans>
          </label>
          <Input
            id="export-since"
            type="date"
            value={since}
            onChange={(e) => setSince(e.target.value)}
            className="sm:max-w-48"
          />
        </div>
        <ExportButtons
          testId="list-export"
          run={(format) =>
            saveDownload(
              api.GET("/v1/lists/{listId}/consent-export", {
                params: {
                  path: { listId: list.id },
                  query: { format, since: since ? new Date(`${since}T00:00:00`).toISOString() : undefined },
                },
                parseAs: "blob",
              }),
              `consent-${list.name}.${format}`
            )
          }
        />
      </Card>
    </Section>
  )
}
