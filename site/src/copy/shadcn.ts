import type { I18n } from "@lingui/core"
import { msg } from "@lingui/core/macro"

export function shadcnCopy(i18n: I18n) {
  return {
    title: i18n._(msg`Yuva for shadcn/ui`),
    suffix: i18n._(msg`Yuva for shadcn/ui`),
    groups: {
      start: i18n._(msg`Get started`),
      blocks: i18n._(msg`Blocks`),
      components: i18n._(msg`Components`),
      hooks: i18n._(msg`Hooks`),
    },
    preview: i18n._(msg`Preview`),
    code: i18n._(msg`Code`),
    previewNote: i18n._(msg`The preview runs on sample data. It needs no network and no channel.`),
    installation: i18n._(msg`Installation`),
    installLead: i18n._(msg`Run this in a shadcn project. The files are copied into your own code.`),
    packageManager: i18n._(msg`Package manager`),
    usage: i18n._(msg`Usage`),
    props: i18n._(msg`Props`),
    prop: i18n._(msg`Prop`),
    type: i18n._(msg`Type`),
    labels: i18n._(msg`Labels`),
    labelsLead: i18n._(
      msg`Every label has an English default. Pass a partial labels object to change or translate them.`
    ),
    labelsOn: (title: string) => i18n._(msg`The labels are listed on ${title}.`),
    label: i18n._(msg`Label`),
    default: i18n._(msg`Default`),
    dependencies: i18n._(msg`Dependencies`),
    dependenciesLead: i18n._(msg`Installing this item also adds the following.`),
    shadcnComponents: i18n._(msg`Components from shadcn/ui`),
    yuvaItems: i18n._(msg`Other Yuva items`),
    packages: i18n._(msg`npm packages`),
    intro: {
      description: i18n._(
        msg`React components for live chat, documentation page discussions and mailing list sign-up, copied into your shadcn/ui project.`
      ),
      lead: i18n._(
        msg`The Yuva registry is a set of components and hooks that the shadcn CLI copies into your project. They use your theme and talk to Yuva through the useyuva client, and they are yours to edit.`
      ),
      requirements: i18n._(msg`Requirements`),
      requireProject: i18n._(msg`A project that already uses shadcn/ui, with React and Tailwind CSS.`),
      requireChannel: i18n._(msg`A Yuva chat channel and its public key (yuva_pk_…).`),
      channel: i18n._(msg`Set up a channel`),
      step1: i18n._(msg`In the Yuva panel, open Settings, your inbox, Channels, and add a chat channel.`),
      step2: i18n._(
        msg`Add every origin that will show the components as an allowed origin: your site and your development server.`
      ),
      step3: i18n._(msg`Copy the channel's public key. It is not a secret.`),
      step4: i18n._(
        msg`For discussions, set the channel's discussions mode to moderated or open. For the subscribe form, add a public mailing list.`
      ),
      env: i18n._(
        msg`Give the key and the address of your Yuva server to the components, for example in the environment:`
      ),
      first: i18n._(msg`Add your first item`),
      firstLead: i18n._(msg`Install the chat block, then render it:`),
      items: i18n._(msg`Items`),
      itemsLead: i18n._(msg`Every item in the registry, grouped like the sidebar.`),
    },
  }
}
