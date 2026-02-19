load("@bazel_skylib//lib:paths.bzl", "paths")

# Prefix prepended to each generated repository to avoid name clashes with existing repositories.
GENERATED_REPO_PREFIX = "compilation_test_"

# Name of the build_test target in the root BUILD.bazel file of each generated repository.
# See: scripts/prepare_test_repo/prepare_test_repo.py
BUILD_TEST_NAME = "repo_build_test"

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
    repo_tool = repository_ctx.attr._repo_tool

    repository_ctx.watch_tree(source_dir)
    repository_ctx.watch(repo_tool)

    result = repository_ctx.execute([repo_tool, source_dir, "."])
    if result.return_code != 0:
        fail("Failed to generate filegroups in {}: {}".format(repository_ctx.path("."), result.stderr))

_compilation_test_repo = repository_rule(
    implementation = _compilation_test_repo_impl,
    attrs = {
        "source_dir": attr.string(
            doc = "Source directory to copy files from",
            mandatory = True,
        ),
        "_repo_tool": attr.label(
            doc = "Tool to copy files from the source directory and generate filegroups",
            default = Label("@gazelle_cc//scripts/prepare_test_repo:prepare_test_repo.py"),
            allow_single_file = True,
            cfg = "exec",
            executable = True,
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

def _generated_repo_name(test_dir):
    if len(test_dir.split("/")) != 1:
        fail("gazelle_compilation_tests does not support recursive discovery: {}".format(test_dir))

    return GENERATED_REPO_PREFIX + test_dir.replace("/", "_")

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
                generated_repo_name = _generated_repo_name(relative_workspace_path)

                _compilation_test_repo(
                    name = generated_repo_name,
                    source_dir = str(workspace_path),
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
            doc = "Base directory (relative to the repository root) to discover test repositories (subdirectories " +
                  "containing WORKSPACE or MODULE.bazel files)",
            mandatory = True,
        ),
    },
)

gazelle_compilation_tests = module_extension(
    implementation = _gazelle_compilation_tests_impl,
    tag_classes = {"discover": _discover_tag},
)

def gazelle_compilation_test(*, name, test_dir, **kwargs):
    """
    gazelle_compilation_test joins a generated compilation test into the scope of the root module.

    The compilation test becomes visible within `//...` wildcard and is covered by `bazel test //...` command.

    Args:
        name: Name of the created test_suite target
        test_dir: Name of the directory containing the test repository, relative to the "base_directory" attribute of
            the "discover" tag
        **kwargs: Attributes that are passed directly to the test_suite declaration
    """

    test_label = "@{repo}//:{target}".format(
        repo = _generated_repo_name(test_dir),
        target = BUILD_TEST_NAME,
    )

    # We can't use `native.alias` here. As mentioned in the docs:
    #
    #   Tests are not run if their alias is mentioned on the command line. To define an alias that runs the referenced
    #   test, use a test_suite rule with a single target in its tests attribute.
    #
    # https://bazel.build/reference/be/general#alias
    native.test_suite(
        name = name,
        tests = [test_label],
        **kwargs
    )
