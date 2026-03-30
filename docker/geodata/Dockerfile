FROM alpine:3.21

RUN apk add --no-cache gdal-tools tippecanoe curl

# Install go-pmtiles
ARG PMTILES_VERSION=1.30.1
RUN ARCH=$(uname -m) && \
    case "$ARCH" in \
      x86_64)  SUFFIX="Linux_x86_64.tar.gz" ;; \
      aarch64) SUFFIX="Linux_arm64.tar.gz" ;; \
    esac && \
    curl -fsSL "https://github.com/protomaps/go-pmtiles/releases/download/v${PMTILES_VERSION}/go-pmtiles_${PMTILES_VERSION}_${SUFFIX}" \
      | tar xz -C /usr/local/bin pmtiles && \
    chmod +x /usr/local/bin/pmtiles

RUN apk del curl
