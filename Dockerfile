FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /openim-node ./cmd/openim-node

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /openim-node /openim-node
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/openim-node"]
