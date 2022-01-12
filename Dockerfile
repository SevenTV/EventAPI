FROM golang:1.17.6-alpine as builder

WORKDIR /tmp/events

ARG BUILDER
ARG VERSION

ENV EVENTS_BUILDER=${BUILDER}
ENV EVENTS_VERSION=${VERSION}

RUN apk add --no-cache make git

COPY go.mod go.sum Makefile ./

RUN go mod download

COPY . .

RUN make

FROM alpine:latest

WORKDIR /app

COPY --from=builder /tmp/events/bin/events .

ENTRYPOINT ["./events"]
