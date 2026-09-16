package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/replay"
	"github.com/7uyash/routa/traffic"
)

// ScenarioRecorder controls active traffic recording sessions.
type ScenarioRecorder struct {
	mu           sync.RWMutex
	isRecording  bool
	scenarioName string
	startTime    time.Time
	entries      []*recorder.Entry
}

// NewScenarioRecorder creates a recorder manager.
func NewScenarioRecorder() *ScenarioRecorder {
	return &ScenarioRecorder{}
}

// Start Recording session.
func (sr *ScenarioRecorder) Start(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.isRecording = true
	sr.scenarioName = name
	sr.startTime = time.Now()
	sr.entries = make([]*recorder.Entry, 0)
}

// RecordEntry captures an entry if recording is currently active.
func (sr *ScenarioRecorder) RecordEntry(entry *recorder.Entry) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if !sr.isRecording {
		return
	}
	// Make a shallow copy for recording buffer
	eCopy := *entry
	sr.entries = append(sr.entries, &eCopy)
}

// Stop Recording session and return captured entries.
func (sr *ScenarioRecorder) Stop() (string, []*recorder.Entry) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.isRecording = false
	entries := sr.entries
	name := sr.scenarioName
	sr.entries = nil
	return name, entries
}

// Status returns the current recording status.
type RecordingStatus struct {
	IsRecording  bool      `json:"is_recording"`
	ScenarioName string    `json:"scenario_name"`
	RequestCount int       `json:"request_count"`
	DurationSec  float64   `json:"duration_sec"`
	StartTime    time.Time `json:"start_time"`
}

// GetStatus retrieves active recording state.
func (sr *ScenarioRecorder) GetStatus() RecordingStatus {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	var dur float64
	if sr.isRecording {
		dur = time.Since(sr.startTime).Seconds()
	}
	return RecordingStatus{
		IsRecording:  sr.isRecording,
		ScenarioName: sr.scenarioName,
		RequestCount: len(sr.entries),
		DurationSec:  dur,
		StartTime:    sr.startTime,
	}
}

// AssertionResult holds result of a step assertion.
type AssertionResult struct {
	Type     string `json:"type"`
	Target   string `json:"target,omitempty"`
	Expected string `json:"expected"`
	Received string `json:"received"`
	Passed   bool   `json:"passed"`
	Error    string `json:"error,omitempty"`
}

// StepExecutionResult holds outcome of running a single scenario step.
type StepExecutionResult struct {
	StepID           string             `json:"step_id"`
	StepName         string             `json:"step_name"`
	Method           string             `json:"method"`
	Path             string             `json:"path"`
	FullURL          string             `json:"full_url"`
	StatusCode       int                `json:"status_code"`
	ExpectedStatus   int                `json:"expected_status"`
	DurationMs       int64              `json:"duration_ms"`
	DelayMs          int64              `json:"delay_ms"`
	Passed           bool               `json:"passed"`
	AssertionResults []AssertionResult  `json:"assertion_results"`
	ExtractedVars    map[string]string  `json:"extracted_vars,omitempty"`
	ResponseBody     string             `json:"response_body,omitempty"`
	Error            string             `json:"error,omitempty"`
}

// ScenarioExecutionResult represents the overall outcome of a scenario replay.
type ScenarioExecutionResult struct {
	ScenarioID       string                 `json:"scenario_id"`
	ScenarioName     string                 `json:"scenario_name"`
	Passed           bool                   `json:"passed"`
	TotalSteps       int                    `json:"total_steps"`
	PassedSteps      int                    `json:"passed_steps"`
	FailedSteps      int                    `json:"failed_steps"`
	TotalDurationMs  int64                  `json:"total_duration_ms"`
	StepResults      []*StepExecutionResult `json:"step_results"`
	RuntimeVariables map[string]string      `json:"runtime_variables"`
}

// ScenarioRunner executes saved scenarios.
type ScenarioRunner struct {
	store    *ScenarioStore
	executor *replay.Executor
}

// NewScenarioRunner creates a scenario runner.
func NewScenarioRunner(store *ScenarioStore, p *proxy.Forwarder, r *recorder.Recorder) *ScenarioRunner {
	return &ScenarioRunner{
		store:    store,
		executor: replay.NewExecutor(p, r),
	}
}

