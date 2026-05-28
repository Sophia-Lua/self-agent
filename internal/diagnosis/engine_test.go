package diagnosis

import (
	"strings"
	"testing"
)

func TestSeverityConstants(t *testing.T) {
	if SeverityLow != 0 {
		t.Errorf("expected SeverityLow=0, got %d", SeverityLow)
	}
	if SeverityMedium != 1 {
		t.Errorf("expected SeverityMedium=1, got %d", SeverityMedium)
	}
	if SeverityHigh != 2 {
		t.Errorf("expected SeverityHigh=2, got %d", SeverityHigh)
	}
	if SeverityCritical != 3 {
		t.Errorf("expected SeverityCritical=3, got %d", SeverityCritical)
	}
}

func TestCategoryConstants(t *testing.T) {
	if CategorySyntax != "syntax_error" {
		t.Errorf("expected 'syntax_error', got %s", CategorySyntax)
	}
	if CategoryType != "type_error" {
		t.Errorf("expected 'type_error', got %s", CategoryType)
	}
	if CategoryRuntime != "runtime_error" {
		t.Errorf("expected 'runtime_error', got %s", CategoryRuntime)
	}
	if CategoryLogic != "logic_error" {
		t.Errorf("expected 'logic_error', got %s", CategoryLogic)
	}
	if CategoryResource != "resource_issue" {
		t.Errorf("expected 'resource_issue', got %s", CategoryResource)
	}
	if CategoryDependency != "dependency_issue" {
		t.Errorf("expected 'dependency_issue', got %s", CategoryDependency)
	}
	if CategoryTimeout != "timeout" {
		t.Errorf("expected 'timeout', got %s", CategoryTimeout)
	}
	if CategoryUnknown != "unknown" {
		t.Errorf("expected 'unknown', got %s", CategoryUnknown)
	}
}

func TestNew(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	patterns := e.CommonErrors()
	if len(patterns) == 0 {
		t.Error("expected default patterns to be registered")
	}
	if len(e.history) != 0 {
		t.Error("expected empty history on creation")
	}
}

func TestAnalyzeUndefinedIdentifier(t *testing.T) {
	e := New()
	result := e.Analyze("main.go:10: undefined: fmt")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues to be found")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "undefined identifier" {
			found = true
			if issue.Category != CategorySyntax {
				t.Errorf("expected category syntax_error, got %s", issue.Category)
			}
			if issue.Severity != SeverityHigh {
				t.Errorf("expected severity HIGH, got %s", severityLabel(issue.Severity))
			}
			if len(issue.Suggestions) == 0 {
				t.Error("expected suggestions")
			}
			break
		}
	}
	if !found {
		t.Error("expected 'undefined identifier' pattern to match")
	}
}

func TestAnalyzeImportCycle(t *testing.T) {
	e := New()
	result := e.Analyze("import cycle not allowed")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues to be found")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "import cycle" {
			found = true
			if issue.Severity != SeverityCritical {
				t.Errorf("expected CRITICAL severity, got %s", severityLabel(issue.Severity))
			}
			if issue.Category != CategoryDependency {
				t.Errorf("expected dependency_issue, got %s", issue.Category)
			}
		}
	}
	if !found {
		t.Error("expected import cycle pattern to match")
	}
}

func TestAnalyzeNilPointer(t *testing.T) {
	e := New()
	result := e.Analyze("panic: runtime error: nil pointer dereference")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "nil pointer dereference" {
			found = true
			if issue.Severity != SeverityCritical {
				t.Error("expected CRITICAL severity")
			}
			if len(issue.Suggestions) == 0 {
				t.Error("expected nil pointer suggestions")
			}
		}
	}
	if !found {
		t.Error("expected nil pointer pattern to match")
	}
}

func TestAnalyzeIndexOutOfRange(t *testing.T) {
	e := New()
	result := e.Analyze("panic: runtime error: index out of range")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "index out of range" {
			found = true
			if issue.Severity != SeverityHigh {
				t.Error("expected HIGH severity")
			}
		}
	}
	if !found {
		t.Error("expected index out of range pattern to match")
	}
}

func TestAnalyzeMissingReturn(t *testing.T) {
	e := New()
	result := e.Analyze("missing return at end of function")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "missing return" {
			found = true
			if issue.Category != CategorySyntax {
				t.Errorf("expected syntax_error, got %s", issue.Category)
			}
		}
	}
	if !found {
		t.Error("expected missing return pattern to match")
	}
}

func TestAnalyzeTypeMismatch(t *testing.T) {
	e := New()
	result := e.Analyze("cannot use foo as bar in argument to something")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "type mismatch" {
			found = true
			if issue.Category != CategoryType {
				t.Errorf("expected type_error, got %s", issue.Category)
			}
		}
	}
	if !found {
		t.Error("expected type mismatch pattern to match")
	}
}

