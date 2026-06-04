package auth

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
)

// TestBuildMeResponse_NullFieldsSerializeAsNull guards the consistency contract:
// GET /api/auth/me MUST surface unset avatar_url/email as JSON null (not ""),
// matching GET /api/creators/me (toPrivateDTO) and the codebase-wide nullable
// serialization convention.
func TestBuildMeResponse_NullFieldsSerializeAsNull(t *testing.T) {
	creator := db.Creator{
		ID:        uuid.New(),
		Nickname:  "nullfields",
		AvatarUrl: sql.NullString{Valid: false},
		Email:     sql.NullString{Valid: false},
	}

	raw, err := json.Marshal(buildMeResponse(creator))
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	for _, key := range []string{"avatar_url", "email"} {
		v, present := got[key]
		if !present {
			t.Fatalf("%q must be present in response, got %s", key, raw)
		}
		if string(v) != "null" {
			t.Fatalf("unset %q must serialize as JSON null, got %s (full: %s)", key, v, raw)
		}
	}
}

// TestBuildMeResponse_SetFieldsSerializeAsString is the regression guard: when
// avatar_url/email have values they must be exposed verbatim as JSON strings.
func TestBuildMeResponse_SetFieldsSerializeAsString(t *testing.T) {
	creator := db.Creator{
		ID:        uuid.New(),
		Nickname:  "setfields",
		AvatarUrl: sql.NullString{String: "https://cdn.example/a.png", Valid: true},
		Email:     sql.NullString{String: "u@example.com", Valid: true},
	}

	raw, err := json.Marshal(buildMeResponse(creator))
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got struct {
		AvatarURL *string `json:"avatar_url"`
		Email     *string `json:"email"`
		Nickname  string  `json:"nickname"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.AvatarURL == nil || *got.AvatarURL != "https://cdn.example/a.png" {
		t.Fatalf("set avatar_url must expose stored string, got %v", got.AvatarURL)
	}
	if got.Email == nil || *got.Email != "u@example.com" {
		t.Fatalf("set email must expose stored string, got %v", got.Email)
	}
	if got.Nickname != "setfields" {
		t.Fatalf("nickname regression: got %q", got.Nickname)
	}
}
