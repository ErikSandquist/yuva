export interface Doc {
  slug: string
  title: string
  description: string
  sections: { heading: string; body: string }[]
}

export const docs: Doc[] = [
  {
    slug: "getting-started",
    title: "Getting started",
    description: "Install the CLI and make your first request.",
    sections: [
      {
        heading: "Install",
        body: "Install the command line tool with your package manager, then sign in with the token from your account settings.",
      },
      {
        heading: "First request",
        body: "Every request carries the token in the Authorization header. A missing or expired token answers 401.",
      },
    ],
  },
  {
    slug: "configuration",
    title: "Configuration",
    description: "Options, environment variables and their defaults.",
    sections: [
      {
        heading: "Config file",
        body: "The tool reads config.toml from the working directory, then from your home directory. Environment variables override both.",
      },
      {
        heading: "Timeouts",
        body: "Requests time out after 30 seconds. Raise the limit for large uploads, never for ordinary calls.",
      },
    ],
  },
  {
    slug: "deployment",
    title: "Deployment",
    description: "Run it in production behind a reverse proxy.",
    sections: [
      {
        heading: "Behind a proxy",
        body: "Terminate TLS at the proxy and forward the original host and scheme headers, so links in e-mails point at the public address.",
      },
      {
        heading: "Upgrades",
        body: "Migrations run on start. Take a database backup first, then replace the binary and restart.",
      },
    ],
  },
]
