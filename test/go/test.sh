#!/bin/bash

go test ./...
if [ $? -ne 0 ]; then
  exit 1
fi

go vet ./...
if [ $? -ne 0 ]; then
  exit 1
fi

staticcheck ./...
if [ $? -ne 0 ]; then
  exit 1
fi

echo "=================================================================================="
echo "                             GO LINTING PASSED"
echo "=================================================================================="
