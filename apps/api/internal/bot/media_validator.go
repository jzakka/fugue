package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	// Register stdlib image decoders so image.DecodeConfig can probe
	// width/height from GIF/PNG/JPEG headers without a full decode.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// MediaValidationReason is a stable, externally-observable category for why
// a media candidate was rejected during validation. The set is intentionally
// small so that og_data record + metrics can aggregate by reason without an
// exploding cardinality.
type MediaValidationReason string

const (
	// MediaValidationOK indicates the media candidate passed validation. It
	// is recorded only on the per-candidate result; rejections are what get
	// aggregated into MediaValidationRecord.
	MediaValidationOK MediaValidationReason = "ok"
	// MediaValidationDownloadFailed is returned when the candidate URL
	// could not be fetched (DNS, TLS, non-2xx, transport error).
	MediaValidationDownloadFailed MediaValidationReason = "download_failed"
	// MediaValidationDecodeFailed is returned when the downloaded bytes
	// could not be decoded as the declared media type.
	MediaValidationDecodeFailed MediaValidationReason = "decode_failed"
	// MediaValidationImageTooSmall is returned when an image's pixel
	// dimensions fall below the configured minima (design.md D4).
	MediaValidationImageTooSmall MediaValidationReason = "image_too_small"
	// MediaValidationImageBytesTooFew is returned when an image's byte
	// length is below the configured minimum (design.md D4).
	MediaValidationImageBytesTooFew MediaValidationReason = "image_bytes_too_few"
	// MediaValidationVideoTooShort is returned when a video's duration
	// falls below the configured minimum (design.md D4).
	MediaValidationVideoTooShort MediaValidationReason = "video_too_short"
	// MediaValidationAudioTooShort is returned when an audio clip's
	// duration falls below the configured minimum (design.md D4).
	MediaValidationAudioTooShort MediaValidationReason = "audio_too_short"
	// MediaValidationUnsupportedType is returned when the declared media
	// type is not one of image/video/audio.
	MediaValidationUnsupportedType MediaValidationReason = "unsupported_type"
)

// Default validation thresholds (design.md D4). Operators may tune these via
// the Validator constructor; the spec deliberately leaves the values as
// implementation parameters.
const (
	DefaultImageMinWidth   = 32
	DefaultImageMinHeight  = 32
	DefaultImageMinBytes   = 1024
	DefaultVideoMinSeconds = 1.0
	DefaultAudioMinSeconds = 3.0
	// DefaultMediaDownloadMaxBytes caps the per-candidate download size
	// during validation to keep harvest worker memory bounded. Candidates
	// strictly larger than this cap are rejected as download_failed (the
	// extra +1 byte read in download() makes the boundary unambiguous —
	// see download() below). Not part of the spec contract; documented in
	// design.md D4 as an implementation safety constant. Operators may tune
	// MaxDownloadSize on the validator instance directly.
	DefaultMediaDownloadMaxBytes int64 = 50 * 1024 * 1024 // 50 MiB
	// defaultFFProbeBin is the single source of truth for the ffprobe
	// fallback used when DefaultMediaValidator.FFProbePath is empty (issue
	// noted during impl review: avoid duplicating the literal "ffprobe"
	// across the constructor and probeDuration).
	defaultFFProbeBin = "ffprobe"
	// defaultFFProbeTimeout is the fallback per-call ffprobe subprocess
	// timeout when DefaultMediaValidator.ProbeTimeout is unset.
	defaultFFProbeTimeout = 10 * time.Second
)

// MediaValidationResult is the per-candidate outcome produced by a
// MediaValidator. When Valid is true Bytes contains the downloaded payload
// so callers can hand it directly to ObjectStorage without re-downloading
// (design.md D3 "temp buffer → validated → canonical").
type MediaValidationResult struct {
	Valid       bool
	Reason      MediaValidationReason
	Bytes       []byte
	ContentType string
	Width       int           // image only
	Height      int           // image only
	Duration    time.Duration // video/audio only
}

// MediaValidator validates a media candidate URL by downloading and probing
// its bytes against type-specific thresholds. Implementations MUST NOT mutate
// any external state on rejection (design.md D3): the bytes for a rejected
// candidate are returned in-memory only and discarded by the caller.
type MediaValidator interface {
	Validate(ctx context.Context, url string, declaredType string) MediaValidationResult
}

// DefaultMediaValidator wires the stdlib image decoders for image
// validation and an ffprobe subprocess (design.md D5) for video/audio
// duration probing.
type DefaultMediaValidator struct {
	HTTP            *http.Client
	ImageMinWidth   int
	ImageMinHeight  int
	ImageMinBytes   int64
	VideoMinSeconds float64
	AudioMinSeconds float64
	MaxDownloadSize int64
	// FFProbePath overrides the ffprobe binary location. Empty means use
	// "ffprobe" from PATH.
	FFProbePath string
	// ProbeTimeout is the per-call timeout for ffprobe subprocess calls.
	ProbeTimeout time.Duration
}

