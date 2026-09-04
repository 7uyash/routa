package diff

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/7uyash/routa/recorder"
)

// Compare compares a replayed entry against its original entry.
// It returns a DiffResult highlighting the changes.
func Compare(original, replay *recorder.Entry) *recorder.DiffResult {
	if original == nil || replay == nil {
		return nil
	}

	result := &recorder.DiffResult{
		HeadersDiff: make(map[string]string),
	}

	// 1. Status Code
	if original.StatusCode != replay.StatusCode {
		result.StatusDiff = fmt.Sprintf("%d -> %d", original.StatusCode, replay.StatusCode)
	}

	// 2. Headers
	for k, vOrig := range original.ResponseHeaders {
		vReplay, ok := replay.ResponseHeaders[k]
		if !ok {
			result.HeadersDiff[k] = fmt.Sprintf("removed (was: %s)", strings.Join(vOrig, ", "))
		} else if strings.Join(vOrig, ", ") != strings.Join(vReplay, ", ") {
			result.HeadersDiff[k] = fmt.Sprintf("changed: %s -> %s", strings.Join(vOrig, ", "), strings.Join(vReplay, ", "))
		}
	}
	for k, vReplay := range replay.ResponseHeaders {
		if _, ok := original.ResponseHeaders[k]; !ok {
			result.HeadersDiff[k] = fmt.Sprintf("added: %s", strings.Join(vReplay, ", "))
		}
	}

	// 3. Body
	// Try JSON first, fallback to basic length/string comparison.
	if isJSON(original.ResponseBody) && isJSON(replay.ResponseBody) {
		result.BodyDiff = diffJSON(original.ResponseBody, replay.ResponseBody)
	} else if string(original.ResponseBody) != string(replay.ResponseBody) {
		// Just indicate change if it's not JSON
		if len(original.ResponseBody) != len(replay.ResponseBody) {
			result.BodyDiff = fmt.Sprintf("changed length: %d bytes -> %d bytes", len(original.ResponseBody), len(replay.ResponseBody))
		} else {
			result.BodyDiff = "content changed (same length)"
		}
	}

	return result
}

func isJSON(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	var js json.RawMessage
	return json.Unmarshal(b, &js) == nil
}

func diffJSON(b1, b2 []byte) string {
	var m1, m2 map[string]interface{}
	if err := json.Unmarshal(b1, &m1); err != nil {
		return "invalid JSON 1"
	}
	if err := json.Unmarshal(b2, &m2); err != nil {
		return "invalid JSON 2"
	}

	var diffs []string
	compareMapJSON(m1, m2, "", &diffs)
	compareMapJSONAdded(m1, m2, "", &diffs)

	if len(diffs) == 0 {
		return ""
	}
	return strings.Join(diffs, "\n")
}

func compareMapJSON(m1, m2 map[string]interface{}, prefix string, diffs *[]string) {
	for k, v1 := range m1 {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		v2, ok := m2[k]
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("- %s: removed", path))
			continue
		}

		if map1, ok1 := v1.(map[string]interface{}); ok1 {
			if map2, ok2 := v2.(map[string]interface{}); ok2 {
				compareMapJSON(map1, map2, path, diffs)
				continue
			}
		}

		s1 := fmt.Sprintf("%v", v1)
		s2 := fmt.Sprintf("%v", v2)
		if s1 != s2 {
			*diffs = append(*diffs, fmt.Sprintf("~ %s: %s -> %s", path, s1, s2))
		}
	}
}

func compareMapJSONAdded(m1, m2 map[string]interface{}, prefix string, diffs *[]string) {
	for k, v2 := range m2 {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		if _, ok := m1[k]; !ok {
			*diffs = append(*diffs, fmt.Sprintf("+ %s: added (%v)", path, v2))
		} else if map2, ok2 := v2.(map[string]interface{}); ok2 {
			if map1, ok1 := m1[k].(map[string]interface{}); ok1 {
				compareMapJSONAdded(map1, map2, path, diffs)
			}
		}
	}
}
