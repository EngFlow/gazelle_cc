#!/usr/bin/env python3

import argparse
import ast
import os
import shutil
from pathlib import Path
from typing import Optional


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Prepare test repository by copying and transforming BUILD files. The input repository follows the layout "
            "used by gazelle_generation_test (with BUILD.in/BUILD.out files). The output is a real Bazel repository "
            "with a build_test target that checks compilation of all collected rules."
        ),
    )
    parser.add_argument(
        "input_repo",
        metavar="INPUT_REPO",
        type=Path,
        help="Input repository directory, or path to WORKSPACE/MODULE.bazel file (will use parent directory)",
    )
    parser.add_argument(
        "output_repo",
        metavar="OUTPUT_REPO",
        type=Path,
        help="Output repository directory",
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
        default="repo_build_test",
        help="Name of the public build_test target in the root BUILD.bazel file (default: repo_build_test)",
    )
    return parser.parse_args()


def parse_rule_name(node: ast.stmt) -> Optional[str]:
    if isinstance(node, ast.Expr) and isinstance(node.value, ast.Call) and isinstance(node.value.func, ast.Name):
        # Treat all top-level call expressions with a "name" keyword argument as Bazel rules
        for keyword in node.value.keywords:
            if keyword.arg == "name" and isinstance(keyword.value, ast.Constant):
                return str(keyword.value.value)

    return None


def parse_rule_names(build_file_content: str) -> set[str]:
    return {name for node in ast.parse(build_file_content).body if (name := parse_rule_name(node))}


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
    package_filegroup_name: str,
    build_test_name: str,
) -> None:
    """Copy directory structure, transform BUILD files, create filegroups, and add a public build_test target.

    Copies all files from input directory to output directory. BUILD files matching build_file_name are renamed to
    BUILD.bazel and appended with filegroups collecting all rules in a Bazel package. Finally, a public build_test
    target is added to the root BUILD.bazel that depends on all created filegroups.

    Args:
        input_dir: Source directory
        output_dir: Destination directory
        build_file_name: Name of BUILD files to rename to BUILD.bazel
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
                if rule_names := parse_rule_names(src_file.read_text()):
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
        input_dir=args.input_repo if args.input_repo.is_dir() else args.input_repo.parent,
        output_dir=args.output_repo,
        build_file_name=args.build_file_name,
        package_filegroup_name=args.package_filegroup_name,
        build_test_name=args.build_test_name,
    )


if __name__ == "__main__":
    main()
