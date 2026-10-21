# Build environment for Anthropic's OSS Scanner. The audit runs without network, so every
# dependency is fetched here: Go modules, the panel's npm packages and the SDK's bun packages.
FROM golang:1.27.3-trixie

ENV DEBIAN_FRONTEND=noninteractive \
    GOTOOLCHAIN=local

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential ca-certificates curl git jq procps postgresql \
    && rm -rf /var/lib/apt/lists/*

COPY --from=node:24-trixie-slim /usr/local/bin/node /usr/local/bin/node
COPY --from=node:24-trixie-slim /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx
COPY --from=oven/bun:1.4.2 /usr/local/bin/bun /usr/local/bin/bun
RUN ln -s bun /usr/local/bin/bunx

COPY . /src
WORKDIR /src

RUN cd web && npm ci && npm run build
RUN cd sdk/js && bun install --frozen-lockfile && bun run build
RUN cd edge && bun install --frozen-lockfile

RUN cd api \
    && go mod download \
    && find internal/ui/dist -mindepth 1 -delete && cp -R ../web/dist/. internal/ui/dist/ \
    && cp ../sdk/js/dist/yuva.js ../sdk/js/dist/yuva-chat.js ../sdk/js/dist/yuva-docs.js \
          ../sdk/js/dist/yuva-subscribe.js ../sdk/js/dist/yuva-analytics.js ../sdk/js/dist/yuva-analytics-consent.js \
          internal/widget/dist/ \
    && rm -rf internal/widget/dist/widget \
    && if [ -d ../sdk/js/dist/widget ]; then cp -R ../sdk/js/dist/widget internal/widget/dist/; fi \
    && go build ./... \
    && go build -o /usr/local/bin/yuva ./cmd/yuva
RUN cd sdk/go && go mod download && go build ./...

# Tests that need Postgres skip here; .oss-scanner/test-db.sh starts one for them.
RUN cd api && (go test -short ./... || echo "WARNING: api tests failed")

ENV GOPROXY=off \
    GOSUMDB=off

CMD ["bash"]
