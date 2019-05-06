#!/bin/bash

mkdir -p "$PGDATA"
rm -rf "$PGDATA"/*
chmod 0700 "$PGDATA"

# set wal-g ENV
CRED_PATH="/srv/wal-g/restore/secrets"

if [[ ${RESTORE_S3_PREFIX} != "" ]]; then
  export WALE_S3_PREFIX="$RESTORE_S3_PREFIX"
  [[ -e "$CRED_PATH/AWS_ACCESS_KEY_ID" ]] &&  export AWS_ACCESS_KEY_ID=$(cat "$CRED_PATH/AWS_ACCESS_KEY_ID")
  [[ -e "$CRED_PATH/AWS_SECRET_ACCESS_KEY" ]] &&  export AWS_SECRET_ACCESS_KEY=$(cat "$CRED_PATH/AWS_SECRET_ACCESS_KEY")
  if [[ ${RESTORE_S3_ENDPOINT} != "" ]]; then
    [[ -e "$CRED_PATH/CA_CERT_DATA" ]] &&  export WALG_S3_CA_CERT_FILE="$CRED_PATH/CA_CERT_DATA"
    export AWS_ENDPOINT=$RESTORE_S3_ENDPOINT
    export AWS_S3_FORCE_PATH_STYLE="true"
    export AWS_REGION="us-east-1"
  fi

elif [[ ${RESTORE_GS_PREFIX} != "" ]]; then
  export WALE_GS_PREFIX="$RESTORE_GS_PREFIX"
  [[ -e "$CRED_PATH/GOOGLE_APPLICATION_CREDENTIALS" ]] && export GOOGLE_APPLICATION_CREDENTIALS="$CRED_PATH/GOOGLE_APPLICATION_CREDENTIALS"
  [[ -e "$CRED_PATH/GOOGLE_SERVICE_ACCOUNT_JSON_KEY" ]] &&  export GOOGLE_APPLICATION_CREDENTIALS="$CRED_PATH/GOOGLE_SERVICE_ACCOUNT_JSON_KEY"

elif [[ ${RESTORE_FILE_PREFIX} != "" ]]; then
  export WALG_FILE_PREFIX="$RESTORE_FILE_PREFIX"
  mkdir -p $WALG_FILE_PREFIX

elif [[ ${RESTORE_AZ_PREFIX} != "" ]]; then
  export WALE_AZ_PREFIX="$RESTORE_AZ_PREFIX"
  [[ -e "$CRED_PATH/AZURE_STORAGE_ACCESS_KEY" ]] && export AZURE_STORAGE_ACCESS_KEY=$(cat "$CRED_PATH/AZURE_STORAGE_ACCESS_KEY")
  [[ -e "$CRED_PATH/AZURE_ACCOUNT_KEY" ]] && export AZURE_STORAGE_ACCESS_KEY=$(cat "$CRED_PATH/AZURE_ACCOUNT_KEY")
  [[ -e "$CRED_PATH/AZURE_STORAGE_ACCOUNT" ]] && export AZURE_STORAGE_ACCOUNT=$(cat "$CRED_PATH/AZURE_STORAGE_ACCOUNT")
  [[ -e "$CRED_PATH/AZURE_ACCOUNT_NAME" ]] && export AZURE_STORAGE_ACCOUNT=$(cat "$CRED_PATH/AZURE_ACCOUNT_NAME")

elif [[ ${RESTORE_SWIFT_PREFIX} != "" ]]; then
  export WALE_SWIFT_PREFIX="$RESTORE_SWIFT_PREFIX"
  [[ -e "$CRED_PATH/OS_USERNAME" ]] &&  export OS_USERNAME=$(cat "$CRED_PATH/OS_USERNAME")
  [[ -e "$CRED_PATH/OS_PASSWORD" ]] &&  export OS_PASSWORD=$(cat "$CRED_PATH/OS_PASSWORD")
  [[ -e "$CRED_PATH/OS_REGION_NAME" ]] &&  export OS_REGION_NAME=$(cat "$CRED_PATH/OS_REGION_NAME")
  [[ -e "$CRED_PATH/OS_AUTH_URL" ]] &&  export OS_AUTH_URL=$(cat "$CRED_PATH/OS_AUTH_URL")
  #v2
  [[ -e "$CRED_PATH/OS_TENANT_NAME" ]] &&  export OS_TENANT_NAME=$(cat "$CRED_PATH/OS_TENANT_NAME")
  [[ -e "$CRED_PATH/OS_TENANT_ID" ]] &&  export OS_TENANT_ID=$(cat "$CRED_PATH/OS_TENANT_ID")
  #v3
  [[ -e "$CRED_PATH/OS_USER_DOMAIN_NAME" ]] && export OS_USER_DOMAIN_NAME=$(cat "$CRED_PATH/OS_USER_DOMAIN_NAME")

  [[ -e "$CRED_PATH/OS_PROJECT_NAME" ]] && export OS_PROJECT_NAME=$(cat "$CRED_PATH/OS_PROJECT_NAME")

  [[ -e "$CRED_PATH/OS_PROJECT_DOMAIN_NAME" ]] && export OS_PROJECT_DOMAIN_NAME=$(cat "$CRED_PATH/OS_PROJECT_DOMAIN_NAME")
  #manual
  [[ -e "$CRED_PATH/OS_STORAGE_URL" ]] && export OS_STORAGE_URL=$(cat "$CRED_PATH/OS_STORAGE_URL")
  [[ -e "$CRED_PATH/OS_AUTH_TOKEN" ]] && export OS_AUTH_TOKEN=$(cat "$CRED_PATH/OS_AUTH_TOKEN")
  #v1
  [[ -e "$CRED_PATH/ST_AUTH" ]] && export ST_AUTH=$(cat "$CRED_PATH/ST_AUTH")
  [[ -e "$CRED_PATH/ST_USER" ]] && export ST_USER=$(cat "$CRED_PATH/ST_USER")
  [[ -e "$CRED_PATH/ST_KEY" ]] && export ST_KEY=$(cat "$CRED_PATH/ST_KEY")
fi

PITR=${PITR:-false}
TARGET_INCLUSIVE=${TARGET_INCLUSIVE:-true}
TARGET_TIME=${TARGET_TIME:-}
TARGET_TIMELINE=${TARGET_TIMELINE:-}
TARGET_XID=${TARGET_XID:-}

until wal-g backup-list &>/dev/null; do
  echo "waiting for archived backup..."
  sleep 5
done

echo "Fetching archived backup..."
# fetch backup
wal-g backup-fetch "$PGDATA" "$BACKUP_NAME" >/dev/null

# create missing folders
mkdir -p "$PGDATA"/{pg_tblspc,pg_twophase,pg_stat,pg_commit_ts}/
mkdir -p "$PGDATA"/pg_logical/{snapshots,mappings}/

# setup recovery.conf
cp /scripts/replica/recovery.conf /tmp

# ref: https://www.postgresql.org/docs/10/static/recovery-target-settings.html
if [ "$PITR" = true ]; then
  echo "recovery_target_inclusive = '$TARGET_INCLUSIVE'" >>/tmp/recovery.conf
  echo "recovery_target_action = 'promote'" >>/tmp/recovery.conf

  if [ ! -z "$TARGET_TIME" ]; then
    echo "recovery_target_time = '$TARGET_TIME'" >>/tmp/recovery.conf
  fi
  if [ ! -z "$TARGET_TIMELINE" ]; then
    echo "recovery_target_timeline = '$TARGET_TIMELINE'" >>/tmp/recovery.conf
  fi
  if [ ! -z "$TARGET_XID" ]; then
    echo "recovery_target_xid = '$TARGET_XID'" >>/tmp/recovery.conf
  fi
fi

echo "restore_command = 'wal-g wal-fetch %f %p'" >>/tmp/recovery.conf
mv /tmp/recovery.conf "$PGDATA/recovery.conf"

# setup postgresql.conf
touch /tmp/postgresql.conf
echo "wal_level = replica" >>/tmp/postgresql.conf
echo "max_wal_senders = 90" >>/tmp/postgresql.conf # default is 10.  value must be less than max_connections minus superuser_reserved_connections. ref: https://www.postgresql.org/docs/11/runtime-config-replication.html#GUC-MAX-WAL-SENDERS
echo "wal_keep_segments = 32" >>/tmp/postgresql.conf
if [ "$STREAMING" == "synchronous" ]; then
  # setup synchronous streaming replication
  echo "synchronous_commit = remote_write" >>/tmp/postgresql.conf
  echo "synchronous_standby_names = '*'" >>/tmp/postgresql.conf
fi

if [ "$ARCHIVE" == "wal-g" ]; then
  # setup postgresql.conf
  echo "archive_command = 'wal-g wal-push %p'" >>/tmp/postgresql.conf
  echo "archive_timeout = 60" >>/tmp/postgresql.conf
  echo "archive_mode = always" >>/tmp/postgresql.conf
fi
cat /scripts/primary/postgresql.conf >> /tmp/postgresql.conf
mv /tmp/postgresql.conf "$PGDATA/postgresql.conf"

rm "$PGDATA/recovery.done" &>/dev/null

# start server for recovery process
pg_ctl -D "$PGDATA" -W start >/dev/null

# this file will trigger recovery
touch '/tmp/pg-failover-trigger'

# This will hold until recovery completed
while [ ! -e "$PGDATA/recovery.done" ]; do
  echo "replaying wal files..."
  sleep 5
done

# create PID if misssing
postmaster -D "$PGDATA" &>/dev/null

pg_ctl -D "$PGDATA" -w stop >/dev/null
