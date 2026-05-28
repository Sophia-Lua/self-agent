package router

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"autodev/internal/agents"
)

// Route represents a routing decision with confidence score.
type Route struct {
	AgentName  string
	Confidence float64
	Reason     string
}

// Router selects the best agent(s) for a given task.
type Router struct {
	agents     map[string]*AgentProfile
	strategies []RoutingStrategy
}

// AgentProfile describes an agent's capabilities for routing purposes.
type AgentProfile struct {
	Name        string
	Roles       []string
	Keywords    []string
	Executor    *agents.Executor
	Priority    int
	MaxTokens   int
	CanUseTools bool
}

// RoutingStrategy defines how to score agent matching.
type RoutingStrategy interface {
	Name() string
	Score(task string, profile *AgentProfile) float64
}

// New creates a Router with built-in strategies.
func New() *Router {
	return &Router{
		agents: make(map[string]*AgentProfile),
		strategies: []RoutingStrategy{
			&KeywordStrategy{},
			&RoleStrategy{},
			&PriorityStrategy{},
		},
	}
}

// Register adds an agent to the routing table.
func (r *Router) Register(profile *AgentProfile) error {
	if profile.Name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}
	if _, exists := r.agents[profile.Name]; exists {
		return fmt.Errorf("agent %q already registered", profile.Name)
	}
	r.agents[profile.Name] = profile
	return nil
}

// Route returns the best matching agent for the task.
func (r *Router) Route(ctx context.Context, task string) (*Route, error) {
	if len(r.agents) == 0 {
		return nil, fmt.Errorf("no agents registered")
	}

	scores := make(map[string]float64)
	reasons := make(map[string]string)

	for name, profile := range r.agents {
		totalScore := 0.0
		reasonParts := []string{}

		for _, strategy := range r.strategies {
			score := strategy.Score(task, profile)
			totalScore += score
			if score > 0.2 {
				reasonParts = append(reasonParts, fmt.Sprintf("%s:%.2f", strategy.Name(), score))
			}
		}

		scores[name] = totalScore
		reasons[name] = strings.Join(reasonParts, ", ")
	}

	// Find highest scoring agent
	type scoredAgent struct {
		Name  string
		Score float64
	}

	var ranked []scoredAgent
	for name, score := range scores {
		ranked = append(ranked, scoredAgent{name, score})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	if len(ranked) == 0 {
		return nil, fmt.Errorf("no suitable agent found for task")
	}

	best := ranked[0]
	return &Route{
		AgentName:  best.Name,
		Confidence: best.Score / float64(len(r.strategies)),
		Reason:     reasons[best.Name],
	}, nil
}

// RouteAll returns all agents sorted by suitability.
func (r *Router) RouteAll(ctx context.Context, task string) ([]Route, error) {
	if len(r.agents) == 0 {
		return nil, fmt.Errorf("no agents registered")
	}

	type scoredRoute struct {
		Route
		score float64
	}

	var routes []scoredRoute
	for name, profile := range r.agents {
		totalScore := 0.0
		reasonParts := []string{}

		for _, strategy := range r.strategies {
			score := strategy.Score(task, profile)
			totalScore += score
			if score > 0.1 {
				reasonParts = append(reasonParts, fmt.Sprintf("%s:%.2f", strategy.Name(), score))
			}
		}

		routes = append(routes, scoredRoute{
			Route: Route{
				AgentName:  name,
				Confidence: totalScore / float64(len(r.strategies)),
				Reason:     strings.Join(reasonParts, ", "),
			},
			score: totalScore,
		})
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].score > routes[j].score
	})

	result := make([]Route, len(routes))
	for i, r := range routes {
		result[i] = r.Route
	}
	return result, nil
}

// GetExecutor returns the executor for a named agent.
func (r *Router) GetExecutor(name string) (*agents.Executor, error) {
	profile, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return profile.Executor, nil
}

// ListAgents returns all registered agent names.
func (r *Router) ListAgents() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// KeywordStrategy scores based on keyword matching.
type KeywordStrategy struct{}

func (k *KeywordStrategy) Name() string { return "keyword" }

func (k *KeywordStrategy) Score(task string, profile *AgentProfile) float64 {
	taskLower := strings.ToLower(task)
	words := strings.Fields(taskLower)

	matchCount := 0
	totalKeywords := len(profile.Keywords)

	if totalKeywords == 0 {
		return 0.3 // Default neutral score
	}

	for _, keyword := range profile.Keywords {
		kw := strings.ToLower(keyword)
		if strings.Contains(taskLower, kw) {
			matchCount++
			continue
		}
		for _, word := range words {
			if strings.Contains(word, kw) || strings.Contains(kw, word) {
				matchCount++
				break
			}
		}
	}

	return float64(matchCount) / float64(totalKeywords)
}

// RoleStrategy scores based on role matching.
type RoleStrategy struct{}

func (r *RoleStrategy) Name() string { return "role" }

func (r *RoleStrategy) Score(task string, profile *AgentProfile) float64 {
	taskLower := strings.ToLower(task)
	matchCount := 0
	totalRoles := len(profile.Roles)

	if totalRoles == 0 {
		return 0.2
	}

	for _, role := range profile.Roles {
		roleLower := strings.ToLower(role)
		if strings.Contains(taskLower, roleLower) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(totalRoles)
}

// PriorityStrategy returns the agent priority as a normalized score.
type PriorityStrategy struct{}

func (p *PriorityStrategy) Name() string { return "priority" }

func (p *PriorityStrategy) Score(task string, profile *AgentProfile) float64 {
	if profile.Priority <= 0 {
		return 0.1
	}
	return float64(profile.Priority) / 10.0
}

// RegisterBuiltIn registers standard pipeline agents with routing profiles.
func RegisterBuiltIn(r *Router, parser, dev, tester, checker *agents.Executor) error {
	builtIns := []*AgentProfile{
		{
			Name:        "parser",
			Roles:       []string{"parser", "analyst", "requirements"},
			Keywords:    []string{"analyze", "parse", "understand", "requirements", "spec", "design", "plan", "investigate"},
			Executor:    parser,
			Priority:    8,
			CanUseTools: true,
		},
		{
			Name:        "developer",
			Roles:       []string{"developer", "engineer", "coder"},
			Keywords:    []string{"implement", "create", "write", "code", "develop", "build", "add", "feature", "function", "method", "class", "component"},
			Executor:    dev,
			Priority:    10,
			CanUseTools: true,
		},
		{
			Name:        "tester",
			Roles:       []string{"tester", "qa", "quality"},
			Keywords:    []string{"test", "verify", "validate", "check", "unit", "integration", "coverage", "assert", "bug", "fail", "error"},
			Executor:    tester,
			Priority:    7,
			CanUseTools: true,
		},
		{
			Name:        "reviewer",
			Roles:       []string{"reviewer", "auditor", "inspector"},
			Keywords:    []string{"review", "audit", "inspect", "lint", "quality", "standard", "refactor", "improve", "optimize", "clean"},
			Executor:    checker,
			Priority:    6,
			CanUseTools: true,
		},
	}

	for _, profile := range builtIns {
		if profile.Executor != nil {
			if err := r.Register(profile); err != nil {
				return fmt.Errorf("failed to register agent %s: %w", profile.Name, err)
			}
		}
	}

	return nil
}
