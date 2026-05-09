FROM golang:1.26.3@sha256:2981696eed011d747340d7252620932677929cce7d2d539602f56a8d7e9b660b AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/jobhunt-os ./cmd/jobhunt-os

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/firblab-blog/jobhunt-os"
LABEL org.opencontainers.image.description="Local-first, self-hosted job hunt command center"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=build /out/jobhunt-os /jobhunt-os

USER 65532:65532

ENV JOBHUNT_ADDR=0.0.0.0:8080
ENV JOBHUNT_ALLOW_NETWORK=true
ENV JOBHUNT_DATA_DIR=/data
ENV TMPDIR=/data/tmp

EXPOSE 8080

ENTRYPOINT ["/jobhunt-os"]
