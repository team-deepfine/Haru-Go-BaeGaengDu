# Stage 1: Build
FROM --platform=linux/amd64 golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /haru ./cmd/server

# Stage 2: Runtime
FROM --platform=linux/amd64 alpine:3.19

RUN apk add --no-cache tzdata ca-certificates
RUN adduser -D -u 1000 appuser
COPY --from=builder /haru /haru
COPY --from=builder /app/prompts /prompts
COPY --from=builder /app/firebase-credentials.json /app/firebase-credentials.json

USER appuser
EXPOSE 8080
CMD ["/haru"]