// NewDefaultMediaValidator returns a validator configured with design.md D4
// default thresholds. Callers that need to tune values for tests or operations
// can replace fields on the returned struct directly.
func NewDefaultMediaValidator() *DefaultMediaValidator {
	return &DefaultMediaValidator{
		HTTP:            &http.Client{Timeout: 30 * time.Second},
		ImageMinWidth:   DefaultImageMinWidth,
		ImageMinHeight:  DefaultImageMinHeight,
		ImageMinBytes:   DefaultImageMinBytes,
		VideoMinSeconds: DefaultVideoMinSeconds,
		AudioMinSeconds: DefaultAudioMinSeconds,
		MaxDownloadSize: DefaultMediaDownloadMaxBytes,
		FFProbePath:     defaultFFProbeBin,
		ProbeTimeout:    defaultFFProbeTimeout,
	}
}

// Validate downloads the candidate and dispatches to the type-specific
// validator. Always returns a non-nil-style MediaValidationResult; on
// failure Valid=false and Reason describes the category.
func (v *DefaultMediaValidator) Validate(ctx context.Context, url string, declaredType string) MediaValidationResult {
	if declaredType != "image" && declaredType != "video" && declaredType != "audio" {
		// Bytes/ContentType are intentionally empty here: nothing was
		// downloaded because the type was rejected before any network call.
		return MediaValidationResult{Valid: false, Reason: MediaValidationUnsupportedType}
	}

	body, contentType, err := v.download(ctx, url)
	if err != nil {
		// Bytes/ContentType are intentionally empty: download failed before
		// or during body read, so there is no payload to surface.
		return MediaValidationResult{Valid: false, Reason: MediaValidationDownloadFailed}
	}

	switch declaredType {
	case "image":
		return v.validateImage(body, contentType)
	case "video":
		return v.validateVideo(ctx, body, contentType)
	case "audio":
		return v.validateAudio(ctx, body, contentType)
	}
	return MediaValidationResult{Valid: false, Reason: MediaValidationUnsupportedType}
}

