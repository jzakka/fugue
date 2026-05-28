// qa-media-validator-ssrf is a temporary QA harness for cycle 134
// (system-20260528-bot-media-validator-bare-http-client-ssrf). It exercises
// the production NewDefaultMediaValidator constructor against:
//   - loopback (127.0.0.1:1)            → expect download_failed
//   - link-local IMDS (169.254.169.254) → expect download_failed
//   - RFC 1918 (10.0.0.1)               → expect download_failed
//   - public happy-path (httpbin.org/image/png) → expect Valid=true
//
// This binary is deleted after the merge (loop-system convention; cf. the
// design-track temp QA harnesses removed in cycles 60-72). It is NOT a
// substitute for unit tests — those pin the contract in CI. It IS the
// real-env QA evidence required by the loop system before PR merge.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
)

type caseSpec struct {
	name        string
	url         string
	declared    string
	wantValid   bool
	wantReason  bot.MediaValidationReason
	description string
}

func main() {
	cases := []caseSpec{
		{
			name:        "loopback-port-1",
			url:         "http://127.0.0.1:1/scan-target",
			declared:    "image",
			wantValid:   false,
			wantReason:  bot.MediaValidationDownloadFailed,
			description: "SSRF dial-time block on IPv4 loopback",
		},
		{
			name:        "imds-aws",
			url:         "http://169.254.169.254/latest/meta-data/iam/security-credentials/foo",
			declared:    "image",
			wantValid:   false,
			wantReason:  bot.MediaValidationDownloadFailed,
			description: "SSRF dial-time block on AWS IMDS link-local",
		},
		{
			name:        "rfc1918-10-0-0-1",
			url:         "http://10.0.0.1/internal",
			declared:    "image",
			wantValid:   false,
			wantReason:  bot.MediaValidationDownloadFailed,
			description: "SSRF dial-time block on RFC 1918",
		},
		{
			name:        "rfc1918-192-168",
			url:         "http://192.168.1.1/internal",
			declared:    "image",
			wantValid:   false,
			wantReason:  bot.MediaValidationDownloadFailed,
			description: "SSRF dial-time block on RFC 1918 (192.168)",
		},
		{
			name:        "happy-path-public-png",
			url:         "https://httpbin.org/image/png",
			declared:    "image",
			wantValid:   true,
			wantReason:  bot.MediaValidationOK,
			description: "public httpbin PNG must pass through SSRF guard and validate",
		},
	}

	v := bot.NewDefaultMediaValidator()
	pass := 0
	fail := 0
	for _, c := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		start := time.Now()
		r := v.Validate(ctx, c.url, c.declared)
		cancel()
		elapsed := time.Since(start)

		ok := r.Valid == c.wantValid && r.Reason == c.wantReason
		status := "PASS"
		if !ok {
			status = "FAIL"
			fail++
		} else {
			pass++
		}
		fmt.Printf("[%s] %s\n", status, c.name)
		fmt.Printf("  url       : %s\n", c.url)
		fmt.Printf("  desc      : %s\n", c.description)
		fmt.Printf("  want      : Valid=%v Reason=%q\n", c.wantValid, c.wantReason)
		fmt.Printf("  got       : Valid=%v Reason=%q (width=%d height=%d duration=%v bytes=%d)\n",
			r.Valid, r.Reason, r.Width, r.Height, r.Duration, len(r.Bytes))
		fmt.Printf("  elapsed   : %v\n", elapsed)
		fmt.Println()
	}

	fmt.Printf("=== summary: %d pass, %d fail ===\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}
