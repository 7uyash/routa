package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/7uyash/routa/recorder"
)

// AssertionRule defines a verification check for a step.
type AssertionRule struct {
	Type     string `json:"type"`     // "status_code", "body_contains", "json_path_eq"
	Target   string `json:"target"`   // e.g. "$.status" or path/expression
	Expected string `json:"expected"` // e.g. "200" or "success"
}

// ExtractionRule defines how to capture dynamic variables from a step response.
type ExtractionRule struct {
	VarName    string `json:"var_name"`    // e.g. "token"
	Source     string `json:"source"`      // "body_json", "header", "body_regex"
	Expression string `json:"expression"`  // e.g. "token" or "Authorization" or regex pattern
}

// ScenarioStep represents a single HTTP request step in a scenario sequence.
type ScenarioStep struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Method          string              `json:"method"`
	Path            string              `json:"path"`
	Query           string              `json:"query,omitempty"`
	Headers         map[string][]string `json:"headers,omitempty"`
	Body            string              `json:"body,omitempty"`
	TargetService   string              `json:"target_service,omitempty"`
	DelayMs         int64               `json:"delay_ms"`
	ExpectedStatus  int                 `json:"expected_status"`
	Assertions      []AssertionRule     `json:"assertions,omitempty"`
	Extractions     []ExtractionRule    `json:"extractions,omitempty"`
	CapturedResponse *StepResponse      `json:"captured_response,omitempty"`
}

// StepResponse holds snapshot details of the captured response for reference.
type StepResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string              `json:"body,omitempty"`
}

// Scenario holds a full recorded or custom user flow scenario.
type Scenario struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Variables   map[string]string `json:"variables,omitempty"`
	Steps       []*ScenarioStep   `json:"steps"`
}

