#!/bin/bash
set -e

# Initialize PostgreSQL if not already done
if [ ! -f "$PGDATA/PG_VERSION" ]; then
  echo "==> Initializing PostgreSQL..."
  su-exec postgres initdb -D "$PGDATA" --auth=trust
  echo "host all all 0.0.0.0/0 md5" >> "$PGDATA/pg_hba.conf"
  echo "listen_addresses='127.0.0.1'" >> "$PGDATA/postgresql.conf"
fi

# Start PostgreSQL
echo "==> Starting PostgreSQL..."
su-exec postgres pg_ctl -D "$PGDATA" -l /var/lib/postgresql/pg.log start -w

# Create user and database if not exist
su-exec postgres psql -tc "SELECT 1 FROM pg_roles WHERE rolname='$POSTGRES_USER'" | grep -q 1 || \
  su-exec postgres psql -c "CREATE USER $POSTGRES_USER WITH PASSWORD '$POSTGRES_PASSWORD';"
su-exec postgres psql -tc "SELECT 1 FROM pg_database WHERE datname='$POSTGRES_DB'" | grep -q 1 || \
  su-exec postgres psql -c "CREATE DATABASE $POSTGRES_DB OWNER $POSTGRES_USER;"

export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable"

echo "==> Starting Haru API server on port $PORT..."
exec /haru
