package lcc

import "testing"

func TestTokenizeComparisonOperators(t *testing.T) {
	src := "if (a == b != c <= d >= e) { return; }"
	tokens, err := Tokenize(src)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	var operators []string
	for _, token := range tokens {
		if token.Kind == TokOp {
			operators = append(operators, token.Value)
		}
	}
	expected := []string{"==", "!=", "<=", ">="}
	if len(operators) != len(expected) {
		t.Fatalf("expected %d operators, got %d (%v)", len(expected), len(operators), operators)
	}
	for index, want := range expected {
		if operators[index] != want {
			t.Fatalf("expected operator %d to be %q, got %q", index, want, operators[index])
		}
	}
}

func TestTokenizeColonDelimiter(t *testing.T) {
	src := "switch (value) { case 1: default: }"
	tokens, err := Tokenize(src)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	colonCount := 0
	for _, token := range tokens {
		if token.Kind == TokDelimiter && token.Value == ":" {
			colonCount++
		}
	}
	if colonCount != 2 {
		t.Fatalf("expected 2 colon delimiters, got %d", colonCount)
	}
}

func TestTokenizeControlFlowKeywords(t *testing.T) {
	src := "break case continue default else for switch while"
	tokens, err := Tokenize(src)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	expected := []string{"break", "case", "continue", "default", "else", "for", "switch", "while"}
	if len(tokens) != len(expected)+1 {
		t.Fatalf("expected %d tokens plus eof, got %d", len(expected), len(tokens))
	}
	for index, want := range expected {
		token := tokens[index]
		if token.Kind != TokKeyword {
			t.Fatalf("expected token %d (%q) to be keyword, got kind %d", index, want, token.Kind)
		}
		if token.Value != want {
			t.Fatalf("expected token %d value %q, got %q", index, want, token.Value)
		}
	}
}

func TestTokenizeNumericLiteralSupportsIntegerFloatAndScientific(t *testing.T) {
	src := "1 2.5 6e3 7.25e-2 8E+4"
	tokens, err := Tokenize(src)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	values := []string{tokens[0].Value, tokens[1].Value, tokens[2].Value, tokens[3].Value, tokens[4].Value}
	expected := []string{"1", "2.5", "6e3", "7.25e-2", "8E+4"}
	for index, want := range expected {
		if values[index] != want {
			t.Fatalf("expected token %d value %q, got %q", index, want, values[index])
		}
	}
}

func TestTokenizeNumericLiteralSupportsFloatSuffixes(t *testing.T) {
	src := "0.5 1.5f 2.5d 6e3F 7.25E-2D"
	tokens, err := Tokenize(src)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}
	values := []string{tokens[0].Value, tokens[1].Value, tokens[2].Value, tokens[3].Value, tokens[4].Value}
	expected := []string{"0.5", "1.5f", "2.5d", "6e3F", "7.25E-2D"}
	for index, want := range expected {
		if values[index] != want {
			t.Fatalf("expected token %d value %q, got %q", index, want, values[index])
		}
	}
}

func TestTokenizeNumericLiteralRejectsInvalidScientificNotation(t *testing.T) {
	for _, src := range []string{"1e", "1e+", "1e-", "2.E3"} {
		if _, err := Tokenize(src); err == nil {
			t.Fatalf("expected Tokenize to reject %q", src)
		}
	}
}
