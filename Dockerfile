FROM golang:1.17.2 AS build_base

WORKDIR /tmp/app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go test ./... && go build -o seventv

FROM alpine:3.14
RUN apk update && apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=build_base /tmp/app/seventv /app/seventv

ENTRYPOINT ["/app/seventv"]
