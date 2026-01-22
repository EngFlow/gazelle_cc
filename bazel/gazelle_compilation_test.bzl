load("@bazel_binaries//:defs.bzl", "bazel_binaries")
load("@bazel_skylib//lib:paths.bzl", "paths")
load("@rules_bazel_integration_test//bazel_integration_test:defs.bzl", "bazel_integration_test")

def _rename_input_file(ctx, root_dir, input_file):
    output_basename = "BUILD.bazel" if input_file.basename == "BUILD.out" else input_file.basename
    output_dir = paths.relativize(input_file.dirname, ctx.label.package)
    output_path = paths.join(root_dir, output_dir, output_basename)
    return ctx.actions.declare_file(output_path)

def _convert_directory_structure_impl(ctx):
    # Ignore BUILD.in files
    input_files = [file for file in ctx.files.test_data if file.basename != "BUILD.in"]

    # Rename BUILD.out to BUILD.bazel
    root_dir = ctx.attr.name + "_"
    output_files = [_rename_input_file(ctx, root_dir, input_file) for input_file in input_files]

    # Copy file contents
    for input_file, output_file in zip(input_files, output_files):
        ctx.actions.run_shell(
            mnemonic = "GazelleTestConversion",
            inputs = [input_file],
            outputs = [output_file],
            arguments = [input_file.path, output_file.path],
            command = 'cp "$1" "$2"',
        )

    return DefaultInfo(files = depset(output_files))

convert_directory_structure = rule(
    doc = """
    Prepares a test workspace intended for gazelle_generation_test() to make it
    buildable by Bazel. To achieve this, all BUILD.out files are renamed to
    BUILD.bazel files.
    """,
    implementation = _convert_directory_structure_impl,
    attrs = {
        "test_data": attr.label_list(
            allow_files = True,
            mandatory = True,
            doc = "Workspace contents intended for gazelle_generation_test()",
        ),
    },
)

def gazelle_compilation_test(name, test_data, **kwargs):
    convert_directory_structure(
        name = name + "_workspace",
        test_data = test_data,
        testonly = True,
    )

    # Resolve as parent directory of the shallowest file path
    workspace_path = paths.dirname(min(test_data, key = lambda p: len(p.split("/"))))

    bazel_integration_test(
        name = name,
        bazel_binaries = bazel_binaries,
        bazel_version = bazel_binaries.versions.current,
        test_runner = "//bazel:bazel_builder",
        workspace_files = [":" + name + "_workspace"],
        workspace_path = workspace_path,
        **kwargs
    )
