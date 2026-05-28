package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Lang represents a detected programming language.
type Lang string

const (
	LangGo     Lang = "go"
	LangJS     Lang = "javascript"
	LangTS     Lang = "typescript"
	LangPython Lang = "python"
	LangRust   Lang = "rust"
	LangJava   Lang = "java"
	LangRuby   Lang = "ruby"
	LangPHP    Lang = "php"
	LangUnknown Lang = "unknown"
)

// Framework identifies a detected framework.
type Framework string

const (
	FrameworkNone     Framework = "none"
	FrameworkNextJS   Framework = "next.js"
	FrameworkVite     Framework = "vite"
	FrameworkReact    Framework = "react"
	FrameworkVue      Framework = "vue"
	FrameworkAngular  Framework = "angular"
	FrameworkDjango   Framework = "django"
	FrameworkFlask    Framework = "flask"
	FrameworkFastAPI  Framework = "fastapi"
	FrameworkGin      Framework = "gin"
	FrameworkEcho     Framework = "echo"
	FrameworkRails    Framework = "rails"
	FrameworkSpring   Framework = "spring"
	FrameworkLaravel  Framework = "laravel"
	FrameworkUnknown  Framework = "unknown"
)

// ProjectType classifies the kind of project.
type ProjectType string

const (
	TypeWebFrontend  ProjectType = "web-frontend"
	TypeWebBackend   ProjectType = "web-backend"
	TypeFullStack    ProjectType = "fullstack"
	TypeCLI          ProjectType = "cli"
	TypeLibrary      ProjectType = "library"
	TypeMonorepo     ProjectType = "monorepo"
	TypeUnknown      ProjectType = "unknown"
)

// Profile holds comprehensive project analysis.
type Profile struct {
	Name         string      `json:"name"`
	RootPath     string      `json:"root_path"`
	Language     Lang        `json:"language"`
	Framework    Framework   `json:"framework"`
	Type         ProjectType `json:"type"`
	BuildCommand string      `json:"build_command"`
	TestCommand  string      `json:"test_command"`
	DevCommand   string      `json:"dev_command"`
	MainFile     string      `json:"main_file"`
	Dependencies []string    `json:"dependencies"`
	HasTests     bool        `json:"has_tests"`
	HasCI        bool        `json:"has_ci"`
	HasDocker    bool        `json:"has_docker"`
	HasLint      bool        `json:"has_lint"`
	HasFormat    bool        `json:"has_format"`
	Dirs         DirSummary  `json:"dirs"`
}

// DirSummary holds key directories found in the project.
type DirSummary struct {
	Src     []string `json:"src"`
	Tests   []string `json:"tests"`
	Config  []string `json:"config"`
	Docs    []string `json:"docs"`
	Assets  []string `json:"assets"`
}

// Analyzer scans a project directory and produces a Profile.
type Analyzer struct {
	root string
}

// New creates a project analyzer starting from the given root.
func New(root string) *Analyzer {
	return &Analyzer{root: root}
}

// Analyze scans the project and returns a comprehensive profile.
func (a *Analyzer) Analyze() (*Profile, error) {
	profile := &Profile{
		RootPath:  a.root,
		Language:  LangUnknown,
		Framework: FrameworkUnknown,
		Type:      TypeUnknown,
		Name:      filepath.Base(a.root),
	}

	// Detect language
	profile.Language = a.detectLanguage()

	// Detect framework
	profile.Framework = a.detectFramework(profile.Language)

	// Detect project type
	profile.Type = a.detectProjectType(profile)

	// Extract commands
	profile.BuildCommand = a.findBuildCommand(profile.Language, profile.Framework)
	profile.TestCommand = a.findTestCommand(profile.Language)
	profile.DevCommand = a.findDevCommand(profile.Language, profile.Framework)
	profile.MainFile = a.findMainFile(profile.Language)

	// Extract dependencies
	profile.Dependencies = a.findDependencies(profile.Language)

	// Check for tests
	profile.HasTests = a.hasTests(profile.Language)

	// Check for CI/CD
	profile.HasCI = a.hasCI()

	// Check for Docker
	profile.HasDocker = a.hasDocker()

	// Check for linting
	profile.HasLint = a.hasLint(profile.Language)

	// Check for formatting
	profile.HasFormat = a.hasFormat(profile.Language)

	// Scan directories
	profile.Dirs = a.scanDirectories()

	return profile, nil
}

