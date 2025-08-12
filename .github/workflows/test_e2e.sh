#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

scriptDir="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
rootDir=$(realpath "$scriptDir/../..")

function runBazelCommandTreatingWarningsAsErrors() {
  local CMD="$1"
  shift
  local OUTPUT_BASE_DIR=$(bazel info output_base)
  local BAZEL_COMMAND_LOG_FILE="$OUTPUT_BASE_DIR/command.log"

  # Disable colors and control characters to make grep work properly
  bazel "$CMD" --color=no --curses=no "$@"

  # Expect no warnings in the command output
  local COLLECTED_WARNINGS=$(grep "^WARNING" "$BAZEL_COMMAND_LOG_FILE" || true)
  if [[ -n "$COLLECTED_WARNINGS" ]]; then
    echo >&2
    echo "Unexpected warnings found:" >&2
    echo "$COLLECTED_WARNINGS" >&2
    exit 1
  fi
}

function testExampleBzlMod() {
  echo "Test example/bzlmod"
  cd "$rootDir/example/bzlmod"

  # Ensure previous BUILD files are removed
  rm -f mylib/BUILD.bazel proto/BUILD.bazel

  # Run gazelle to generate BUILD files
  runBazelCommandTreatingWarningsAsErrors run :gazelle

  # Verify that BUILD files were generated
  test -f mylib/BUILD.bazel
  test -f proto/BUILD.bazel

  bazel build //...
  bazel test --test_output=errors //...
  bazel run //proto:example
}

function testExampleWorkspace() {
  echo "Test example/workspace"
  cd "$rootDir/example/workspace"

  # Ensure previous BUILD files are removed
  rm -f mylib/BUILD.bazel app/BUILD.bazel

  # Run gazelle to generate BUILD files
  runBazelCommandTreatingWarningsAsErrors run :gazelle

  # Verify that BUILD files were generated
  test -f mylib/BUILD.bazel
  test -f app/BUILD.bazel

  bazel build //...
  bazel test --test_output=errors //...
  bazel run //app:main
}

testExampleBzlMod
testExampleWorkspace

cd $rootDir
