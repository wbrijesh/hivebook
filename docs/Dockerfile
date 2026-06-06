# Build the Antora site (with Kroki/D2 diagrams), then serve it with nginx.
# Build context is the repo root (Antora reads ./docs via the playbook).
FROM node:22-alpine AS build
RUN apk add --no-cache git
WORKDIR /build
# Local install so the asciidoctor-kroki asciidoc extension resolves reliably.
RUN npm init -y >/dev/null \
 && npm install --no-audit --no-fund \
      @antora/cli@3.1 @antora/site-generator@3.1 asciidoctor-kroki@0.18.1
COPY antora-playbook.yml ./
COPY docs/ ./docs/
# Antora reads content from a git ref; commit the docs tree (not node_modules)
# so the `url: .` / `branches: HEAD` content source has a HEAD to resolve.
RUN git init -q \
 && git config user.email build@hivebook.localhost \
 && git config user.name build \
 && git add docs antora-playbook.yml && git commit -qm build
# kroki-fetch-diagram (set in the playbook) downloads each diagram's SVG at
# build time and bakes it into the site — the served pages never call Kroki.
RUN npx antora antora-playbook.yml

FROM nginx:1.27-alpine AS prod
COPY infra/docs/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /build/public /usr/share/nginx/html
EXPOSE 80
