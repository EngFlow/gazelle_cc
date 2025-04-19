#!/bin/bash

REPO_ROOT=$(bazel info workspace)

bazel test "//index:integration_tests" \
  --test_env=REPOSITORY_ROOT="$REPO_ROOT" \
  # --test_output=streamed
