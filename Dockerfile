FROM golang:1.16.5-alpine3.13 AS build_base

RUN apk add --no-cache git

WORKDIR /tmp/app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go test ./... && go test -race ./... && go build -o seventv

FROM alpine:3.14
RUN apk update && apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=build_base /tmp/app/seventv /app/seventv

ENTRYPOINT ["/app/seventv"]
