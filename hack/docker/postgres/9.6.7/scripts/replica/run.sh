#!/usr/bin/env bash

set -eou pipefail

echo "Running as Replica"

# set password ENV
export PGPASSWORD=${POSTGRES_PASSWORD:-postgres}

export ARCHIVE=${ARCHIVE:-}

# Waiting for running Postgres
while true; do
  echo "Attempting pg_isready on primary"
  pg_isready --host="$PRIMARY_HOST" --timeout=2 &>/dev/null && break
  # check if current pod became leader itself
  if [[ -e "/tmp/pg-failover-trigger" ]]; then
    echo "Postgres promotion trigger_file found. Running primary run script"
    exec /scripts/primary/run.sh
  fi
  sleep 2
done

while true; do
  echo "Attempting query on primary"
  psql -h "$PRIMARY_HOST" --no-password --username=postgres --command="select now();" &>/dev/null && break
  # check if current pod became leader itself
  if [[ -e "/tmp/pg-failover-trigger" ]]; then
    echo "Postgres promotion trigger_file found. Running primary run script"
    exec /scripts/primary/run.sh
  fi
  sleep 2
done

# get basebackup
mkdir -p "$PGDATA"
rm -rf "$PGDATA"/*
chmod 0700 "$PGDATA"

pg_basebackup -X fetch --no-password --pgdata "$PGDATA" --username=postgres --host="$PRIMARY_HOST"

# setup recovery.conf
cp /scripts/replica/recovery.conf /tmp
echo "recovery_target_timeline = 'latest'" >>/tmp/recovery.conf
echo "archive_cleanup_command = 'pg_archivecleanup $PGWAL %r'" >>/tmp/recovery.conf
# primary_conninfo is used for streaming replication
echo "primary_conninfo = 'application_name=$HOSTNAME host=$PRIMARY_HOST'" >>/tmp/recovery.conf
mv /tmp/recovery.conf "$PGDATA/recovery.conf"

# setup postgresql.conf
touch /tmp/postgresql.conf
echo "wal_level = replica" >>/tmp/postgresql.conf
echo "max_wal_senders = 99" >>/tmp/postgresql.conf
echo "wal_keep_segments = 32" >>/tmp/postgresql.conf
if [ "$STANDBY" == "hot" ]; then
  echo "hot_standby = on" >>/tmp/postgresql.conf
fi
if [ "$STREAMING" == "synchronous" ]; then
   # setup synchronous streaming replication
   echo "synchronous_commit = remote_write" >>/tmp/postgresql.conf
   echo "synchronous_standby_names = '*'" >>/tmp/postgresql.conf
fi

# push base-backup
if [ "$ARCHIVE" == "wal-g" ]; then
  # set walg ENV
  CRED_PATH="/srv/wal-g/archive/secrets"

  if [[ ${ARCHIVE_S3_PREFIX} != "" ]]; then
    export WALE_S3_PREFIX="$ARCHIVE_S3_PREFIX"
    if [[ -e "$CRED_PATH/AWS_ACCESS_KEY_ID" ]]; then
      export AWS_ACCESS_KEY_ID=$(cat "$CRED_PATH/AWS_ACCESS_KEY_ID")
      export AWS_SECRET_ACCESS_KEY=$(cat "$CRED_PATH/AWS_SECRET_ACCESS_KEY")
    fi
  elif [[ ${ARCHIVE_GS_PREFIX} != "" ]]; then
    export WALE_GS_PREFIX="$ARCHIVE_GS_PREFIX"
    if [[ -e "$CRED_PATH/GOOGLE_APPLICATION_CREDENTIALS" ]]; then
      export GOOGLE_APPLICATION_CREDENTIALS="$CRED_PATH/GOOGLE_APPLICATION_CREDENTIALS"
    elif [[ -e "$CRED_PATH/GOOGLE_SERVICE_ACCOUNT_JSON_KEY" ]]; then
      export GOOGLE_APPLICATION_CREDENTIALS="$CRED_PATH/GOOGLE_SERVICE_ACCOUNT_JSON_KEY"
    fi
  elif [[ ${ARCHIVE_AZ_PREFIX} != "" ]]; then
    export WALE_AZ_PREFIX="$ARCHIVE_AZ_PREFIX"
    if [[ -e "$CRED_PATH/AZURE_STORAGE_ACCESS_KEY" ]]; then
      export AZURE_STORAGE_ACCESS_KEY=$(cat "$CRED_PATH/AZURE_STORAGE_ACCESS_KEY")
     elif [[ -e "$CRED_PATH/AZURE_ACCOUNT_KEY" ]]; then
      export AZURE_STORAGE_ACCESS_KEY=$(cat "$CRED_PATH/AZURE_ACCOUNT_KEY")
    fi
    if [[ -e "$CRED_PATH/AZURE_STORAGE_ACCOUNT" ]]; then
      export AZURE_STORAGE_ACCOUNT=$(cat "$CRED_PATH/AZURE_STORAGE_ACCOUNT")
      elif [[ -e "$CRED_PATH/AZURE_ACCOUNT_NAME" ]]; then
      export AZURE_STORAGE_ACCOUNT=$(cat "$CRED_PATH/AZURE_ACCOUNT_NAME")
    fi
  elif [[ ${ARCHIVE_SWIFT_PREFIX} != "" ]]; then
    export WALE_SWIFT_PREFIX="$ARCHIVE_SWIFT_PREFIX"
    if [[ -e "$CRED_PATH/OS_USERNAME" ]]; then
      export OS_USERNAME=$(cat "$CRED_PATH/OS_USERNAME")
    fi
    if [[ -e "$CRED_PATH/OS_PASSWORD" ]]; then
      export OS_PASSWORD=$(cat "$CRED_PATH/OS_PASSWORD")
    fi
    if [[ -e "$CRED_PATH/OS_REGION_NAME" ]]; then
      export OS_REGION_NAME=$(cat "$CRED_PATH/OS_REGION_NAME")
    fi
    if [[ -e "$CRED_PATH/OS_AUTH_URL" ]]; then
      export OS_AUTH_URL=$(cat "$CRED_PATH/OS_AUTH_URL")
    fi
    #v2
    if [[ -e "$CRED_PATH/OS_TENANT_NAME" ]]; then
      export OS_TENANT_NAME=$(cat "$CRED_PATH/OS_TENANT_NAME")
    fi
    if [[ -e "$CRED_PATH/OS_TENANT_ID" ]]; then
      export OS_TENANT_ID=$(cat "$CRED_PATH/OS_TENANT_ID")
    fi
    #v3
    if [[ -e "$CRED_PATH/OS_USER_DOMAIN_NAME" ]]; then
      export OS_USER_DOMAIN_NAME=$(cat "$CRED_PATH/OS_USER_DOMAIN_NAME")
    fi
    if [[ -e "$CRED_PATH/OS_PROJECT_NAME" ]]; then
      export OS_PROJECT_NAME=$(cat "$CRED_PATH/OS_PROJECT_NAME")
    fi
    if [[ -e "$CRED_PATH/OS_PROJECT_DOMAIN_NAME" ]]; then
      export OS_PROJECT_DOMAIN_NAME=$(cat "$CRED_PATH/OS_PROJECT_DOMAIN_NAME")
    fi
    #manual
    if [[ -e "$CRED_PATH/OS_STORAGE_URL" ]]; then
      export OS_STORAGE_URL=$(cat "$CRED_PATH/OS_STORAGE_URL")
    fi
    if [[ -e "$CRED_PATH/OS_AUTH_TOKEN" ]]; then
      export OS_AUTH_TOKEN=$(cat "$CRED_PATH/OS_AUTH_TOKEN")
    fi
    #v1
    if [[ -e "$CRED_PATH/ST_AUTH" ]]; then
      export ST_AUTH=$(cat "$CRED_PATH/ST_AUTH")
    fi
    if [[ -e "$CRED_PATH/ST_USER" ]]; then
      export ST_USER=$(cat "$CRED_PATH/ST_USER")
    fi
    if [[ -e "$CRED_PATH/ST_KEY" ]]; then
      export ST_KEY=$(cat "$CRED_PATH/ST_KEY")
    fi
  fi

  # setup postgresql.conf
  echo "archive_command = 'wal-g wal-push %p'" >>/tmp/postgresql.conf
  echo "archive_timeout = 60" >>/tmp/postgresql.conf
  echo "archive_mode = always" >>/tmp/postgresql.conf
fi
cat /scripts/primary/postgresql.conf >> /tmp/postgresql.conf
mv /tmp/postgresql.conf "$PGDATA/postgresql.conf"

exec postgres
