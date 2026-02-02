load("@bazel_skylib//lib:paths.bzl", "paths")

REPO_PREFIX = "compilation_test_"
MAIN_FILEGROUP_NAME = "all_workspace_rules"
WORKSPACE_FILES = ["WORKSPACE", "WORKSPACE.bazel", "MODULE.bazel"]

def _execute_find_cmd(ctx, base_directory, *filenames):
    find_command = ["find", base_directory, "-type", "f"]

    if filenames:
        find_command.extend(["(", "-name", filenames[0]])
        for filename in filenames[1:]:
            find_command.extend(["-o", "-name", filename])
        find_command.append(")")

    return ctx.execute(find_command)

def _compilation_test_repo_impl(repository_ctx):
    source_dir = repository_ctx.attr.source_dir
    filegroup_tool = repository_ctx.path(Label("@gazelle_cc//:scripts/generate_filegroups.sh"))

    repository_ctx.watch_tree(source_dir)
    repository_ctx.watch(filegroup_tool)

    # List all files in the source directory
    result = _execute_find_cmd(repository_ctx, source_dir)
    if result.return_code != 0:
        fail("Failed to list files in {}: {}".format(source_dir, result.stderr))

    for source_file in result.stdout.strip().split("\n"):
        dest_path = paths.relativize(source_file, source_dir)

        # Mirror the directory structure in the new repository
        repository_ctx.execute(["mkdir", "-p", paths.dirname(dest_path)])

        # Copy or symlink files into the new repository
        if source_file.endswith(repository_ctx.attr.suffix):
            # Replace .out suffix with .bazel and copy, since these files are to be modified by filegroup_tool
            dest_path = dest_path.removesuffix(repository_ctx.attr.suffix) + ".bazel"
            repository_ctx.execute(["cp", source_file, dest_path])
        else:
            # Symlink all other files for efficiency, they are read-only
            repository_ctx.symlink(source_file, dest_path)

    # Generate filegroups
    result = repository_ctx.execute([filegroup_tool, "-m", MAIN_FILEGROUP_NAME, "."] + repository_ctx.attr.rule_kinds)
    if result.return_code != 0:
        fail("Failed to generate filegroups in {}: {}".format(repository_ctx.path("."), result.stderr))

_compilation_test_repo = repository_rule(
    implementation = _compilation_test_repo_impl,
    attrs = {
        "source_dir": attr.string(
            mandatory = True,
            doc = "Source directory to copy files from",
        ),
        "rule_kinds": attr.string_list(
            allow_empty = False,
            mandatory = True,
            doc = "List of rule kinds to include in the generated filegroups",
        ),
        "suffix": attr.string(
            mandatory = True,
            doc = "Files with this suffix will be renamed to .bazel in the repository",
        ),
    },
    local = True,
)

def _get_module_path(module_ctx, module):
    module_file_path = module_ctx.path(Label("@{}//:MODULE.bazel".format(module.name)))
    if module_file_path.exists:
        return module_file_path.dirname

    fail("Could not find MODULE.bazel file in the root module")

def _find_workspace_paths(module_ctx, base_directory):
    result = _execute_find_cmd(module_ctx, base_directory, *WORKSPACE_FILES)
    if result.return_code != 0:
        fail("Failed to discover test workspaces in {}: {}".format(base_directory, result.stderr))

    return [paths.dirname(workspace_file) for workspace_file in result.stdout.strip().split("\n")]

def _make_repo_name(workspace_path):
    return REPO_PREFIX + workspace_path.replace("/", "_")

def _gazelle_compilation_tests_impl(module_ctx):
    generated_repos = []

    for module in module_ctx.modules:
        if not module.is_root:
            fail("gazelle_compilation_tests extension should only be used with dev_dependency=True in the root module")

        for discover_tag in module.tags.discover:
            absolute_base_directory = _get_module_path(module_ctx, module).get_child(discover_tag.base_directory)
            absolute_base_directory.readdir(watch = "yes")
            absolute_base_directory = str(absolute_base_directory)

            for workspace_path in _find_workspace_paths(module_ctx, absolute_base_directory):
                relative_workspace_path = paths.relativize(workspace_path, absolute_base_directory)
                generated_repo_name = _make_repo_name(relative_workspace_path)

                _compilation_test_repo(
                    name = generated_repo_name,
                    source_dir = str(workspace_path),
                    rule_kinds = discover_tag.rule_kinds,
                    suffix = discover_tag.suffix,
                )

                generated_repos.append(generated_repo_name)

    return module_ctx.extension_metadata(
        root_module_direct_deps = [],
        root_module_direct_dev_deps = generated_repos,
        reproducible = True,
    )

_discover_tag = tag_class(
    attrs = {
        "base_directory": attr.string(
            mandatory = True,
            doc = "Base directory to discover test workspaces (subdirectories containing WORKSPACE or MODULE.bazel files)",
        ),
        "rule_kinds": attr.string_list(
            allow_empty = False,
            default = ["cc_library", "cc_binary", "cc_test"],
            doc = "List of rule kinds to include in the generated filegroups",
        ),
        "suffix": attr.string(
            default = ".out",
            doc = "Files with this suffix will be renamed to .bazel in the test repositories",
        ),
    },
)

gazelle_compilation_tests = module_extension(
    implementation = _gazelle_compilation_tests_impl,
    tag_classes = {"discover": _discover_tag},
)

def gazelle_compilation_test(*, name, workspace_path):
    native.alias(
        name = name,
        actual = "@{}//:{}".format(_make_repo_name(workspace_path), MAIN_FILEGROUP_NAME),
    )
