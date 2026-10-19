import { Plural, Trans, useLingui } from "@lingui/react/macro"
import { useMutation } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate } from "react-router"

import { ErrorLine, toast, useConfirm } from "@/components/common"
import { ExportButtons } from "@/components/common/ExportButtons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  canSend,
  nameTaken,
  NameFields,
  RulesToggles,
  SenderMissing,
  SenderPicker,
} from "@/features/broadcasts/ListForm"
import { useDeleteList, useInboxChannels, useSetList } from "@/features/broadcasts/queries"
import { PageSection } from "@/features/settings/channels/sections"
import { Card, FormActions, FormCard, Section } from "@/features/settings/ui"
import { api, unwrap, type MailingList, type MailingListUpdate } from "@/lib/api"
import { saveDownload } from "@/lib/download"
import { useSession } from "@/lib/session"

function usePatchList(list: MailingList) {
  const setList = useSetList()
  return useMutation({
    mutationFn: (body: MailingListUpdate) =>
      unwrap(api.PATCH("/v1/lists/{listId}", { params: { path: { listId: list.id } }, body })),
    onSuccess: setList,
  })
}

function NameSection({ list }: { list: MailingList }) {
  const patch = usePatchList(list)
  const [name, setName] = useState(list.name)
  const [description, setDescription] = useState(list.description)
  const dirty = name.trim() !== list.name || description.trim() !== list.description
  return (
    <PageSection id="general" title={<Trans>Name</Trans>}>
      <FormCard
        onSubmit={() =>
          name.trim() &&
          patch.mutate(
            { name: name.trim(), description: description.trim() },
            {
              onSuccess: (saved) => {
                setName(saved.name)
                setDescription(saved.description)
              },
            }
          )
        }
        footer={
          <FormActions
            pending={patch.isPending}
            saved={patch.isSuccess && !dirty}
            error={patch.error}
            disabled={!dirty || !name.trim()}
          />
        }
      >
        <NameFields
          name={name}
          description={description}
          taken={nameTaken(patch.error)}
          onChange={(p) => {
            patch.reset()
            if (p.name !== undefined) setName(p.name)
            if (p.description !== undefined) setDescription(p.description)
          }}
        />
      </FormCard>
    </PageSection>
  )
}

function SenderSection({ list }: { list: MailingList }) {
  const { t } = useLingui()
  const patch = usePatchList(list)
  const channels = useInboxChannels(list.inbox_id).data
  const sender = list.channel_id ? channels?.find((c) => c.id === list.channel_id) : undefined
  return (
    <PageSection id="sender" title={<Trans>Sender</Trans>}>
      <Card>
        <SenderPicker
          inboxId={list.inbox_id}
          value={list.channel_id ?? ""}
          onChange={(id) => patch.mutate({ channel_id: id || null }, { onSuccess: () => toast(t`Saved`) })}
          invalid={false}
        />
        {channels && !(sender && canSend(sender)) && <SenderMissing inboxId={list.inbox_id} />}
        <ErrorLine error={patch.error} className="text-caption" />
      </Card>
    </PageSection>
  )
}

function SignupSection({ list }: { list: MailingList }) {
  const { t } = useLingui()
  const patch = usePatchList(list)
  return (
    <PageSection id="signup" title={<Trans>Sign-up</Trans>}>
      <Card>
        <RulesToggles
          values={list}
          disabled={patch.isPending}
          onChange={(p) => patch.mutate(p, { onSuccess: () => toast(t`Saved`) })}
        />
        <ErrorLine error={patch.error} className="text-caption" />
      </Card>
    </PageSection>
  )
}

export function ListSettings({ list }: { list: MailingList }) {
  const { t } = useLingui()
  const navigate = useNavigate()
  const { canManage } = useSession()
  const setList = useSetList()
  const [confirm, confirmDialog] = useConfirm()
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
      <fieldset disabled={!canManage} className="contents" data-testid="list-settings">
        <NameSection list={list} />
        <SenderSection list={list} />
        <SignupSection list={list} />
      </fieldset>
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
                          navigate("/broadcasts/lists")
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
          formats={list.kind === "status" ? (["csv", "json"] as const) : (["csv", "json", "iys"] as const)}
          run={(format) =>
            saveDownload(
              api.GET("/v1/lists/{listId}/consent-export", {
                params: {
                  path: { listId: list.id },
                  query: { format, since: since ? new Date(`${since}T00:00:00`).toISOString() : undefined },
                },
                parseAs: "blob",
              }),
              format === "iys" ? `iys-${list.name}.csv` : `consent-${list.name}.${format}`
            )
          }
        />
        {list.kind !== "status" && (
          <p className="text-small text-muted-foreground">
            <Trans>
              The İYS file has one row per subscriber in İYS&apos;s columns, ONAY for subscribed and RET for
              unsubscribed, for Turkish senders to upload to İYS themselves. Pending subscriptions are left out.
            </Trans>
          </p>
        )}
      </Card>
    </Section>
  )
}
