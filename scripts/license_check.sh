#!/bin/bash
set -e  # Exit on script errors
shopt -s extglob

# Paths ignored when checking the headers
IGNORE_PATHS=(
  "language/cpp/testdata/*",
)
# Source extensions that should be checked
EXTS=(".go")

HEADER=$(cat <<EOF
// Copyright 2025 EngFlow, Inc. All rights reserved.
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

SEARCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"  # Resolve to parent dir
FIX_MODE=false

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


# Check for the --fix flag
if [[ "$1" == "--fix" ]]; then
  FIX_MODE=true
fi

headerLines=$(echo -e "$HEADER" | wc -l)
startsWithHeader() {
  file="$1"
  
  # Normalize both first n lines of file and the header
  file_content=$(head -n $headerLines "$file" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
  expected_text=$(echo "$HEADER" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
  
  # Compare processed content with expected text
  if [[ "$file_content" != "$expected_text" ]]; then
    return 1 
  fi
}

# Find and check files in subdirectories
MISSING_FILES=()
for ext in "${EXTS[@]}"; do
  for file in $(find "$SEARCH_DIR" -type f -name "*$ext"); do
    if should_ignore "$file"; then
        continue
    fi
    
    # Check if the header is missing
    if ! startsWithHeader $file; then
      MISSING_FILES+=("$file")
      if $FIX_MODE; then
        # Create a temporary file with the header + existing content
        tmp_file=$(mktemp)
        echo -e "$HEADER\n" > "$tmp_file"
        cat "$file" >> "$tmp_file"
        mv "$tmp_file" "$file"
        echo "Added missing header: $file"
      else
        echo "Missing header: $file"
      fi
    fi
  done
done

# If any files were missing the header, return a non-zero exit code
if [[ ${#MISSING_FILES[@]} -gt 0 ]]; then
  if ! $FIX_MODE; then
    echo "Found ${#MISSING_FILES[@]} files without license header, use '$0 --fix' add fix this."
    exit 1
  fi
fi
