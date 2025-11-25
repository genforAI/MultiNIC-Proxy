package main

import (
	"testing"
)

func TestPolicyManager_CheckPolicy(t *testing.T) {
	// Initialize policy manager
	pm := &PolicyManager{
		policies: make(map[string]HostPolicy),
	}

	// Set up test policies
	pm.policies["example.com"] = HostPolicy{Action: ActionAccelerate}
	pm.policies["bypass.com"] = HostPolicy{Action: ActionPassThrough}
	pm.policies["*"] = HostPolicy{Action: ActionPassThrough}

	tests := []struct {
		name         string
		host         string
		expectAction TrafficAction
	}{
		{
			name:         "Exact match - Accelerate",
			host:         "example.com",
			expectAction: ActionAccelerate,
		},
		{
			name:         "Exact match - PassThrough",
			host:         "bypass.com",
			expectAction: ActionPassThrough,
		},
		{
			name:         "Wildcard match",
			host:         "unknown.com",
			expectAction: ActionPassThrough,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := pm.CheckPolicy(tt.host)
			if policy.Action != tt.expectAction {
				t.Errorf("expected action=%d, got %d", tt.expectAction, policy.Action)
			}
		})
	}
}

func TestPolicyManager_CheckPolicy_NoDefault(t *testing.T) {
	// Initialize policy manager without wildcard default
	pm := &PolicyManager{
		policies: make(map[string]HostPolicy),
	}
	pm.policies["example.com"] = HostPolicy{Action: ActionAccelerate}

	// When no default exists, should return ActionAccelerate
	policy := pm.CheckPolicy("unknown.com")
	if policy.Action != ActionAccelerate {
		t.Errorf("expected default action=%d when no policies match, got %d", ActionAccelerate, policy.Action)
	}
}
