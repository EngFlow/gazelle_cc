#!/usr/bin/env bash

# Bazel expects the helper to read stdin.
cat /dev/stdin > /dev/null

# `OPAL_RPC_CREDENTIALS` is provided as a secret.
echo "${OPAL_RPC_CREDENTIALS}"
