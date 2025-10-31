Check that the cc_strip_include_prefix and cc_include_prefix directories are
applied to include paths when deciding how to group source files.

The src directory contains files that should be compiled together as a single
target, but headers are included as foo/a.h and foo/b.h rather than src/a.h
and src/b.h.