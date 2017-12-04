#!/bin/bash

exec 1> >(logger -s -p daemon.info -t pg)
exec 2> >(logger -s -p daemon.error -t pg)

RETVAL=0

backup() {
    # 1 - host
    # 2 - username
    # 3 - password

    path=/var/pg_dumpall
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

    path=/var/pg_dumpall/
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

    src_path=/var/pg_dumpall/dumpfile.sql
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

    dst_path=/var/pg_dumpall/
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
