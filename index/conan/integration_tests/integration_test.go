// Copyright 2025 EngFlow Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"os"
	"testing"

	"github.com/EngFlow/gazelle_cc/index/internal/tests"
)

func TestConanIndexerIntegration(t *testing.T) {
	conanHomeDir := t.TempDir() // unfortunate requirement, no write access to ~/.conan2

	tests.ExecuteIndexerIntegrationTest(t, tests.IndexerIntegration{
		BeforeTestCase: func(t *testing.T, ctx tests.IndexerIntegrationContext) {
			t.Logf("==> [%s] Running conan install...", ctx.Dir)
			conanConfig := tests.ExecConfig{
				Dir: ctx.Dir,
				Env: append(os.Environ(),
					"CONAN_HOME="+conanHomeDir,
					"CONAN_NON_INTERACTIVE=1",
					"CONAN_SKIP_STATS=1"),
				CanFail: true,
			}
			// Can be replaced by --install arg to conan indexer
			tests.Execute(t, conanConfig, "conan", "profile", "detect")
			tests.Execute(t, conanConfig, "conan", "install", ".", "--build=missing")
		},
	})
}
