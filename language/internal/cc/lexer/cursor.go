// Copyright 2026 EngFlow Inc. All rights reserved.
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

package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Position in the source code. Line and Column are 1-based, which is natural for humans.
type Cursor struct {
	Line, Column int
}

var (
	// Initial cursor position, at the beginning of the file or string.
	CursorInit = Cursor{Line: 1, Column: 1}
	// Special cursor value indicating the end of the file or string.
	CursorEOF = Cursor{}
)

func (c Cursor) String() string {
	if c == CursorEOF {
		return "EOF"
	}
	return fmt.Sprintf("%d:%d", c.Line, c.Column)
}

// Return a new Cursor advanced by the given lookAhead string. Assumes the current cursor points at the beginning of
// lookAhead and returns the cursor position right after lookAhead.
//
// Newlines in lookAhead increment the line number and reset the column; other characters increment the column.
func (c Cursor) AdvancedBy(lookAhead string) Cursor {
	newlinesCount := strings.Count(lookAhead, "\n")
	tailBegin := 1 + strings.LastIndex(lookAhead, "\n")
	tailLength := utf8.RuneCountInString(lookAhead[tailBegin:])

	if newlinesCount == 0 {
		c.Column += tailLength
	} else {
		c.Line += newlinesCount
		c.Column = 1 + tailLength
	}

	return c
}
