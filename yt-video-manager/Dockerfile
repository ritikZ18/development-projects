# ---- build stage ----
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /out/ytm .

# ---- runtime stage ----
FROM python:3.12-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir -U yt-dlp

COPY --from=build /out/ytm /usr/local/bin/ytm

RUN mkdir -p /downloads
ENV YTM_DOWNLOAD_DIR=/downloads
EXPOSE 8080
WORKDIR /root

ENTRYPOINT ["ytm"]