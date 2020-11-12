#!/bin/sh

echo "Running DBT Reposerver with:"

echo "  ADDRESS: ${ADDRESS}"
echo "  PORT: ${PORT}"
echo "  SERVER_ROOT: ${SERVER_ROOT}"

if [ -e "${CONFIG_FILE}" ]; then
  echo "  CONFIG_FILE: ${CONFIG_FILE}"
  /usr/local/bin/reposerver -a "${ADDRESS}" -p "${PORT}" -r "${SERVER_ROOT}" -f "${CONFIG_FILE}"
else
  /usr/local/bin/reposerver -a "${ADDRESS}" -p "${PORT}" -r "${SERVER_ROOT}"
fi

