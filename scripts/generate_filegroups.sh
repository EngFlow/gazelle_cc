#!/usr/bin/env sh
set -euo pipefail

ARG_BUILD_FILE_NAME="BUILD.bazel"
ARG_PACKAGE_FILEGROUP_NAME="package_rules"
ARG_MAIN_FILEGROUP_NAME="all_workspace_rules"
ARG_WORKSPACE_ROOT=""
ARG_RULE_KINDS=""

help() {
    name=$(basename $0)
cat << EOF
Usage:
  $name [options] WORKSPACE_ROOT RULE_KIND [RULE_KIND ...]

Generates filegroups in Bazel BUILD files aggregating all the targets of
specified rule kinds. Parsing rule expressions is done using very primitive
pattern matching to extract rule kinds and names. It may not work in all cases,
but it should be sufficient for simple, well-formatted BUILD files.

Arguments:
  WORKSPACE_ROOT    Root directory of the Bazel workspace.
  RULE_KIND         At least one rule kind to include in filegroups.

Options:
  -h, --help    Show this help message and exit.
  -b NAME       Name of the BUILD files to process (default: "$ARG_BUILD_FILE_NAME").
  -p NAME       Name of the filegroup to create in each package (default: "$ARG_PACKAGE_FILEGROUP_NAME").
  -m NAME       Name of the main filegroup in the workspace (default: "$ARG_MAIN_FILEGROUP_NAME").

Example:
  $name /path/to/workspace cc_library cc_binary
EOF
}

errmsg_and_exit() {
    echo "$@" >&2
    exit 1
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
        -h|--help)
            help
            exit 0
        ;;
        -b)
            ARG_BUILD_FILE_NAME="$2"
            shift 2
        ;;
        -p)
            ARG_PACKAGE_FILEGROUP_NAME="$2"
            shift 2
        ;;
        -m)
            ARG_MAIN_FILEGROUP_NAME="$2"
            shift 2
        ;;
        *)
            if [ -z "$ARG_WORKSPACE_ROOT" ]; then
                ARG_WORKSPACE_ROOT="$1"
            else
                # Join rule kinds with '|' operator for regex matching
                ARG_RULE_KINDS="${ARG_RULE_KINDS+$ARG_RULE_KINDS|}$1"
            fi
            shift
        ;;
        esac
    done

    if [ -z "$ARG_WORKSPACE_ROOT" ]; then
        errmsg_and_exit "WORKSPACE_ROOT is required"
    elif [ -z "$ARG_RULE_KINDS" ]; then
        errmsg_and_exit "at least one RULE_KIND item is required"
    fi
}

filegroup_expr() {
    name="$1"
    shift
    labels="$@"

    echo
    echo "filegroup("
    echo "    name = \"$name\","
    echo "    srcs = ["
    for label in $labels; do
        echo "        \"$label\","
    done
    echo "    ],"
    echo "    testonly = True,"
    echo "    visibility = [\"//visibility:public\"],"
    echo ")"
}

parse_rules() {
    build_file="$1"

    # State machine states: "scan_rules", "in_rule"
    # - "scan_rules": looking for rule kind
    # - "in_rule": looking for rule name
    state="scan_rules"

    while read line; do
        if [ "$state" = "scan_rules" ] && echo "$line" | grep -Eq "($ARG_RULE_KINDS)\("; then
            state="in_rule"
        elif [ "$state" = "in_rule" ] && echo "$line" | grep -Eq 'name\s*=\s*"'; then
            rule_name="${line#*\"}"
            rule_name="${rule_name%%\"*}"
            echo ":$rule_name"
            state="scan_rules"
        fi
    done <"$build_file"
}

append_filegroup() {
    name="$1"
    package="$2"
    build_file="$3"
    shift 3
    labels="$@"

    filegroup_expr "$name" $labels >> "$build_file"
    echo "//$package:$name"
}

walk_and_generate_package_filegroups() {
    find "$ARG_WORKSPACE_ROOT" -name "$ARG_BUILD_FILE_NAME" | while read build_file; do
        package_dir="$(dirname "$build_file")"

        if [ "$package_dir" = "$ARG_WORKSPACE_ROOT" ]; then
            package=""
        else
            package="${package_dir#$ARG_WORKSPACE_ROOT/}"
        fi

        labels=$(parse_rules "$build_file")
        append_filegroup "$ARG_PACKAGE_FILEGROUP_NAME" "$package" "$build_file" $labels
    done
}

main() {
    parse_args "$@"

    # Package-level filegroups
    filegroups=$(walk_and_generate_package_filegroups)

    # Main workspace-level filegroup aggregating all rules
    append_filegroup "$ARG_MAIN_FILEGROUP_NAME" "" "$ARG_WORKSPACE_ROOT/$ARG_BUILD_FILE_NAME" $filegroups
}

main "$@"
