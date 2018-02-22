#!/usr/bin/env bash

set -e

source /scripts/lib.sh

echo "Running as Primary"

export MODE="primary"

if [ ! -e "$PGDATA/PG_VERSION" ]; then

    if [ "$RESTORE" = true ]; then
        echo "Restoring Postgres from base_backup using wal-g"
        restore_from_walg
    else
        # Initialize postgres
        initialize

        # Set password
        set_password

        # Configure postgreSQL.conf
        configure_primary_postgres

        # Configure pg_hba.conf
        configure_pghba

        # Initialize database
        init_database

        # Push base_backup using wal-g if possible
        push_backup
    fi
else
    if [[ -v ARCHIVE ]]; then
        if [ "$ARCHIVE" == "wal-g" ]; then
            export WALE_S3_PREFIX=$(echo "$ARCHIVE_S3_PREFIX")
            set_walg_env "/srv/wal-g/archive/secrets"
            create_pgpass_file
        fi
    fi
fi

exec postgres
