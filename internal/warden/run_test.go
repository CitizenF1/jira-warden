package warden

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmWorklogWrite(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "yes", input: "yes\n", expected: true},
		{name: "y", input: "y\n", expected: true},
		{name: "default no", input: "\n", expected: false},
		{name: "no", input: "no\n", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			actual, err := confirmWorklogWrite(&out, strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("confirm worklog write: %v", err)
			}
			if actual != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}
