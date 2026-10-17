import type { APIRoute } from "astro"
import { searchIndex } from "@/lib/search"
import { localePaths, type Locale } from "@/lib/routes"

export const getStaticPaths = localePaths

export const GET: APIRoute = ({ params }) => searchIndex(params.locale as Locale)
