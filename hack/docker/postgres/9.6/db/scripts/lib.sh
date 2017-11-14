#!/usr/bin/env bash

reset_owner() {
	chown -R postgres:postgres "$PGDATA"
	chmod g+s /var/run/postgresql
	chown -R postgres /var/run/postgresql
}

initialize() {
	mkdir -p "$PGDATA"
	rm -rf "$PGDATA/*"
	reset_owner
	initdb "$PGDATA"
}

set_password() {
	load_password
	pg_ctl -D $PGDATA  -w start

	psql --username postgres <<-EOSQL
ALTER USER postgres WITH SUPERUSER PASSWORD '$POSTGRES_PASSWORD';
EOSQL
	pg_ctl -D $PGDATA -m fast -w stop
}

load_password() {
    PASSWORD_PATH='/srv/postgres/secrets/.admin'
	###### get postgres user password ######
	if [ -f "$PASSWORD_PATH" ]; then
		POSTGRES_PASSWORD=$(cat "$PASSWORD_PATH/POSTGRES_PASSWORD")
	else
		echo
		echo 'Missing environment file '${PASSWORD_PATH}'. Using default password.'
		echo
		POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-postgres}
	fi
}

use_standby() {
    echo "Creating wal directory at " "$PGWAL"
    mkdir -p "$PGWAL"
    chmod 0700 "$PGWAL"

    # Adding additional configuration in /tmp/postgresql.conf
    echo "# ====== Archiving ======" >> /tmp/postgresql.conf
    echo "archive_mode = always" >> /tmp/postgresql.conf
    echo "archive_command = 'test ! -f $PGWAL/%f && cp %p $PGWAL/%f'" >> /tmp/postgresql.conf
    echo "archive_timeout = 0" >> /tmp/postgresql.conf
    echo "# ====== Archiving ======" >> /tmp/postgresql.conf
    echo "# ====== WRITE AHEAD LOG ======" >> /tmp/postgresql.conf
    echo "wal_level = $1" >> /tmp/postgresql.conf
    echo "max_wal_senders = 99" >> /tmp/postgresql.conf
    echo "wal_keep_segments = 32" >> /tmp/postgresql.conf

    if [[ -v STREAMING ]]; then
        if [ "$STREAMING" == "synchronous" ]; then
            echo "synchronous_commit = on" >> /tmp/postgresql.conf
            #
            echo "synchronous_standby_names = '3 (*)'" >> /tmp/postgresql.conf
        fi
    fi

    echo "# ====== WRITE AHEAD LOG ======" >> /tmp/postgresql.conf
}

configure_primary_postgres() {
    if [[ -v STANDBY ]]; then
        if [ "$STANDBY" == "warm" ]; then
            use_standby "archive"
        elif [ "$STANDBY" == "hot" ]; then
            use_standby "hot_standby"
        fi
    fi

    if [ -s /tmp/postgresql.conf ]; then
        cat /tmp/postgresql.conf >> "$PGDATA/postgresql.conf"
    fi
}

configure_replica_postgres() {
    if [[ -v STANDBY ]]; then
        if [ "$STANDBY" == "hot" ]; then
            echo "hot_standby = on" >> /tmp/postgresql.conf
        fi
    fi

    if [ -s /tmp/postgresql.conf ]; then
        cat /tmp/postgresql.conf >> "$PGDATA/postgresql.conf"
    fi
}

configure_pghba() {
	{ echo; echo 'local all         all                         trust'; }   >> "$PGDATA/pg_hba.conf"
	{       echo 'host  all         all         127.0.0.1/32    trust'; }   >> "$PGDATA/pg_hba.conf"
	{       echo 'host  all         all         0.0.0.0/0       md5'; }     >> "$PGDATA/pg_hba.conf"
	{       echo 'host  replication postgres    0.0.0.0/0       md5'; }     >> "$PGDATA/pg_hba.conf"
}

create_pgpass_file() {
cat >> "/tmp/.pgpass" <<-EOF
*:*:*:*:${POSTGRES_PASSWORD}
EOF
chmod 0600 "/tmp/.pgpass"
export PGPASSFILE=/tmp/.pgpass
}

wait_for_running() {
	while true; do
		pg_isready --host="$PRIMARY_HOST" --timeout=2 &>/dev/null && break
		echo "Attempting pg_isready on primary"
		sleep 2
	done

	while true; do
		psql -h "$PRIMARY_HOST" --no-password --command="select now();" &>/dev/null && break
		echo "Attempting query on primary"
		sleep 2
	done
}

base_backup() {
    pg_basebackup -X fetch --no-password --pgdata "$PGDATA" --host="$PRIMARY_HOST"

    cp /scripts/replica/recovery.conf /tmp
    echo "restore_command = 'cp $PGWAL/%f %p'" >> /tmp/recovery.conf
    echo "archive_cleanup_command = 'pg_archivecleanup $PGWAL %r'" >> /tmp/recovery.conf
    # primary_conninfo is used for streaming replication
    echo "primary_conninfo = 'application_name=$HOSTNAME host=$PRIMARY_HOST'" >> /tmp/recovery.conf
    cp /tmp/recovery.conf "$PGDATA/recovery.conf"
}
