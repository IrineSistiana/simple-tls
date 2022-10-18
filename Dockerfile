FROM --platform=${TARGETPLATFORM} golang:latest as builder
ARG REPOSITORY=IrineSistiana/simple-tls
ARG TAG
ARG CGO_ENABLED=0

WORKDIR /root
RUN git clone https://github.com/${REPOSITORY} simple-tls \
	&& cd ./simple-tls \
	&& git fetch --all --tags \
	&& git checkout tags/${TAG} \
	&& go build -ldflags "-s -w -X main.version=${TAG}" -trimpath -o simple-tls

FROM --platform=${TARGETPLATFORM} alpine:latest
LABEL maintainer="IrineSistiana <github.com/IrineSistiana>"

COPY --from=builder /root/simple-tls/simple-tls /usr/bin/

RUN apk add --no-cache ca-certificates