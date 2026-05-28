package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Platform identifies the Git hosting platform.
type Platform string

const (
	PlatformGitHub    Platform = "github"
	PlatformGitLab    Platform = "gitlab"
	PlatformBitbucket Platform = "bitbucket"
	PlatformUnknown   Platform = "unknown"
)

// PRInfo holds Pull Request details.
type PRInfo struct {
	ID          int       `json:"id"`
	Number      int       `json:"number"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	SourceBranch string  `json:"source_branch"`
	TargetBranch string  `json:"target_branch"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	Labels      []string  `json:"labels,omitempty"`
	Assignees   []string  `json:"assignees,omitempty"`
	Reviewers   []string  `json:"reviewers,omitempty"`
}

// PRConfig holds PR creation configuration.
type PRConfig struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	SourceBranch string  `json:"source_branch"`
	TargetBranch string  `json:"target_branch"`
	Labels      []string `json:"labels,omitempty"`
	Reviewers   []string `json:"reviewers,omitempty"`
	Draft       bool     `json:"draft,omitempty"`
}

// PRClient provides platform-agnostic PR/MR creation.
type PRClient struct {
	platform  Platform
	token     string
	owner     string
	repo      string
	baseURL   string
	httpClient *http.Client
}

// NewPRClient creates a PR client.
func NewPRClient(platform Platform, token, owner, repo string) *PRClient {
	baseURL := detectBaseURL(platform, owner, repo)
	return &PRClient{
		platform:   platform,
		token:      token,
		owner:      owner,
		repo:       repo,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreatePR creates a new Pull Request.
func (c *PRClient) CreatePR(ctx context.Context, cfg PRConfig) (*PRInfo, error) {
	switch c.platform {
	case PlatformGitHub:
		return c.createGitHubPR(ctx, cfg)
	case PlatformGitLab:
		return c.createGitLabMR(ctx, cfg)
	case PlatformBitbucket:
		return c.createBitbucketPR(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", c.platform)
	}
}

// ListPRs lists existing pull requests.
func (c *PRClient) ListPRs(ctx context.Context, state string) ([]PRInfo, error) {
	switch c.platform {
	case PlatformGitHub:
		return c.listGitHubPRs(ctx, state)
	case PlatformGitLab:
		return c.listGitLabMRs(ctx, state)
	case PlatformBitbucket:
		return c.listBitbucketPRs(ctx, state)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", c.platform)
	}
}

// GetPR retrieves a specific pull request.
func (c *PRClient) GetPR(ctx context.Context, number int) (*PRInfo, error) {
	switch c.platform {
	case PlatformGitHub:
		return c.getGitHubPR(ctx, number)
	case PlatformGitLab:
		return c.getGitLabMR(ctx, number)
	case PlatformBitbucket:
		return c.getBitbucketPR(ctx, number)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", c.platform)
	}
}

// DetectPlatform auto-detects the Git platform from remote URL.
func DetectPlatform(remoteURL string) (Platform, string, string) {
	// Normalize URL
	url := strings.ToLower(remoteURL)

	if strings.Contains(url, "github.com") {
		owner, repo := extractOwnerRepo(url)
		return PlatformGitHub, owner, repo
	}
	if strings.Contains(url, "gitlab.com") {
		owner, repo := extractOwnerRepo(url)
		return PlatformGitLab, owner, repo
	}
	if strings.Contains(url, "bitbucket.org") {
		owner, repo := extractOwnerRepo(url)
		return PlatformBitbucket, owner, repo
	}

	return PlatformUnknown, "", ""
}

func extractOwnerRepo(url string) (owner, repo string) {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Remove protocol
	if strings.Contains(url, "://") {
		parts := strings.SplitN(url, "://", 2)
		url = parts[1]
	}

	// Remove user@ for SSH URLs
	if strings.Contains(url, "@") {
		parts := strings.SplitN(url, "@", 2)
		url = parts[1]
	}

	// Handle SSH URL format: host:owner/repo (colon instead of /)
	// E.g., "github.com:owner/repo" -> "github.com/owner/repo"
	colonIdx := strings.Index(url, ":")
	slashIdx := strings.Index(url, "/")
	if colonIdx > 0 && (slashIdx == -1 || colonIdx < slashIdx) {
		url = url[:colonIdx] + "/" + url[colonIdx+1:]
	}

	// Split by /
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}

	return "", ""
}

func detectBaseURL(platform Platform, owner, repo string) string {
	switch platform {
	case PlatformGitHub:
		return fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	case PlatformGitLab:
		// GitLab uses project ID or URL-encoded path
		path := owner + "%2F" + repo
		return fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", path)
	case PlatformBitbucket:
		return fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s", owner, repo)
	default:
		return ""
	}
}

// GitHub PR methods

func (c *PRClient) createGitHubPR(ctx context.Context, cfg PRConfig) (*PRInfo, error) {
	body := map[string]interface{}{
		"title": cfg.Title,
		"body":  cfg.Description,
		"head":  cfg.SourceBranch,
		"base":  cfg.TargetBranch,
		"draft": cfg.Draft,
	}

	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/pulls", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(respBody))
	}

	var ghPR struct {
		ID     int    `json:"id"`
		Number int    `json:"number"`
		URL    string `json:"html_url"`
		Title  string `json:"title"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(respBody, &ghPR); err != nil {
		return nil, err
	}

	pr := &PRInfo{
		ID:           ghPR.ID,
		Number:       ghPR.Number,
		URL:          ghPR.URL,
		Title:        ghPR.Title,
		Description:  cfg.Description,
		SourceBranch:  cfg.SourceBranch,
		TargetBranch:  cfg.TargetBranch,
		State:        ghPR.State,
		Labels:       cfg.Labels,
		Reviewers:    cfg.Reviewers,
		CreatedAt:    time.Now(),
	}

	// Add labels if specified
	if len(cfg.Labels) > 0 {
		c.addGitHubLabels(ctx, ghPR.Number, cfg.Labels)
	}

	// Add reviewers if specified
	if len(cfg.Reviewers) > 0 {
		c.addGitHubReviewers(ctx, ghPR.Number, cfg.Reviewers)
	}

	return pr, nil
}

func (c *PRClient) listGitHubPRs(ctx context.Context, state string) ([]PRInfo, error) {
	url := c.baseURL + "/pulls"
	if state != "" {
		url += "?state=" + state
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var prs []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"html_url"`
		Head   struct {Ref string} `json:"head"`
		Base   struct {Ref string} `json:"base"`
		State  string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}

	result := make([]PRInfo, 0, len(prs))
	for _, p := range prs {
		result = append(result, PRInfo{
			Number:       p.Number,
			Title:        p.Title,
			URL:          p.URL,
			SourceBranch:  p.Head.Ref,
			TargetBranch:  p.Base.Ref,
			State:        p.State,
		})
	}

	return result, nil
}

func (c *PRClient) getGitHubPR(ctx context.Context, number int) (*PRInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/pulls/%d", c.baseURL, number), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API error %d", resp.StatusCode)
	}

	var pr PRInfo
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}

	return &pr, nil
}

