#!/usr/bin/env bash

set -xe

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

  pg_ctl -D "$PGDATA" -w start
  PGUSER="postgres" wal-g backup-push "$PGDATA" >/dev/null
  pg_ctl -D "$PGDATA" -m fast -w stop
fi

exec postgres
