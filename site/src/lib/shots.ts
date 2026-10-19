import type { ImageMetadata } from "astro"
import analyticsEnDark from "@/assets/shots/panel-analytics-en-dark.webp"
import analyticsEnLight from "@/assets/shots/panel-analytics-en-light.webp"
import analyticsTrDark from "@/assets/shots/panel-analytics-tr-dark.webp"
import analyticsTrLight from "@/assets/shots/panel-analytics-tr-light.webp"
import broadcastEnDark from "@/assets/shots/panel-broadcast-en-dark.webp"
import broadcastEnLight from "@/assets/shots/panel-broadcast-en-light.webp"
import broadcastTrDark from "@/assets/shots/panel-broadcast-tr-dark.webp"
import broadcastTrLight from "@/assets/shots/panel-broadcast-tr-light.webp"
import migrationEnDark from "@/assets/shots/panel-migration-en-dark.webp"
import migrationEnLight from "@/assets/shots/panel-migration-en-light.webp"
import migrationTrDark from "@/assets/shots/panel-migration-tr-dark.webp"
import migrationTrLight from "@/assets/shots/panel-migration-tr-light.webp"
import feedbackEnDark from "@/assets/shots/panel-feedback-en-dark.webp"
import feedbackEnLight from "@/assets/shots/panel-feedback-en-light.webp"
import feedbackTrDark from "@/assets/shots/panel-feedback-tr-dark.webp"
import feedbackTrLight from "@/assets/shots/panel-feedback-tr-light.webp"
import phoneThreadEnDark from "@/assets/shots/panel-phone-thread-en-dark.webp"
import phoneThreadEnLight from "@/assets/shots/panel-phone-thread-en-light.webp"
import phoneThreadTrDark from "@/assets/shots/panel-phone-thread-tr-dark.webp"
import phoneThreadTrLight from "@/assets/shots/panel-phone-thread-tr-light.webp"
import iosListEnDark from "@/assets/shots/ios-list-en-dark.webp"
import iosListEnLight from "@/assets/shots/ios-list-en-light.webp"
import iosListTrDark from "@/assets/shots/ios-list-tr-dark.webp"
import iosListTrLight from "@/assets/shots/ios-list-tr-light.webp"
import iosThreadEnDark from "@/assets/shots/ios-thread-en-dark.webp"
import iosThreadEnLight from "@/assets/shots/ios-thread-en-light.webp"
import iosThreadTrDark from "@/assets/shots/ios-thread-tr-dark.webp"
import iosThreadTrLight from "@/assets/shots/ios-thread-tr-light.webp"
import androidListEnDark from "@/assets/shots/android-list-en-dark.webp"
import androidListEnLight from "@/assets/shots/android-list-en-light.webp"
import androidListTrDark from "@/assets/shots/android-list-tr-dark.webp"
import androidListTrLight from "@/assets/shots/android-list-tr-light.webp"
import androidThreadEnDark from "@/assets/shots/android-thread-en-dark.webp"
import androidThreadEnLight from "@/assets/shots/android-thread-en-light.webp"
import androidThreadTrDark from "@/assets/shots/android-thread-tr-dark.webp"
import androidThreadTrLight from "@/assets/shots/android-thread-tr-light.webp"
import phoneEnDark from "@/assets/shots/panel-phone-list-en-dark.webp"
import phoneEnLight from "@/assets/shots/panel-phone-list-en-light.webp"
import phoneTrDark from "@/assets/shots/panel-phone-list-tr-dark.webp"
import phoneTrLight from "@/assets/shots/panel-phone-list-tr-light.webp"
import threadEnDark from "@/assets/shots/panel-thread-en-dark.webp"
import threadEnLight from "@/assets/shots/panel-thread-en-light.webp"
import threadTrDark from "@/assets/shots/panel-thread-tr-dark.webp"
import threadTrLight from "@/assets/shots/panel-thread-tr-light.webp"
import widgetEnDark from "@/assets/shots/widget-en-dark.webp"
import widgetEnLight from "@/assets/shots/widget-en-light.webp"
import widgetTrDark from "@/assets/shots/widget-tr-dark.webp"
import widgetTrLight from "@/assets/shots/widget-tr-light.webp"
import type { Locale } from "./routes"

type Pair = { light: ImageMetadata; dark: ImageMetadata }
export type Shots = {
  analytics: Pair
  broadcast: Pair
  migration: Pair
  thread: Pair
  phoneThread: Pair
  feedback: Pair
  phone: Pair
  widget: Pair
  ios: { list: Pair; thread: Pair }
  android: { list: Pair; thread: Pair }
}

const en: Shots = {
  analytics: { light: analyticsEnLight, dark: analyticsEnDark },
  broadcast: { light: broadcastEnLight, dark: broadcastEnDark },
  migration: { light: migrationEnLight, dark: migrationEnDark },
  thread: { light: threadEnLight, dark: threadEnDark },
  phoneThread: { light: phoneThreadEnLight, dark: phoneThreadEnDark },
  feedback: { light: feedbackEnLight, dark: feedbackEnDark },
  phone: { light: phoneEnLight, dark: phoneEnDark },
  widget: { light: widgetEnLight, dark: widgetEnDark },
  ios: {
    list: { light: iosListEnLight, dark: iosListEnDark },
    thread: { light: iosThreadEnLight, dark: iosThreadEnDark },
  },
  android: {
    list: { light: androidListEnLight, dark: androidListEnDark },
    thread: { light: androidThreadEnLight, dark: androidThreadEnDark },
  },
}

const byLocale: Partial<Record<Locale, Shots>> = {
  tr: {
    analytics: { light: analyticsTrLight, dark: analyticsTrDark },
    broadcast: { light: broadcastTrLight, dark: broadcastTrDark },
    migration: { light: migrationTrLight, dark: migrationTrDark },
    thread: { light: threadTrLight, dark: threadTrDark },
    phoneThread: { light: phoneThreadTrLight, dark: phoneThreadTrDark },
    feedback: { light: feedbackTrLight, dark: feedbackTrDark },
    phone: { light: phoneTrLight, dark: phoneTrDark },
    widget: { light: widgetTrLight, dark: widgetTrDark },
    ios: {
      list: { light: iosListTrLight, dark: iosListTrDark },
      thread: { light: iosThreadTrLight, dark: iosThreadTrDark },
    },
    android: {
      list: { light: androidListTrLight, dark: androidListTrDark },
      thread: { light: androidThreadTrLight, dark: androidThreadTrDark },
    },
  },
}

export function shots(locale: Locale): Shots {
  return byLocale[locale] ?? en
}
