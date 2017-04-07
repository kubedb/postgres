#####################################
#      Image for database dump      #
#  Image: appscode/postgres:util    #
#####################################

FROM appscode/python:2.7

## postgres client major version
ENV PG_MAJOR 9.5
## osm version
ENV OSM_VERSION 0.3.0

RUN groupadd -r postgres --gid=999 && useradd -r -g postgres --uid=999 postgres
RUN apt-key adv --keyserver ha.pool.sks-keyservers.net --recv-keys B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8

RUN echo 'deb http://apt.postgresql.org/pub/repos/apt/ jessie-pgdg main' $PG_MAJOR > /etc/apt/sources.list.d/pgdg.list

RUN set -x \
  && apt-get update \
  && apt-get install -y curl ca-certificates netcat \
  && apt-get install -y --no-install-recommends postgresql-client-$PG_MAJOR --force-yes \
  && pip install --upgrade pip \
  && pip install pyyaml \
  && rm -rf /var/lib/apt/lists/* /usr/share/doc /usr/share/man /tmp/*

RUN set -x \
  && curl -L "https://cdn.appscode.com/binaries/osm/$OSM_VERSION/osm-linux-amd64" -o /usr/local/bin/osm \
  && chmod +x /usr/local/bin/osm

COPY execute.py /execute.py
COPY utils.sh /utils.sh
RUN chmod +x /utils.sh

ENTRYPOINT ["python2.7", "execute.py"]
