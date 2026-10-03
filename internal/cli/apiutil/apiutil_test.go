package apiutil

import "testing"

func TestErrorMessage(t *testing.T) {
	cases := []struct {
		result map[string]interface{}
		status int
		want   string
	}{
		{map[string]interface{}{"error": map[string]interface{}{"code": "NOT_FOUND", "message": "manga not found"}}, 404, "manga not found"},
		{map[string]interface{}{"error": "failed to submit rating"}, 500, "failed to submit rating"},
		{map[string]interface{}{"message": "Authentication required"}, 401, "Authentication required"},
		{nil, 404, "HTTP 404 Not Found"}, // body wasn't JSON (e.g. gin's plain-text 404)
	}
	for _, c := range cases {
		if got := ErrorMessage(c.result, c.status); got != c.want {
			t.Errorf("ErrorMessage(%v, %d) = %q, want %q", c.result, c.status, got, c.want)
		}
	}
}
