#!/usr/bin/env bash

set -e

echo "Running as Primary"

# set password ENV
export PGPASSWORD=${POSTGRES_PASSWORD:-postgres}

export ARCHIVE=${ARCHIVE:-}

if [ ! -e "$PGDATA/PG_VERSION" ]; then
  if [ "$RESTORE" = true ]; then
    echo "Restoring Postgres from base_backup using wal-g"
    /scripts/primary/restore.sh
  else
    /scripts/primary/start.sh
  fi
fi

# This node can become new leader while not able to create trigger file, So, left over recovery.conf from
# last bootup (when this node was standby) may exists. And, that will force this node to become STANDBY.
# So, delete recovery.conf.
if [[ -e $PGDATA/recovery.conf ]] && [[ $(cat $PGDATA/recovery.conf | grep -c "primary_conninfo") -gt 0 ]]; then
  # recovery.conf file exists and contains "primary_conninfo". So, this is left over from previous standby state.
  rm $PGDATA/recovery.conf
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
  fi

  pg_ctl -D "$PGDATA" -w start
  PGUSER="postgres" wal-g backup-push "$PGDATA" >/dev/null
  pg_ctl -D "$PGDATA" -m fast -w stop
fi

exec postgres
