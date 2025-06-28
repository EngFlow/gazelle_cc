# Copyright 2025 EngFlow Inc. All rights reserved.
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
        sha256 = "4bca4db6350daec6e30900001993e276669770e7e820b2538ecd61b56b5f08e4",
        strip_prefix = "supply-chain-0.0.4.rc8/metadata",
        urls = [
            "https://github.com/bazel-contrib/supply-chain/releases/download/v0.0.4.rc8/supply-chain-v0.0.4.rc8.tar.gz",
        ],
    )
