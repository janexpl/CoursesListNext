#!/bin/sh
# Inicjalizacja bazy deweloperskiej (profil `db` w docker-compose.dev.yml).
#
# Schemat bierzemy ze zrzutu bazy produkcyjnej (stan po migracji 0018) i nakładamy
# nowsze migracje - dokładnie tak, jak robią to testy integracyjne. internal/db/schema.sql
# się tu nie nadaje: służy sqlc i deklaruje klucze główne bez sekwencji, więc wstawienie
# wiersza bez jawnego id kończyłoby się błędem.
set -e

BASELINE_MIGRATION=18

echo "== schemat bazowy"
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /dev-schema/baseline_schema.sql

for file in /dev-migrations/*.sql; do
    number=$(basename "$file" | cut -c1-4 | sed 's/^0*//')
    [ -n "$number" ] || continue
    [ "$number" -gt "$BASELINE_MIGRATION" ] || continue
    echo "== migracja $(basename "$file")"
    psql --single-transaction -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$file"
done
