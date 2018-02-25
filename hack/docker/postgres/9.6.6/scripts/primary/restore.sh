#!/bin/bash

mkdir -p "$PGDATA"
rm -rf "$PGDATA"/*
chmod 0700 "$PGDATA"

# set walg ENV
CRED_PATH="/srv/wal-g/restore/secrets"
export WALE_S3_PREFIX=$(echo "$RESTORE_S3_PREFIX")
export AWS_ACCESS_KEY_ID=$(cat "$CRED_PATH/AWS_ACCESS_KEY_ID")
export AWS_SECRET_ACCESS_KEY=$(cat "$CRED_PATH/AWS_SECRET_ACCESS_KEY")
# fetch backup
wal-g backup-fetch "$PGDATA" "$BACKUP_NAME" >/dev/null

# create missing folders
mkdir -p "$PGDATA"/{pg_tblspc,pg_twophase,pg_stat,pg_commit_ts}/
mkdir -p "$PGDATA"/pg_logical/{snapshots,mappings}/

# setup recovery.conf
cp /scripts/replica/recovery.conf /tmp
echo "recovery_target_timeline = '$PITR'" >> /tmp/recovery.conf
echo "restore_command = 'wal-g wal-fetch %f %p'" >> /tmp/recovery.conf
cp /tmp/recovery.conf "$PGDATA/recovery.conf"

# start server for recovery process
pg_ctl -D "$PGDATA" -W start >/dev/null
# this file will trigger recovery
touch '/tmp/pg-failover-trigger'
# This will hold until recovery completed
while [ ! -e "$PGDATA/recovery.done" ]
do
    sleep 2
done
# create PID if misssing
postmaster -D "$PGDATA" >/dev/null
pg_ctl -D "$PGDATA" -w stop >/dev/null

# setup postgresql.conf
echo "wal_level = replica" >> "$PGDATA/postgresql.conf"
echo "max_wal_senders = 99" >> "$PGDATA/postgresql.conf"
echo "wal_keep_segments = 32" >> "$PGDATA/postgresql.conf"

if [ "$ARCHIVE" == "wal-g" ]; then
    # setup postgresql.conf
    echo "archive_command = 'wal-g wal-push %p'" >> "$PGDATA/postgresql.conf"
    echo "archive_timeout = 60" >> "$PGDATA/postgresql.conf"
    echo "archive_mode = always" >> "$PGDATA/postgresql.conf"
fi
