FROM postgres:17-alpine
RUN apk add --no-cache gnupg tzdata
COPY backup.sh restore.sh loop.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/backup.sh /usr/local/bin/restore.sh /usr/local/bin/loop.sh
ENTRYPOINT ["/usr/local/bin/loop.sh"]
