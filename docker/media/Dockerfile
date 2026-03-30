FROM python:3.12-alpine3.21

RUN apk add --no-cache \
    ca-certificates \
    ffmpeg \
    wget

RUN pip install --no-cache-dir \
    libzim \
    yle-dl \
    yt-dlp

COPY build-media-zim.py /usr/local/bin/build-media-zim.py

ENTRYPOINT ["python", "/usr/local/bin/build-media-zim.py"]
