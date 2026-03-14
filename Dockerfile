FROM alpine:latest

RUN apk add --no-cache bash ca-certificates curl

RUN mkdir -p /etc/file-uploader

COPY file-uploader /usr/local/bin/file-uploader
COPY file-uploader.toml.tmp /etc/file-uploader/file-uploader.toml
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

RUN mkdir -p /data
VOLUME /data

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
