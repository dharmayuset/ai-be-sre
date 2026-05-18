package utils

import (
	"strings"
	"testing"
)

func TestGenerateTemporaryPassword(t *testing.T) {
	for _, length := range []int{12, 16, 20, 32} {
		pwd, err := GenerateTemporaryPassword(length)
		if err != nil {
			t.Fatalf("len=%d: unexpected error: %v", length, err)
		}
		if len(pwd) != length {
			t.Errorf("len=%d: got len=%d", length, len(pwd))
		}
		if !strings.ContainsAny(pwd, upperChars) {
			t.Errorf("len=%d: missing upper", length)
		}
		if !strings.ContainsAny(pwd, lowerChars) {
			t.Errorf("len=%d: missing lower", length)
		}
		if !strings.ContainsAny(pwd, digitChars) {
			t.Errorf("len=%d: missing digit", length)
		}
		if !strings.ContainsAny(pwd, specialChars) {
			t.Errorf("len=%d: missing special", length)
		}
	}
}

func TestGenerateTemporaryPasswordTooShort(t *testing.T) {
	_, err := GenerateTemporaryPassword(8)
	if err == nil {
		t.Error("expected error for length 8")
	}
}

func TestMaskEmail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"john.doe@example.com", "jo***@example.com"},
		{"a@b.com", "a***@b.com"},
		{"ab@c.com", "a***@c.com"},
		{"abc@d.com", "ab***@d.com"},
		{"no-at-sign", "***"},
	}
	for _, c := range cases {
		if got := MaskEmail(c.in); got != c.want {
			t.Errorf("MaskEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestConstantTimeEqual(t *testing.T) {
	if !ConstantTimeEqual("abc", "abc") {
		t.Error("equal strings should match")
	}
	if ConstantTimeEqual("abc", "abd") {
		t.Error("different strings should not match")
	}
}
