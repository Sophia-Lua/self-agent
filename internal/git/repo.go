package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repo represents a git repository.
type Repo struct {
	dir string
}

// New opens or initializes a git repository.
func New(dir string) (*Repo, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	r := &Repo{dir: absDir}

	// Check if already a git repo
	if !r.isRepo() {
		if err := r.init(); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Status returns the current git status.
func (r *Repo) Status() (*Status, error) {
	// Get branch name
	branch, err := r.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}

	// Get status summary
	output, err := r.run("status", "--porcelain")
	if err != nil {
		return nil, err
	}

	status := &Status{
		Branch: strings.TrimSpace(branch),
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		entry := StatusEntry{
			XY:     line[:2],
			Path:   strings.TrimSpace(line[3:]),
			Status: line[:2],
		}
		status.Entries = append(status.Entries, entry)

		switch entry.XY {
		case "??":
			status.Untracked++
		case "A ", "AM":
			status.Added++
		case "M ", "MM", " M":
			status.Modified++
		case "D ", "DM", " D":
			status.Deleted++
		case "R ", "RM":
			status.Renamed++
		}
	}

	// Get last commit info
	logOutput, err := r.run("log", "-1", "--format=%H|%s|%an|%ai")
	if err == nil {
		parts := strings.SplitN(strings.TrimSpace(logOutput), "|", 4)
		if len(parts) == 4 {
			status.LastCommit = &CommitInfo{
				Hash:    parts[0],
				Subject: parts[1],
				Author:  parts[2],
				Date:    parts[3],
			}
		}
	}

	return status, nil
}

// Add stages files.
func (r *Repo) Add(paths ...string) error {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	_, err := r.run(append([]string{"add"}, paths...)...)
	return err
}

// Commit creates a commit with the given message.
func (r *Repo) Commit(message string) error {
	_, err := r.run("commit", "-m", message)
	return err
}

// CommitWithAuthor creates a commit with custom author info.
func (r *Repo) CommitWithAuthor(message, name, email string) error {
	_, err := r.run("-c", fmt.Sprintf("user.name=%s", name),
		"-c", fmt.Sprintf("user.email=%s", email),
		"commit", "-m", message)
	return err
}

// Push pushes the current branch to remote.
func (r *Repo) Push(remote, branch string, flags ...string) error {
	args := []string{"push"}
	if len(flags) > 0 {
		args = append(args, flags...)
	}
	args = append(args, remote, branch)
	_, err := r.run(args...)
	return err
}

// Pull pulls changes from remote.
func (r *Repo) Pull(remote, branch string) error {
	_, err := r.run("pull", remote, branch)
	return err
}

// Fetch fetches from remote.
func (r *Repo) Fetch(remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := r.run("fetch", remote)
	return err
}

// Checkout switches to or creates a branch.
func (r *Repo) Checkout(branch string, create bool) error {
	if create {
		_, err := r.run("checkout", "-b", branch)
		return err
	}
	_, err := r.run("checkout", branch)
	return err
}

// Branch returns list of branches.
func (r *Repo) Branch(remote bool) ([]string, error) {
	args := []string{"branch"}
	if remote {
		args = append(args, "-r")
	}
	output, err := r.run(args...)
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// Merge merges a branch into the current branch.
func (r *Repo) Merge(branch string, noFF bool) error {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	_, err := r.run(args...)
	return err
}

// Rebase rebases the current branch onto another.
func (r *Repo) Rebase(onto string) error {
	_, err := r.run("rebase", onto)
	return err
}

// Stash saves uncommitted changes.
func (r *Repo) Stash(message string) error {
	if message == "" {
		_, err := r.run("stash")
		return err
	}
	_, err := r.run("stash", "push", "-m", message)
	return err
}

// StashPop restores stashed changes.
func (r *Repo) StashPop() error {
	_, err := r.run("stash", "pop")
	return err
}

// Diff returns the diff of unstaged changes.
func (r *Repo) Diff(paths ...string) (string, error) {
	args := []string{"diff"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return r.run(args...)
}

// DiffStaged returns the diff of staged changes.
func (r *Repo) DiffStaged() (string, error) {
	return r.run("diff", "--cached")
}

// Log returns recent commits.
func (r *Repo) Log(limit int) ([]CommitInfo, error) {
	if limit <= 0 {
		limit = 10
	}
	output, err := r.run("log",
		fmt.Sprintf("-%d", limit),
		"--format=%H|%s|%an|%ai",
	)
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			commits = append(commits, CommitInfo{
				Hash:    parts[0],
				Subject: parts[1],
				Author:  parts[2],
				Date:    parts[3],
			})
		}
	}
	return commits, nil
}

// Tag creates a tag.
func (r *Repo) Tag(name, message string) error {
	if message != "" {
		_, err := r.run("tag", "-a", name, "-m", message)
		return err
	}
	_, err := r.run("tag", name)
	return err
}

// Tags returns all tags.
func (r *Repo) Tags() ([]string, error) {
	output, err := r.run("tag", "--sort=-creatordate")
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tags = append(tags, line)
		}
	}
	return tags, nil
}

