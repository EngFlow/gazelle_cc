# gcc_search directive

This is a test for lazy indexing, configured with the `cc_search` directive. It's based on Gazelle's `go_search` test, which tests similar functionality in the `go` extension.

With the `-r=false -index=lazy` flags, Gazelle only visits directories named on the command line but still indexes libraries. This should be very fast.

The `use` library includes other libraries with various paths. This test
checks that it can find those libraries with lazy indexing. The libraries are:

- `self`: a library that logically belongs to this project. No `go_search`
  directive needed.
- `stripped_prefix_rel`: a library with a relative stripped include prefix.
- `stripped_prefix_abs`: a library with an absolute stripped include prefix.
- `prefix`: a library with an additional include prefix.
- `stripped_prefix_and_prefix`: a library with a stripped prefix and an
  additional prefix.
