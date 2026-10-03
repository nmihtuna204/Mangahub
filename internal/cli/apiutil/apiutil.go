// Package apiutil holds small helpers shared by the CLI command groups.
package apiutil

import (
	"fmt"
	"net/http"
)

// ErrorMessage extracts a readable message from a decoded API response.
// It never panics, whatever shape the body had (standard error envelope,
// {"error": "text"}, or not JSON at all).
func ErrorMessage(result map[string]interface{}, status int) string {
	switch e := result["error"].(type) {
	case map[string]interface{}:
		if msg, ok := e["message"].(string); ok && msg != "" {
			return msg
		}
	case string:
		if e != "" {
			return e
		}
	}
	if msg, ok := result["message"].(string); ok && msg != "" {
		return msg
	}
	return fmt.Sprintf("HTTP %d %s", status, http.StatusText(status))
}
