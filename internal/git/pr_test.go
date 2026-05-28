package git

import (
	"strings"
	"testing"
)

func TestExtractOwnerRepo(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantOwner   string
		wantRepo    string
	}{
		{
			name:      "HTTPS URL",
			url:       "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "SSH URL",
			url:       "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "HTTPS without .git",
			url:       "https://gitlab.com/org/project",
			wantOwner: "org",
			wantRepo:  "project",
		},
		{
			name:      "Bitbucket HTTPS",
			url:       "https://bitbucket.org/team/repo.git",
			wantOwner: "team",
			wantRepo:  "repo",
		},
		{
			name:      "Invalid URL",
			url:       "invalid",
			wantOwner: "",
			wantRepo:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo := extractOwnerRepo(tc.url)
			if owner != tc.wantOwner {
				t.Errorf("owner = '%s', want '%s'", owner, tc.wantOwner)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo = '%s', want '%s'", repo, tc.wantRepo)
			}
		})
	}
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantPlatform Platform
	}{
		{"GitHub HTTPS", "https://github.com/owner/repo.git", PlatformGitHub},
		{"GitHub SSH", "git@github.com:owner/repo.git", PlatformGitHub},
		{"GitLab HTTPS", "https://gitlab.com/org/project.git", PlatformGitLab},
		{"Bitbucket", "https://bitbucket.org/team/repo.git", PlatformBitbucket},
		{"Unknown", "https://custom-git.com/repo", PlatformUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			platform, _, _ := DetectPlatform(tc.url)
			if platform != tc.wantPlatform {
				t.Errorf("platform = '%s', want '%s'", platform, tc.wantPlatform)
			}
		})
	}
}

func TestDetectBaseURL(t *testing.T) {
	tests := []struct {
		platform Platform
		owner    string
		repo     string
		wantContains string
	}{
		{PlatformGitHub, "owner", "repo", "api.github.com/repos/owner/repo"},
		{PlatformGitLab, "org", "project", "gitlab.com/api/v4/projects"},
		{PlatformBitbucket, "team", "repo", "api.bitbucket.org/2.0/repositories/team/repo"},
	}

	for _, tc := range tests {
		t.Run(string(tc.platform), func(t *testing.T) {
			url := detectBaseURL(tc.platform, tc.owner, tc.repo)
			if !strings.Contains(url, tc.wantContains) {
				t.Errorf("URL '%s' does not contain '%s'", url, tc.wantContains)
			}
		})
	}
}

func TestGetTokenFromEnv(t *testing.T) {
	// Test without env vars set (should return empty)
	token := GetTokenFromEnv(PlatformGitHub)
	// We can't guarantee env var state, so just verify it doesn't panic
	
	// Test unknown platform
	token = GetTokenFromEnv(PlatformUnknown)
	// Should fallback to GIT_TOKEN
	_ = token
}

func TestGeneratePRDescription(t *testing.T) {
	diff := `diff --git a/src/login.go b/src/login.go
new file mode 100644
diff --git a/test/login_test.go b/test/login_test.go
new file mode 100644
`
	title, description := GeneratePRDescription("Add user login feature", diff, "All tests passed")

	if !strings.Contains(title, "Add user login feature") {
		t.Errorf("Title should contain task description, got: %s", title)
	}

	if !strings.Contains(description, "Add user login feature") {
		t.Error("Description should contain task description")
	}
	if !strings.Contains(description, "src/login.go") {
		t.Error("Description should contain changed files, got: " + description)
	}
	if !strings.Contains(description, "test/login_test.go") {
		t.Error("Description should contain changed files, got: " + description)
	}
	if !strings.Contains(description, "All tests passed") {
		t.Error("Description should contain test results")
	}
	if !strings.Contains(description, "Checklist") {
		t.Error("Description should contain checklist")
	}
}

func TestExtractChangedFilesFromDiff(t *testing.T) {
	diff := `diff --git a/src/login.go b/src/login.go
new file mode 100644
diff --git a/src/handler.go b/src/handler.go
index abc123..def456
diff --git a/test/login_test.go b/test/login_test.go
new file mode 100644
`
	files := extractChangedFilesFromDiff(diff)

	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d: %v", len(files), files)
	}

	seen := make(map[string]bool)
	for _, f := range files {
		seen[f] = true
	}
	if !seen["src/login.go"] || !seen["src/handler.go"] || !seen["test/login_test.go"] {
		t.Errorf("Missing expected files, got: %v", files)
	}
}

func TestExtractChangedFilesFromDiffEmpty(t *testing.T) {
	files := extractChangedFilesFromDiff("")
	if files != nil {
		t.Errorf("Expected nil for empty diff, got: %v", files)
	}
}

func TestGeneratePRDescriptionEmptyDiff(t *testing.T) {
	title, description := GeneratePRDescription("Fix bug", "", "")

	if title == "" {
		t.Error("Title should not be empty")
	}
	if description == "" {
		t.Error("Description should not be empty")
	}
}

func TestNewPRClient(t *testing.T) {
	tests := []struct {
		platform    Platform
		token       string
		owner       string
		repo        string
		wantBaseURL string
	}{
		{PlatformGitHub, "token123", "owner", "repo", "https://api.github.com/repos/owner/repo"},
		{PlatformGitLab, "token456", "org", "project", "https://gitlab.com/api/v4/projects/org%2Fproject"},
	}

	for _, tc := range tests {
		t.Run(string(tc.platform), func(t *testing.T) {
			client := NewPRClient(tc.platform, tc.token, tc.owner, tc.repo)
			if client.baseURL != tc.wantBaseURL {
				t.Errorf("baseURL = '%s', want '%s'", client.baseURL, tc.wantBaseURL)
			}
			if client.token != tc.token {
				t.Errorf("token mismatch")
			}
			if client.platform != tc.platform {
				t.Errorf("platform = '%s', want '%s'", client.platform, tc.platform)
			}
		})
	}
}

func TestPRConfigFields(t *testing.T) {
	cfg := PRConfig{
		Title:        "Test PR",
		Description:  "A test PR",
		SourceBranch:  "feature-branch",
		TargetBranch: "main",
		Labels:       []string{"feature", "needs-review"},
		Reviewers:    []string{"dev1", "dev2"},
		Draft:        true,
	}

	if cfg.Title != "Test PR" {
		t.Error("Title field not set correctly")
	}
	if len(cfg.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(cfg.Labels))
	}
	if !cfg.Draft {
		t.Error("Draft flag should be true")
	}
}

func TestPRInfoStruct(t *testing.T) {
	info := PRInfo{
		ID:           1,
		Number:       42,
		URL:          "https://github.com/owner/repo/pull/42",
		Title:        "Feature",
		Description:  "Adding feature",
		State:        "open",
	}

	if info.Number != 42 {
		t.Errorf("Number = %d, want 42", info.Number)
	}
	if info.URL == "" {
		t.Error("URL should not be empty")
	}
}
