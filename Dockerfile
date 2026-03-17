# Build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /instasae ./cmd/instasae/
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /instasae /usr/local/bin/instasae
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY migrations/ /migrations/

EXPOSE 8080

ENTRYPOINT ["instasae"]
