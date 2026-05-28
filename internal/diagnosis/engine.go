package diagnosis

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Severity represents the criticality of a diagnosed issue.
type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// Category classifies the type of issue.
type Category string

const (
	CategorySyntax     Category = "syntax_error"
	CategoryType       Category = "type_error"
	CategoryRuntime    Category = "runtime_error"
	CategoryLogic      Category = "logic_error"
	CategoryResource   Category = "resource_issue"
	CategoryDependency Category = "dependency_issue"
	CategoryTimeout    Category = "timeout"
	CategoryUnknown    Category = "unknown"
)

// Issue represents a diagnosed problem with suggested fixes.
type Issue struct {
	ID          string
	Severity    Severity
	Category    Category
	Title       string
	Description string
	Suggestions []string
	Context     string
	Timestamp   time.Time
}

// DiagnosisResult holds all issues found for a given error.
type DiagnosisResult struct {
	OriginalError string
	Issues        []Issue
	Summary       string
	Confidence    float64
}

// Pattern defines a regex pattern for error recognition.
type Pattern struct {
	Name     string
	Regex    *regexp.Regexp
	Category Category
	Severity Severity
	Fixer    func(matches []string) []string
}

// Engine performs automated error diagnosis using pattern matching.
type Engine struct {
	mu      sync.RWMutex
	patterns []Pattern
	history  []DiagnosisResult
}

// New creates a diagnosis engine with built-in patterns.
func New() *Engine {
	e := &Engine{
		patterns: make([]Pattern, 0),
	}
	e.registerDefaultPatterns()
	return e
}

// Analyze diagnoses an error message and returns suggestions.
func (e *Engine) Analyze(errMsg string) DiagnosisResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var issues []Issue
	totalConfidence := 0.0
	matchCount := 0

	for _, pattern := range e.patterns {
		matches := pattern.Regex.FindStringSubmatch(errMsg)
		if matches != nil {
			matchCount++
			suggestions := pattern.Fixer(matches)
			issue := Issue{
				ID:          fmt.Sprintf("ISSUE-%d", len(issues)+1),
				Severity:    pattern.Severity,
				Category:    pattern.Category,
				Title:       pattern.Name,
				Description: errMsg,
				Suggestions: suggestions,
				Timestamp:   time.Now(),
			}
			issues = append(issues, issue)
			totalConfidence += 0.3
		}
	}

	confidence := totalConfidence
	if matchCount > 0 {
		confidence = totalConfidence / float64(matchCount)
		if confidence > 1.0 {
			confidence = 1.0
		}
	}

	summary := generateSummary(issues, errMsg)

	result := DiagnosisResult{
		OriginalError: errMsg,
		Issues:        issues,
		Summary:       summary,
		Confidence:    confidence,
	}

	e.history = append(e.history, result)
	return result
}

// RegisterPattern adds a custom error pattern.
func (e *Engine) RegisterPattern(name, regex string, category Category, severity Severity, fixer func([]string) []string) error {
	re, err := regexp.Compile(regex)
	if err != nil {
		return fmt.Errorf("invalid regex for pattern %q: %w", name, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.patterns = append(e.patterns, Pattern{
		Name:     name,
		Regex:    re,
		Category: category,
		Severity: severity,
		Fixer:    fixer,
	})
	return nil
}

// History returns all past diagnosis results.
func (e *Engine) History() []DiagnosisResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]DiagnosisResult(nil), e.history...)
}

// ClearHistory removes all past diagnoses.
func (e *Engine) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = nil
}

// CommonErrors returns known error patterns and their fixes.
func (e *Engine) CommonErrors() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var names []string
	for _, p := range e.patterns {
		names = append(names, p.Name)
	}
	return names
}

