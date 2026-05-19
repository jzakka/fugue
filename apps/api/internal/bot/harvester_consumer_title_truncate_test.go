package bot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateRunes_TitleCap verifies that truncateRunes applied with n=200
// (the pins.title VARCHAR(200) cap) cuts inputs at exactly 200 runes on
// rune boundaries and leaves shorter/equal inputs untouched.
//
// The harvester_consumer.processOne path applies this call to doc.Title
// immediately after the analogous doc.BodyText cut at 500 runes. The cases
// below cover the four boundary conditions the spec ADDED Requirement
// "title은 pins.title 컬럼 cap에 맞춰 rune-safe 사전 절단된다" enumerates.
func TestTruncateRunes_TitleCap(t *testing.T) {
	const titleCap = 200

	cases := []struct {
		name           string
		input          string
		wantRuneLen    int
		wantUnchanged  bool
		wantStartsWith string
	}{
		{
			name:           "ASCII 201 chars → 200 runes",
			input:          strings.Repeat("A", 201),
			wantRuneLen:    titleCap,
			wantStartsWith: strings.Repeat("A", 10),
		},
		{
			name:           "Korean 201 runes → 200 runes (multibyte boundary)",
			input:          strings.Repeat("가", 201),
			wantRuneLen:    titleCap,
			wantStartsWith: strings.Repeat("가", 10),
		},
		{
			name:          "ASCII 100 chars → unchanged",
			input:         strings.Repeat("B", 100),
			wantRuneLen:   100,
			wantUnchanged: true,
		},
		{
			name:          "Empty string → empty",
			input:         "",
			wantRuneLen:   0,
			wantUnchanged: true,
		},
		{
			name:          "Exactly 200 chars → unchanged (boundary)",
			input:         strings.Repeat("C", titleCap),
			wantRuneLen:   titleCap,
			wantUnchanged: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateRunes(c.input, titleCap)

			if gotLen := utf8.RuneCountInString(got); gotLen != c.wantRuneLen {
				t.Errorf("rune length = %d, want %d", gotLen, c.wantRuneLen)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncated output is not valid UTF-8 (byte boundary cut on multibyte rune)")
			}
			if c.wantUnchanged && got != c.input {
				t.Errorf("expected unchanged input, got %q", got)
			}
			if c.wantStartsWith != "" && !strings.HasPrefix(got, c.wantStartsWith) {
				t.Errorf("output prefix mismatch: got %q, want prefix %q", got, c.wantStartsWith)
			}
		})
	}
}
