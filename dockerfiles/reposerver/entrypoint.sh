#!/bin/sh
set -e

echo "Starting DBT Reposerver..."
echo "  ADDRESS: ${ADDRESS}"
echo "  PORT: ${PORT}"
echo "  SERVER_ROOT: ${SERVER_ROOT}"

if [ -e "${CONFIG_FILE}" ]; then
  echo "  CONFIG_FILE: ${CONFIG_FILE}"
  exec /app/reposerver -a "${ADDRESS}" -p "${PORT}" -r "${SERVER_ROOT}" -f "${CONFIG_FILE}"
else
  echo "  CONFIG_FILE: (not provided, using defaults)"
  exec /app/reposerver -a "${ADDRESS}" -p "${PORT}" -r "${SERVER_ROOT}"
fi