func (e *Engine) registerDefaultPatterns() {
	// Go compilation errors
	e.patterns = append(e.patterns, Pattern{
		Name:     "undefined identifier",
		Regex:    regexp.MustCompile(`undefined:\s+(\w+)`),
		Category: CategorySyntax,
		Severity: SeverityHigh,
		Fixer: func(matches []string) []string {
			return []string{
				fmt.Sprintf("Check if %q is imported or declared", matches[1]),
				"Verify package imports are correct",
				"Check for typos in the identifier name",
			}
		},
	})

	e.patterns = append(e.patterns, Pattern{
		Name:     "import cycle",
		Regex:    regexp.MustCompile(`import cycle not allowed`),
		Category: CategoryDependency,
		Severity: SeverityCritical,
		Fixer: func([]string) []string {
			return []string{
				"Restructure code to break the import cycle",
				"Use interfaces to decouple packages",
				"Move shared types to a separate package",
			}
		},
	})

	e.patterns = append(e.patterns, Pattern{
		Name:     "missing return",
		Regex:    regexp.MustCompile(`missing return at end of function`),
		Category: CategorySyntax,
		Severity: SeverityHigh,
		Fixer: func([]string) []string {
			return []string{
				"Add a return statement at the end of the function",
				"Check all code paths have explicit returns",
			}
		},
	})

	e.patterns = append(e.patterns, Pattern{
		Name:     "type mismatch",
		Regex:    regexp.MustCompile(`cannot use\s+(.+)\s+as\s+(.+)\s+in`),
		Category: CategoryType,
		Severity: SeverityHigh,
		Fixer: func(matches []string) []string {
			return []string{
				fmt.Sprintf("Type mismatch: %s vs %s", matches[1], matches[2]),
				"Check variable types and convert if necessary",
				"Use type assertion or type conversion",
			}
		},
	})

	// Runtime errors
	e.patterns = append(e.patterns, Pattern{
		Name:     "nil pointer dereference",
		Regex:    regexp.MustCompile(`panic:\s+runtime error:\s+nil pointer dereference`),
		Category: CategoryRuntime,
		Severity: SeverityCritical,
		Fixer: func([]string) []string {
			return []string{
				"Add nil checks before accessing pointers",
				"Check if all objects are properly initialized",
				"Review the stack trace to find the nil source",
			}
		},
	})

	e.patterns = append(e.patterns, Pattern{
		Name:     "index out of range",
		Regex:    regexp.MustCompile(`panic:\s+runtime error:\s+index out of range`),
		Category: CategoryRuntime,
		Severity: SeverityHigh,
		Fixer: func([]string) []string {
			return []string{
				"Check array/slice bounds before accessing",
				"Use len() to validate index range",
				"Review loop conditions for off-by-one errors",
			}
		},
	})

	// Dependency errors
	e.patterns = append(e.patterns, Pattern{
		Name:     "module not found",
		Regex:    regexp.MustCompile(`module\s+([^\s]+):\s+not a known dependency`),
		Category: CategoryDependency,
		Severity: SeverityMedium,
		Fixer: func(matches []string) []string {
			return []string{
				fmt.Sprintf("Run: go get %s", matches[1]),
				"Check go.mod for the correct module path",
				"Verify the module version exists",
			}
		},
	})

	e.patterns = append(e.patterns, Pattern{
		Name:     "go mod tidy required",
		Regex:    regexp.MustCompile(`go:\s+(.+):\s+no required module provides package`),
		Category: CategoryDependency,
		Severity: SeverityMedium,
		Fixer: func(matches []string) []string {
			return []string{
				fmt.Sprintf("Run: go get %s", matches[1]),
				"Run: go mod tidy",
				"Check if the package name is correct",
			}
		},
	})

	// Timeout errors
	e.patterns = append(e.patterns, Pattern{
		Name:     "context deadline exceeded",
		Regex:    regexp.MustCompile(`context deadline exceeded`),
		Category: CategoryTimeout,
		Severity: SeverityMedium,
		Fixer: func([]string) []string {
			return []string{
				"Increase the timeout duration",
				"Check for deadlocks or infinite loops",
				"Review network calls for latency issues",
			}
		},
	})

	// Test failures
	e.patterns = append(e.patterns, Pattern{
		Name:     "test assertion failed",
		Regex:    regexp.MustCompile(`--- FAIL:\s+(\S+)`),
		Category: CategoryLogic,
		Severity: SeverityMedium,
		Fixer: func(matches []string) []string {
			return []string{
				fmt.Sprintf("Test %q failed", matches[1]),
				"Check test expectations vs actual output",
				"Review the test case setup",
			}
		},
	})
}

func generateSummary(issues []Issue, originalErr string) string {
	if len(issues) == 0 {
		return fmt.Sprintf("No specific pattern matched for: %s\nTry reviewing the error manually.", truncate(originalErr, 100))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Diagnosed %d issue(s):\n", len(issues)))
	for _, issue := range issues {
		sb.WriteString(fmt.Sprintf("\n[%s] %s\n", severityLabel(issue.Severity), issue.Title))
		if len(issue.Suggestions) > 0 {
			sb.WriteString("Suggestions:\n")
			for i, s := range issue.Suggestions {
				sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
			}
		}
	}
	return sb.String()
}

func severityLabel(s Severity) string {
	switch s {
	case SeverityLow:
		return "LOW"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityHigh:
		return "HIGH"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
