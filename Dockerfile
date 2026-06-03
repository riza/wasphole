FROM golang:1.25-alpine AS build

RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG TAG=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.tag=${TAG} -X main.buildDate=${BUILD_DATE}" \
    -o /out/wasphole \
    ./cmd/wasphole

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S wasphole \
    && adduser -S -G wasphole wasphole \
    && mkdir -p /data/cache /data/sessions /etc/wasphole \
    && chown -R wasphole:wasphole /data /etc/wasphole

COPY --from=build /out/wasphole /usr/local/bin/wasphole
COPY docker/config.yaml /etc/wasphole/config.yaml

USER wasphole
WORKDIR /data

EXPOSE 8080 8081 9090

ENTRYPOINT ["wasphole"]
CMD ["server", "-config", "/etc/wasphole/config.yaml"]
