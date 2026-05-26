package boards

import "testing"

// Pins the `offset` cap on GET /api/boards/{id}. Sister-handler convention
// from pin/handler.go:568 (`o > 0 && o <= 100000`) and search/handler.go
// (cycle 105 PR #295, `maxSearchOffset = 100000`). Out-of-range offset is
// silently clamped to 0 — see the doc on `maxBoardsOffset` for rationale.
//
// Full end-to-end coverage of the clamp behavior requires a working DB:
// `GetByID` invokes `GetBoard`, `CountBoardPins`, and `ListBoardPinImages`
// before the offset parse runs, so the offset code path is unreachable
// with a nil DB harness. The sister test in search/handler_offset_cap_test.go
// avoids this by stubbing the package's `SearchQuerier` interface, but
// `boards.Handler` is bound to `*sql.DB` directly. Real-environment QA
// during this cycle exercises offset=0/10/100/100000/100001/999999999 via
// curl and observes silent clamp; see `.fugue/decision-log.md` for the
// cycle 106 entry.
//
// The drift-detection assertion below is the unit-layer guard: if the cap
// constant changes (e.g. someone tightens it to 1000 or removes it), this
// test breaks and forces a conscious update — matching the local-readable
// `boardsBodyCap` pattern in handler_body_cap_test.go:22.

func TestMaxBoardsOffset_ValueMatchesSisterConvention(t *testing.T) {
	const wantSisterCap = 100000 // pin/handler.go:568, search.maxSearchOffset
	if maxBoardsOffset != wantSisterCap {
		t.Fatalf("maxBoardsOffset drifted from sister cap: got %d, want %d (pin/handler.go:568, search.maxSearchOffset)",
			maxBoardsOffset, wantSisterCap)
	}
}

// TestGetByID_OffsetCap_DocumentedUnitGap documents why end-to-end clamp
// verification is deferred to real-env QA rather than unit tests. Mirrors
// TestUpdate_BodyTooLarge_DocumentedUnitGap in handler_body_cap_test.go:81.
func TestGetByID_OffsetCap_DocumentedUnitGap(t *testing.T) {
	t.Skip("GetByID offset parse runs only after 3 successful DB calls (GetBoard, CountBoardPins, ListBoardPinImages); unreachable with nil-DB harness. Covered by real-env QA in cycle 106.")
}
