package auth

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateAvatarURL_BelowCap(t *testing.T) {
	in := "https://lh3.googleusercontent.com/a/" + strings.Repeat("x", 50)
	out := truncateAvatarURL(in)
	if out != in {
		t.Fatalf("input under cap must be returned unchanged: input len=%d, output len=%d", utf8.RuneCountInString(in), utf8.RuneCountInString(out))
	}
}

func TestTruncateAvatarURL_AtCap(t *testing.T) {
	in := strings.Repeat("A", 500)
	out := truncateAvatarURL(in)
	if utf8.RuneCountInString(out) != 500 {
		t.Fatalf("input at cap must be returned unchanged: got rune count=%d, want 500", utf8.RuneCountInString(out))
	}
	if out != in {
		t.Fatalf("input at cap must be byte-identical to output")
	}
}

func TestTruncateAvatarURL_AboveCap(t *testing.T) {
	in := strings.Repeat("A", 600)
	out := truncateAvatarURL(in)
	if utf8.RuneCountInString(out) != 500 {
		t.Fatalf("input above cap must be truncated to exactly 500 runes: got %d", utf8.RuneCountInString(out))
	}
	if !strings.HasPrefix(in, out) {
		t.Fatalf("truncated output must be prefix of input")
	}
}

func TestTruncateAvatarURL_MultibyteAboveCap(t *testing.T) {
	// 가 is a 3-byte Korean glyph in UTF-8. 600 runes = 1800 bytes.
	in := strings.Repeat("가", 600)
	out := truncateAvatarURL(in)
	runeCount := utf8.RuneCountInString(out)
	if runeCount != 500 {
		t.Fatalf("multibyte input above cap must be truncated to 500 runes (not 500 bytes): got rune count=%d, byte len=%d", runeCount, len(out))
	}
	// Byte length should be 500 * 3 = 1500 (proves rune-count cap, not byte-count cap).
	if len(out) != 1500 {
		t.Fatalf("500 runes of '가' must be 1500 bytes: got %d", len(out))
	}
}

func TestTruncateAvatarURL_Empty(t *testing.T) {
	out := truncateAvatarURL("")
	if out != "" {
		t.Fatalf("empty input must return empty string (toNullString downstream converts to sql.NullString{}): got %q", out)
	}
}

// TestTruncateNickname_AvatarURLPolicyParity asserts that truncateNickname
// retains the same []rune slice algorithm that truncateAvatarURL uses, so
// future edits don't accidentally diverge the two helpers' truncation
// semantics. The cap value differs (50 vs 500) but the truncation mechanics
// must match.
func TestTruncateNickname_AvatarURLPolicyParity(t *testing.T) {
	// Above-cap input: 60 runes → 50 runes after truncation.
	in := strings.Repeat("A", 60)
	out := truncateNickname(in)
	if utf8.RuneCountInString(out) != 50 {
		t.Fatalf("truncateNickname must cap at 50 runes (parity with truncateAvatarURL's rune-count algorithm): got %d", utf8.RuneCountInString(out))
	}
	// Multibyte: 60 Korean runes → 50 runes (150 bytes).
	inKo := strings.Repeat("가", 60)
	outKo := truncateNickname(inKo)
	if utf8.RuneCountInString(outKo) != 50 {
		t.Fatalf("truncateNickname multibyte cap parity: got rune count=%d, want 50", utf8.RuneCountInString(outKo))
	}
	if len(outKo) != 150 {
		t.Fatalf("truncateNickname multibyte must produce 50 runes × 3 bytes = 150 bytes: got %d", len(outKo))
	}
}
