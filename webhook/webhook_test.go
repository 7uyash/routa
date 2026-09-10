package webhook

import (
	"testing"
)

func TestWebhookLabEndpoints(t *testing.T) {
	lab := NewLab()

	ep := lab.CreateEndpoint("stripe", "stripe", "whsec_test123")
	if ep == nil || ep.ID == "" {
		t.Fatal("expected non-empty webhook endpoint ID")
	}
	if ep.Name != "stripe" || ep.Provider != "stripe" {
		t.Errorf("unexpected endpoint metadata: name=%s provider=%s", ep.Name, ep.Provider)
	}

	fetched := lab.GetEndpoint(ep.ID)
	if fetched == nil {
		t.Fatal("expected to find created endpoint")
	}

	// Test toggle endpoint
	active, ok := lab.ToggleEndpoint(ep.ID)
	if !ok || active != false {
		t.Errorf("expected endpoint toggle to inactive, got active=%v ok=%v", active, ok)
	}

	// Test simulate delivery
	delivery, ok := lab.SimulateTestDelivery(ep.ID)
	if !ok || delivery == nil {
		t.Fatal("expected simulated test delivery")
	}
	if delivery.EndpointID != ep.ID {
		t.Errorf("unexpected delivery endpoint ID: %s", delivery.EndpointID)
	}

	deliveries := lab.GetDeliveries(ep.ID)
	if len(deliveries) != 1 {
		t.Errorf("expected 1 delivery record, got %d", len(deliveries))
	}

	// Test delete endpoint
	lab.DeleteEndpoint(ep.ID)
}
