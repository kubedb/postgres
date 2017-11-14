FROM postgres:9.6.5-alpine

ENV PGDATA /var/pv/data
ENV PGWAL /var/pgwal
ENV INITDB /var/initdb

RUN mkdir -p "$PGDATA" && chown -R postgres:postgres "$PGDATA" && chmod 777 "$PGDATA"
RUN mkdir -p "$PGWAL" && chown -R postgres:postgres "$PGWAL" && chmod 777 "$PGWAL"

VOLUME ["$PGDATA", "$PGWAL"]

COPY leader_election /usr/local/bin/
COPY scripts /scripts

ENTRYPOINT ["leader_election"]

EXPOSE 5432
