package lcc

import "testing"

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

func TestTokenizeNumericLiteralRejectsInvalidScientificNotation(t *testing.T) {
	for _, src := range []string{"1e", "1e+", "1e-", "2.E3"} {
		if _, err := Tokenize(src); err == nil {
			t.Fatalf("expected Tokenize to reject %q", src)
		}
	}
}
