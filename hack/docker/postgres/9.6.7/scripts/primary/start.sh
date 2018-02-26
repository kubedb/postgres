#!/bin/bash

mkdir -p "$PGDATA"
rm -rf "$PGDATA"/*
chmod 0700 "$PGDATA"

initdb "$PGDATA" > /dev/null

# setup postgresql.conf
cp /scripts/primary/postgresql.conf /tmp
echo "wal_level = replica" >> /tmp/postgresql.conf
echo "max_wal_senders = 99" >> /tmp/postgresql.conf
echo "wal_keep_segments = 32" >> /tmp/postgresql.conf
mv /tmp/postgresql.conf "$PGDATA/postgresql.conf"

# setup pg_hba.conf
{ echo; echo 'local all         all                         trust'; }   >> "$PGDATA/pg_hba.conf"
{       echo 'host  all         all         127.0.0.1/32    trust'; }   >> "$PGDATA/pg_hba.conf"
{       echo 'host  all         all         0.0.0.0/0       md5'; }     >> "$PGDATA/pg_hba.conf"
{       echo 'host  replication postgres    0.0.0.0/0       md5'; }     >> "$PGDATA/pg_hba.conf"

# start postgres
pg_ctl -D "$PGDATA"  -w start > /dev/null

# alter postgres superuser
psql --username postgres <<-EOSQL
ALTER USER postgres WITH SUPERUSER PASSWORD '$PGPASSWORD';
EOSQL

# initialize database
psql=( psql -v ON_ERROR_STOP=1 --username "postgres" --dbname "postgres" )
for f in "$INITDB"/*; do
    case "$f" in
        *.sh)     echo "$0: running $f"; . "$f" ;;
        *.sql)    echo "$0: running $f"; "${psql[@]}" -f "$f"; echo ;;
        *.sql.gz) echo "$0: running $f"; gunzip -c "$f" | "${psql[@]}"; echo ;;
        *)        echo "$0: ignoring $f" ;;
    esac
    echo
done

# stop server
pg_ctl -D "$PGDATA" -m fast -w stop > /dev/null


if [ "$ARCHIVE" == "wal-g" ]; then
    # setup postgresql.conf
    echo "archive_command = 'wal-g wal-push %p'" >> "$PGDATA/postgresql.conf"
    echo "archive_timeout = 60" >> "$PGDATA/postgresql.conf"
    echo "archive_mode = always" >> "$PGDATA/postgresql.conf"
fi
