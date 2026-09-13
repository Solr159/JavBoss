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
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go build -trimpath -ldflags="-s -w" -o /out/javboss ./cmd/server

FROM --platform=$BUILDPLATFORM alpine:3.23 AS ffmpeg-build

ARG TARGETARCH
# Keep the month-end BtbN build aligned with scripts/cli/cli.mjs.
# Month-end releases are retained for two years; daily releases expire sooner.
ARG FFMPEG_RELEASE=autobuild-2026-08-31-13-27
ARG FFMPEG_VERSION=n8.1.2-50-g1a748fe2cd
ARG FFPROBE_RELEASE=n8.1.2-1

RUN set -eu; \
  case "${TARGETARCH}" in \
    amd64) \
      ffmpeg_arch="linux64"; \
      ffprobe_arch="x64"; \
      archive_sha256="c733b4b2951e5957e15505f788b2c65a7a41b6da4b289e295852cc38079b4d2b"; \
      ffmpeg_sha256="ad7a8c8e8fe4f50972f32f63705cfcc57f44cd3531f57aa8defe388372242f5e"; \
      ffprobe_sha256="065d3c56926052a76e884c4e4b51b7d95248da9391ab7effdcca6b94ceab98cf" \
      ;; \
    arm64) \
      ffmpeg_arch="linuxarm64"; \
      ffprobe_arch="arm64"; \
      archive_sha256="ae5da4f51b9052390f414005f8ab26c1eed1268f327cce7cb79aa076b29bd66e"; \
      ffmpeg_sha256="1b216dbbe46adf213945d357ef958404eed1632e7fb38a85f7f030ed9c212bcf"; \
      ffprobe_sha256="fd2aca1456f0261cabef4514b6d97a70fa342003347f51b39c473dd364328089" \
      ;; \
    *) \
      echo "unsupported target architecture: ${TARGETARCH}" >&2; \
      exit 1 \
      ;; \
  esac; \
  archive_name="ffmpeg-${FFMPEG_VERSION}-${ffmpeg_arch}-gpl-8.1"; \
  release_url="https://github.com/BtbN/FFmpeg-Builds/releases/download/${FFMPEG_RELEASE}"; \
  wget -q -O /ffmpeg.tar.xz "${release_url}/${archive_name}.tar.xz"; \
  echo "${archive_sha256}  /ffmpeg.tar.xz" | sha256sum -c -; \
  tar -xJf /ffmpeg.tar.xz -C / --strip-components=2 "${archive_name}/bin/ffmpeg"; \
  echo "${ffmpeg_sha256}  /ffmpeg" | sha256sum -c -; \
  rm /ffmpeg.tar.xz; \
  ffprobe_url="https://github.com/shaka-project/static-ffmpeg-binaries/releases/download/${FFPROBE_RELEASE}"; \
  wget -q -O /ffprobe "${ffprobe_url}/ffprobe-linux-${ffprobe_arch}"; \
  echo "${ffprobe_sha256}  /ffprobe" | sha256sum -c -; \
  chmod 0755 /ffmpeg /ffprobe

# BtbN FFmpeg needs libgcc_s, supplied by the distroless cc image.
FROM gcr.io/distroless/cc-debian12:latest@sha256:e5d81ddde149641e2a9ba55be4545bc125c67de07508b03ba4c22e6eb0ded5aa

WORKDIR /app
COPY --from=ffmpeg-build /ffmpeg ./internal/bin/ffmpeg
COPY --from=ffmpeg-build /ffprobe ./internal/bin/ffprobe
COPY --from=go-build /out/javboss ./javboss
COPY --from=web-build /src/web/dist ./web/dist

ENV JAVBOSS_CONTAINER=1 \
  JAVBOSS_DISABLE_DESKTOP_INTEGRATION=1 \
  JAVBOSS_DISABLE_MPV=1 \
  JAVBOSS_USE_FFMPEG_SCREENSHOTS=1 \
  JAVBOSS_HOST_PATH_PREFIX=1 \
  JAVBOSS_PROXY_HOST_GATEWAY=1

EXPOSE 17654
VOLUME ["/app/data"]

CMD ["./javboss"]
