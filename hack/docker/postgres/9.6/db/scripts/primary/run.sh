#!/usr/bin/env bash

set -e

source /scripts/lib.sh

echo "Running as Primary"

if [ ! -s "$PGDATA/PG_VERSION" ]; then
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
fi

exec postgres
