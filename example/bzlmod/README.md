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

# VS Code debugging integration (for contributors)

You can play around here, testing new features, with step-by-step debugging of `gazelle_cc` binary. Activate the **"Run and Debug"** panel on the VS Code sidebar and select the configuration **"gazelle_cc in example/bzlmod"**, prepared especially for this purpose.

See https://code.visualstudio.com/docs/debugtest/debugging for more details about debugging in VS Code.
