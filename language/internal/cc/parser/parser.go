// Copyright 2025 EngFlow, Inc. All rights reserved.
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

package parser

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"unicode"
)

type SourceInfo struct {
	Includes Includes
	HasMain  bool
}

type Includes struct {
	DoubleQuote []string
	Bracket     []string
}

func ParseSource(input string) SourceInfo {
	reader := strings.NewReader(input)
	return extractSourceInfo(reader)
}

func ParseSourceFile(filename string) (SourceInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return SourceInfo{}, err
	}
	defer file.Close()

	return extractSourceInfo(file), nil
}

func isParanthesis(char rune) bool {
	switch char {
	case '(', ')', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

// bufio.SplitFunc that skips both whitespaces, line comments (//...) and block comments (/*...*/)
// The tokenizer splits not only by whitespace seperated words but also by: parenthesis, curly/square brackets
func tokenizer(data []byte, atEOF bool) (advance int, token []byte, err error) {
	i := 0
	for i < len(data) {
		char := rune(data[i])
		// log.Printf("i=%d / %d\n", i, len(data))
		switch {
		// Skip line comments
		case bytes.HasPrefix(data[i:], []byte("//")):
			i += 2
			for i < len(data) && data[i] != '\n' {
				i++
			}
		// Skip block comments
		case bytes.HasPrefix(data[i:], []byte("/*")):
			i += 2
			for i < len(data)-1 {
				if data[i] == '*' && data[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
		// Skip whitespace
		case unicode.IsSpace(char):
			i++

		case isParanthesis(char):
			return i + 1, data[i : i+1], nil

		default:
			start := i
			for i < len(data) {
				char := rune(data[i])
				if unicode.IsSpace(char) || isParanthesis(char) {
					return i, data[start:i], nil
				}
				i++
			}
			return i, data[start:i], nil
		}
	}

	if atEOF {
		return len(data), nil, io.EOF
	}
	return i, nil, nil
}

func extractSourceInfo(input io.Reader) SourceInfo {
	scanner := bufio.NewScanner(input)
	scanner.Split(tokenizer)

	sourceInfo := SourceInfo{}
	lastToken := ""
	for scanner.Scan() {
		prevToken := lastToken
		token := scanner.Text()
		lastToken = token

		if token == "#include" && scanner.Scan() {
			include := scanner.Text()
			if strings.ContainsAny(include, "<>") {
				sourceInfo.Includes.Bracket = append(sourceInfo.Includes.Bracket, strings.Trim(include, "<>"))
			} else if strings.Contains(include, "\"") {
				sourceInfo.Includes.DoubleQuote = append(sourceInfo.Includes.DoubleQuote, strings.Trim(include, "\""))
			}
			continue
		}

		if token == "main" && scanner.Scan() {
			// TOOD: better detection of main signature
			// We should also check for return type aliases and check if input args
			if scanner.Text() == "(" {
				if prevToken == "int" {
					sourceInfo.HasMain = true
				}
				continue
			}
		}
	}
	return sourceInfo
}
