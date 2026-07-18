package controller

import "testing"

func TestLastFinishReason(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"none", `{"choices":[{"delta":{"content":"hi"}}]}`, ""},
		{"null ignored", `"finish_reason":null`, ""},
		{"single stop", `data: {"choices":[{"finish_reason":"stop"}]}`, "stop"},
		{"tool_calls", `{"choices":[{"finish_reason":"tool_calls"}]}`, "tool_calls"},
		{
			"last wins",
			`{"finish_reason":null}` + "\n" + `{"finish_reason":"stop"}`,
			"stop",
		},
		{"spaced json", `"finish_reason" : "length"`, "length"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastFinishReason([]byte(c.in)); got != c.want {
				t.Errorf("lastFinishReason(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestIsSessionDone(t *testing.T) {
	done := map[string]bool{
		"stop":           true,
		"length":         true,
		"content_filter": true,
		"tool_calls":     false,
		"":               false,
		"unknown":        false,
	}
	for fr, want := range done {
		if got := isSessionDone(fr); got != want {
			t.Errorf("isSessionDone(%q) = %v, want %v", fr, got, want)
		}
	}
}
