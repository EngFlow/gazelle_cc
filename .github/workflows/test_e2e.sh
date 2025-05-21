#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

scriptDir="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
rootDir=$(realpath "$scriptDir/../..")

function testExampleBzlMod() {
  echo "Test example/bzlmod"
  cd "$rootDir/example/bzlmod"
  
  # Ensure previous BUILD files are removed
  rm -f mylib/BUILD.bazel proto/BUILD.bazel
  
  # Run gazelle to generate BUILD files
  bazel run :gazelle
  
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
  bazel run :gazelle
  
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
