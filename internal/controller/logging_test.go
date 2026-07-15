package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatBodyForLogTruncatesBase64Image(t *testing.T) {
	image := bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47}, 500000) // 2MB
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
	payload := []byte(fmt.Sprintf(`{"messages":[{"content":[{"image_url":{"url":"%s"}}]}]}`, dataURL))

	logged := formatBodyForLog(payload, "application/json")

	if len(logged) > MAX_LOGGED_BODY_BYTES+128 {
		t.Errorf("logged %d bytes for a %d byte image request; expected truncation", len(logged), len(payload))
	}

	if !strings.Contains(logged, "truncated") {
		t.Errorf("truncated body should say so, got: %.120s", logged)
	}

	// The start of the payload is still useful for debugging.
	if !strings.HasPrefix(logged, `{"messages"`) {
		t.Errorf("expected the head of the body to survive, got: %.60s", logged)
	}
}

func TestFormatBodyForLogOmitsBinaryAudio(t *testing.T) {
	audio := bytes.Repeat([]byte{0x00, 0x01, 0x02, 0xFF}, 250000) // 1MB of raw bytes

	logged := formatBodyForLog(audio, "multipart/form-data; boundary=abc123")

	if strings.ContainsRune(logged, 0x00) {
		t.Error("raw binary leaked into the log line")
	}

	if !strings.Contains(logged, "omitted") || !strings.Contains(logged, "1000000") {
		t.Errorf("expected a size summary, got: %.120s", logged)
	}
}

func TestFormatBodyForLogKeepsSmallJSONIntact(t *testing.T) {
	payload := []byte(`{"model":"llama","messages":[{"role":"user","content":"hi"}]}`)

	if logged := formatBodyForLog(payload, "application/json"); logged != string(payload) {
		t.Errorf("small body should be logged verbatim, got: %s", logged)
	}
}

func TestFormatBodyForLogTruncationIsValidUTF8(t *testing.T) {
	// "é" is 2 bytes, so an odd cut boundary would split a rune mid-way.
	payload := []byte(strings.Repeat("é", MAX_LOGGED_BODY_BYTES))

	logged := formatBodyForLog(payload, "application/json")

	if !utf8.ValidString(logged) {
		t.Error("truncation produced invalid UTF-8")
	}
}
