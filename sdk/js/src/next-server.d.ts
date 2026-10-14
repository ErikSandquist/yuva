// The one Next.js API this package uses; next is an optional peer and not installed here.
declare module "next/server" {
  export function after<T>(task: Promise<T> | (() => T | Promise<T>)): void
}
