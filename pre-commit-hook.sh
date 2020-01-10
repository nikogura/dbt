#!/usr/bin/env bash
/usr/local/go/bin/gofmt -w ./

METADATA_VERSION=$(grep version metadata.json | awk '{print $2}' | sed 's/[",]//g')

CODE_VERSION=$(grep Version: cmd/root.go | grep -v version | awk '{print $2}' | sed 's/"//g' | sed 's/,//g')

if [[ ${METADATA_VERSION} != ${CODE_VERSION} ]]; then
  echo "Versions do not match!"
  echo "'VERSION' in cmd/dbt/main.go must match 'version' in metadata.json"
  exit 1
fi
