#!/bin/bash
set -e  # Exit on script errors
shopt -s extglob

# Paths ignored when checking the headers
IGNORE_PATHS=(
  "example/*"
  "index/internal/bazel/proto/build.proto"
  "language/cc/testdata/*"
)

# Source extensions that should be checked
EXTS=(
  ".go"
  ".proto"
)

THIS_YEAR=$(date +"%Y")

HEADER=$(cat <<EOF
// Copyright $THIS_YEAR EngFlow Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
EOF
)

HEADER_LINES=$(echo "$HEADER" | wc -l)

SEARCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"  # Resolve to parent dir

# Check for the --fix flag
if [[ "$1" == "--fix" ]]; then
  FIX_MODE=true
else
  FIX_MODE=false
fi

# Function to check if a file should be ignored in header checks
should_ignore() {
  file="$1"
  for pattern in "${IGNORE_PATHS[@]}"; do
    if [[ "$file" == $SEARCH_DIR/$pattern ]]; then
      return 0  # File matches an ignore pattern
    fi
  done
  return 1  # File does not match any ignore patterns
}

# Check if the file starts with the expected header
starts_with_header() {
  file="$1"
  
  # Normalize both first n lines of file and the header
  file_content=$(head -n $HEADER_LINES "$file" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
  expected_text=$(echo "$HEADER" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
  
  # Compare processed content with expected text
  if [[ "$file_content" != "$expected_text" ]]; then
    return 1 
  fi
}

# Strip the old header (if exists) from the file
strip_old_header() {
  file="$1"

  # Find the first line not starting with "//"
  first_non_header_line=$(awk '/^\/\// {next} {print NR; exit}' "$file")

  # Print the rest of the file
  tail -n +$first_non_header_line "$file"
}

# Find and check files in subdirectories
MISSING_FILES=()
for ext in "${EXTS[@]}"; do
  for file in $(find "$SEARCH_DIR" -type f -name "*$ext"); do
    if should_ignore "$file" || starts_with_header "$file"; then
        continue
    fi

    MISSING_FILES+=("$file")

    if $FIX_MODE; then
      # Create a temporary file with the header + existing content
      tmp_file=$(mktemp)
      echo "$HEADER" > "$tmp_file"
      strip_old_header "$file" >> "$tmp_file"
      mv "$tmp_file" "$file"
      echo "Added missing header: $file"
    else
      echo "Missing/incorrect header: $file"
    fi
  done
done

# If any files were missing the header, return a non-zero exit code
if [[ ${#MISSING_FILES[@]} -gt 0 ]] && ! $FIX_MODE; then
  echo "Found ${#MISSING_FILES[@]} file(s) with no license header or an incorrect one. Use '$0 --fix' to fix this."
  exit 1
fi
