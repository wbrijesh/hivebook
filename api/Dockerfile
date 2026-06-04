FROM golang:1.26-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o main cmd/api/main.go

FROM alpine:3.20 AS prod
WORKDIR /app
# ca-certificates populates the base system trust store; the mkcert CA for the
# ZITADEL issuer is mounted separately and appended to the pool at runtime.
RUN apk add --no-cache ca-certificates
COPY --from=build /app/main /app/main
EXPOSE 8080
CMD ["./main"]
