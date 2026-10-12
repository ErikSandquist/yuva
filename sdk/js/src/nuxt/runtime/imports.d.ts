import type { PublicRuntimeConfig } from "@nuxt/schema"

export declare function useRuntimeConfig(): { public: Partial<PublicRuntimeConfig> }
export declare function useNuxtApp(): Record<string, unknown>
export declare function defineNuxtPlugin<T>(plugin: () => T): () => T
