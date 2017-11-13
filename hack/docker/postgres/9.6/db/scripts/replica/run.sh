#!/usr/bin/env bash

set -e

source /scripts/lib.sh

echo "Running as Replica"

rm -rf "$PGDATA/*"
chmod 0700 "$PGDATA"

# Load password
load_password

# Create PGPASSFILE
create_pgpass_file

# Waiting for running Postgres
wait_for_running

# Get basebackup
base_backup

# Configure postgreSQL.conf
configure_replica_postgres

postgres -D "$PGDATA"

exec postgres