func (c *PRClient) addGitHubLabels(ctx context.Context, prNumber int, labels []string) error {
	body := map[string]interface{}{"labels": labels}
	payload, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/issues/%d/labels", c.baseURL, prNumber),
		bytes.NewReader(payload))
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add labels: %s", string(body))
	}

	return nil
}

func (c *PRClient) addGitHubReviewers(ctx context.Context, prNumber int, reviewers []string) error {
	body := map[string]interface{}{"reviewers": reviewers}
	payload, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/pulls/%d/requested_reviewers", c.baseURL, prNumber),
		bytes.NewReader(payload))
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add reviewers: %s", string(body))
	}

	return nil
}

// GitLab MR methods

func (c *PRClient) createGitLabMR(ctx context.Context, cfg PRConfig) (*PRInfo, error) {
	params := map[string]interface{}{
		"title": cfg.Title,
		"description": cfg.Description,
		"source_branch": cfg.SourceBranch,
		"target_branch": cfg.TargetBranch,
	}

	if cfg.Draft {
		params["draft"] = true
	}
	if len(cfg.Reviewers) > 0 {
		params["reviewer_ids"] = cfg.Reviewers
	}

	payload, _ := json.Marshal(params)

	url := c.baseURL + "/merge_requests"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(respBody))
	}

	var glMR struct {
		IID        int    `json:"iid"`
		ID         int    `json:"id"`
		Title      string `json:"title"`
		URL        string `json:"web_url"`
		State      string `json:"state"`
	}
	if err := json.Unmarshal(respBody, &glMR); err != nil {
		return nil, err
	}

	return &PRInfo{
		ID:           glMR.ID,
		Number:       glMR.IID,
		URL:          glMR.URL,
		Title:        glMR.Title,
		Description:  cfg.Description,
		SourceBranch:  cfg.SourceBranch,
		TargetBranch:  cfg.TargetBranch,
		State:        glMR.State,
		CreatedAt:    time.Now(),
	}, nil
}

