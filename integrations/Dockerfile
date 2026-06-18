FROM golang:1.26-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# One image, two entrypoints: the Connect server + enqueuer, and the sync worker
# (design-doc 0010). The worker Deployment overrides the command to ./worker.
RUN CGO_ENABLED=0 go build -o integrations ./cmd/integrations \
 && CGO_ENABLED=0 go build -o worker ./cmd/worker

FROM alpine:3.20 AS prod
WORKDIR /app
# ca-certificates for outbound TLS to the OAuth providers (Google, GitHub) and S3.
RUN apk add --no-cache ca-certificates
COPY --from=build /app/integrations /app/integrations
COPY --from=build /app/worker /app/worker
EXPOSE 8080
# Non-root; matches runAsUser in the deployment securityContext.
USER 65532:65532
CMD ["./integrations"]
