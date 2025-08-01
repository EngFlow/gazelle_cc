# Gazelle C++ Extension

This repository contains a [Gazelle](https://github.com/bazel-contrib/bazel-gazelle) extension for C++ projects.

Gazelle is a build file generator for Bazel projects. This extension adds support for automatically generating and maintaining BUILD files for C/C++ codebases.

## Installation

### Using Bzlmod

Add the following to your `MODULE.bazel` file:

```bazel
bazel_dep(name = "gazelle", version = "0.42.0")
bazel_dep(name = "gazelle_cc", version = "0.1.0") # This extension, use the latest version

bazel_dep(name = "rules_cc", version = "0.1.1")
```

Add the `gazelle` task in the top-level `BUILD.bazel` file:

```bazel
load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

# Define a gazelle binary with a list of enabled extensions
gazelle_binary(
    name = "gazelle_cc",
    languages = [
        "@gazelle//language/proto",  # Optional, should be defined before cc 
        "@gazelle_cc//language/cc",
    ],
)

# `gazelle` rule can be used to provide additional arguments, eg. for CI integration
gazelle(
    name = "gazelle",
    gazelle = ":gazelle_cc",
)
```

### Using WORKSPACE

The `gazelle_cc` is built and distributed using Bazel modules. While it can still be used with legacy `WORKSPACE` definitions—especially for users on Bazel 8.x — this setup is not the primary focus and may not receive the same level of testing. Note that Bazel has announced plans to remove support for `WORKSPACE` in version 9.0, so users are encouraged to migrate to Bazel modules when possible.

For instructions how to setup `gazelle_cc` using WORKSPACEs visit [this guide](./docs/workspace_setup.md)

## Custom Directives

The extension defines the following custom directives:

### `# gazelle:cc_group [directory|unit]`

Controls how C++ source files are grouped into rules:

- `directory`: Creates one `cc_library` per directory **(default)**
- `unit`: Creates one `cc_library`/`cc_test` per translation unit or group of cyclicly dependent translation units. Corresponding `.h` and `.cc` files are always defined in the same group

### `# gazelle:cc_group_unit_cycles [merge|warn]`

Controls how to handle cyclic dependencies between translation units:

- `merge`: All groups forming a cycle will be merged into a single one **(default)**
- `warn`: Don't modify rules forming a cycle, let user handle it manually

### `# gazelle:cc_generate [true|false]`

Specifies whether Gazelle should create C/C++ specific targets, e.g. `cc_library` (default: `true`).
Setting this to `false` disables rule generation in the current directory and its subdirectories, allowing manual rule management instead.
Existing `cc_library` rules are still indexed and may be used to resolve internal dependencies.

### `# gazelle:cc_generate_proto [true|false]`

Specifies whether Gazelle should create `cc_proto_library` targets (default: `true`).
It can override the broader `# gazelle:proto` setting, letting you suppress proto-target generation specifically for C/C++ rules.

### `# gazelle:cc_indexfile <path>`

Loads an index file, containing a map from header include paths to Bazel labels.
An index lets Gazelle resolve dependencies on targets outside the current project,
for example, those provided by a Bazel module or separate package manager.
Equivalently, you can use `# gazelle:resolve` directives, but you can more easily
generate these mappings in bulk with an index file.
See [external dependenices section](#external-dependencies) for instructions on
generating index files.

Multiple `cc_indexfile` directives can be used, and their values are inherited by subprojects.
To clear inherited cc_indexfile values, provide an empty argument, e.g. `# gazelle:cc_indexfile`.
When resolving dependencies, indexes are visited in the same order as the corresponding `cc_indexfile` definitions.

The argument must be a repository-root relative path.

### `# gazelle:cc_search <strip_include_prefix> <include_prefix>`

Lazy indexing may be enabled with the Gazelle arguments `-index=lazy` and `-r=false`. When enabled, Gazelle only indexes libraries for dependency resolution in specific directories, based on configuration directives and the included headers it sees. This dramatically speeds up Gazelle when run in specific directories, compared with indexing the whole repository.

The `cc_search` directive configures Gazelle for C++ lazy indexing, adding a rule that translates header paths into directories to search.

For example, suppose you have a library in the directory `third_party/foo/` with the label `//third_party/foo:foo`. It has a header file in `third_party/foo/inc/foo.h` that you include from your main source code as `foo/foo.h`. The library's `cc_library` target might be written as:

```bzl
cc_library(
    name = "foo",
    hdrs = ["inc/foo.h"],
    strip_include_prefix = "third_party/foo/inc",
    include_prefix = "foo",
    visibility = ["//visibility:public"],
)
```

You can tell Gazelle where to find this library using the directive:

```
# gazelle:cc_search foo third_party/foo
```

The `cc_search` directive accepts two arguments: a prefix to strip, and a prefix to add, analogous to `strip_include_prefix` and `include_prefix`. Both arguments must be clean slash-separated relative paths. Arguments may be quoted, so empty strings may be written as `''` or `""`.

`gazelle_cc` first removes the prefix to strip, so `foo/foo.h` becomes `foo.h` in the example above. If the include path does not start with the prefix, the search rule is ignored. Then, `gazelle_cc` prepends the prefix to add, so `foo.h` becomes `third_party/foo/foo.h`. Finally, `gazelle_cc` trims the basename, to get the directory `third_party/foo`. Gazelle indexes all library rules in this directory, making them available for dependency resolution.

You can specify `cc_search` directives multiple times. A directive applies to the directory where it's written and to subdirectories. An empty `cc_search` directive resets the list of translation rules for the current directory.

## Rules for target rule selection

The extension automatically selects the appropriate rule type based on the following criteria:

### Rule Type Selection

1. **cc_library**: Created for:
   - Header files (`.h`, `.hh`, `.hpp`, `.hxx`)
   - Source files that don't contain a `main()` function and aren't test files
   - Pregenerated `.pb.h` files in case when generation of `cc_proto_library` rules is disabled `# gazelle:proto [legacy|disable|disable_global]`

2. **cc_binary**: Created for:
   - Source files containing a `main()` function
   - Main function signature is detected only based on the source file content, it does not handle custom macros wrapping the `main` method
  
3. **cc_test**: Created for:
   - Files with names starting with `test` or ending with `test` suffix (excluding file extension)

4. **cc_proto_library**: Created for:
   - Each corresponding `proto_library` rule generated by `"@gazelle//language/proto`
   - Generated only if `cc_proto_library` rules are enabled generation of rules, that is `# gazelle:proto [default|file|package]`

### Source Grouping

Sources are grouped according to the `cc_group` directive:

- **directory mode**: All source files in a directory are grouped based on their kind. Generated `BUILD.bazel` would contain at most only one rule of `cc_library` and `cc_test` kind.
- **unit mode**: Files are grouped based on their dependencies:
  - Header files and their corresponding implementation files are grouped together
  - Files with mutual dependencies form a single group
  - Cyclic dependencies are handled according to the `cc_group_unit_cycles` directive
  - The generated `BUILD.bazel` would contain multiple `cc_library` / `cc_test` rules, one for each group.

The `cc_binary` rule is always generated once per found translation unit containing a `main` method

## Dependency Resolution

Dependency resolution between both internal and external dependencies is based only on `#include` directives used in sources. Gazelle C++ extension parses the C/C++ source files to extract required information using preprocessor directives.

### Internal dependencies

Every build target managed by Gazelle C++ extension registers information about the header files defined in `hdrs` attribute of each `cc_library` rule. It allows one to create an index of fully-qualified paths relative to the root directory of the repository.

Each source file path extracted from `#include` directives is looked up in the index, if a target rule could be found it would be added to the list of rule dependencies.
In case of source-file relative includes the path is resolved based on the directory defining the source before the lookup.

Rules/subdirectories that are not managed by the Gazelle do not populate the internal dependencies index and would not be automatically resolved. Gazelle can be instructed to use user defined resolution rules to work around this limitation

```bazel
# gazelle:resolve cc path/to/my_include.h //target/defining:library
```

would allow to resolve includes in your sources

```c
#include "path/to/my_include.h" // Resolves to //target/defining:library
#include "some/other/lib.hpp"   // Unresolved, not dependency would be added
```

### External dependencies

External dependencies are resolved using similar mechanism as [internal dependencies](#internal-dependencies), but requiring always a fully-qualified path to the rule, based on `includes` and prefixes defined by library authors.

The knowledge about the headers and their defining rules of external repositories is limited and depends on the used package manager.

#### `bazel_dep`

Gazelle C++ extension is using a [built-in index](./language/cc/bzldep-index.json) created based on all the `cc_library` rules found in [Bazel Central Registry](https://registry.bazel.build/) repositories.

Currently that's the recommended way of defining external dependencies

```bazel
# MODULE.bazel
bazel_dep(name = "googletest", version = "1.16.0")
bazel_dep(name = "fmt", version = "11.1.4", repo_name = "fmt_repo")
```

```cpp
// source.cc
#include "gmock/gmock.h"          // Resolved to @googletest//:gtest
#include <gmock/gmock-matchers.h> // Resolved to @googletest//:gtest
#include "fmt/core.h"             // Resolved to @fmt_repo//:fmt
#include "boost/chrono.hpp"       // Warning: defined in @boost.chrono//:boost.chrono but not added as bazel_dep
```

#### `conan`

Resolving external dependencies managed by [Conan](https://docs.conan.io/2/integrations/bazel.html) requires creation of index by the user using `@gazelle_cc//index/conan` binary.

```bash
conan profile detect 
conan install . --build=missing
bazel run @gazelle_cc//index/conan -- --output=conan.ccindex
```

The resulting index needs to be added to Gazelle directive in top-level `BUILD` file.

```bazel
# gazelle:cc_indexfile conan.ccindex
```

Additional options for `@gazelle_cc//index/conan`:

| Flag | Default | Definition |
| ---- | ------- | ---------- |
| --output=\<path> | ./output.ccidx | Output file for created index |
| --install | false | Should conan profile detection and installation be done automatically before indexing |
| --conanDir=\<path> | ./conan | Controls the paths contains conan specific and external dependencies definitions. Typically created during `conan install .` invocation |
| --verbose | false | Enable verbose logging and debug information |

#### `rules_foreign_cc`

Resolving external dependencies managed by [rules_foreign_cc](https://github.com/bazel-contrib/rules_foreign_cc) requires creation of index by the user using `@gazelle_cc//index/rules_foreign_cc` binary. It would use `bazel query` to find definitions of `rules_foreign_cc` rules, eg. `cmake` and would use their assigned sources and rules to create an index.

```bash
bazel run @gazelle_cc//index/rules_foreign_cc -- --output=foreign.ccindex
```

The resulting index needs to be added to Gazelle directive in top-level `BUILD` file.

```bazel
# gazelle:cc_indexfile foreign.ccindex
```

Additional options for `@gazelle_cc//index/rules_foreign_cc`:

| Flag | Default | Definition |
| ---- | ------- | ---------- |
| --output=\<path> | ./output.ccidx | Output file for created index |
| --verbose | false | Enable verbose logging and debug information |

#### Other package managers

Other package managers like [vcpkg](https://vcpkg.io/en/) are currently not yet supported. Please create an issue in this repository if you need additional integrations.

These can still be used by defining a manual mapping between header and defining rules using `# gazelle:resolve` directives

## C++20 Modules support

C++20 modules are currently not supported, but are planned to be introduced in the future.

## Example Usage

Here's an example of how to use the extension in your C++ project:

1. Provide the default configuration in `BUILD.bazel` created in [installation step](#installation) and provide configuration for gazelle (optional):

```bazel
## Exclude following subtrees from being managed by gazelle. Be aware that it also prevents other targets from automatic dependency resolution in these modules using gazelle
# gazelle:exclude third-party
# gazelle:exclude examples/usage

## Define how the cc sources should be managed, use `unit` for small file-based targets. 
# gazelle:cc_group unit

## Warn if there are some unresolved cyclic dependencies between source files
# gazelle:cc_group_unit_cycles warn

## Overwrite how some C/C++ includes should be resolved  
# gazelle:resolve cc gtest/gtest.h @googletest//:gtest_main
# gazelle:resolve cc fmt.h @googletest//:gtest_main
```

2. Run Gazelle to generate BUILD files:

```bash
bazel run //:gazelle
```

This will:

- Scan your C/C++ source files
- Generate appropriate `cc_library`, `cc_proto_library`, `cc_binary`, and `cc_test` rules
- Handle dependencies automatically
- Group source files according to your directives

After running Gazelle, it will generate appropriate BUILD files with dependencies and visibility settings.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to contribute to this project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](./LICENSE) file for details.