func (v *DefaultMediaValidator) download(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := v.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	max := v.MaxDownloadSize
	if max <= 0 {
		max = DefaultMediaDownloadMaxBytes
	}
	// Read up to max+1 bytes so we can distinguish "exactly at the cap"
	// (acceptable) from "strictly above the cap" (rejected). LimitReader
	// itself yields no signal at the boundary, so the caller compares the
	// resulting len against max explicitly below.
	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > max {
		return nil, "", errors.New("payload exceeds max download size")
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func (v *DefaultMediaValidator) validateImage(body []byte, contentType string) MediaValidationResult {
	if int64(len(body)) < v.ImageMinBytes {
		return MediaValidationResult{Valid: false, Reason: MediaValidationImageBytesTooFew, Bytes: body, ContentType: contentType}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return MediaValidationResult{Valid: false, Reason: MediaValidationDecodeFailed, Bytes: body, ContentType: contentType}
	}
	if cfg.Width < v.ImageMinWidth || cfg.Height < v.ImageMinHeight {
		return MediaValidationResult{
			Valid:       false,
			Reason:      MediaValidationImageTooSmall,
			Bytes:       body,
			ContentType: contentType,
			Width:       cfg.Width,
			Height:      cfg.Height,
		}
	}
	return MediaValidationResult{
		Valid:       true,
		Reason:      MediaValidationOK,
		Bytes:       body,
		ContentType: contentType,
		Width:       cfg.Width,
		Height:      cfg.Height,
	}
}

func (v *DefaultMediaValidator) validateVideo(ctx context.Context, body []byte, contentType string) MediaValidationResult {
	dur, err := v.probeDuration(ctx, body)
	if err != nil {
		return MediaValidationResult{Valid: false, Reason: MediaValidationDecodeFailed, Bytes: body, ContentType: contentType}
	}
	if dur.Seconds() < v.VideoMinSeconds {
		return MediaValidationResult{
			Valid:       false,
			Reason:      MediaValidationVideoTooShort,
			Bytes:       body,
			ContentType: contentType,
			Duration:    dur,
		}
	}
	return MediaValidationResult{Valid: true, Reason: MediaValidationOK, Bytes: body, ContentType: contentType, Duration: dur}
}

func (v *DefaultMediaValidator) validateAudio(ctx context.Context, body []byte, contentType string) MediaValidationResult {
	dur, err := v.probeDuration(ctx, body)
	if err != nil {
		return MediaValidationResult{Valid: false, Reason: MediaValidationDecodeFailed, Bytes: body, ContentType: contentType}
	}
	if dur.Seconds() < v.AudioMinSeconds {
		return MediaValidationResult{
			Valid:       false,
			Reason:      MediaValidationAudioTooShort,
			Bytes:       body,
			ContentType: contentType,
			Duration:    dur,
		}
	}
	return MediaValidationResult{Valid: true, Reason: MediaValidationOK, Bytes: body, ContentType: contentType, Duration: dur}
}

// ffprobeFormat models the subset of `ffprobe -of json -show_format` output
// the validator consumes.
type ffprobeFormat struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// probeDuration calls ffprobe via subprocess on stdin to extract duration in
// seconds. Returns an error if ffprobe is not installed, exits non-zero, or
// emits a payload without a parseable duration.
func (v *DefaultMediaValidator) probeDuration(ctx context.Context, body []byte) (time.Duration, error) {
	bin := v.FFProbePath
	if bin == "" {
		bin = defaultFFProbeBin
	}
	timeout := v.ProbeTimeout
	if timeout <= 0 {
		timeout = defaultFFProbeTimeout
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, bin,
		"-loglevel", "error",
		"-of", "json",
		"-show_format",
		"-i", "pipe:0",
	)
	cmd.Stdin = bytes.NewReader(body)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var parsed ffprobeFormat
	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0, err
	}
	secs, err := strconv.ParseFloat(parsed.Format.Duration, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// FilterValidMedia is the integration helper that applies a MediaValidator
// to a PinDocument's thumbnail and media candidates in place. Invalid
// candidates are removed from MediaCandidates; an invalid ThumbnailURL is
// cleared. The aggregate rejection record is written to OGData.MediaValidation
// so downstream observability can read it. The validator is invoked at most
// once per unique candidate URL.
//
// This helper is purely in-memory: it does NOT upload media or persist
// anything. design.md D3 keeps "temp buffer → validate → canonical" as the
// upload-path concern; this function only filters PinDocument before the
// classifier sees it (design.md D2).
//
// Concurrency: NOT goroutine-safe. The function mutates *doc in place. Each
// PinDocument should be owned by exactly one harvester worker for the
// duration of the call (matches the current consumer architecture, where
// one document flows linearly through extract → filter → classify → pipeline).
func FilterValidMedia(ctx context.Context, validator MediaValidator, doc *PinDocument) {
	if validator == nil || doc == nil {
		return
	}

	// Snapshot whether og_data.media_candidates was originally populated.
	// This decides whether we keep og_data.media_candidates synchronized
	// after filtering (issue noted during impl review: previously the sync
	// branch fired only if og_data was already non-empty AND the filtered
	// doc.MediaCandidates was non-empty, leaving stale data when filtering
	// emptied the list).
	ogHadCandidates := len(doc.OGData.MediaCandidates) > 0

	// Cache results by URL so the same candidate referenced as both
	// ThumbnailURL and a MediaCandidates entry only costs one network round.
	results := make(map[string]MediaValidationResult)
	probe := func(url, declared string) MediaValidationResult {
		if r, ok := results[url]; ok {
			return r
		}
		if declared == "" {
			declared = "image"
		}
		r := validator.Validate(ctx, url, declared)
		results[url] = r
		return r
	}

	rejected := MediaValidationRecord{Reasons: map[string]int{}}
	// rejectedURLs deduplicates the rejection tally by URL: if the same URL
	// appears as both ThumbnailURL and a MediaCandidates entry, the failure
	// counts once. This keeps RejectedCount aligned with "distinct media
	// references that were rejected" so downstream metrics don't double-count
	// the dual-slot case.
	rejectedURLs := make(map[string]struct{})
	addReject := func(url string, reason MediaValidationReason) {
		if _, seen := rejectedURLs[url]; seen {
			return
		}
		rejectedURLs[url] = struct{}{}
		rejected.RejectedCount++
		rejected.Reasons[string(reason)]++
	}

	// 1. Filter MediaCandidates. Allocate a fresh slice so we never mutate
	// the caller's backing array (which may be aliased by og_data).
	if len(doc.MediaCandidates) > 0 {
		kept := make([]MediaCandidate, 0, len(doc.MediaCandidates))
		for _, c := range doc.MediaCandidates {
			if c.URL == "" {
				continue
			}
			r := probe(c.URL, c.Type)
			if !r.Valid {
				addReject(c.URL, r.Reason)
				continue
			}
			// Backfill width/height for images if known.
			if c.Type == "image" {
				if r.Width > 0 && c.Width == 0 {
					c.Width = r.Width
				}
				if r.Height > 0 && c.Height == 0 {
					c.Height = r.Height
				}
			}
			kept = append(kept, c)
		}
		doc.MediaCandidates = kept
	}

	// 2. Validate ThumbnailURL.
	if doc.ThumbnailURL != "" {
		r := probe(doc.ThumbnailURL, "image")
		if !r.Valid {
			addReject(doc.ThumbnailURL, r.Reason)
			doc.ThumbnailURL = ""
		}
	}

	// 3. Record observability data on the document.
	if rejected.RejectedCount > 0 {
		doc.OGData.MediaValidation = &rejected
	}

	// 4. Keep og_data.media_candidates in sync if og_data originally tracked
	// candidates. Use an explicit slice (not append on nil) so an emptied
	// filtered list produces a non-nil zero-length slice that is
	// distinguishable from "og_data never tracked candidates".
	if ogHadCandidates {
		synced := make([]MediaCandidate, len(doc.MediaCandidates))
		copy(synced, doc.MediaCandidates)
		doc.OGData.MediaCandidates = synced
	}
}
