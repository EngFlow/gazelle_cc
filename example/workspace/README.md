# Example usage of gazelle_cc

Run `bazel run :gazelle` to start `gazelle` with `gazelle_cc` extensions to generate `mylib/BUILD` and `app/BUILD`
It would create following targets:

| Target | Kind |
| - | - |
| //mylib:mylib | cc_library |
| //mylib:mylib_test | cc_test |
| //app:main | cc_binary |

These can be built using `bazel build //...`.