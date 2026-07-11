package lcc

import (
	"fmt"
	"unicode"
)

type TokenKind int

const (
	TokEOF TokenKind = iota
	TokKeyword
	TokIdent
	TokNum
	TokOp
	TokDelimiter
)

type Token struct {
	Kind  TokenKind
	Value string
	Line  int
}

var keywords = map[string]struct{}{
	"global": {},
	"if":     {},
	"int":    {},
	"return": {},
	"void":   {},
}

func Tokenize(src string) ([]Token, error) {
	tokens := make([]Token, 0, len(src)/2)
	line := 1

	for index := 0; index < len(src); {
		char := rune(src[index])

		switch {
		case char == '\n':
			line++
			index++
		case unicode.IsSpace(char):
			index++
		case char == '/' && index+1 < len(src) && src[index+1] == '/':
			index += 2
			for index < len(src) && src[index] != '\n' {
				index++
			}
		case isIdentStart(char):
			start := index
			index++
			for index < len(src) && isIdentPart(rune(src[index])) {
				index++
			}
			value := src[start:index]
			kind := TokIdent
			if _, ok := keywords[value]; ok {
				kind = TokKeyword
			}
			tokens = append(tokens, Token{Kind: kind, Value: value, Line: line})
		case unicode.IsDigit(char):
			start := index
			index++
			for index < len(src) && unicode.IsDigit(rune(src[index])) {
				index++
			}
			tokens = append(tokens, Token{Kind: TokNum, Value: src[start:index], Line: line})
		case isOperator(char):
			tokens = append(tokens, Token{Kind: TokOp, Value: string(char), Line: line})
			index++
		case isDelimiter(char):
			tokens = append(tokens, Token{Kind: TokDelimiter, Value: string(char), Line: line})
			index++
		default:
			return nil, fmt.Errorf("lex error on line %d: unrecognized symbol %q", line, string(char))
		}
	}

	tokens = append(tokens, Token{Kind: TokEOF, Line: line})
	return tokens, nil
}

func isIdentStart(char rune) bool {
	return char == '_' || unicode.IsLetter(char)
}

func isIdentPart(char rune) bool {
	return isIdentStart(char) || unicode.IsDigit(char)
}

func isOperator(char rune) bool {
	switch char {
	case '+', '-', '*', '/', '=':
		return true
	default:
		return false
	}
}

func isDelimiter(char rune) bool {
	switch char {
	case '(', ')', '{', '}', ';', ',':
		return true
	default:
		return false
	}
}
