#!/bin/sh
set -e
if psql -h postgres -U apollo -d apollo -tAc "SELECT to_regclass('public.accounts')" | grep -q accounts; then
  echo "schema already loaded, skipping"
else
  psql -h postgres -U apollo -d apollo -v ON_ERROR_STOP=1 -f /schema.sql
  echo "schema loaded"
fi

# Idempotent patches for tables added after a deployment first loaded
# schema.sql. The full-schema path above only runs on a clean database, so
# existing installs pick up additions here.
if psql -h postgres -U apollo -d apollo -tAc "SELECT to_regclass('public.live_activities')" | grep -q live_activities; then
  echo "live_activities present, skipping patch"
else
  psql -h postgres -U apollo -d apollo -v ON_ERROR_STOP=1 -f /patches/000013_restore_live_activities.up.sql
  echo "live_activities patch applied"
fi
