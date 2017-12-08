#!/usr/bin/env bash

set -e

source /scripts/lib.sh

echo "Running as Replica"

export MODE="replica"

reset_owner

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

# Push base_backup using wal-g if possible
push_backup

exec postgres
