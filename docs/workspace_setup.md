# WORKSPACE setup

## `gazelle_cc` dependencies

For up to date versions of `gazelle_cc` dependencies see the [MODULE.bazel file](../MODULE.bazel)

| Dependency | Version |
| - | -|
| [gazelle](https://github.com/bazel-contrib/bazel-gazelle) | 0.43.0 |
| [rules_go](https://github.com/bazel-contrib/rules_go) | 0.53.0 |
| [Go SDK](https://go.dev/) | 1.24.0 |
  
## Installation

Example WORKSPACE-based setup can see found in [example/workspace](../example/workspace/WORKSPACE) directory.

### Gazelle

See [Gazelle WORKSPACE setup guide](https://github.com/bazel-contrib/bazel-gazelle/?tab=readme-ov-file#workspace) for up to date instructions.

```bazel
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    integrity = "sha256-t493RY53Fi9FtFZNayC2+S9WQx7VnqqrCeeBnR2FAxM=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.53.0/rules_go-v0.53.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.53.0/rules_go-v0.53.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    integrity = "sha256-fEC3Rjh80Mmk1bsLIDWr0TSz91EQFXEKXuXgdZEAjd4=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.43.0/bazel-gazelle-v0.43.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.43.0/bazel-gazelle-v0.43.0.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

go_rules_dependencies()
go_register_toolchains(version = "1.24.0")
gazelle_dependencies(go_sdk = "go_sdk")
```

### gazelle_cc

This Gazelle extensions uses by default the new naming convention for external dependencies using bzlmod.
You might be required to use `repo_mapping` to adjust dependenices installed using WORKSPACE under different naming convention.

```bazel
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "gazelle_cc",
    sha256 = "8e990e454b06c529e383239e0692a1f17b003e3e7a5a0f967ee5f6aeb400105a",
    strip_prefix = "gazelle_cc-0.1.0",
    url = "https://github.com/engflow/gazelle_cc/releases/download/v0.1.0/gazelle_cc-v0.1.0.tar.gz",
    repo_mapping = {
        "@rules_go": "@io_bazel_rules_go",
        "@gazelle":  "@bazel_gazelle",
    },
)
```
