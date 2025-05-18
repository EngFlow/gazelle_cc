# Example usage of gazelle_cc

Run `bazel run :gazelle` to start `gazelle` with `gazelle_cc` extensions to generate `mylib/BUILD` and `proto/BUILD`
It would create following targets:

| Target | Kind |
| - | - |
| //mylib:mylib | cc_library |
| //mylib:mylib_test | cc_test |
| //proto:sample_proto | proto_library |
| //proto:sample_cc_proto | cc_proto_library |
| //proto:example | cc_binary |

These can be built using `bazel build //...`.