func (c *PRClient) listGitLabMRs(ctx context.Context, state string) ([]PRInfo, error) {
	url := c.baseURL + "/merge_requests"
	if state != "" {
		url += "?state=" + state
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var mrs []struct {
		IID         int    `json:"iid"`
		ID          int    `json:"id"`
		Title       string `json:"title"`
		URL         string `json:"web_url"`
		State       string `json:"state"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return nil, err
	}

	result := make([]PRInfo, 0, len(mrs))
	for _, m := range mrs {
		result = append(result, PRInfo{
			ID:           m.ID,
			Number:       m.IID,
			URL:          m.URL,
			Title:        m.Title,
			SourceBranch:  m.SourceBranch,
			TargetBranch:  m.TargetBranch,
			State:        m.State,
		})
	}

	return result, nil
}

func (c *PRClient) getGitLabMR(ctx context.Context, iid int) (*PRInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/merge_requests/%d", c.baseURL, iid), nil)
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API error %d", resp.StatusCode)
	}

	var mr PRInfo
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, err
	}

	return &mr, nil
}

// Bitbucket PR methods

func (c *PRClient) createBitbucketPR(ctx context.Context, cfg PRConfig) (*PRInfo, error) {
	body := map[string]interface{}{
		"title":       cfg.Title,
		"description": cfg.Description,
		"source": map[string]interface{}{
			"branch": map[string]string{"name": cfg.SourceBranch},
		},
		"destination": map[string]interface{}{
			"branch": map[string]string{"name": cfg.TargetBranch},
		},
	}

	if cfg.Draft {
		body["draft"] = true
	}

	payload, _ := json.Marshal(body)

	baseURL := detectBaseURLFromPlatform(c.platform, c.owner, c.repo)
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/pullrequests", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Bitbucket API error %d: %s", resp.StatusCode, string(respBody))
	}

	var bbPR struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		URL      string `json:"links,omitempty"`
		State    string `json:"state"`
		Source   struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"source"`
		Destination struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"destination"`
	}
	if err := json.Unmarshal(respBody, &bbPR); err != nil {
		return nil, err
	}

	pr := &PRInfo{
		ID:           bbPR.ID,
		Number:       bbPR.ID,
		URL:          bbPR.URL,
		Title:        bbPR.Title,
		Description:  cfg.Description,
		SourceBranch:  bbPR.Source.Branch.Name,
		TargetBranch:  bbPR.Destination.Branch.Name,
		State:        bbPR.State,
		CreatedAt:    time.Now(),
	}

	return pr, nil
}

func (c *PRClient) listBitbucketPRs(ctx context.Context, state string) ([]PRInfo, error) {
	baseURL := detectBaseURLFromPlatform(c.platform, c.owner, c.repo)
	url := baseURL + "/pullrequests"
	if state != "" {
		url += "?state=" + state
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Bitbucket API error %d: %s", resp.StatusCode, string(respBody))
	}

	var response struct {
		Values []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
			State string `json:"state"`
			Links struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
			Source struct {
				Branch struct{ Name string } `json:"branch"`
			} `json:"source"`
			Destination struct {
				Branch struct{ Name string } `json:"branch"`
			} `json:"destination"`
		} `json:"values"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	result := make([]PRInfo, 0, len(response.Values))
	for _, pr := range response.Values {
		result = append(result, PRInfo{
			ID:           pr.ID,
			Number:       pr.ID,
			URL:          pr.Links.HTML.Href,
			Title:        pr.Title,
			State:        pr.State,
			SourceBranch:  pr.Source.Branch.Name,
			TargetBranch:  pr.Destination.Branch.Name,
		})
	}

	return result, nil
}

func (c *PRClient) getBitbucketPR(ctx context.Context, number int) (*PRInfo, error) {
	baseURL := detectBaseURLFromPlatform(c.platform, c.owner, c.repo)
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/pullrequests/%d", baseURL, number), nil)
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Bitbucket API error %d: %s", resp.StatusCode, string(respBody))
	}

	var bbPR struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		State string `json:"state"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
		Source struct {
			Branch struct{ Name string } `json:"branch"`
		} `json:"source"`
		Destination struct {
			Branch struct{ Name string } `json:"branch"`
		} `json:"destination"`
	}
	if err := json.Unmarshal(respBody, &bbPR); err != nil {
		return nil, err
	}

	return &PRInfo{
		ID:           bbPR.ID,
		Number:       bbPR.ID,
		URL:          bbPR.Links.HTML.Href,
		Title:        bbPR.Title,
		State:        bbPR.State,
		SourceBranch:  bbPR.Source.Branch.Name,
		TargetBranch:  bbPR.Destination.Branch.Name,
	}, nil
}

// detectBaseURLFromPlatform builds the API base URL for a given platform.
func detectBaseURLFromPlatform(platform Platform, owner, repo string) string { switch platform {
	case PlatformGitHub:
		return fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	case PlatformGitLab:
		path := owner + "%2F" + repo
		return fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", path)
	case PlatformBitbucket:
		return fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s", owner, repo)
	default:
		return ""
	}
}

// GeneratePRDescription creates a PR title and description from changes.
func GeneratePRDescription(taskDesc string, diff string, testResult string) (title, description string) {
	title = fmt.Sprintf("feat: %s", taskDesc)

	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("# %s\n\n", taskDesc))
	desc.WriteString("## Changes\n\n")

	// Parse diff to identify changed files
	changedFiles := extractChangedFilesFromDiff(diff)
	if len(changedFiles) > 0 {
		for _, f := range changedFiles {
			desc.WriteString(fmt.Sprintf("- Modified: `%s`\n", f))
		}
	} else {
		desc.WriteString("- Automated changes by AutoDev pipeline\n")
	}

	if testResult != "" {
		desc.WriteString("\n## Test Results\n\n")
		desc.WriteString(fmt.Sprintf("```\n%s\n```\n", testResult))
	}

	desc.WriteString("\n## Checklist\n\n")
	desc.WriteString("- [ ] Code follows project style guidelines\n")
	desc.WriteString("- [ ] Self-review completed\n")
	desc.WriteString("- [ ] Tests added/updated\n")
	desc.WriteString("- [ ] Documentation updated\n")

	return title, desc.String()
}

// extractChangedFilesFromDiff parses a unified diff to extract changed file paths.
func extractChangedFilesFromDiff(diff string) []string {
	if diff == "" {
		return nil
	}

	var files []string
	seen := make(map[string]bool)

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			// diff --git a/path/to/file b/path/to/file
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				filePath := strings.TrimPrefix(parts[3], "b/")
				if !seen[filePath] {
					seen[filePath] = true
					files = append(files, filePath)
				}
			}
		} else if strings.HasPrefix(line, "--- a/") {
			// Fallback for patches without diff --git
			filePath := strings.TrimPrefix(line, "--- a/")
			// Skip /dev/null (new files)
			if !seen[filePath] && filePath != "/dev/null" {
				seen[filePath] = true
				files = append(files, filePath)
			}
		} else if strings.HasPrefix(line, "+++ b/") {
			filePath := strings.TrimPrefix(line, "+++ b/")
			if !seen[filePath] && filePath != "/dev/null" {
				seen[filePath] = true
				files = append(files, filePath)
			}
		}
	}

	return files
}

// GetTokenFromEnv retrieves the platform token from environment.
func GetTokenFromEnv(platform Platform) string {
	switch platform {
	case PlatformGitHub:
		return os.Getenv("GITHUB_TOKEN")
	case PlatformGitLab:
		return os.Getenv("GITLAB_TOKEN")
	case PlatformBitbucket:
		return os.Getenv("BITBUCKET_TOKEN")
	default:
		return os.Getenv("GIT_TOKEN")
	}
}
