FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk add --no-cache docker-cli
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/main .

RUN chown -R appuser:appgroup /app
USER appuser

EXPOSE 3000
CMD ["./main"]
