import type { I18n } from "@lingui/core"
import { msg } from "@lingui/core/macro"

export function cloudflareEmailCopy(i18n: I18n) {
  return {
    meta: {
      title: i18n._(msg`Support e-mail through Cloudflare Email Routing | Yuva`),
      description: i18n._(
        msg`Receive your support e-mail in Yuva with Cloudflare Email Routing and the open-source Yuva Email Worker. Signed delivery, threaded conversations, no mail server to run.`
      ),
    },
    kicker: i18n._(msg`Yuva Email Worker for Cloudflare`),
    title: i18n._(msg`Support e-mail through Cloudflare.`),
    titleAccent: i18n._(msg`No mail server to run.`),
    lead: i18n._(
      msg`Cloudflare® Email Routing receives the mail for your support addresses, and the open-source Yuva Email Worker hands each message to Yuva, signed. It lands in your shared inbox as a conversation your team answers.`
    ),
    guide: i18n._(msg`Read the setup guide`),
    source: i18n._(msg`Worker source on GitHub`),
    video: {
      label: i18n._(msg`Film: support e-mail from Cloudflare to the Yuva inbox`),
    },
    whatTitle: i18n._(msg`What it is`),
    what: [
      {
        title: i18n._(msg`Email Routing in front`),
        body: i18n._(
          msg`Cloudflare Email Routing becomes the MX of your domain and receives the mail. Yuva does not listen for SMTP, so there is no mail server to install or keep patched.`
        ),
      },
      {
        title: i18n._(msg`A small open-source Worker`),
        body: i18n._(
          msg`The Yuva Email Worker reads the raw message and the envelope, signs the request and posts it to Yuva. Parsing, threading and spam decisions stay on the server.`
        ),
      },
      {
        title: i18n._(msg`Real conversations`),
        body: i18n._(
          msg`Mail to a support address opens or continues a conversation in the inbox, and the team answers from the same panel as chat and app messages.`
        ),
      },
    ],
    howTitle: i18n._(msg`How a message gets from the customer to your team`),
    how: [
      i18n._(msg`A customer writes to one of your support addresses, for example support@example.com.`),
      i18n._(msg`Cloudflare Email Routing receives the message and passes it to the Yuva Email Worker.`),
      i18n._(
        msg`The Worker signs the raw message, both envelope addresses and a timestamp with HMAC-SHA256 under a secret you share with Yuva, and sends them to /ingress/email on your Yuva server.`
      ),
      i18n._(
        msg`Yuva checks the signature and that the timestamp is within 5 minutes before it reads the message. A request with a wrong signature is refused.`
      ),
      i18n._(
        msg`Yuva finds the e-mail channel for the recipient address and threads the message by In-Reply-To and References into the conversation it belongs to. Anything else starts a new conversation. A reply reopens the conversation.`
      ),
      i18n._(
        msg`Your team answers in the panel. The reply is sent through the e-mail channel's own SMTP account, such as Amazon SES, Postmark or your own relay, from the same address and in the same thread. Replies do not go through the Worker.`
      ),
    ],
    refusedTitle: i18n._(msg`Refusals reach the sender`),
    refused: i18n._(
      msg`When Yuva refuses a message for good, for example an unknown address, a blocked sender or a message that is too large, the Worker rejects it and the sender gets a permanent SMTP error with Yuva's reason.`
    ),
    fallbackTitle: i18n._(msg`If Yuva is unreachable`),
    fallback: i18n._(
      msg`If Yuva answers with a server error, does not answer within 20 seconds or cannot be reached, the Worker can forward the message to an address you choose, as long as you set FALLBACK_FORWARD. That address must be a verified destination in Email Routing. The message then arrives as a plain forward and does not appear in Yuva. Without FALLBACK_FORWARD the delivery fails with an error and the sending server may retry later.`
    ),
    setupTitle: i18n._(msg`Set it up in a few steps`),
    setupLead: i18n._(msg`You need a Yuva server and a Cloudflare account that manages the domain.`),
    setup: [
      {
        text: i18n._(
          msg`In Yuva, open Settings → your inbox → Channels → E-mail and add the support address. Set a secret on the server and restart it:`
        ),
        code: "YUVA_INGRESS_SECRET=<openssl rand -hex 32>\nYUVA_INGRESS_AUTHSERV_ID=mx.cloudflare.net",
      },
      {
        text: i18n._(msg`Deploy the Worker to your Cloudflare account, using the same secret:`),
        code: "cd edge\nbun install\nbunx wrangler login\nbunx wrangler secret put INGRESS_SECRET\nbunx wrangler deploy --var YUVA_URL:https://support.example.com",
      },
      {
        text: i18n._(
          msg`In the Cloudflare dashboard, open your domain → Email → Email Routing and enable it. Cloudflare adds its MX and SPF records.`
        ),
      },
      {
        text: i18n._(
          msg`Under Routing rules, send each support address to the Worker yuva-edge. For a catch-all channel, point the catch-all rule at it.`
        ),
      },
      { text: i18n._(msg`Send a test message and watch the conversation appear in the panel.`) },
      {
        text: i18n._(msg`Optional: keep mail during an outage by adding a fallback address to the deploy command:`),
        code: "bunx wrangler deploy --var YUVA_URL:https://support.example.com --var FALLBACK_FORWARD:you@example.org",
      },
    ],
    setupNote: i18n._(
      msg`Email Routing becomes the MX of the domain, so use a domain or subdomain that no other mail server handles.`
    ),
    openTitle: i18n._(msg`Not tied to Cloudflare`),
    open: i18n._(
      msg`The Worker is one way to call /ingress/email. Any mail server or relay can post the raw message with the same signed headers, and a mail server that delivers to a command can pipe the message into yuva ingest-email. The request format is documented in the Worker's README.`
    ),
    contract: i18n._(msg`Read the ingress contract`),
    mta: i18n._(msg`Other ways to bring mail in`),
    faqTitle: i18n._(msg`Questions and answers`),
    faq: [
      {
        q: i18n._(msg`Do I need to run a mail server?`),
        a: i18n._(
          msg`Not for incoming mail. Yuva does not listen for SMTP; Cloudflare Email Routing receives the mail and the Worker passes it on. Replies need an SMTP account for the e-mail channel, such as Amazon SES, Postmark or your own relay.`
        ),
      },
      {
        q: i18n._(msg`What happens when Yuva is down?`),
        a: i18n._(
          msg`With FALLBACK_FORWARD set, the message is forwarded to that address instead. Without it the delivery fails and the sending server may retry. Messages that Yuva refuses on purpose, such as mail to an unknown address, are rejected, not forwarded.`
        ),
      },
      {
        q: i18n._(msg`Which domains and addresses can I use?`),
        a: i18n._(
          msg`Any domain you manage in Cloudflare and whose mail no other server handles, because Email Routing becomes its MX. Each e-mail channel has an address, or the catch-all *@example.com for every address of the domain that no channel has exactly. A subaddress such as support+billing@example.com reaches the channel of support@example.com.`
        ),
      },
      {
        q: i18n._(msg`How big can a message be?`),
        a: i18n._(
          msg`A message can be up to 25 MiB including attachments, which is the Cloudflare Email Routing limit. Yuva keeps up to 50 attachments per message, each within the server's attachment size limit (25 MiB by default) and allowed types; it leaves out the rest.`
        ),
      },
      {
        q: i18n._(msg`Can someone forge mail to my Yuva server?`),
        a: i18n._(
          msg`Yuva refuses any request without a valid HMAC signature or with a timestamp more than 5 minutes off, before it reads the body. Keep the secret to yourself.`
        ),
      },
      {
        q: i18n._(msg`Does Yuva use the DMARC result from Cloudflare?`),
        a: i18n._(
          msg`Yes, when you set YUVA_INGRESS_AUTHSERV_ID to mx.cloudflare.net. Yuva then reads the DMARC verdict that Email Routing adds. A new conversation that fails DMARC is marked as spam.`
        ),
      },
    ],
    trademark: i18n._(
      msg`Cloudflare, the Cloudflare logo and Cloudflare Workers are trademarks and/or registered trademarks of Cloudflare, Inc. in the United States and other jurisdictions. Yuva is an independent open-source project, not affiliated with or endorsed by Cloudflare.`
    ),
  }
}
