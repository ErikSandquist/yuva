import { msg } from "@lingui/core/macro"
import { ApiError, type ClientMailingList } from "../client/api"
import { el } from "../panel/helpers"
import { icons } from "../panel/icons"
import type { AppContext } from "../types"
import type { Translate } from "./apps"

const tokenMinimumAge = 2100

export function subscribeApp(context: AppContext, t: Translate, root: HTMLElement): () => void {
  const { client, locale } = context
  let lists: ClientMailingList[] = []
  let token = ""
  let tokenAt = 0
  let loaded = false
  let failed = false
  let phase: "idle" | "sending" | "done" = "idle"
  let confirmation = false
  let error = ""
  let emailError = ""
  let consentError = ""
  let destroyed = false
  const chosen = new Set<string>()

  const email = el("input", "field")
  email.type = "email"
  email.autocomplete = "email"
  email.value = client.session?.contact.email ?? ""
  email.placeholder = t(msg`you@example.com`)
  email.setAttribute("aria-label", t(msg`E-mail address`))
  const consent = el("input")
  consent.type = "checkbox"
  consent.id = "yuva-consent"
  consent.addEventListener("change", () => {
    consentError = ""
    render()
  })

  function selected(): ClientMailingList[] {
    return lists.filter((list) => chosen.has(list.id))
  }

  function consentText(items: ClientMailingList[]): string {
    const list = new Intl.ListFormat(locale, { type: "conjunction" }).format(items.map((item) => item.name))
    const inbox = context.inbox.name
    return t(msg`I want to receive e-mails about ${list} from ${inbox}. I can unsubscribe at any time.`)
  }

  async function load(): Promise<void> {
    try {
      const result = await client.lists()
      lists = result.lists
      token = result.form_token
      tokenAt = Date.now()
      for (const list of lists) chosen.add(list.id)
    } catch {
      failed = true
    }
    loaded = true
    render()
  }

  async function submit(): Promise<void> {
    if (phase !== "idle") return
    const items = selected()
    const address = email.value.trim()
    error = items.length === 0 ? t(msg`Choose at least one list.`) : ""
    emailError = email.checkValidity() && address ? "" : t(msg`Enter a valid e-mail address.`)
    consentError = consent.checked ? "" : t(msg`Tick the box to agree.`)
    if (error || emailError || consentError) {
      render()
      return
    }
    phase = "sending"
    render()
    const send = async (retry: boolean): Promise<void> => {
      const wait = tokenAt + tokenMinimumAge - Date.now()
      if (wait > 0) await new Promise((resolve) => setTimeout(resolve, wait))
      try {
        const result = await client.subscribe({
          email: address,
          list_ids: items.map((item) => item.id),
          locale,
          page_url: location.href.split("#")[0]!.slice(0, 2000),
          consent_text: consentText(items),
          form_token: token,
        })
        confirmation = result.confirmation
      } catch (failure) {
        if (retry && failure instanceof ApiError && failure.code === "form_token") {
          const fresh = await client.lists()
          token = fresh.form_token
          tokenAt = Date.now()
          return send(false)
        }
        throw failure
      }
    }
    try {
      await send(true)
      phase = "done"
    } catch (failure) {
      phase = "idle"
      if (failure instanceof ApiError && failure.code === "invalid_email")
        emailError = t(msg`Enter a valid e-mail address.`)
      else
        error =
          failure instanceof ApiError && failure.code === "rate_limited"
            ? t(msg`Too many attempts. Wait a moment and try again.`)
            : failure instanceof ApiError && failure.code === "network"
              ? t(msg`You seem to be offline.`)
              : t(msg`Could not subscribe. Try again.`)
    }
    if (!destroyed) render()
  }

  function render(): void {
    if (destroyed) return
    const sec = el("section", "sec")
    sec.append(el("h2", "label", `01 — ${t(msg`Get updates by e-mail`)}`))
    if (!loaded) sec.append(el("p", "hint", t(msg`Loading…`)))
    else if (failed || lists.length === 0) sec.append(el("p", "hint", t(msg`Could not subscribe. Try again.`)))
    else if (phase === "done") {
      const card = el("div", "done-card")
      card.insertAdjacentHTML("beforeend", icons.check)
      card.append(
        el(
          "p",
          "empty",
          confirmation
            ? t(msg`Almost done. Check your inbox and confirm your subscription.`)
            : t(msg`Thanks! You are subscribed.`)
        )
      )
      sec.append(card)
    } else {
      const form = el("form", "form")
      form.noValidate = true
      form.addEventListener("submit", (event) => {
        event.preventDefault()
        void submit()
      })
      const row = el("div", "row2")
      const button = el("button", "btn", phase === "sending" ? t(msg`Subscribing…`) : t(msg`Subscribe`))
      button.type = "submit"
      button.disabled = phase === "sending"
      row.append(email, button)
      form.append(row)
      if (emailError) form.append(el("p", "error", emailError))
      if (lists.length > 1) {
        const group = el("div", "lists")
        group.setAttribute("role", "group")
        group.setAttribute("aria-label", t(msg`Lists`))
        for (const list of lists) {
          const label = el("label", "check")
          const box = el("input")
          box.type = "checkbox"
          box.checked = chosen.has(list.id)
          box.addEventListener("change", () => {
            if (box.checked) chosen.add(list.id)
            else chosen.delete(list.id)
            error = ""
            render()
          })
          const text = el("span")
          text.append(el("b", "", list.name))
          if (list.description) text.append(list.description)
          label.append(box, text)
          group.append(label)
        }
        form.append(group)
      }
      const agree = el("label", "check")
      agree.htmlFor = consent.id
      agree.append(consent, el("span", "", consentText(selected().length > 0 ? selected() : lists)))
      form.append(agree)
      if (consentError) form.append(el("p", "error", consentError))
      if (error) form.append(el("p", "error", error))
      sec.append(form)
    }
    root.replaceChildren(sec)
  }

  render()
  void load()
  return () => {
    destroyed = true
  }
}
