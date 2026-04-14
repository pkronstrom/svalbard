FROM python:3.12-alpine3.21 AS builder

RUN apk add --no-cache build-base sqlite-dev zlib-dev curl cmake meson \
    xapian-core-dev icu-dev zstd-dev xz-dev libmagic

# Build tippecanoe from source
ARG TIPPECANOE_VERSION=2.75.0
RUN curl -fsSL "https://github.com/felt/tippecanoe/archive/refs/tags/${TIPPECANOE_VERSION}.tar.gz" \
    | tar xz && cd tippecanoe-${TIPPECANOE_VERSION} && make -j$(nproc) && make install

# Install go-pmtiles
ARG PMTILES_VERSION=1.30.1
RUN ARCH=$(uname -m) && \
    case "$ARCH" in \
      x86_64)  SUFFIX="Linux_x86_64.tar.gz" ;; \
      aarch64) SUFFIX="Linux_arm64.tar.gz" ;; \
    esac && \
    curl -fsSL "https://github.com/protomaps/go-pmtiles/releases/download/v${PMTILES_VERSION}/go-pmtiles_${PMTILES_VERSION}_${SUFFIX}" \
    | tar xz -C /usr/local/bin pmtiles

# Build libzim + zim-tools from source (native arm64)
ARG LIBZIM_VERSION=9.6.0
RUN curl -fsSL "https://github.com/openzim/libzim/archive/refs/tags/${LIBZIM_VERSION}.tar.gz" \
    | tar xz && cd libzim-${LIBZIM_VERSION} \
    && meson setup build --prefix=/usr/local -Ddefault_library=static \
    && ninja -C build && ninja -C build install

ARG ZIM_TOOLS_VERSION=3.6.0
RUN curl -fsSL "https://github.com/openzim/zim-tools/archive/refs/tags/${ZIM_TOOLS_VERSION}.tar.gz" \
    | tar xz && cd zim-tools-${ZIM_TOOLS_VERSION} \
    && meson setup build --prefix=/usr/local \
    && ninja -C build && ninja -C build install

# Build zim-dither (Go image processing tool)
FROM golang:1.24-alpine3.21 AS go-builder
RUN apk add --no-cache git
COPY build-tools/ /src/
WORKDIR /src
RUN go build -o /usr/local/bin/zim-dither ./cmd/zim-dither/

FROM python:3.12-alpine3.21

RUN apk add --no-cache \
    ca-certificates \
    ffmpeg \
    gdal-tools \
    wget \
    xapian-core \
    icu-libs \
    zstd-libs \
    xz-libs \
    libmagic

COPY --from=builder /usr/local/bin/tippecanoe /usr/local/bin/
COPY --from=builder /usr/local/bin/tile-join /usr/local/bin/
COPY --from=builder /usr/local/bin/tippecanoe-decode /usr/local/bin/
COPY --from=builder /usr/local/bin/pmtiles /usr/local/bin/
COPY --from=builder /usr/local/bin/zimdump /usr/local/bin/
COPY --from=builder /usr/local/bin/zimwriterfs /usr/local/bin/
COPY --from=builder /usr/local/bin/zimcheck /usr/local/bin/
COPY --from=go-builder /usr/local/bin/zim-dither /usr/local/bin/

RUN pip install --no-cache-dir \
    libzim \
    nautiluszim \
    yle-dl \
    yt-dlp

COPY recipes/builders/media-zim.py /usr/local/bin/build-media-zim.py