// ReplayOptions configures a scenario replay run.
type ReplayOptions struct {
	ScenarioName     string            `json:"scenario_name"`
	TargetBaseURL    string            `json:"target_base_url"`
	MaintainDelay    bool              `json:"maintain_delay"`
	InitialVariables map[string]string `json:"initial_variables,omitempty"`
}

// Replay executes a scenario step-by-step.
func (sr *ScenarioRunner) Replay(ctx context.Context, opts ReplayOptions) (*ScenarioExecutionResult, error) {
	scenario, err := sr.store.LoadScenario(opts.ScenarioName)
	if err != nil {
		return nil, err
	}

	runtimeVars := make(map[string]string)
	// Initialize default variables from scenario definition
	for k, v := range scenario.Variables {
		runtimeVars[k] = v
	}
	// Override with initial variables from options
	for k, v := range opts.InitialVariables {
		runtimeVars[k] = v
	}

	execResult := &ScenarioExecutionResult{
		ScenarioID:       scenario.ID,
		ScenarioName:     scenario.Name,
		Passed:           true,
		TotalSteps:       len(scenario.Steps),
		StepResults:      make([]*StepExecutionResult, 0, len(scenario.Steps)),
		RuntimeVariables: runtimeVars,
	}

	startTime := time.Now()

	for _, step := range scenario.Steps {
		select {
		case <-ctx.Done():
			execResult.Passed = false
			return execResult, ctx.Err()
		default:
		}

		// Inter-request delay maintenance
		if opts.MaintainDelay && step.DelayMs > 0 && step.DelayMs < 30000 {
			time.Sleep(time.Duration(step.DelayMs) * time.Millisecond)
		}

		// Template substitution
		renderedPath := replaceVariables(step.Path, runtimeVars)
		renderedQuery := replaceVariables(step.Query, runtimeVars)
		renderedBody := replaceVariables(step.Body, runtimeVars)

		renderedHeaders := make(map[string][]string)
		for hK, hVals := range step.Headers {
			var newVals []string
			for _, hV := range hVals {
				newVals = append(newVals, replaceVariables(hV, runtimeVars))
			}
			renderedHeaders[hK] = newVals
		}

		// Target resolution
		baseURL := opts.TargetBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}

		fullURL := replay.BuildTargetURL(baseURL, renderedPath, renderedQuery)

		stepReq := traffic.Request{
			Method:  step.Method,
			Path:    renderedPath,
			Query:   renderedQuery,
			Headers: renderedHeaders,
			Body:    []byte(renderedBody),
		}

		execOpts := replay.ExecuteOptions{
			Source:   "scenario",
			IsReplay: true,
			Tags:     []string{"scenario", scenario.Name},
			Record:   true,
		}

		stepStart := time.Now()
		_, resp, fwdErr := sr.executor.Execute(stepReq, fullURL, execOpts)
		stepDuration := time.Since(stepStart).Milliseconds()

		stepRes := &StepExecutionResult{
			StepID:         step.ID,
			StepName:       step.Name,
			Method:         step.Method,
			Path:           renderedPath,
			FullURL:        fullURL,
			ExpectedStatus: step.ExpectedStatus,
			DurationMs:     stepDuration,
			DelayMs:        step.DelayMs,
			Passed:         true,
			ExtractedVars:  make(map[string]string),
		}

		if fwdErr != nil {
			stepRes.StatusCode = 502
			stepRes.Error = fwdErr.Error()
			stepRes.Passed = false
		} else {
			stepRes.StatusCode = resp.StatusCode
			stepRes.ResponseBody = string(resp.Body)
		}

		// Evaluate Assertions
		for _, assertion := range step.Assertions {
			asRes := evaluateAssertion(assertion, stepRes.StatusCode, resp)
			stepRes.AssertionResults = append(stepRes.AssertionResults, asRes)
			if !asRes.Passed {
				stepRes.Passed = false
			}
		}

		// If no custom assertions existed, fall back to matching expected status code
		if len(step.Assertions) == 0 && step.ExpectedStatus > 0 {
			passed := stepRes.StatusCode == step.ExpectedStatus
			stepRes.AssertionResults = append(stepRes.AssertionResults, AssertionResult{
				Type:     "status_code",
				Expected: strconv.Itoa(step.ExpectedStatus),
				Received: strconv.Itoa(stepRes.StatusCode),
				Passed:   passed,
			})
			if !passed {
				stepRes.Passed = false
			}
		}

		// Perform Variable Extractions if step succeeded/partially succeeded
		if resp != nil {
			for _, ext := range step.Extractions {
				val := extractVariable(ext, resp)
				if val != "" {
					stepRes.ExtractedVars[ext.VarName] = val
					runtimeVars[ext.VarName] = val
				}
			}
		}

		execResult.StepResults = append(execResult.StepResults, stepRes)
		if stepRes.Passed {
			execResult.PassedSteps++
		} else {
			execResult.FailedSteps++
			execResult.Passed = false
		}
	}

	execResult.TotalDurationMs = time.Since(startTime).Milliseconds()
	return execResult, nil
}