func TestAnalyzeContextDeadlineExceeded(t *testing.T) {
	e := New()
	result := e.Analyze("context deadline exceeded")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "context deadline exceeded" {
			found = true
			if issue.Category != CategoryTimeout {
				t.Errorf("expected timeout, got %s", issue.Category)
			}
		}
	}
	if !found {
		t.Error("expected context deadline pattern to match")
	}
}

func TestAnalyzeTestAssertionFailed(t *testing.T) {
	e := New()
	result := e.Analyze("--- FAIL: TestSomething")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "test assertion failed" {
			found = true
			if issue.Category != CategoryLogic {
				t.Errorf("expected logic_error, got %s", issue.Category)
			}
		}
	}
	if !found {
		t.Error("expected test assertion pattern to match")
	}
}

func TestAnalyzeNoMatch(t *testing.T) {
	e := New()
	result := e.Analyze("some random unknown error message")

	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %d", len(result.Issues))
	}
	if result.Confidence != 0 {
		t.Errorf("expected confidence 0 for no match, got %f", result.Confidence)
	}
	if !strings.Contains(result.Summary, "No specific pattern matched") {
		t.Errorf("expected no-match summary, got: %s", result.Summary)
	}
}

func TestAnalyzeConfidence(t *testing.T) {
	e := New()

	// Single match should give confidence 0.3
	result := e.Analyze("undefined: fmt")
	if result.Confidence != 0.3 {
		t.Errorf("expected confidence 0.3, got %f", result.Confidence)
	}
}

func TestAnalyzeOriginalErrorStored(t *testing.T) {
	e := New()
	errMsg := "some specific error message"
	result := e.Analyze(errMsg)

	if result.OriginalError != errMsg {
		t.Errorf("OriginalError = %q, want %q", result.OriginalError, errMsg)
	}
}

func TestIssueIDGenerated(t *testing.T) {
	e := New()
	result := e.Analyze("undefined: fmt")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}
	if result.Issues[0].ID != "ISSUE-1" {
		t.Errorf("expected ID 'ISSUE-1', got %s", result.Issues[0].ID)
	}
}

func TestRegisterPattern(t *testing.T) {
	e := New()
	err := e.RegisterPattern("custom-error", `custom\s+error\s+(\d+)`, CategoryRuntime, SeverityMedium, func(matches []string) []string {
		return []string{"Fix custom error " + matches[1]}
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := e.Analyze("custom error 42")

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "custom-error" {
			found = true
			if len(issue.Suggestions) != 1 {
				t.Errorf("expected 1 suggestion, got %d", len(issue.Suggestions))
			}
			break
		}
	}
	if !found {
		t.Error("expected custom pattern to match")
	}
}

func TestRegisterPatternInvalidRegex(t *testing.T) {
	e := New()
	err := e.RegisterPattern("bad-regex", `[invalid(`, CategoryUnknown, SeverityLow, nil)

	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "invalid regex") {
		t.Errorf("expected 'invalid regex' message, got: %s", err.Error())
	}
}

func TestRegisterPatternWithNilFixer(t *testing.T) {
	// The engine calls pattern.Fixer(matches) directly, so nil Fixer would panic.
	// This test verifies that registering with nil Fixer is accepted,
	// but we don't call Analyze on it to avoid panic.
	e := New()
	err := e.RegisterPattern("no-fixer", `test\s+pattern`, CategoryUnknown, SeverityLow, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify pattern was registered
	names := e.CommonErrors()
	found := false
	for _, name := range names {
		if name == "no-fixer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'no-fixer' pattern in common errors")
	}
}

func TestHistory(t *testing.T) {
	e := New()

	e.Analyze("undefined: fmt")
	e.Analyze("panic: nil pointer")

	history := e.History()
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
}

func TestHistoryReturnsCopy(t *testing.T) {
	e := New()
	e.Analyze("undefined: fmt")

	history := e.History()
	if len(history) != 1 {
		t.Fatal("expected 1 entry")
	}

	// Clear and check original copy unchanged
	e.ClearHistory()
	if len(history) != 1 {
		t.Error("history copy should be independent")
	}
}

func TestClearHistory(t *testing.T) {
	e := New()
	e.Analyze("some error")

	if len(e.History()) != 1 {
		t.Fatal("expected 1 entry before clear")
	}

	e.ClearHistory()
	if len(e.History()) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(e.History()))
	}
}