// Summary returns a human-readable summary.
func (p *Profile) Summary() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Project: %s\n", p.Name))
	b.WriteString(fmt.Sprintf("Language: %s\n", p.Language))
	b.WriteString(fmt.Sprintf("Framework: %s\n", p.Framework))
	b.WriteString(fmt.Sprintf("Type: %s\n", p.Type))
	if p.BuildCommand != "" {
		b.WriteString(fmt.Sprintf("Build: %s\n", p.BuildCommand))
	}
	if p.TestCommand != "" {
		b.WriteString(fmt.Sprintf("Test: %s\n", p.TestCommand))
	}
	if p.DevCommand != "" {
		b.WriteString(fmt.Sprintf("Dev: %s\n", p.DevCommand))
	}
	b.WriteString(fmt.Sprintf("Tests: %v\n", p.HasTests))
	b.WriteString(fmt.Sprintf("CI/CD: %v\n", p.HasCI))
	b.WriteString(fmt.Sprintf("Docker: %v\n", p.HasDocker))
	return b.String()
}

// JSON returns the profile as JSON.
func (p *Profile) JSON() (string, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *Analyzer) detectLanguage() Lang {
	indicators := map[Lang][]string{
		LangGo:     {"go.mod", "main.go", "*.go"},
		LangJS:     {"package.json", "*.js"},
		LangTS:     {"tsconfig.json", "*.ts"},
		LangPython: {"requirements.txt", "setup.py", "pyproject.toml", "*.py"},
		LangRust:   {"Cargo.toml", "Cargo.lock", "*.rs"},
		LangJava:   {"pom.xml", "build.gradle", "*.java"},
		LangRuby:   {"Gemfile", "*.rb", "*.gemspec"},
		LangPHP:    {"composer.json", "*.php"},
	}

	// Check for unique indicators first
	for lang, files := range indicators {
		for _, f := range files {
			if strings.HasPrefix(f, "*") {
				continue // Skip glob patterns for now
			}
			if a.exists(f) {
				return lang
			}
		}
	}

	// Check for TypeScript before JavaScript (TypeScript is a superset)
	if a.exists("tsconfig.json") {
		return LangTS
	}
	if a.exists("package.json") {
		return LangJS
	}

	// Check Go
	if a.exists("go.mod") {
		return LangGo
	}

	// Check Python
	if a.exists("requirements.txt") || a.exists("setup.py") || a.exists("pyproject.toml") {
		return LangPython
	}

	// Check Rust
	if a.exists("Cargo.toml") {
		return LangRust
	}

	// Check Java
	if a.exists("pom.xml") || a.exists("build.gradle") {
		return LangJava
	}

	// Check Ruby
	if a.exists("Gemfile") {
		return LangRuby
	}

	// Check PHP
	if a.exists("composer.json") {
		return LangPHP
	}

	return LangUnknown
}

func (a *Analyzer) detectFramework(lang Lang) Framework {
	if lang == LangJS || lang == LangTS {
		return a.detectJSFramework()
	}
	if lang == LangPython {
		return a.detectPythonFramework()
	}
	if lang == LangGo {
		return a.detectGoFramework()
	}
	return FrameworkUnknown
}

func (a *Analyzer) detectJSFramework() Framework {
	// Read package.json to check dependencies
	content, err := os.ReadFile(filepath.Join(a.root, "package.json"))
	if err != nil {
		return FrameworkNone
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return FrameworkNone
	}

	deps := a.extractDeps(pkg)

	if deps["next"] {
		return FrameworkNextJS
	}
	if deps["@sveltejs/kit"] {
		return FrameworkNone // SvelteKit not in list
	}
	if deps["react"] || deps["react-dom"] {
		if deps["vite"] {
			return FrameworkVite
		}
		return FrameworkReact
	}
	if deps["vue"] {
		return FrameworkVue
	}
	if deps["@angular/core"] {
		return FrameworkAngular
	}
	if deps["vite"] {
		return FrameworkVite
	}

	return FrameworkNone
}

func (a *Analyzer) detectPythonFramework() Framework {
	// Check common framework indicators
	if a.exists("manage.py") {
		return FrameworkDjango
	}
	if a.exists("app.py") || a.hasFileContent("from flask", ".py") || a.hasFileContent("import flask", ".py") {
		return FrameworkFlask
	}
	if a.hasFileContent("from fastapi", ".py") || a.hasFileContent("import fastapi", ".py") {
		return FrameworkFastAPI
	}
	return FrameworkNone
}

