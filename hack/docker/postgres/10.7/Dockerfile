FROM postgres:10.7-alpine

RUN set -x \
  && apk add --update --no-cache ca-certificates

ENV PV /var/pv
ENV PGDATA $PV/data
ENV PGWAL $PGDATA/pg_wal
ENV INITDB /var/initdb
ENV WALG_D /etc/wal-g.d/env

COPY pg-operator /usr/bin/
COPY wal-g /usr/bin/

COPY scripts /scripts

VOLUME ["$PV"]

ENV STANDBY warm
ENV RESTORE false
ENV BACKUP_NAME LATEST
ENV PITR false
ENV ARCHIVE_S3_PREFIX ""
ENV ARCHIVE_S3_ENDPOINT ""
ENV RESTORE_S3_PREFIX ""
ENV RESTORE_S3_ENDPOINT ""

ENV ARCHIVE_GS_PREFIX ""
ENV RESTORE_GS_PREFIX ""

ENV ARCHIVE_AZ_PREFIX ""
ENV RESTORE_AZ_PREFIX ""

ENV ARCHIVE_SWIFT_PREFIX ""
ENV RESTORE_SWIFT_PREFIX ""

ENV ARCHIVE_FILE_PREFIX ""
ENV RESTORE_FILE_PREFIX ""

ENTRYPOINT ["pg-operator"]
CMD ["leader_election"]

EXPOSE 5432

# https://github.com/abkfenris/kubedb-postgis/blob/master/10.2/Dockerfile
ENV POSTGIS_VERSION 2.5.2

RUN set -ex \
    \
    && apk add --no-cache --virtual .fetch-deps \
    ca-certificates \
    openssl \
    tar \
    && wget -O postgis.tar.gz "https://github.com/postgis/postgis/archive/$POSTGIS_VERSION.tar.gz" \

    && mkdir -p /usr/src/postgis \
    &&  tar \
    --extract \
    --file postgis.tar.gz \
    --directory /usr/src/postgis \
    --strip-components 1 \
    && rm postgis.tar.gz \
    && apk add --no-cache --virtual .build-deps \
    autoconf \
    automake \
    g++ \
    json-c-dev \
    libtool \
    libxml2-dev \
    make \
    perl \
    && apk add -v --no-cache --virtual .postgis-rundeps \
    json-c \
    && apk add --no-cache --virtual .build-deps-edge \
    --repository http://dl-cdn.alpinelinux.org/alpine/edge/main \
    --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing \
    apk-tools \
    gdal-dev \
    geos-dev \
    proj4-dev \
    protobuf-c-dev \
    # RUN apk add --no-cache --virtual .postgis-rundeps-edge \
    #     --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing \
    #     --repository http://dl-cdn.alpinelinux.org/alpine/edge/main \        
    #     geos \
    #     gdal \
    #     proj4 \
    #     protobuf-c
    && cd /usr/src/postgis \
    && ./autogen.sh \
    # configure options taken from:
    # https://anonscm.debian.org/cgit/pkg-grass/postgis.git/tree/debian/rules?h=jessie
    && ./configure \
    #       --with-gui \
    && make \
    && make install \
    && cd / \
    && rm -rf /usr/src/postgis
# && apk del .fetch-deps
# .build-deps
# .build-deps-edge
