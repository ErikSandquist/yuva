#!/bin/sh
# Starts the image's Postgres and prints the URL the api tests read:
#   export YUVA_TEST_DATABASE_URL="$(.oss-scanner/test-db.sh)"
#   cd api && go test ./internal/api/ -run TestName
set -e
version=$(ls /usr/lib/postgresql | sort -n | tail -n 1)
cluster=/var/lib/postgresql/yuva-test
if [ ! -d "$cluster" ]; then
  su postgres -c "/usr/lib/postgresql/$version/bin/initdb -D $cluster -A trust" >/dev/null
fi
if ! su postgres -c "/usr/lib/postgresql/$version/bin/pg_ctl -D $cluster status" >/dev/null 2>&1; then
  su postgres -c "/usr/lib/postgresql/$version/bin/pg_ctl -D $cluster -o '-k /tmp -c listen_addresses=127.0.0.1' -l /tmp/yuva-test-pg.log -w start" >/dev/null
fi
su postgres -c "psql -h 127.0.0.1 -tAc \"select 1 from pg_database where datname = 'yuva_test'\"" | grep -q 1 \
  || su postgres -c "createdb -h 127.0.0.1 yuva_test"
echo "postgres://postgres@127.0.0.1:5432/yuva_test?sslmode=disable"
