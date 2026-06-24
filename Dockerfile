FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /iptrack .

FROM alpine:3
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /iptrack /usr/local/bin/iptrack
ENTRYPOINT ["iptrack"]
