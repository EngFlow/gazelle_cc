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

package benchmark

import (
	"bytes"
	"slices"
	"testing"

	lexer_latest "github.com/EngFlow/gazelle_cc/language/internal/cc/lexer"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/lexer/benchmark/snapshots/lexer_v1"
)

func runBenchmark(b *testing.B, input []byte) {
	implementations := []struct {
		name   string
		runner func(input []byte)
	}{
		{
			name: "lexer_v1",
			runner: func(input []byte) {
				_ = slices.Collect(lexer_v1.NewLexer(input).AllTokens())
			},
		},
		{
			name: "lexer_latest",
			runner: func(input []byte) {
				_ = slices.Collect(lexer_latest.NewLexer(input).AllTokens())
			},
		},
	}

	for _, impl := range implementations {
		b.Run(impl.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				impl.runner(input)
			}
		})
	}
}

func BenchmarkRepeatedToken(b *testing.B) {
	runBenchmark(b, bytes.Repeat([]byte(";"), 1000))
}

const helloWorldInput = `
#include <iostream>

int main(int argc, char **argv) {
    std::cout << "Hello, World!" << std::endl;
	return 0;
}
`

func BenchmarkHelloWorld(b *testing.B) {
	runBenchmark(b, []byte(helloWorldInput))
}

func BenchmarkRepeatedHelloWorld(b *testing.B) {
	runBenchmark(b, bytes.Repeat([]byte(helloWorldInput), 100))
}
