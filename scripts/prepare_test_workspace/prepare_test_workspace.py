#!/usr/bin/env python3

import ast
import os
import shutil
from argparse import ArgumentParser
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Optional


@dataclass
class CmdlineArgs:
    input_workspace: Path
    output_workspace: Path
    rule_kinds: set[str]
    build_file_name: str
    package_filegroup_name: str
    build_test_name: str


@dataclass
class Rule:
    kind: str
    name: str


def parse_args() -> CmdlineArgs:
    parser = ArgumentParser(
        description=(
            "Prepare test workspace by copying and transforming BUILD files. The input workspace follows the layout "
            "used by gazelle_generation_test (with BUILD.in/BUILD.out files). The output is a real Bazel workspace "
            "with a build_test target that checks compilation of all specified rule kinds."
        ),
    )
    parser.add_argument(
        "input_workspace",
        metavar="INPUT_WORKSPACE",
        type=Path,
        help="Input workspace directory, or path to WORKSPACE/MODULE.bazel file (will use parent directory)",
    )
    parser.add_argument(
        "output_workspace",
        metavar="OUTPUT_WORKSPACE",
        type=Path,
        help="Output workspace directory",
    )
    parser.add_argument(
        "rule_kinds",
        metavar="RULE_KIND",
        nargs="+",
        help="Rule kinds to collect (at least one required)",
    )
    parser.add_argument(
        "-b",
        dest="build_file_name",
        metavar="FILE_NAME",
        default="BUILD.out",
        help="Name of BUILD files to rename to BUILD.bazel (default: BUILD.out)",
    )
    parser.add_argument(
        "-p",
        dest="package_filegroup_name",
        metavar="NAME",
        default="package_rules",
        help="Name of the filegroup to create in each package (default: package_rules)",
    )
    parser.add_argument(
        "-t",
        dest="build_test_name",
        metavar="NAME",
        default="workspace_build_test",
        help="Name of the public build_test target in the root BUILD.bazel file (default: workspace_build_test)",
    )
    raw_args = parser.parse_args()
    input_workspace = raw_args.input_workspace if raw_args.input_workspace.is_dir() else raw_args.input_workspace.parent

    return CmdlineArgs(
        input_workspace=input_workspace,
        output_workspace=raw_args.output_workspace,
        rule_kinds=set(raw_args.rule_kinds),
        build_file_name=raw_args.build_file_name,
        package_filegroup_name=raw_args.package_filegroup_name,
        build_test_name=raw_args.build_test_name,
    )


def parse_rule(node: ast.stmt) -> Optional[Rule]:
    if (
        not isinstance(node, ast.Expr)
        or not isinstance(node.value, ast.Call)
        or not isinstance(node.value.func, ast.Name)
    ):
        return None

    for keyword in node.value.keywords:
        if keyword.arg == "name" and isinstance(keyword.value, ast.Constant):
            return Rule(kind=str(node.value.func.id), name=str(keyword.value.value))

    return None


def parse_rules(content: str) -> Iterable[Rule]:
    for node in ast.parse(content).body:
        if rule := parse_rule(node):
            yield rule


def collect_rule_names_from_file(build_file: Path, rule_kinds: set[str]) -> set[str]:
    with open(build_file, "r") as f:
        return {rule.name for rule in parse_rules(f.read()) if rule.kind in rule_kinds}


def append_filegroup(build_file: Path, filegroup_name: str, rule_names: set[str]) -> None:
    with open(build_file, "a") as f:
        f.write("\nfilegroup(\n")
        f.write(f'    name = "{filegroup_name}",\n')
        f.write("    srcs = [\n")
        f.writelines(f'        ":{name}",\n' for name in sorted(rule_names))
        f.write("    ],\n")
        f.write("    testonly = True,\n")
        f.write('    visibility = ["//:__pkg__"],\n')
        f.write(")\n")


def append_build_test(build_file: Path, build_test_name: str, filegroup_labels: set[str]) -> None:
    # Read original content first (if file exists)
    original_content = build_file.read_text() if build_file.exists() else ""

    with open(build_file, "w") as f:
        # Add load statement at the beginning
        f.write('load("@bazel_skylib//rules:build_test.bzl", "build_test")\n\n')

        # Preserve the original content
        f.write(original_content)

        # Append build_test at the end
        f.write("\nbuild_test(\n")
        f.write(f'    name = "{build_test_name}",\n')
        f.write("    targets = [\n")
        f.writelines(f'        "{label}",\n' for label in sorted(filegroup_labels))
        f.write("    ],\n")
        f.write('    visibility = ["//visibility:public"],\n')
        f.write(")\n")


def copy_and_transform(
    input_dir: Path,
    output_dir: Path,
    build_file_name: str,
    rule_kinds: set[str],
    package_filegroup_name: str,
    build_test_name: str,
) -> None:
    """Copy directory structure, transform BUILD files, create filegroups, and add a public build_test target.

    Copies all files from input directory to output directory. BUILD files matching build_file_name are renamed to
    BUILD.bazel and appended with filegroups collecting rules of specified kinds. Finally, a public build_test target
    is added to the root BUILD.bazel that depends on all created filegroups.

    Args:
        input_dir: Source directory
        output_dir: Destination directory
        build_file_name: Name of BUILD files to rename to BUILD.bazel
        rule_kinds: Rule kinds to collect
        package_filegroup_name: Name for the filegroup to create in each package
        build_test_name: Name for the top-level build_test target
    """
    filegroup_labels: set[str] = set()

    # Create output directory if it doesn't exist
    output_dir.mkdir(parents=True, exist_ok=True)

    # Walk through input directory
    for root, _, files in os.walk(input_dir):
        rel_root = Path(root).relative_to(input_dir)
        bazel_package = "//" if rel_root == Path(".") else f"//{rel_root}"
        filegroup_label = f"{bazel_package}:{package_filegroup_name}"

        # Create corresponding directory in output
        out_root = output_dir / rel_root
        out_root.mkdir(parents=True, exist_ok=True)

        for file in files:
            src_file = Path(root) / file

            if file == build_file_name:
                # If this is a BUILD.bazel file (after renaming), append a filegroup with the collected rules
                dest_file = out_root / "BUILD.bazel"
                shutil.copy2(src_file, dest_file)
                if rule_names := collect_rule_names_from_file(dest_file, rule_kinds):
                    append_filegroup(dest_file, package_filegroup_name, rule_names)
                    filegroup_labels.add(filegroup_label)
            else:
                # Just copy other files as-is
                dest_file = out_root / file
                shutil.copy2(src_file, dest_file)

    # Finally add the public top-level build_test target that depends on all package filegroups
    append_build_test(output_dir / "BUILD.bazel", build_test_name, filegroup_labels)


def main() -> None:
    args = parse_args()

    copy_and_transform(
        input_dir=args.input_workspace,
        output_dir=args.output_workspace,
        build_file_name=args.build_file_name,
        rule_kinds=args.rule_kinds,
        package_filegroup_name=args.package_filegroup_name,
        build_test_name=args.build_test_name,
    )


if __name__ == "__main__":
    main()
