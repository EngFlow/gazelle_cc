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

package cc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitQuoted(t *testing.T) {
	for _, test := range []struct {
		name    string
		text    string
		want    []string
		wantErr bool
	}{
		{
			name: "empty",
			text: "",
			want: nil,
		},
		{
			name: "one",
			text: "ab",
			want: []string{"ab"},
		},
		{
			name: "two",
			text: "a bc",
			want: []string{"a", "bc"},
		},
		{
			name: "tab",
			text: "a\tbc",
			want: []string{"a", "bc"},
		},
		{
			name: "single_quote",
			text: `'a b' c`,
			want: []string{"a b", "c"},
		},
		{
			name: "double_quote",
			text: `a "b c"`,
			want: []string{"a", "b c"},
		},
		{
			name: "quote_in_word",
			text: `a' b 'c d`,
			want: []string{"a b c", "d"},
		},
		{
			name: "escape_quote",
			text: `a\'b c`,
			want: []string{"a'b", "c"},
		},
		{
			name: "escape_backslash",
			text: `a\\b c`,
			want: []string{"a\\b", "c"},
		},
		{
			name:    "unclosed_quote",
			text:    `'`,
			wantErr: true,
		},
		{
			name:    "unfinished_escape",
			text:    `\`,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, gotErr := splitQuoted(test.text)
			if test.wantErr && gotErr == nil {
				t.Fatalf("unexpected success")
			} else if !test.wantErr && gotErr != nil {
				t.Fatal(gotErr)
			} else if !test.wantErr {
				require.Equal(t, got, test.want)
			}
		})
	}
}
