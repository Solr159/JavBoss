# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS web-build

ARG NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
ENV NPM_CONFIG_REGISTRY=${NPM_CONFIG_REGISTRY}

WORKDIR /src/web
COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --prefer-offline
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS go-build

ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.org
ENV GOPROXY=${GOPROXY} \
  GOSUMDB=${GOSUMDB}

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web-build /src/web/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go build -trimpath -ldflags="-s -w" -o /out/javboss ./cmd/server

FROM mwader/static-ffmpeg:8.1.1 AS ffmpeg-build

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=ffmpeg-build /ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg-build /ffprobe /usr/local/bin/ffprobe
COPY --from=go-build /out/javboss ./javboss
COPY --from=web-build /src/web/dist ./web/dist

ENV JAVBOSS_CONTAINER=1 \
  JAVBOSS_DISABLE_API_TOKEN=1 \
  JAVBOSS_DISABLE_DIRECTORY_PICKER=1 \
  JAVBOSS_DISABLE_DESKTOP_INTEGRATION=1 \
  JAVBOSS_DISABLE_MPV=1 \
  JAVBOSS_USE_FFMPEG_SCREENSHOTS=1 \
  JAVBOSS_HOST_PATH_PREFIX=1 \
  JAVBOSS_PROXY_HOST_GATEWAY=1 \
  FFMPEG_PATH=/usr/local/bin/ffmpeg \
  FFPROBE_PATH=/usr/local/bin/ffprobe

EXPOSE 17654
VOLUME ["/app/data"]

CMD ["./javboss", "-addr", ":17654", "-static", "web/dist"]
