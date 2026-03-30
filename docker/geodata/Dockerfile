FROM ubuntu:24.04 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential libsqlite3-dev zlib1g-dev curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Build tippecanoe from source (small, fast build)
ARG TIPPECANOE_VERSION=2.75.0
RUN curl -fsSL "https://github.com/felt/tippecanoe/archive/refs/tags/${TIPPECANOE_VERSION}.tar.gz" \
    | tar xz && cd tippecanoe-${TIPPECANOE_VERSION} && make -j$(nproc) && make install

# Install go-pmtiles
ARG PMTILES_VERSION=1.30.1
RUN ARCH=$(dpkg --print-architecture) && \
    case "$ARCH" in \
      amd64)  SUFFIX="Linux_x86_64.tar.gz" ;; \
      arm64)  SUFFIX="Linux_arm64.tar.gz" ;; \
    esac && \
    curl -fsSL "https://github.com/protomaps/go-pmtiles/releases/download/v${PMTILES_VERSION}/go-pmtiles_${PMTILES_VERSION}_${SUFFIX}" \
    | tar xz -C /usr/local/bin pmtiles

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    gdal-bin libsqlite3-0 zlib1g ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/local/bin/tippecanoe /usr/local/bin/
COPY --from=builder /usr/local/bin/tile-join /usr/local/bin/
COPY --from=builder /usr/local/bin/tippecanoe-decode /usr/local/bin/
COPY --from=builder /usr/local/bin/pmtiles /usr/local/bin/
