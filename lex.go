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
	TokString
	TokOp
	TokDelimiter
)

type Token struct {
	Kind  TokenKind
	Value string
	Line  int
}

var keywords = map[string]struct{}{
	"break":   {},
	"case":    {},
	"const":   {},
	"continue": {},
	"default": {},
	"else":    {},
	"extern":  {},
	"false":   {},
	"bool":    {},
	"byte":    {},
	"float32": {},
	"float64": {},
	"for":     {},
	"if":      {},
	"int8":    {},
	"int16":   {},
	"int32":   {},
	"int64":   {},
	"return":  {},
	"switch":  {},
	"true":    {},
	"uint8":   {},
	"uint16":  {},
	"uint32":  {},
	"uint64":  {},
	"void":    {},
	"while":   {},
	"int":     {}, // alias for int32
	"float":   {}, // alias for float32
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
			token, newIndex, err := tokenizeNumericLiteral(src, index, line)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			index = newIndex
		case char == '"':
			token, newIndex, err := tokenizeStringLiteral(src, index, line)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			index = newIndex
		case isOperator(char):
			value, width := tokenizeOperator(src, index)
			tokens = append(tokens, Token{Kind: TokOp, Value: value, Line: line})
			index += width
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

func tokenizeNumericLiteral(src string, start int, line int) (Token, int, error) {
	index := start
	for index < len(src) && unicode.IsDigit(rune(src[index])) {
		index++
	}
	if index < len(src) && src[index] == '.' {
		index++
		if index >= len(src) || !unicode.IsDigit(rune(src[index])) {
			return Token{}, index, fmt.Errorf("lex error on line %d: invalid numeric literal", line)
		}
		for index < len(src) && unicode.IsDigit(rune(src[index])) {
			index++
		}
	}
	if index < len(src) && (src[index] == 'e' || src[index] == 'E') {
		index++
		if index < len(src) && (src[index] == '+' || src[index] == '-') {
			index++
		}
		if index >= len(src) || !unicode.IsDigit(rune(src[index])) {
			return Token{}, index, fmt.Errorf("lex error on line %d: invalid numeric literal", line)
		}
		for index < len(src) && unicode.IsDigit(rune(src[index])) {
			index++
		}
	}
	if index < len(src) {
		switch src[index] {
		case 'f', 'F', 'd', 'D':
			index++
		}
	}
	return Token{Kind: TokNum, Value: src[start:index], Line: line}, index, nil
}

func tokenizeStringLiteral(src string, start int, line int) (Token, int, error) {
	index := start + 1
	value := make([]rune, 0, 16)
	for index < len(src) {
		char := rune(src[index])
		switch char {
		case '"':
			return Token{Kind: TokString, Value: string(value), Line: line}, index + 1, nil
		case '\\':
			if index+1 >= len(src) {
				return Token{}, index, fmt.Errorf("lex error on line %d: unterminated string literal", line)
			}
			escaped := rune(src[index+1])
			switch escaped {
			case '"':
				value = append(value, '"')
			case '\\':
				value = append(value, '\\')
			case 'n':
				value = append(value, '\n')
			case 'r':
				value = append(value, '\r')
			case 't':
				value = append(value, '\t')
			case '0':
				value = append(value, 0)
			default:
				return Token{}, index, fmt.Errorf("lex error on line %d: unsupported string escape %q", line, "\\"+string(escaped))
			}
			index += 2
		case '\n', '\r':
			return Token{}, index, fmt.Errorf("lex error on line %d: unterminated string literal", line)
		default:
			value = append(value, char)
			index++
		}
	}
	return Token{}, index, fmt.Errorf("lex error on line %d: unterminated string literal", line)
}

func isIdentStart(char rune) bool {
	return char == '_' || unicode.IsLetter(char)
}

func isIdentPart(char rune) bool {
	return isIdentStart(char) || unicode.IsDigit(char)
}

func isOperator(char rune) bool {
	switch char {
	case '+', '-', '*', '/', '=', '!', '<', '>', '&', '|':
		return true
	default:
		return false
	}
}

func tokenizeOperator(src string, index int) (string, int) {
	if index+1 < len(src) {
		switch src[index : index+2] {
		case "==", "!=", "<=", ">=", "&&", "||":
			return src[index : index+2], 2
		}
	}
	return src[index : index+1], 1
}

func isDelimiter(char rune) bool {
	switch char {
	case '(', ')', '{', '}', ';', ',', ':':
		return true
	default:
		return false
	}
}
