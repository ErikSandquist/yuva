import type { I18n } from "@lingui/core"
import { msg } from "@lingui/core/macro"

export function templatesCopy(i18n: I18n) {
  const categories: Record<string, string> = {
    newsletter: i18n._(msg`Newsletter`),
    product: i18n._(msg`Product`),
    announcement: i18n._(msg`Announcement`),
    onboarding: i18n._(msg`Onboarding`),
    event: i18n._(msg`Event`),
    digest: i18n._(msg`Digest`),
  }
  return {
    navLabel: i18n._(msg`Templates`),
    metaTitle: i18n._(msg`E-mail templates`),
    title: i18n._(msg`E-mail templates`),
    kicker: i18n._(msg`Template gallery`),
    description: i18n._(
      msg`Free starter designs for newsletters, product updates, announcements and more. Preview them here and open one in Yuva's e-mail designer.`
    ),
    lead: i18n._(
      msg`Starter designs for the e-mail designer, in English, Turkish and German. Preview one, then open it in Yuva and make it yours.`
    ),
    openCard: i18n._(msg`Preview and use`),
    category: (key: string) => categories[key] ?? key,
    all: i18n._(msg`All templates`),
    previewOf: (name: string) => i18n._(msg`Preview of the ${name} template`),
    pageDescription: (name: string, description: string) =>
      i18n._(msg`${name}: ${description} A free Yuva e-mail template.`),
    live: i18n._(msg`Live preview`),
    width: i18n._(msg`Preview width`),
    desktop: i18n._(msg`Desktop`),
    phone: i18n._(msg`Phone`),
    scheme: i18n._(msg`Preview colours`),
    light: i18n._(msg`Light`),
    dark: i18n._(msg`Dark`),
    darkUnsupported: i18n._(
      msg`This browser does not switch the preview to dark mode. Use your system's dark setting to see it, or open the template to check it in the designer.`
    ),
    frameTitle: (name: string) => i18n._(msg`${name} e-mail preview`),
    use: i18n._(msg`Use this template`),
    useBody: i18n._(msg`Opens Yuva's e-mail designer with this template. You sign in first if you have not.`),
    ownServer: i18n._(msg`On your own server`),
    ownServerBody: i18n._(
      msg`Enter the address of your Yuva server to open the template there. It is kept in this browser only.`
    ),
    serverLabel: i18n._(msg`Server address`),
    serverPlaceholder: i18n._(msg`https://yuva.example.com`),
    serverOpen: i18n._(msg`Open on my server`),
    serverInvalid: i18n._(msg`Enter a full address starting with http:// or https://.`),
    licenseTitle: i18n._(msg`License and contributions`),
    license: i18n._(msg`These templates are MIT licensed: use, change and share them in your own mails.`),
    licenseLink: i18n._(msg`Read the license`),
    add: i18n._(
      msg`Want to add a template? Send a pull request; the guide explains the folder, the rules and the preview.`
    ),
    addLink: i18n._(msg`How to add a template`),
    back: i18n._(msg`All templates`),
    by: i18n._(msg`By`),
  }
}
