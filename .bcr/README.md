# Bazel Central Registry publication

[.github/workflows/publish-to-bcr.yml](../.github/workflows/publish-to-bcr.yaml) uses these files to configure the [Publish to BCR](https://github.com/bazel-contrib/publish-to-bcr) workflow for publishing to the
[Bazel Central Registry (BCR)](https://registry.bazel.build/).

- [Publish to BCR workflow setup](https://github.com/bazel-contrib/publish-to-bcr?tab=readme-ov-file#setup)
- [.bcr/templates](https://github.com/bazel-contrib/publish-to-bcr/tree/main/templates)
- [.github/workflows/publish.yaml reusable workflow](https://github.com/bazel-contrib/publish-to-bcr/blob/main/.github/workflows/publish.yaml)

## Inspiration

Based on bazel-contrib/rules_scala#1731 which itself was originally based on the examples from aspect-build/rules_lint#498 and
aspect-build/rules_lint#501. See also:

- bazelbuild/bazel-central-registry#4060
- bazelbuild/bazel-central-registry#4146
- slsa-framework/slsa-verifier#840
