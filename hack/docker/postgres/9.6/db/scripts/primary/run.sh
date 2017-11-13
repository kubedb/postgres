#!/usr/bin/env bash

set -e

source /scripts/lib.sh

echo "Running as Primary"

# Initialize postgres
initialize

# Set password
set_password

# Configure postgreSQL.conf
configure_primary_postgres

# Configure pg_hba.conf
configure_pghba

exec postgres
