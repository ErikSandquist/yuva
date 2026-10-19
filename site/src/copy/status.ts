import type { I18n } from "@lingui/core"
import { msg } from "@lingui/core/macro"

export function statusSampleCopy(i18n: I18n) {
  return {
    alt: i18n._(
      msg`A sample status page: the overall state, an open incident and four components with their last 90 days as coloured bars.`
    ),
    state: i18n._(msg`Some systems are down`),
    line: i18n._(msg`The API is slow and some chats are delayed.`),
    incident: i18n._(msg`Incident`),
    identified: i18n._(msg`Identified`),
    incidentTitle: i18n._(msg`Chat messages arrive late`),
    components: i18n._(msg`Components`),
    days: i18n._(msg`90 days`),
    uptime: i18n._(msg`uptime`),
    ok: i18n._(msg`Operational`),
    degraded: i18n._(msg`Degraded performance`),
    partial: i18n._(msg`Partial outage`),
    website: i18n._(msg`Website`),
    email: i18n._(msg`E-mail`),
    chat: i18n._(msg`Live chat`),
  }
}

export type StatusSampleCopy = ReturnType<typeof statusSampleCopy>

export function statusCopy(i18n: I18n) {
  return {
    meta: {
      title: i18n._(msg`Status pages for your products | Yuva`),
      description: i18n._(
        msg`A public status page with components, incidents, maintenance and history, kept current by checks and by your team. E-mail subscribers, a line in the chat widget and the apps, and one reply to everyone who wrote.`
      ),
    },
    kicker: i18n._(msg`Status pages`),
    title: i18n._(msg`Tell customers what is down before they have to ask.`),
    lead: i18n._(
      msg`A Yuva status page shows whether your product works. It sits in your support inbox, so your chat widget and apps show an open incident, and one reply answers everyone who wrote about it.`
    ),
    guide: i18n._(msg`Read the guide`),
    signUp: i18n._(msg`Create an account`),
    sample: statusSampleCopy(i18n),

    page: {
      label: i18n._(msg`The public page`),
      lead: i18n._(
        msg`Yuva renders it as plain HTML, complete without JavaScript, and answers are cached for 30 seconds so a CDN in front of your server can carry an outage's traffic.`
      ),
      points: [
        {
          term: i18n._(msg`Overall state`),
          text: i18n._(msg`The worst state among the components you show, in words and in colour.`),
        },
        {
          term: i18n._(msg`Components`),
          text: i18n._(
            msg`In groups, each with its state, a bar for every day of up to 90 days and, if you like, its uptime.`
          ),
        },
        {
          term: i18n._(msg`Incidents`),
          text: i18n._(
            msg`From investigating to resolved, with every update kept. A published update is corrected, never removed.`
          ),
        },
        {
          term: i18n._(msg`Maintenance`),
          text: i18n._(msg`Announce planned work with a window. Yuva starts and completes it on time.`),
        },
        {
          term: i18n._(msg`History`),
          text: i18n._(msg`Past incidents and maintenance by month, each with its own page.`),
        },
        {
          term: i18n._(msg`Response times`),
          text: i18n._(
            msg`When you turn it on: the last 24 hours of each component with an HTTP or TCP check, with median and p95.`
          ),
        },
        {
          term: i18n._(msg`Feed, badge and JSON`),
          text: i18n._(
            msg`An Atom feed that needs no subscription, a badge for your README, and the same page as JSON for your own site or app.`
          ),
        },
        {
          term: i18n._(msg`Your look and language`),
          text: i18n._(
            msg`Your logo and accent colour. The page speaks English, Turkish and German; visitors see times in their own time zone.`
          ),
        },
      ],
      preview: i18n._(
        msg`A page stays private until you publish it, and your team previews it in the panel before that.`
      ),
    },

    checks: {
      label: i18n._(msg`Checks keep it current`),
      lead: i18n._(
        msg`A check sets a component's state without anyone's hand, and Yuva tells your team when it goes down or up.`
      ),
      points: [
        {
          term: i18n._(msg`Heartbeats`),
          text: i18n._(
            msg`Your cron job or server calls a URL on a schedule. When the calls stop, the component goes down.`
          ),
        },
        {
          term: i18n._(msg`Pushed states`),
          text: i18n._(
            msg`Send a working or an outage state yourself. Alertmanager and Grafana alerts are accepted as they are.`
          ),
        },
        {
          term: i18n._(msg`HTTP`),
          text: i18n._(
            msg`Yuva requests a URL on an interval and checks the status code, optionally a keyword and the certificate's expiry.`
          ),
        },
        {
          term: i18n._(msg`TCP`),
          text: i18n._(msg`Yuva opens a connection to a host and port and checks that it answers.`),
        },
      ],
      draftTitle: i18n._(msg`Incidents drafted for you`),
      draft: i18n._(
        msg`When a check goes down, Yuva can open a draft incident for you to review, publish one at once, or open none. When it recovers, Yuva resolves what it opened.`
      ),
      thresholdTitle: i18n._(msg`Your word first`),
      threshold: i18n._(
        msg`You choose how many failures in a row count, and a state your team set by hand is never overridden by a check.`
      ),
    },

    subscribers: {
      label: i18n._(msg`E-mail subscribers`),
      lead: i18n._(
        msg`Visitors subscribe to notices about incidents and maintenance, for every component or only the ones they use. A subscription always starts with a confirmation mail.`
      ),
      points: [
        {
          term: i18n._(msg`Double opt-in`),
          text: i18n._(msg`The answer is the same for every address, and nobody is subscribed without confirming.`),
        },
        {
          term: i18n._(msg`Their choice`),
          text: i18n._(msg`One checkbox per component, and a link in every mail to change it or leave.`),
        },
        {
          term: i18n._(msg`Plain notices`),
          text: i18n._(
            msg`The state in words, the affected components and the updates, with no images, tracking or promotion. In English, Turkish or German.`
          ),
        },
        {
          term: i18n._(msg`Held, then sent`),
          text: i18n._(
            msg`A notice waits 60 seconds so you can undo it, and notices about one incident keep their distance, so a long outage does not flood inboxes.`
          ),
        },
      ],
      channel: i18n._(
        msg`Notices go out through your inbox's own e-mail channel, from its address, and the people on the list are kept apart from your newsletters.`
      ),
    },

    apps: {
      label: i18n._(msg`In the chat widget and the apps`),
      lead: i18n._(
        msg`While your page is published and an incident or maintenance is open, the web widget and the iOS and Android SDKs show a line above the conversations, with the state in words and a link to the incident.`
      ),
      points: [
        {
          term: i18n._(msg`Nothing to set up`),
          text: i18n._(msg`The line appears and goes away with the incident.`),
        },
        {
          term: i18n._(msg`Dismissable`),
          text: i18n._(msg`A customer can dismiss it. It returns when the incident gets a new update.`),
        },
        {
          term: i18n._(msg`Their language`),
          text: i18n._(msg`The widget and the apps show it in each language they speak.`),
        },
      ],
      widget: {
        title: i18n._(msg`Messages`),
        line: i18n._(msg`Elevated API response times`),
        minor: i18n._(msg`Minor`),
        investigating: i18n._(msg`Investigating`),
        details: i18n._(msg`Details`),
        dismiss: i18n._(msg`Dismiss`),
        send: i18n._(msg`Send us a message`),
        alt: i18n._(
          msg`The chat widget with a line above the conversations: elevated API response times, minor, investigating, with a link to the details.`
        ),
      },
    },

    reply: {
      label: i18n._(msg`Reply to linked`),
      lead: i18n._(
        msg`An outage brings the same question by e-mail, chat and in the apps. Link those conversations to the incident, write one answer, and send it to all of them.`
      ),
      steps: [
        i18n._(msg`Link conversations to the incident in the panel. The incident page lists them.`),
        i18n._(
          msg`Write one reply. Review shows how many conversations get it, and you can close each one afterwards.`
        ),
        i18n._(
          msg`Yuva sends it as you to every linked conversation that is still open, through each one's own channel. It cannot be recalled once sent.`
        ),
      ],
    },

    domain: {
      label: i18n._(msg`Your own domain`),
      lead: i18n._(
        msg`Serve the page at an address such as status.example.com. Add a CNAME record and a TXT record that shows the domain is yours, and Yuva checks them for you.`
      ),
      points: [
        {
          term: i18n._(msg`Certificates`),
          text: i18n._(
            msg`Issued and renewed automatically, by an on-demand proxy such as Caddy or by Cloudflare custom hostnames.`
          ),
        },
        {
          term: i18n._(msg`Everything moves`),
          text: i18n._(msg`Links, the feed, the badge, the JSON and e-mail notices all use your domain.`),
        },
        {
          term: i18n._(msg`Only yours`),
          text: i18n._(msg`Only the page is served on your domain, never the API or the panel.`),
        },
      ],
    },

    band: {
      label: i18n._(msg`The band for your own site`),
      lead: i18n._(
        msg`Show your status in your site's footer: the state in words, a strip of the last 90 days with uptime, and a link to your page. It is one script and one element.`
      ),
      light: i18n._(msg`On a light site`),
      dark: i18n._(msg`On a dark site`),
      by: i18n._(msg`Status by Yuva`),
      pricing: i18n._(msg`Pricing`),
      docs: i18n._(msg`Docs`),
      privacy: i18n._(msg`Privacy`),
      title: i18n._(msg`The API is running slow`),
      line: i18n._(msg`Chat and e-mail work normally.`),
      attributes: [
        {
          name: "page",
          text: i18n._(msg`The address of your status page.`),
        },
        {
          name: "theme",
          text: i18n._(msg`light, dark or auto, which follows the visitor's colour scheme. The default is auto.`),
        },
        {
          name: "components",
          text: i18n._(msg`Add it for a second row with each component's state.`),
        },
      ],
      alt: i18n._(
        msg`A footer band: a coloured square and the state in words, a strip of 90 days with the uptime, and Status by Yuva.`
      ),
    },

    developers: {
      label: i18n._(msg`For developers`),
      lead: i18n._(
        msg`Everything is in the API, with the scopes status:read and status:manage. Webhooks announce component, incident and monitor changes, and assistants connected over MCP can open drafts and resolve incidents.`
      ),
      api: i18n._(msg`Open the API reference`),
    },

    close: {
      title: i18n._(msg`Start with the guide.`),
      lead: i18n._(msg`Make a page, add your components and publish it. The guide is in English, Turkish and German.`),
    },
  }
}

export type StatusCopy = ReturnType<typeof statusCopy>
