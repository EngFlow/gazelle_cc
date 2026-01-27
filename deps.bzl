# Copyright 2026 EngFlow Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

"""Provides gazelle_cc dependenices for `WORKSPACE` mode."""

def gazelle_cc_dependencies():
    http_archive(
        name = "package_metadata",
        sha256 = "49ed11e5d6b752c55fa539cbb10b2736974f347b081d7bd500a80dacb7dbec06",
        strip_prefix = "supply-chain-0.0.5/metadata",
        urls = [
            "https://github.com/bazel-contrib/supply-chain/releases/download/v0.0.5/supply-chain-v0.0.5.tar.gz",
        ],
    )
