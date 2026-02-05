#!/usr/bin/env python3

import ast
import os
import shutil
import argparse
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, Optional


def parse_args() -> argparse.Namespace:
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
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
        dest="test_name",
        metavar="NAME",
        default="workspace_build_test",
        help="Name of the public build_test target in the root BUILD.bazel file (default: workspace_build_test)",
    )
    return parser.parse_args()


@dataclass
class Rule:
    kind: str
    name: str


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


def collect_rules_from_file(build_file: Path, rule_kinds: set[str]) -> set[str]:
    with open(build_file, "r") as f:
        return {rule.name for rule in parse_rules(f.read()) if rule.kind in rule_kinds}


def append_filegroup(build_file: Path, filegroup_name: str, rule_names: set[str]) -> None:
    with open(build_file, "a") as f:
        f.write("\nfilegroup(\n")
        f.write(f'    name = "{filegroup_name}",\n')
        f.write("    srcs = [\n")
        for name in sorted(rule_names):
            f.write(f'        ":{name}",\n')
        f.write("    ],\n")
        f.write("    testonly = True,\n")
        f.write('    visibility = ["//:__pkg__"],\n')
        f.write(")\n")


def copy_and_transform(
    input_dir: Path, output_dir: Path, build_file_name: str, rule_kinds: set[str], package_filegroup_name: str
) -> set[str]:
    """Copy directory structure, transform BUILD files, and create filegroups collecting specified rule kinds.

    Args:
        input_dir: Source directory
        output_dir: Destination directory
        build_file_name: Name of BUILD files to rename to BUILD.bazel
        rule_kinds: Rule kinds to collect
        package_filegroup_name: Name for the filegroup to create in each package

    Returns:
        Set of created filegroup labels
    """
    filegroup_labels = set()

    # Create output directory if it doesn't exist
    output_dir.mkdir(parents=True, exist_ok=True)

    # Walk through input directory
    for root, dirs, files in os.walk(input_dir):
        rel_root = Path(root).relative_to(input_dir)
        bazel_package = "//" if rel_root == Path(".") else f"//{rel_root}"

        # Create corresponding directory in output
        out_root = output_dir / rel_root
        out_root.mkdir(parents=True, exist_ok=True)

        for file in files:
            src_file = Path(root) / file
            dest_file = out_root / "BUILD.bazel" if file == build_file_name else out_root / file

            shutil.copy2(src_file, dest_file)

            # If this is a .bazel file (after renaming), append a filegroup with the collected rules
            if dest_file.suffix == ".bazel":
                rule_names = collect_rules_from_file(dest_file, rule_kinds)
                if rule_names:
                    append_filegroup(dest_file, package_filegroup_name, rule_names)
                    filegroup_labels.add(f"{bazel_package}:{package_filegroup_name}")

    return filegroup_labels


def update_root_build_file(output_dir: Path, filegroup_labels: set[str], test_name: str) -> None:
    build_file = output_dir / "BUILD.bazel"

    # Check if file exists
    if build_file.exists():
        with open(build_file, "r") as f:
            content = f.read()
    else:
        content = ""

    # Add load statement at the beginning
    load_stmt = 'load("@bazel_skylib//rules:build_test.bzl", "build_test")\n\n'
    content = load_stmt + content

    # Append build_test at the end
    with open(build_file, "w") as f:
        f.write(content)
        f.write("\nbuild_test(\n")
        f.write(f'    name = "{test_name}",\n')
        f.write("    targets = [\n")
        for label in sorted(filegroup_labels):
            f.write(f'        "{label}",\n')
        f.write("    ],\n")
        f.write('    visibility = ["//visibility:public"],\n')
        f.write(")\n")


def main() -> None:
    args = parse_args()

    input_dir = args.input_workspace.parent if args.input_workspace.is_file() else args.input_workspace

    filegroup_labels = copy_and_transform(
        input_dir, args.output_workspace, args.build_file_name, args.rule_kinds, args.package_filegroup_name
    )

    update_root_build_file(args.output_workspace, filegroup_labels, args.test_name)


if __name__ == "__main__":
    main()
