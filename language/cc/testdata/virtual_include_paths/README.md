# virtual_include_paths

The test checks if the strategy of generating `resolve.ImportSpec` conforms the way how `cc_library` handles
`"include_prefix"`, `"strip_include_prefix"` and `"includes"` attributes. Following rules are enforced:

- Repository-root-relative path is always a correct way to include a header, regardless of the rule's attributes:
  `"include_prefix"`, `"strip_include_prefix"`, `"includes"`.
- When at least one of `"include_prefix"` or `"strip_include_prefix"` is present, a new virtual include path becomes
  accessible, alongside with the repository-root-relative path mentioned above.
    - `"strip_include_prefix"` can be expressed in both ways: as repository-root-relative path or package-relative path.
      In the second case it is eventually converted to repository-root-relative path.
    - `"strip_include_prefix"` is removed from the beginning of repository-root-relative path, then `"include_prefix"`
      is prepended.
    - The important caveat is, when `"strip_include_prefix"` is empty, `"include_prefix"` is prepended to
      **package-relative** path (so as `"strip_include_prefix"` was implicitly set to the package directory).
- For each path in `"includes"` list, a new virtual include path becomes accessible, alongside with the
  repository-root-relative path and the potential virtual path created from the mix of `"include_prefix"` and
  `"strip_include_prefix"`. These paths are handled independently of `"include_prefix"` and `"strip_include_prefix"`.


The test checks the following cases:

- `"include_prefix"` - set OR unset
- `"strip_include_prefix"` - unset OR set as relative path OR set as absolute path
- `"includes"` - empty OR contains one element