func (a *Analyzer) detectGoFramework() Framework {
	if a.hasFileContent("github.com/gin-gonic/gin", ".go") {
		return FrameworkGin
	}
	if a.hasFileContent("github.com/labstack/echo", ".go") {
		return FrameworkEcho
	}
	return FrameworkNone
}

func (a *Analyzer) detectProjectType(profile *Profile) ProjectType {
	hasFrontend := false
	hasBackend := false

	switch profile.Language {
	case LangJS, LangTS:
		hasFrontend = true
	case LangGo, LangPython, LangRust, LangJava, LangPHP, LangRuby:
		hasBackend = true
	}

	// Check for monorepo structure
	if a.exists("packages") || a.exists("apps") {
		if dirs, _ := os.ReadDir(filepath.Join(a.root, "packages")); len(dirs) > 1 {
			return TypeMonorepo
		}
	}

	// Check for both frontend and backend indicators
	if a.exists("frontend") && a.exists("backend") {
		return TypeFullStack
	}

	if hasFrontend && hasBackend {
		return TypeFullStack
	}

	if hasFrontend {
		return TypeWebFrontend
	}

	if hasBackend {
		return TypeWebBackend
	}

	// CLI detection
	if profile.MainFile == "main.go" || profile.MainFile == "cmd" {
		return TypeCLI
	}

	// Library detection
	if !profile.HasTests && profile.Dependencies == nil {
		return TypeLibrary
	}

	return TypeUnknown
}

func (a *Analyzer) findBuildCommand(lang Lang, framework Framework) string {
	switch lang {
	case LangGo:
		return "go build ./..."
	case LangJS, LangTS:
		if framework == FrameworkNextJS {
			return "npm run build"
		}
		if a.exists("package.json") {
			return "npm run build"
		}
	case LangPython:
		if framework == FrameworkDjango {
			return "python manage.py collectstatic"
		}
	case LangRust:
		return "cargo build"
	case LangJava:
		if a.exists("pom.xml") {
			return "mvn package"
		}
		if a.exists("build.gradle") {
			return "gradle build"
		}
	}
	return ""
}

func (a *Analyzer) findTestCommand(lang Lang) string {
	switch lang {
	case LangGo:
		return "go test ./..."
	case LangJS, LangTS:
		if a.exists("package.json") {
			return "npm test"
		}
	case LangPython:
		return "python -m pytest"
	case LangRust:
		return "cargo test"
	case LangJava:
		if a.exists("pom.xml") {
			return "mvn test"
		}
		if a.exists("build.gradle") {
			return "gradle test"
		}
	}
	return ""
}

func (a *Analyzer) findDevCommand(lang Lang, framework Framework) string {
	switch lang {
	case LangGo:
		if a.exists("air.toml") || a.exists(".air.conf") {
			return "air"
		}
		return "go run ./cmd/..."
	case LangJS, LangTS:
		if framework == FrameworkNextJS {
			return "npm run dev"
		}
		if a.exists("package.json") {
			return "npm run dev"
		}
	case LangPython:
		if framework == FrameworkDjango {
			return "python manage.py runserver"
		}
		if framework == FrameworkFlask {
			return "flask run"
		}
		if framework == FrameworkFastAPI {
			return "uvicorn main:app --reload"
		}
	}
	return ""
}

func (a *Analyzer) findMainFile(lang Lang) string {
	switch lang {
	case LangGo:
		if a.exists("main.go") {
			return "main.go"
		}
		if a.exists("cmd") {
			return "cmd"
		}
	case LangPython:
		if a.exists("main.py") {
			return "main.py"
		}
		if a.exists("app.py") {
			return "app.py"
		}
	case LangJS, LangTS:
		if a.exists("index.js") {
			return "index.js"
		}
		if a.exists("index.ts") {
			return "index.ts"
		}
		if a.exists("src/index.js") {
			return "src/index.js"
		}
		if a.exists("src/index.ts") {
			return "src/index.ts"
		}
	}
	return ""
}

func (a *Analyzer) findDependencies(lang Lang) []string {
	var deps []string

	switch lang {
	case LangGo:
		if data, err := os.ReadFile(filepath.Join(a.root, "go.mod")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "require") || strings.HasPrefix(line, "// indirect") {
					continue
				}
				if parts := strings.Fields(line); len(parts) >= 1 && !strings.HasPrefix(parts[0], "//") {
					deps = append(deps, parts[0])
				}
			}
		}
	case LangJS, LangTS:
		if data, err := os.ReadFile(filepath.Join(a.root, "package.json")); err == nil {
			var pkg map[string]interface{}
			if json.Unmarshal(data, &pkg) == nil {
				if dependencies, ok := pkg["dependencies"].(map[string]interface{}); ok {
					for dep := range dependencies {
						deps = append(deps, dep)
					}
				}
				if devDependencies, ok := pkg["devDependencies"].(map[string]interface{}); ok {
					for dep := range devDependencies {
						deps = append(deps, dep)
					}
				}
			}
		}
	case LangPython:
		if data, err := os.ReadFile(filepath.Join(a.root, "requirements.txt")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					deps = append(deps, line)
				}
			}
		}
	}

	return deps
}

