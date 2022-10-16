FROM --platform=${TARGETPLATFORM} golang:alpine as builder
ARG CGO_ENABLED=0
ARG TAG
ARG REPOSITORY

WORKDIR /root
RUN apk add --update git \
	&& git clone https://github.com/${REPOSITORY} simple-tls \
	&& cd ./simple-tls \
	&& git fetch --all --tags \
	&& git checkout tags/${TAG} \
	&& go build -ldflags "-s -w -X main.version=${TAG}" -trimpath -o simple-tls

FROM --platform=${TARGETPLATFORM} alpine:latest
LABEL maintainer="IrineSistiana <github.com/IrineSistiana>"

COPY --from=builder /root/simple-tls/simple-tls /usr/bin/

RUN apk add --no-cache ca-certificates

ENTRYPOINT /usr/bin/simple-tls