package bot

import (
	"context"
	"strings"
	"testing"
)

// TestMediaValidator_SSRFWiring_RejectsLoopback verifies that the production
// HTTP client wired by NewDefaultMediaValidator (httpclient.NewSSRFSafeClient)
// refuses to dial a loopback IP supplied as a caller-untrusted media URL. The
// validator must surface this as a download_failed rejection (the standard
// pre-decode failure path in download() — see media_validator.go L169-198) so
// downstream metrics aggregate the SSRF rejection under the same reason
// bucket as ordinary fetch failures, without leaking the upstream URL or the
// internal IP into a panic or hang.
//
// This pins the security contract: an external publisher planting
// `<img src="http://127.0.0.1:1/...">` (or a media candidate URL that
// resolves to loopback at probe time) cannot use the harvester as an SSRF
// pivot, even though the rejection is observably indistinguishable from a
// real network failure. Mirror of robots_filter_ssrf_test.go pattern (same
// package).
func TestMediaValidator_SSRFWiring_RejectsLoopback(t *testing.T) {
	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), "http://127.0.0.1:1/scan-target", "image")

	if r.Valid {
		t.Fatalf("loopback host must be rejected by SSRF guard, got Valid=true reason=%q", r.Reason)
	}
	if r.Reason != MediaValidationDownloadFailed {
		t.Fatalf("expected download_failed (SSRF dial rejection surfaces via download err path), got %q", r.Reason)
	}
}

// TestMediaValidator_SSRFWiring_RejectsIMDS verifies the same protection for
// AWS EC2 IMDS (169.254.169.254 — link-local IPv4). Without this guard a
// publisher could plant a media URL pointing at the metadata service and
// have the harvester fetch the IAM credentials response into og_data.
func TestMediaValidator_SSRFWiring_RejectsIMDS(t *testing.T) {
	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(),
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/foo",
		"image")

	if r.Valid {
		t.Fatalf("IMDS host must be rejected by SSRF guard, got Valid=true reason=%q", r.Reason)
	}
	if r.Reason != MediaValidationDownloadFailed {
		t.Fatalf("expected download_failed for IMDS rejection, got %q", r.Reason)
	}
}

// TestMediaValidator_SSRFWiring_RejectsRFC1918 verifies the same protection
// for an RFC 1918 address (10.x.x.x). Without this guard, a media URL that
// either is or DNS-rebinds to a private cluster IP could be used to scan
// internal services from the harvester worker.
func TestMediaValidator_SSRFWiring_RejectsRFC1918(t *testing.T) {
	v := NewDefaultMediaValidator()
	r := v.Validate(context.Background(), "http://10.0.0.1/scan", "image")

	if r.Valid {
		t.Fatalf("RFC 1918 host must be rejected by SSRF guard, got Valid=true reason=%q", r.Reason)
	}
	if r.Reason != MediaValidationDownloadFailed {
		t.Fatalf("expected download_failed for RFC 1918 rejection, got %q", r.Reason)
	}
}

// TestMediaValidator_NewDefaultMediaValidator_WiresSSRFSafeTransport pins the
// constructor-level wiring: NewDefaultMediaValidator MUST install a non-nil
// custom Transport on HTTP (the SSRF-safe Transport installed by
// httpclient.NewSSRFSafeClient). Bare http.Client would leave Transport==nil
// at construction (the default Transport is lazy-initialized by stdlib). This
// guards against accidental reversion to
// `&http.Client{Timeout: 30 * time.Second}` — the exact regression risk this
// migration closes (cycle 134, backlog
// `system-20260528-bot-media-validator-bare-http-client-ssrf`).
func TestMediaValidator_NewDefaultMediaValidator_WiresSSRFSafeTransport(t *testing.T) {
	v := NewDefaultMediaValidator()
	if v.HTTP == nil {
		t.Fatal("HTTP client not wired")
	}
	if v.HTTP.Transport == nil {
		t.Fatal("SSRF-safe client must install a custom Transport (DialContext guard); got nil — likely regressed to bare http.Client")
	}
	if v.HTTP.CheckRedirect == nil {
		t.Fatal("SSRF-safe client must install CheckRedirect to re-run guard on each hop; got nil")
	}
	if v.HTTP.Timeout <= 0 {
		t.Fatalf("HTTP.Timeout must be a positive TotalTimeout from httpclient.Options, got %v", v.HTTP.Timeout)
	}
}

// TestMediaValidator_SSRFWiring_ErrorSurfaceIsStableCategory makes the
// observability contract explicit: SSRF rejections MUST land in
// MediaValidationDownloadFailed, NOT a new MediaValidationReason that would
// (a) expand the externally-visible reason cardinality (the const block in
// media_validator.go is intentionally small) and (b) require operators to
// re-aggregate metrics on rollout. The actual rejection cause is observable
// in process logs via the underlying error chain ("httpclient: blocked
// private/reserved IP ..."), but the metric surface stays stable.
func TestMediaValidator_SSRFWiring_ErrorSurfaceIsStableCategory(t *testing.T) {
	v := NewDefaultMediaValidator()
	// All three input shapes (loopback, link-local, RFC 1918) must map to the
	// same external reason. If a future refactor splits SSRF rejections into
	// their own reason, this test fires and forces a deliberate revisit of
	// the const block + metrics dashboard before merge.
	for _, url := range []string{
		"http://127.0.0.1:80/x",
		"http://169.254.169.254/x",
		"http://192.168.1.1/x",
	} {
		r := v.Validate(context.Background(), url, "image")
		if r.Reason != MediaValidationDownloadFailed {
			t.Errorf("SSRF reject for %q surfaced as %q, want download_failed", url, r.Reason)
		}
		// Defense in depth: also assert the reason string contains nothing
		// hint-leaking like the internal IP itself. (The Reason is a stable
		// enum, so this is a contract check on the const rather than on
		// per-call data.)
		if strings.Contains(string(r.Reason), "127.0.0.1") ||
			strings.Contains(string(r.Reason), "169.254") ||
			strings.Contains(string(r.Reason), "192.168") {
			t.Errorf("reason %q must not leak internal IP into externally-visible category", r.Reason)
		}
	}
}