func (a *Analyzer) hasTests(lang Lang) bool {
	switch lang {
	case LangGo:
		return a.hasFilesWithExt("_test.go")
	case LangJS, LangTS:
		return a.hasFilesWithExt(".test.js") || a.hasFilesWithExt(".test.ts") ||
			a.hasFilesWithExt(".spec.js") || a.hasFilesWithExt(".spec.ts") ||
			a.exists("__tests__") || a.exists("test") || a.exists("tests")
	case LangPython:
		return a.hasFilesWithExt("_test.py") || a.hasFilesWithExt("test_.py") ||
			a.exists("tests") || a.exists("test")
	case LangRust:
		return true // Rust has inline tests
	}
	return false
}

func (a *Analyzer) hasCI() bool {
	return a.exists(".github/workflows") || a.exists(".gitlab-ci.yml") ||
		a.exists(".circleci") || a.exists("Jenkinsfile") || a.exists(".travis.yml")
}

func (a *Analyzer) hasDocker() bool {
	return a.exists("Dockerfile") || a.exists("docker-compose.yml") || a.exists("docker-compose.yaml")
}

func (a *Analyzer) hasLint(lang Lang) bool {
	switch lang {
	case LangGo:
		return a.exists(".golangci.yml") || a.exists(".golangci.yaml")
	case LangJS, LangTS:
		return a.exists(".eslintrc") || a.exists(".eslintrc.js") || a.exists(".eslintrc.json") ||
			a.exists("eslint.config.js")
	case LangPython:
		return a.exists(".pylintrc") || a.exists("pyproject.toml") || a.exists(".flake8")
	}
	return false
}

func (a *Analyzer) hasFormat(lang Lang) bool {
	switch lang {
	case LangGo:
		return true // gofmt is built-in
	case LangJS, LangTS:
		return a.exists(".prettierrc") || a.exists("prettier.config.js") ||
			a.hasFileContent("prettier", "package.json")
	case LangPython:
		return a.exists("pyproject.toml") // May contain black/ruff config
	}
	return false
}

func (a *Analyzer) scanDirectories() DirSummary {
	var summary DirSummary

	entries, err := os.ReadDir(a.root)
	if err != nil {
		return summary
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch {
		case name == "src" || name == "lib" || name == "internal" || name == "pkg":
			summary.Src = append(summary.Src, name)
		case name == "test" || name == "tests" || name == "__tests__" || name == "spec":
			summary.Tests = append(summary.Tests, name)
		case name == "config" || name == "conf" || name == ".config":
			summary.Config = append(summary.Config, name)
		case name == "docs" || name == "doc":
			summary.Docs = append(summary.Docs, name)
		case name == "assets" || name == "static" || name == "public":
			summary.Assets = append(summary.Assets, name)
		}
	}

	return summary
}

// Helper methods

func (a *Analyzer) exists(path string) bool {
	_, err := os.Stat(filepath.Join(a.root, path))
	return err == nil
}

func (a *Analyzer) hasFilesWithExt(ext string) bool {
	found := false
	filepath.Walk(a.root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ext) {
			found = true
		}
		return nil
	})
	return found
}

func (a *Analyzer) hasFileContent(needle, ext string) bool {
	found := false
	filepath.Walk(a.root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ext) {
			if data, err := os.ReadFile(path); err == nil {
				if strings.Contains(strings.ToLower(string(data)), strings.ToLower(needle)) {
					found = true
				}
			}
		}
		return nil
	})
	return found
}

func (a *Analyzer) extractDeps(pkg map[string]interface{}) map[string]bool {
	deps := make(map[string]bool)
	if dependencies, ok := pkg["dependencies"].(map[string]interface{}); ok {
		for dep := range dependencies {
			deps[dep] = true
		}
	}
	if devDependencies, ok := pkg["devDependencies"].(map[string]interface{}); ok {
		for dep := range devDependencies {
			deps[dep] = true
		}
	}
	return deps
}
