FROM golang:1.23.5 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/gitlab-access-token-exporter

FROM scratch
WORKDIR /app
COPY --from=builder /app/app ./
COPY config.yaml ./
ENTRYPOINT ["./app"]