// Remote returns list of remotes.
func (r *Repo) Remote() ([]RemoteInfo, error) {
	output, err := r.run("remote", "-v")
	if err != nil {
		return nil, err
	}

	remotes := make(map[string]*RemoteInfo)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			name := parts[0]
			url := parts[1]
			direction := parts[2]
			if _, exists := remotes[name]; !exists {
				remotes[name] = &RemoteInfo{Name: name}
			}
			if direction == "(fetch)" {
				remotes[name].FetchURL = url
			} else if direction == "(push)" {
				remotes[name].PushURL = url
			}
		}
	}

	var result []RemoteInfo
	for _, r := range remotes {
		result = append(result, *r)
	}
	return result, nil
}

// AddRemote adds a remote.
func (r *Repo) AddRemote(name, url string) error {
	_, err := r.run("remote", "add", name, url)
	return err
}

// RemoveRemote removes a remote.
func (r *Repo) RemoveRemote(name string) error {
	_, err := r.run("remote", "remove", name)
	return err
}

// Reset resets the working tree.
func (r *Repo) Reset(target string, hard bool) error {
	args := []string{"reset"}
	if hard {
		args = append(args, "--hard")
	}
	args = append(args, target)
	_, err := r.run(args...)
	return err
}

// Clean removes untracked files.
func (r *Repo) Clean(dryRun bool) error {
	args := []string{"clean", "-fd"}
	if dryRun {
		args = append(args, "-n")
	}
	_, err := r.run(args...)
	return err
}

// IsClean checks if there are no uncommitted changes.
func (r *Repo) IsClean() (bool, error) {
	output, err := r.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "", nil
}

// CurrentBranch returns the current branch name.
func (r *Repo) CurrentBranch() (string, error) {
	output, err := r.run("rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(output), err
}

// Root returns the repository root directory.
func (r *Repo) Root() string {
	return r.dir
}

// ChangedFiles returns files that have been modified.
func (r *Repo) ChangedFiles() ([]string, error) {
	output, err := r.run("diff", "--name-only")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// UntrackedFiles returns list of untracked files.
func (r *Repo) UntrackedFiles() ([]string, error) {
	output, err := r.run("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// ShortHash returns the short commit hash for HEAD.
func (r *Repo) ShortHash() (string, error) {
	output, err := r.run("rev-parse", "--short", "HEAD")
	return strings.TrimSpace(output), err
}

// Config gets a git config value.
func (r *Repo) Config(key string) (string, error) {
	return r.run("config", key)
}

// SetConfig sets a git config value.
func (r *Repo) SetConfig(key, value string) error {
	_, err := r.run("config", key, value)
	return err
}

func (r *Repo) init() error {
	_, err := r.run("init")
	return err
}

func (r *Repo) isRepo() bool {
	_, err := r.run("rev-parse", "--git-dir")
	return err == nil
}

func (r *Repo) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output), nil
}

// Status represents the current state of the repository.
type Status struct {
	Branch     string
	Entries    []StatusEntry
	Added      int
	Modified   int
	Deleted    int
	Renamed    int
	Untracked  int
	LastCommit *CommitInfo
}

// IsEmpty checks if there are no changes.
func (s *Status) IsEmpty() bool {
	return s.Added == 0 && s.Modified == 0 && s.Deleted == 0 && s.Renamed == 0 && s.Untracked == 0
}

// StatusEntry represents a single file in git status.
type StatusEntry struct {
	XY     string
	Path   string
	Status string
}

// CommitInfo holds commit metadata.
type CommitInfo struct {
	Hash    string
	Subject string
	Author  string
	Date    string
}

// RemoteInfo holds remote metadata.
type RemoteInfo struct {
	Name    string
	FetchURL string
	PushURL string
}

// GenerateBranchName creates a date-based branch name.
func GenerateBranchName(prefix, suffix string) string {
	date := time.Now().Format("060102")
	if suffix == "" {
		suffix = "update"
	}
	return fmt.Sprintf("%s-%s-%s", date, prefix, suffix)
}
