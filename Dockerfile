FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o main .

FROM --platform=linux/amd64 scratch

COPY --from=builder /app/main /app/

ENTRYPOINT ["/app/main"]