// replaceVariables substitutes mustache style templates like {{token}} or {{USER_ID}} with map values.
var varPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

func replaceVariables(input string, vars map[string]string) string {
	if input == "" || len(vars) == 0 {
		return input
	}
	return varPattern.ReplaceAllStringFunc(input, func(m string) string {
		match := varPattern.FindStringSubmatch(m)
		if len(match) > 1 {
			varName := match[1]
			if val, ok := vars[varName]; ok {
				return val
			}
		}
		return m
	})
}

// evaluateAssertion checks an assertion rule against status code and response payload.
func evaluateAssertion(rule AssertionRule, statusCode int, resp *traffic.Response) AssertionResult {
	res := AssertionResult{
		Type:     rule.Type,
		Target:   rule.Target,
		Expected: rule.Expected,
		Passed:   false,
	}

	switch rule.Type {
	case "status_code":
		res.Received = strconv.Itoa(statusCode)
		res.Passed = res.Received == rule.Expected

	case "body_contains":
		if resp != nil {
			res.Received = string(resp.Body)
			res.Passed = strings.Contains(res.Received, rule.Expected)
		} else {
			res.Received = ""
			res.Passed = false
		}

	case "json_path_eq":
		if resp != nil && len(resp.Body) > 0 {
			var js interface{}
			if err := json.Unmarshal(resp.Body, &js); err == nil {
				val := lookupJSONPath(js, rule.Target)
				res.Received = fmt.Sprintf("%v", val)
				res.Passed = res.Received == rule.Expected
			} else {
				res.Error = "invalid response json"
			}
		} else {
			res.Error = "empty response body"
		}

	default:
		res.Error = "unknown assertion type: " + rule.Type
	}

	return res
}

// extractVariable extracts variable values from response body or headers based on extraction rules.
func extractVariable(ext ExtractionRule, resp *traffic.Response) string {
	if resp == nil {
		return ""
	}

	switch ext.Source {
	case "body_json":
		if len(resp.Body) == 0 {
			return ""
		}
		var js interface{}
		if err := json.Unmarshal(resp.Body, &js); err == nil {
			val := lookupJSONPath(js, ext.Expression)
			if val != nil {
				return fmt.Sprintf("%v", val)
			}
		}
	case "header":
		for k, v := range resp.Headers {
			if strings.EqualFold(k, ext.Expression) && len(v) > 0 {
				return v[0]
			}
		}
	case "body_regex":
		re, err := regexp.Compile(ext.Expression)
		if err == nil {
			m := re.FindStringSubmatch(string(resp.Body))
			if len(m) > 1 {
				return m[1]
			} else if len(m) > 0 {
				return m[0]
			}
		}
	}
	return ""
}

// lookupJSONPath navigates nested map/slice objects using dot notation (e.g., "user.token" or "$.token").
func lookupJSONPath(data interface{}, path string) interface{} {
	cleanPath := strings.TrimPrefix(path, "$.")
	cleanPath = strings.TrimPrefix(cleanPath, "$")
	if cleanPath == "" {
		return data
	}

	parts := strings.Split(cleanPath, ".")
	current := data

	for _, part := range parts {
		if part == "" {
			continue
		}
		switch val := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = val[part]
			if !ok {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}
