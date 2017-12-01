#!/bin/bash

exec 1> >(logger -s -p daemon.info -t pg)
exec 2> >(logger -s -p daemon.error -t pg)

RETVAL=0

backup() {
    # 1 - host
    # 2 - username
    # 3 - password

    path=/var/dump-backup
    mkdir -p "$path"
    cd "$path"
    rm -rf "$path"/*

    # Wait for postgres to start
    # ref: http://unix.stackexchange.com/a/5279
    while ! nc -q 1 $1 5432 </dev/null; do echo "Waiting... Master pod is not ready yet"; sleep 5; done

    PGPASSWORD="$3" pg_dumpall -U "$2" -h "$1" > dumpfile.sql
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to take backup"
        exit 1
    fi
    exit 0
}

restore() {
    # 1 - Host
    # 2 - username
    # 3 - password

    path=/var/dump-restore/
    mkdir -p "$path"
    cd "$path"

    # Wait for postgres to start
    # ref: http://unix.stackexchange.com/a/5279
    while ! nc -q 1 $1 5432 </dev/null; do echo "Waiting... Master pod is not ready yet"; sleep 5; done

    PGPASSWORD="$3" psql -U "$2" -h "$1"  -f dumpfile.sql postgres
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to restore"
        exit 1
    fi
    exit 0
}

push() {
    # 1 - bucket
    # 2 - folder
    # 3 - snapshot-name

    src_path=/var/dump-backup/dumpfile.sql
    osm push --osmconfig=/etc/osm/config -c "$1" "$src_path" "$2/$3/dumpfile.sql"
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to push data to cloud"
        exit 1
    fi

    exit 0
}

pull() {
    # 1 - bucket
    # 2 - folder
    # 3 - snapshot-name

    dst_path=/var/dump-restore/
    mkdir -p "$dst_path"
    rm -rf "$dst_path"

    osm pull --osmconfig=/etc/osm/config -c "$1" "$2/$3" "$dst_path"
    retval=$?
    if [ "$retval" -ne 0 ]; then
        echo "Fail to pull data from cloud"
        exit 1
    fi

    exit 0
}

create_pgpass_file() {
    rm -rf /tmp/.pgpass
    cat >> "/tmp/.pgpass" <<-EOF
*:*:*:*:${1}
EOF
    cat /tmp/.pgpass
    chmod 0600 "/tmp/.pgpass"
    export PGPASSFILE=/tmp/.pgpass
}

base_backup() {
    # 1 - Host
    # 2 - username
    # 3 - password
    # 4 - Bucket
    # 5 - Folder
    # 6 - Snapshot

    CRED_PATH="/srv/wal-g/archive/secrets"
    if [ -d "$CRED_PATH" ]; then
        AWS_ACCESS_KEY_ID_PATH="$CRED_PATH/AWS_ACCESS_KEY_ID"
        if [ -f "$AWS_ACCESS_KEY_ID_PATH" ]; then
            export AWS_ACCESS_KEY_ID=$(cat "$AWS_ACCESS_KEY_ID_PATH")
        fi
        AWS_SECRET_ACCESS_KEY_PATH="$CRED_PATH/AWS_SECRET_ACCESS_KEY"
        if [ -f "$AWS_SECRET_ACCESS_KEY_PATH" ]; then
            export AWS_SECRET_ACCESS_KEY=$(cat "$AWS_SECRET_ACCESS_KEY_PATH")
        fi
    fi

    export WALE_S3_PREFIX="s3://$4/$5/$6"
    PGDATA="/var/base-backup"
    create_pgpass_file "$3"
    pg_basebackup -X fetch --no-password --pgdata "$PGDATA" --username="$2" --host="$1" &>/dev/null
    PGHOST="$1" PGPORT="5432" PGUSER="$2" wal-g backup-push "$PGDATA" &>/dev/null
}

process=$1
shift
case "$process" in
    backup)
        backup "$@"
        ;;
    restore)
        restore "$@"
        ;;
    push)
        push "$@"
        ;;
    pull)
        pull "$@"
        ;;
    base_backup)
        base_backup "$@"
        ;;
    *)	(10)
        echo $"Unknown process!"
        RETVAL=1
esac
exit "$RETVAL"