// Summary returns high-level stats for list view display.
type ScenarioSummary struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	RequestCount int       `json:"request_count"`
	ServiceCount int       `json:"service_count"`
	TotalDelayMs int64     `json:"total_delay_ms"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ScenarioStore handles persisting scenarios to disk.
type ScenarioStore struct {
	dir string
}

// NewScenarioStore initializes a ScenarioStore.
func NewScenarioStore(dir string) *ScenarioStore {
	return &ScenarioStore{dir: filepath.Join(dir, "scenarios")}
}

// SaveScenario stores a scenario as JSON.
func (s *ScenarioStore) SaveScenario(sc *Scenario) error {
	if sc.Name == "" {
		return fmt.Errorf("scenario name is required")
	}
	if sc.ID == "" {
		sc.ID = sanitizeName(sc.Name)
	}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create scenarios dir: %w", err)
	}

	sc.UpdatedAt = time.Now()
	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = sc.UpdatedAt
	}

	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scenario: %w", err)
	}

	path := filepath.Join(s.dir, sc.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write scenario file: %w", err)
	}
	return nil
}

// LoadScenario retrieves a scenario by ID or name.
func (s *ScenarioStore) LoadScenario(name string) (*Scenario, error) {
	id := sanitizeName(name)
	path := filepath.Join(s.dir, id+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario %q not found: %w", name, err)
	}

	var sc Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("unmarshal scenario: %w", err)
	}
	return &sc, nil
}

// ListScenarios returns a list of scenario summaries.
func (s *ScenarioStore) ListScenarios() ([]ScenarioSummary, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read scenarios dir: %w", err)
	}

	var summaries []ScenarioSummary
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var sc Scenario
		if err := json.Unmarshal(data, &sc); err != nil {
			continue
		}

		services := make(map[string]bool)
		var totalDelay int64
		for _, step := range sc.Steps {
			if step.TargetService != "" {
				services[step.TargetService] = true
			} else if step.Path != "" {
				// extract first path component as service hint if present
				parts := strings.Split(strings.TrimPrefix(step.Path, "/"), "/")
				if len(parts) > 0 && parts[0] != "" {
					services[parts[0]] = true
				}
			}
			totalDelay += step.DelayMs
		}

		summaries = append(summaries, ScenarioSummary{
			ID:           sc.ID,
			Name:         sc.Name,
			Description:  sc.Description,
			RequestCount: len(sc.Steps),
			ServiceCount: len(services),
			TotalDelayMs: totalDelay,
			CreatedAt:    sc.CreatedAt,
			UpdatedAt:    sc.UpdatedAt,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})

	return summaries, nil
}

// DeleteScenario removes a scenario by name or ID.
func (s *ScenarioStore) DeleteScenario(name string) error {
	id := sanitizeName(name)
	path := filepath.Join(s.dir, id+".json")
	return os.Remove(path)
}

// CreateScenarioFromEntries converts captured recorder entries into a structured scenario sequence.
func CreateScenarioFromEntries(name, description string, entries []*recorder.Entry) *Scenario {
	// Sort chronologically (oldest first)
	sorted := make([]*recorder.Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	steps := make([]*ScenarioStep, 0, len(sorted))
	var lastTime time.Time

	// Map to track extracted variables for auto extraction generation
	responseVars := make(map[string]string) // val -> "varName"

	for idx, entry := range sorted {
		var delay int64
		if !lastTime.IsZero() {
			d := entry.Timestamp.Sub(lastTime).Milliseconds()
			if d > 0 && d < 30000 {
				delay = d
			}
		}
		lastTime = entry.Timestamp

		stepID := fmt.Sprintf("step_%d", idx+1)
		stepName := fmt.Sprintf("%s %s", entry.Method, entry.Path)

		step := &ScenarioStep{
			ID:             stepID,
			Name:           stepName,
			Method:         entry.Method,
			Path:           entry.Path,
			Query:          entry.Query,
			Headers:        entry.RequestHeaders,
			Body:           string(entry.RequestBody),
			TargetService:  entry.Host,
			DelayMs:        delay,
			ExpectedStatus: entry.StatusCode,
			CapturedResponse: &StepResponse{
				StatusCode: entry.StatusCode,
				Headers:    entry.ResponseHeaders,
				Body:       string(entry.ResponseBody),
			},
		}

		// Add default status code assertion
		if entry.StatusCode > 0 {
			step.Assertions = append(step.Assertions, AssertionRule{
				Type:     "status_code",
				Expected: fmt.Sprintf("%d", entry.StatusCode),
			})
		}

		// Auto variable extraction detection from JSON response
		if len(entry.ResponseBody) > 0 {
			var js map[string]interface{}
			if err := json.Unmarshal(entry.ResponseBody, &js); err == nil {
				detectAndRegisterVars(js, "", stepID, &step.Extractions, responseVars)
			}
		}

		steps = append(steps, step)
	}

	// Auto replace discovered variables in subsequent request bodies/headers/paths
	if len(responseVars) > 0 {
		for _, step := range steps {
			for val, varRef := range responseVars {
				placeholder := "{{" + varRef + "}}"
				if len(val) >= 4 { // only replace meaningful values
					step.Path = strings.ReplaceAll(step.Path, val, placeholder)
					step.Query = strings.ReplaceAll(step.Query, val, placeholder)
					step.Body = strings.ReplaceAll(step.Body, val, placeholder)
					for hKey, hVals := range step.Headers {
						for hIdx, hVal := range hVals {
							step.Headers[hKey][hIdx] = strings.ReplaceAll(hVal, val, placeholder)
						}
					}
				}
			}
		}
	}

	return &Scenario{
		ID:          sanitizeName(name),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Variables:   make(map[string]string),
		Steps:       steps,
	}
}

// detectAndRegisterVars recursively scans a JSON response map for strings (e.g. token, id) to generate extraction rules.
func detectAndRegisterVars(data map[string]interface{}, prefix, stepID string, extractions *[]ExtractionRule, varMap map[string]string) {
	for k, v := range data {
		fieldKey := k
		if prefix != "" {
			fieldKey = prefix + "." + k
		}

		switch val := v.(type) {
		case string:
			// check if field name looks like token, id, session, code, etc.
			lk := strings.ToLower(k)
			if (strings.Contains(lk, "token") || strings.Contains(lk, "id") || strings.Contains(lk, "key") || strings.Contains(lk, "code")) && len(val) > 2 {
				varName := stepID + "." + fieldKey
				*extractions = append(*extractions, ExtractionRule{
					VarName:    varName,
					Source:     "body_json",
					Expression: fieldKey,
				})
				varMap[val] = varName
			}
		case map[string]interface{}:
			detectAndRegisterVars(val, fieldKey, stepID, extractions, varMap)
		}
	}
}


