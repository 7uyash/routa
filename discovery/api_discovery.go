package discovery

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/7uyash/routa/recorder"
)

var (
	// Regex patterns for path normalization
	uuidRegex   = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	intRegex    = regexp.MustCompile(`^[0-9]+$`)
	hexHashReg  = regexp.MustCompile(`(?i)^[0-9a-f]{16,64}$`)
	mongoIDReg  = regexp.MustCompile(`(?i)^[0-9a-f]{24}$`)
)

// EndpointMetrics holds aggregated traffic statistics for a normalized API endpoint.
type EndpointMetrics struct {
	Method        string    `json:"method"`
	NormalizedPath string   `json:"normalized_path"`
	SamplePath    string    `json:"sample_path"`
	Count         int       `json:"count"`
	AvgDurationMs int64     `json:"avg_duration_ms"`
	TotalDuration int64     `json:"-"`
	Status2xx     int       `json:"status_2xx"`
	Status4xx     int       `json:"status_4xx"`
	Status5xx     int       `json:"status_5xx"`
	LastSeen      time.Time `json:"last_seen"`
	SampleHeaders map[string][]string `json:"sample_headers,omitempty"`
	SampleBody    string    `json:"sample_body,omitempty"`
}

// MapBuilder analyzes recorded request entries and builds a normalized API endpoint map.
type MapBuilder struct {
	mu sync.RWMutex
}

// NewMapBuilder creates an API discovery MapBuilder.
func NewMapBuilder() *MapBuilder {
	return &MapBuilder{}
}

// NormalizePath normalizes dynamic segments in a URL path (e.g. /users/123 -> /users/{id}).
func NormalizePath(path string) string {
	parts := strings.Split(path, "/")
	normalized := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == "" {
			continue
		}
		if intRegex.MatchString(part) || uuidRegex.MatchString(part) || mongoIDReg.MatchString(part) || hexHashReg.MatchString(part) {
			normalized = append(normalized, "{id}")
		} else {
			normalized = append(normalized, part)
		}
	}

	return "/" + strings.Join(normalized, "/")
}

// BuildMap groups recorded entries into a normalized API endpoint map with per-endpoint analytics.
func (b *MapBuilder) BuildMap(entries []*recorder.Entry) []*EndpointMetrics {
	b.mu.Lock()
	defer b.mu.Unlock()

	endpointMap := make(map[string]*EndpointMetrics)

	for _, entry := range entries {
		normPath := NormalizePath(entry.Path)
		key := entry.Method + " " + normPath

		ep, exists := endpointMap[key]
		if !exists {
			ep = &EndpointMetrics{
				Method:         entry.Method,
				NormalizedPath: normPath,
				SamplePath:     entry.Path,
				Count:          0,
				SampleHeaders:  entry.ResponseHeaders,
				SampleBody:     string(entry.ResponseBody),
			}
			endpointMap[key] = ep
		}

		ep.Count++
		ep.TotalDuration += entry.Duration.Milliseconds()
		ep.AvgDurationMs = ep.TotalDuration / int64(ep.Count)

		if entry.StatusCode >= 200 && entry.StatusCode < 400 {
			ep.Status2xx++
		} else if entry.StatusCode >= 400 && entry.StatusCode < 500 {
			ep.Status4xx++
		} else if entry.StatusCode >= 500 {
			ep.Status5xx++
		}

		if entry.Timestamp.After(ep.LastSeen) {
			ep.LastSeen = entry.Timestamp
			ep.SamplePath = entry.Path
		}
	}

	result := make([]*EndpointMetrics, 0, len(endpointMap))
	for _, ep := range endpointMap {
		result = append(result, ep)
	}

	return result
}
