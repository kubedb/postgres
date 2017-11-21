FROM postgres:9.6.5-alpine

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

ENV PGDATA /var/pv/data
ENV PGWAL /var/pgwal
ENV INITDB /var/initdb
ENV WALG_D /etc/wal-g.d/env

COPY leader_election /usr/bin/
COPY wal-g /usr/bin/

COPY scripts /scripts

RUN mkdir -p "$PGDATA" && chown -R postgres:postgres "$PGDATA" && chmod 0700 "$PGDATA"
RUN mkdir -p "$PGWAL" && chown -R postgres:postgres "$PGWAL" && chmod 0700 "$PGWAL"

VOLUME ["$PGDATA", "$PGWAL"]

ENV RESTORE false

ENTRYPOINT ["leader_election"]
EXPOSE 5432