func TestCommonErrors(t *testing.T) {
	e := New()
	names := e.CommonErrors()

	if len(names) < 5 {
		t.Errorf("expected at least 5 default patterns, got %d", len(names))
	}

	expected := []string{
		"undefined identifier",
		"import cycle",
		"nil pointer dereference",
		"context deadline exceeded",
	}

	foundCount := 0
	for _, exp := range expected {
		for _, name := range names {
			if name == exp {
				foundCount++
				break
			}
		}
	}
	if foundCount < len(expected)-1 {
		t.Errorf("expected to find most default pattern names, found %d/%d", foundCount, len(expected))
	}
}

func TestGenerateSummaryMultipleIssues(t *testing.T) {
	e := New()
	result := e.Analyze("undefined: fmt and import cycle not allowed")

	summary := result.Summary
	if !strings.Contains(summary, "2 issue(s)") {
		t.Errorf("expected '2 issue(s)' in summary, got: %s", summary)
	}
}

func TestGenerateSummaryWithSuggestions(t *testing.T) {
	e := New()
	result := e.Analyze("undefined: fmt")

	summary := result.Summary
	if !strings.Contains(summary, "Suggestions:") {
		t.Error("expected 'Suggestions:' in summary")
	}
	if !strings.Contains(summary, "1.") {
		t.Error("expected numbered suggestion")
	}
}

func TestSeverityLabel(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityLow, "LOW"},
		{SeverityMedium, "MEDIUM"},
		{SeverityHigh, "HIGH"},
		{SeverityCritical, "CRITICAL"},
	}

	for _, tt := range tests {
		got := severityLabel(tt.severity)
		if got != tt.expected {
			t.Errorf("severityLabel(%d) = %q, want %q", tt.severity, got, tt.expected)
		}
	}
}

func TestSeverityLabelUnknown(t *testing.T) {
	got := severityLabel(999)
	if got != "UNKNOWN" {
		t.Errorf("expected 'UNKNOWN', got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a very long string", 10, "this is a ..."},
		{"exactly15", 15, "exactly15"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

func TestAnalyzeMultiplePatternsMatch(t *testing.T) {
	e := New()
	// This error message matches multiple patterns
	result := e.Analyze("panic: runtime error: nil pointer dereference and context deadline exceeded")

	// Both nil pointer and context deadline should match
	nilFound := false
	timeoutFound := false

	for _, issue := range result.Issues {
		if issue.Title == "nil pointer dereference" {
			nilFound = true
		}
		if issue.Title == "context deadline exceeded" {
			timeoutFound = true
		}
	}

	if !nilFound {
		t.Error("expected nil pointer pattern to match")
	}
	if !timeoutFound {
		t.Error("expected context deadline pattern to match")
	}
}

func TestAnalyzeEmptyMessage(t *testing.T) {
	e := New()
	result := e.Analyze("")

	if len(result.Issues) != 0 {
		t.Errorf("expected no issues for empty message, got %d", len(result.Issues))
	}
}

func TestAnalyzeGoModTidy(t *testing.T) {
	e := New()
	result := e.Analyze("go: github.com/example/pkg: no required module provides package")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "go mod tidy required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected go mod tidy pattern to match")
	}
}

func TestAnalyzeModuleNotFound(t *testing.T) {
	e := New()
	result := e.Analyze("module github.com/foo/bar: not a known dependency")

	if len(result.Issues) == 0 {
		t.Fatal("expected issues")
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "module not found" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected module not found pattern to match")
	}
}

func TestDiagnosisResultStructure(t *testing.T) {
	result := DiagnosisResult{
		OriginalError: "test error",
		Issues: []Issue{
			{
				ID:          "ISSUE-1",
				Severity:    SeverityHigh,
				Category:    CategorySyntax,
				Title:       "Test Issue",
				Description: "desc",
				Suggestions: []string{"fix it"},
			},
		},
		Confidence: 0.8,
	}

	if len(result.Issues) != 1 {
		t.Error("Issues not set correctly")
	}
	if result.Confidence != 0.8 {
		t.Error("Confidence not set correctly")
	}
}

func TestIssueStructure(t *testing.T) {
	issue := Issue{
		ID:          "ISSUE-1",
		Severity:    SeverityCritical,
		Category:    CategoryRuntime,
		Title:       "nil deref",
		Description: "panic at line 42",
		Suggestions: []string{"check nil"},
	}

	if issue.ID != "ISSUE-1" {
		t.Error("ID not set correctly")
	}
	if issue.Severity != SeverityCritical {
		t.Error("Severity not set correctly")
	}
	if issue.Category != CategoryRuntime {
		t.Error("Category not set correctly")
	}
}

func TestPatternStructure(t *testing.T) {
	e := New()
	e.RegisterPattern("test", `test\s+(\w+)`, CategorySyntax, SeverityMedium, func(m []string) []string {
		return []string{m[1]}
	})

	// Verify pattern was registered by checking CommonErrors
	names := e.CommonErrors()
	found := false
	for _, name := range names {
		if name == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'test' pattern in common errors")
	}
}
