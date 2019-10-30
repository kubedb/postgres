# Copyright The KubeDB Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM postgres:11.2-alpine

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
