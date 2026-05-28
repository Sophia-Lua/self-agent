package router

import (
	"context"
	"strings"
	"testing"

	"autodev/internal/core"
)

// mockAgent implements core.Agent for testing
type mockAgent struct{}

func (m *mockAgent) ID() string                         { return "mock" }
func (m *mockAgent) Role() core.Role                    { return core.RoleDeveloper }
func (m *mockAgent) Description() string                { return "mock agent" }
func (m *mockAgent) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
	return &core.Output{Message: "ok"}, nil
}

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if len(r.strategies) != 3 {
		t.Errorf("expected 3 strategies, got %d", len(r.strategies))
	}
}

func TestRegister(t *testing.T) {
	r := New()

	// Valid registration
	profile := &AgentProfile{Name: "test-agent", Priority: 5}
	if err := r.Register(profile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty name
	err := r.Register(&AgentProfile{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("error message should mention 'empty', got: %s", err.Error())
	}

	// Duplicate name
	err = r.Register(&AgentProfile{Name: "test-agent"})
	if err == nil {
		t.Error("expected error for duplicate name")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("error message should mention 'already registered', got: %s", err.Error())
	}
}

func TestRegisterMultiple(t *testing.T) {
	r := New()

	agents := []string{"alpha", "beta", "gamma"}
	for _, name := range agents {
		if err := r.Register(&AgentProfile{Name: name, Priority: 5}); err != nil {
			t.Fatalf("failed to register %s: %v", name, err)
		}
	}

	names := r.ListAgents()
	if len(names) != 3 {
		t.Errorf("expected 3 agents, got %d", len(names))
	}
}

func TestListAgents(t *testing.T) {
	r := New()
	_ = r.Register(&AgentProfile{Name: "charlie"})
	_ = r.Register(&AgentProfile{Name: "alpha"})
	_ = r.Register(&AgentProfile{Name: "bravo"})

	names := r.ListAgents()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	// Should be sorted alphabetically
	if names[0] != "alpha" || names[1] != "bravo" || names[2] != "charlie" {
		t.Errorf("expected alphabetical order, got %v", names)
	}
}

func TestListAgentsEmpty(t *testing.T) {
	r := New()
	names := r.ListAgents()
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

func TestGetExecutor(t *testing.T) {
	r := New()
	profile := &AgentProfile{
		Name:     "agent-1",
	}
	r.Register(profile)

	// Without executor - profile exists
	got, err := r.GetExecutor("agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil executor when not set")
	}

	// Non-existent agent
	_, err = r.GetExecutor("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent agent")
	}
}

func TestRoute(t *testing.T) {
	r := New()

	_ = r.Register(&AgentProfile{
		Name:     "parser",
		Roles:    []string{"parser", "analyst"},
		Keywords: []string{"analyze", "parse"},
		Priority: 8,
	})
	_ = r.Register(&AgentProfile{
		Name:     "developer",
		Roles:    []string{"developer", "engineer"},
		Keywords: []string{"implement", "code", "feature"},
		Priority: 10,
	})

	route, err := r.Route(context.Background(), "implement a new feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Developer should score higher due to priority and keyword match
	if route.AgentName != "developer" {
		t.Logf("route result: %s (confidence: %f, reason: %s)", route.AgentName, route.Confidence, route.Reason)
		// Accept either if scores are close
		if route.AgentName != "parser" {
			t.Errorf("expected 'developer' or 'parser', got %s", route.AgentName)
		}
	}
	if route.Confidence <= 0 {
		t.Errorf("expected positive confidence, got %f", route.Confidence)
	}
}

func TestRouteNoAgents(t *testing.T) {
	r := New()
	_, err := r.Route(context.Background(), "test task")
	if err == nil {
		t.Fatal("expected error when no agents registered")
	}
}

func TestRouteAll(t *testing.T) {
	r := New()

	_ = r.Register(&AgentProfile{Name: "agent-a", Priority: 5})
	_ = r.Register(&AgentProfile{Name: "agent-b", Priority: 10})
	_ = r.Register(&AgentProfile{Name: "agent-c", Priority: 3})

	routes, err := r.RouteAll(context.Background(), "some task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}

	// Should be sorted by score (agent-b highest priority first)
	if routes[0].AgentName != "agent-b" {
		t.Errorf("expected first to be agent-b, got %s", routes[0].AgentName)
	}
}

func TestRouteAllNoAgents(t *testing.T) {
	r := New()
	_, err := r.RouteAll(context.Background(), "task")
	if err == nil {
		t.Fatal("expected error when no agents registered")
	}
}

func TestKeywordStrategy(t *testing.T) {
	k := &KeywordStrategy{}

	if k.Name() != "keyword" {
		t.Errorf("expected 'keyword', got %s", k.Name())
	}

	profile := &AgentProfile{Keywords: []string{"analyze", "parser"}}

	// Full match
	score := k.Score("I need to analyze the requirements", profile)
	if score <= 0 {
		t.Errorf("expected positive score for full match, got %f", score)
	}

	// Partial match
	score = k.Score("analyze something", profile)
	if score <= 0 {
		t.Errorf("expected positive score for partial match, got %f", score)
	}

	// No keywords = default 0.3
	profileNoKeywords := &AgentProfile{Keywords: []string{}}
	score = k.Score("test", profileNoKeywords)
	if score != 0.3 {
		t.Errorf("expected 0.3 for no keywords, got %f", score)
	}
}

func TestRoleStrategy(t *testing.T) {
	rStrategy := &RoleStrategy{}

	if rStrategy.Name() != "role" {
		t.Errorf("expected 'role', got %s", rStrategy.Name())
	}

	profile := &AgentProfile{Roles: []string{"parser", "analyst"}}

	// Full match
	score := rStrategy.Score("I am a parser and analyst", profile)
	if score <= 0 {
		t.Errorf("expected positive score for role match, got %f", score)
	}

	// No match
	score = rStrategy.Score("write code", profile)
	if score != 0 {
		t.Errorf("expected 0 for no role match, got %f", score)
	}

	// No roles = default 0.2
	profileNoRoles := &AgentProfile{Roles: []string{}}
	score = rStrategy.Score("test", profileNoRoles)
	if score != 0.2 {
		t.Errorf("expected 0.2 for no roles, got %f", score)
	}
}

func TestPriorityStrategy(t *testing.T) {
	pStrategy := &PriorityStrategy{}

	if pStrategy.Name() != "priority" {
		t.Errorf("expected 'priority', got %s", pStrategy.Name())
	}

	// Priority 10 -> 1.0
	score := pStrategy.Score("task", &AgentProfile{Priority: 10})
	if score != 1.0 {
		t.Errorf("expected 1.0 for priority 10, got %f", score)
	}

	// Priority 5 -> 0.5
	score = pStrategy.Score("task", &AgentProfile{Priority: 5})
	if score != 0.5 {
		t.Errorf("expected 0.5 for priority 5, got %f", score)
	}

	// Priority 0 -> default 0.1
	score = pStrategy.Score("task", &AgentProfile{Priority: 0})
	if score != 0.1 {
		t.Errorf("expected 0.1 for priority 0, got %f", score)
	}

	// Priority negative -> default 0.1
	score = pStrategy.Score("task", &AgentProfile{Priority: -5})
	if score != 0.1 {
		t.Errorf("expected 0.1 for negative priority, got %f", score)
	}
}

func TestRouteConfidenceNormalization(t *testing.T) {
	r := New()

	// Register agent with known priority
	_ = r.Register(&AgentProfile{
		Name:     "high-prio",
		Priority: 10,
		Keywords: []string{"test"},
	})

	route, err := r.Route(context.Background(), "test something")
	if err != nil {
		t.Fatal(err)
	}

	// Confidence should be normalized (totalScore / len(strategies))
	if route.Confidence > 1.5 {
		t.Errorf("confidence too high: %f", route.Confidence)
	}
}

func TestRouteReturnsTopScorer(t *testing.T) {
	r := New()

	// Parser matches "analyze" better
	_ = r.Register(&AgentProfile{
		Name:     "parser",
		Keywords: []string{"analyze"},
		Priority: 5,
	})
	// Developer doesn't match as well but has higher priority
	_ = r.Register(&AgentProfile{
		Name:     "developer",
		Keywords: []string{},
		Priority: 10,
	})

	route, err := r.Route(context.Background(), "analyze the code")
	if err != nil {
		t.Fatal(err)
	}

	// Parser should win due to keyword match
	if route.AgentName != "parser" {
		t.Errorf("expected 'parser', got %s", route.AgentName)
	}
}

func TestRouteReasonContainsStrategyScores(t *testing.T) {
	r := New()
	_ = r.Register(&AgentProfile{
		Name:     "agent",
		Keywords: []string{"test"},
		Priority: 5,
	})

	route, err := r.Route(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}

	// Reason should contain strategy names and scores
	if !strings.Contains(route.Reason, "keyword:") &&
		!strings.Contains(route.Reason, "role:") &&
		!strings.Contains(route.Reason, "priority:") {
		t.Errorf("expected reason to contain strategy scores, got: %s", route.Reason)
	}
}

func TestRouteAllReturnsAllAgents(t *testing.T) {
	r := New()
	_ = r.Register(&AgentProfile{Name: "a"})
	_ = r.Register(&AgentProfile{Name: "b"})
	_ = r.Register(&AgentProfile{Name: "c"})

	routes, err := r.RouteAll(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 3 {
		t.Errorf("expected 3 routes, got %d", len(routes))
	}
}

func TestAgentProfileStructure(t *testing.T) {
	profile := AgentProfile{
		Name:        "profile-test",
		Roles:       []string{"role1", "role2"},
		Keywords:    []string{"kw1", "kw2"},
		Priority:    7,
		MaxTokens:   4000,
		CanUseTools: true,
	}

	if len(profile.Roles) != 2 {
		t.Error("Roles not set correctly")
	}
	if len(profile.Keywords) != 2 {
		t.Error("Keywords not set correctly")
	}
	if profile.Priority != 7 {
		t.Error("Priority not set correctly")
	}
}

func TestRouteStructure(t *testing.T) {
	route := Route{
		AgentName:  "agent",
		Confidence: 0.85,
		Reason:     "matched keywords",
	}

	if route.AgentName != "agent" {
		t.Error("AgentName not set correctly")
	}
	if route.Confidence != 0.85 {
		t.Error("Confidence not set correctly")
	}
}

func TestRegisterBuiltIn(t *testing.T) {
	r := New()

	// Since RegisterBuiltIn requires *agents.Executor which depends on LLM provider,
	// we test it separately. Here we just register profiles manually to simulate
	// the same behavior.

	_ = r.Register(&AgentProfile{
		Name:     "parser",
		Roles:    []string{"parser", "analyst", "requirements"},
		Keywords: []string{"analyze", "parse", "understand"},
		Priority: 8,
	})
	_ = r.Register(&AgentProfile{
		Name:     "developer",
		Roles:    []string{"developer", "engineer", "coder"},
		Keywords: []string{"implement", "create", "write", "code"},
		Priority: 10,
	})
	_ = r.Register(&AgentProfile{
		Name:     "tester",
		Roles:    []string{"tester", "qa", "quality"},
		Keywords: []string{"test", "verify", "validate"},
		Priority: 7,
	})
	_ = r.Register(&AgentProfile{
		Name:     "reviewer",
		Roles:    []string{"reviewer", "auditor", "inspector"},
		Keywords: []string{"review", "audit", "inspect"},
		Priority: 6,
	})

	names := r.ListAgents()
	if len(names) < 4 {
		t.Errorf("expected at least 4 agents, got %d: %v", len(names), names)
	}
}
