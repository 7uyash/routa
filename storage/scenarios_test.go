package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
)

func TestScenarioStoreAndEngine(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "routa-scenarios-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := NewScenarioStore(tempDir)

	// Create a test scenario manually
	sc := &Scenario{
		Name:        "Checkout Flow Test",
		Description: "Test complete checkout flow",
		Variables: map[string]string{
			"BASE_URL": "http://example.local",
			"USER_ID":  "usr_100",
		},
		Steps: []*ScenarioStep{
			{
				ID:             "step_1",
				Name:           "POST /api/login",
				Method:         "POST",
				Path:           "/api/login",
				Body:           `{"username":"testuser"}`,
				ExpectedStatus: 200,
				Extractions: []ExtractionRule{
					{
						VarName:    "token",
						Source:     "body_json",
						Expression: "token",
					},
				},
				Assertions: []AssertionRule{
					{
						Type:     "status_code",
						Expected: "200",
					},
				},
			},
			{
				ID:             "step_2",
				Name:           "GET /api/user",
				Method:         "GET",
				Path:           "/api/user/{{USER_ID}}",
				Headers:        map[string][]string{"Authorization": {"Bearer {{token}}"}},
				ExpectedStatus: 200,
				Assertions: []AssertionRule{
					{
						Type:     "body_contains",
						Expected: "testuser",
					},
				},
			},
		},
	}

	if err := store.SaveScenario(sc); err != nil {
		t.Fatalf("SaveScenario failed: %v", err)
	}

	// Verify Listing
	list, err := store.ListScenarios()
	if err != nil {
		t.Fatalf("ListScenarios failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(list))
	}
	if list[0].Name != "Checkout Flow Test" {
		t.Errorf("unexpected scenario name: %s", list[0].Name)
	}

	// Verify Loading
	loaded, err := store.LoadScenario(sc.ID)
	if err != nil {
		t.Fatalf("LoadScenario failed: %v", err)
	}
	if len(loaded.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(loaded.Steps))
	}

	// Mock target server for replay
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token":"secret_jwt_xyz_999"}`))
		case "/api/user/usr_100":
			auth := r.Header.Get("Authorization")
			if auth != "Bearer secret_jwt_xyz_999" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"user":"testuser","status":"active"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	rec := recorder.New(100)
	forwarder := proxy.New()
	runner := NewScenarioRunner(store, forwarder, rec)

	// Replay scenario against mock server
	result, err := runner.Replay(context.Background(), ReplayOptions{
		ScenarioName:  sc.ID,
		TargetBaseURL: srv.URL,
		MaintainDelay: false,
	})
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected scenario execution to pass, got failed")
	}
	if result.PassedSteps != 2 {
		t.Errorf("expected 2 passed steps, got %d", result.PassedSteps)
	}
	if result.RuntimeVariables["token"] != "secret_jwt_xyz_999" {
		t.Errorf("expected extracted token 'secret_jwt_xyz_999', got %q", result.RuntimeVariables["token"])
	}

	// Test Deletion
	if err := store.DeleteScenario(sc.ID); err != nil {
		t.Fatalf("DeleteScenario failed: %v", err)
	}
	listAfter, _ := store.ListScenarios()
	if len(listAfter) != 0 {
		t.Errorf("expected 0 scenarios after delete, got %d", len(listAfter))
	}
}

func TestCreateScenarioFromEntries(t *testing.T) {
	now := time.Now()
	entries := []*recorder.Entry{
		{
			Timestamp:      now,
			Method:         "POST",
			Path:           "/api/login",
			StatusCode:     200,
			ResponseBody:   []byte(`{"token":"abc123token"}`),
			RequestHeaders: map[string][]string{"Content-Type": {"application/json"}},
		},
		{
			Timestamp:      now.Add(200 * time.Millisecond),
			Method:         "GET",
			Path:           "/api/profile",
			Query:          "token=abc123token",
			StatusCode:     200,
			ResponseBody:   []byte(`{"username":"john"}`),
			RequestHeaders: map[string][]string{"Authorization": {"Bearer abc123token"}},
		},
	}

	sc := CreateScenarioFromEntries("user-flow", "auto generated", entries)
	if len(sc.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(sc.Steps))
	}

	// Step 2 query/header should have been converted to {{step_1.token}} placeholder
	step2 := sc.Steps[1]
	if step2.Query != "token={{step_1.token}}" {
		t.Errorf("expected query templated variable, got %q", step2.Query)
	}
	if step2.Headers["Authorization"][0] != "Bearer {{step_1.token}}" {
		t.Errorf("expected header templated variable, got %q", step2.Headers["Authorization"][0])
	}
}
