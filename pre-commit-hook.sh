#!/usr/bin/env bash
/usr/local/go/bin/gofmt -w ./

METADATA_VERSION=$(grep version metadata.json | awk '{print $2}' | sed 's/[",]//g')

COMMAND_VERSION=$(grep Version: cmd/dbt/cmd/root.go | grep -v version | awk '{print $2}' | sed 's/"//g' | sed 's/,//g')

CODE_VERSION=$(grep "VERSION ="  pkg/dbt/dbt.go | awk '{print $4}' | sed 's/"//g')

if [[ ${METADATA_VERSION} != "${CODE_VERSION}" || "${CODE_VESION}" != "${COMMAND_VESION}" || "${COMMAND_VERSION}" != "${METADATA_VERSION}" ]] ; then
  echo "Versions do not match!"
  echo "Metadata: ${METADATA_VERSION}"
  echo "Command:  ${COMMAND_VERSION}"
  echo "Code:     ${CODE_VERSION}"

  echo "'VERSION' in cmd/dbt/cmd/root.go must match 'version' in metadata.json and in pkg/dbt/dbt.go"
  exit 1
fi
