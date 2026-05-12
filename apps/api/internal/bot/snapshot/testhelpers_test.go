package snapshot

import (
	"bytes"
	"compress/gzip"
	"io"
)

// gunzipBytes is a test-only inverse of gzipBytes used by store_test.go
// to assert round-trip integrity and CRC-based corruption detection.
// It deliberately lives in *_test.go so production builds stay free of
// any read-side helpers; the harvester change will introduce its own
// reader once the consumer side ships.
func gunzipBytes(src []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}
