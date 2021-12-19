FROM golang:1.17.5-alpine3.15 AS build_base

WORKDIR /tmp/app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go test ./... && go build -o seventv

FROM alpine:3.15
RUN apk update && apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=build_base /tmp/app/seventv /app/seventv

ENTRYPOINT ["/app/seventv"]
