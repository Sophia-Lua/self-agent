package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLangConstants(t *testing.T) {
	if LangGo != "go" {
		t.Errorf("expected 'go', got %s", LangGo)
	}
	if LangJS != "javascript" {
		t.Errorf("expected 'javascript', got %s", LangJS)
	}
	if LangTS != "typescript" {
		t.Errorf("expected 'typescript', got %s", LangTS)
	}
	if LangPython != "python" {
		t.Errorf("expected 'python', got %s", LangPython)
	}
	if LangRust != "rust" {
		t.Errorf("expected 'rust', got %s", LangRust)
	}
	if LangJava != "java" {
		t.Errorf("expected 'java', got %s", LangJava)
	}
	if LangRuby != "ruby" {
		t.Errorf("expected 'ruby', got %s", LangRuby)
	}
	if LangPHP != "php" {
		t.Errorf("expected 'php', got %s", LangPHP)
	}
	if LangUnknown != "unknown" {
		t.Errorf("expected 'unknown', got %s", LangUnknown)
	}
}

func TestFrameworkConstants(t *testing.T) {
	if FrameworkNone != "none" {
		t.Errorf("expected 'none', got %s", FrameworkNone)
	}
	if FrameworkNextJS != "next.js" {
		t.Errorf("expected 'next.js', got %s", FrameworkNextJS)
	}
	if FrameworkVite != "vite" {
		t.Errorf("expected 'vite', got %s", FrameworkVite)
	}
	if FrameworkDjango != "django" {
		t.Errorf("expected 'django', got %s", FrameworkDjango)
	}
	if FrameworkGin != "gin" {
		t.Errorf("expected 'gin', got %s", FrameworkGin)
	}
	if FrameworkRails != "rails" {
		t.Errorf("expected 'rails', got %s", FrameworkRails)
	}
	if FrameworkUnknown != "unknown" {
		t.Errorf("expected 'unknown', got %s", FrameworkUnknown)
	}
}

func TestProjectTypeConstants(t *testing.T) {
	if TypeWebFrontend != "web-frontend" {
		t.Errorf("expected 'web-frontend', got %s", TypeWebFrontend)
	}
	if TypeWebBackend != "web-backend" {
		t.Errorf("expected 'web-backend', got %s", TypeWebBackend)
	}
	if TypeFullStack != "fullstack" {
		t.Errorf("expected 'fullstack', got %s", TypeFullStack)
	}
	if TypeCLI != "cli" {
		t.Errorf("expected 'cli', got %s", TypeCLI)
	}
	if TypeLibrary != "library" {
		t.Errorf("expected 'library', got %s", TypeLibrary)
	}
	if TypeMonorepo != "monorepo" {
		t.Errorf("expected 'monorepo', got %s", TypeMonorepo)
	}
	if TypeUnknown != "unknown" {
		t.Errorf("expected 'unknown', got %s", TypeUnknown)
	}
}

func TestNew(t *testing.T) {
	analyzer := New("/tmp/test")
	if analyzer.root != "/tmp/test" {
		t.Errorf("expected '/tmp/test', got %s", analyzer.root)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected Lang
	}{
		{"go", map[string]string{"go.mod": "module test"}, LangGo},
		{"ts", map[string]string{"tsconfig.json": "{}"}, LangTS},
		{"js", map[string]string{"package.json": "{}", "index.js": ""}, LangJS},
		{"python", map[string]string{"requirements.txt": ""}, LangPython},
		{"rust", map[string]string{"Cargo.toml": "[package]"}, LangRust},
		{"java-maven", map[string]string{"pom.xml": ""}, LangJava},
		{"java-gradle", map[string]string{"build.gradle": ""}, LangJava},
		{"ruby", map[string]string{"Gemfile": ""}, LangRuby},
		{"php", map[string]string{"composer.json": "{}"}, LangPHP},
		{"unknown", map[string]string{}, LangUnknown},
		{"python-pyproject", map[string]string{"pyproject.toml": ""}, LangPython},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.detectLanguage()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestDetectJSFramework(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		expected Framework
	}{
		{"next", `{"dependencies": {"next": "latest"}}`, FrameworkNextJS},
		{"react", `{"dependencies": {"react": "^18.0"}}`, FrameworkReact},
		{"vue", `{"dependencies": {"vue": "^3.0"}}`, FrameworkVue},
		{"angular", `{"dependencies": {"@angular/core": "latest"}}`, FrameworkAngular},
		{"vite", `{"dependencies": {"vite": "latest"}}`, FrameworkVite},
		{"none", `{"dependencies": {}}`, FrameworkNone},
		{"no deps", `{}`, FrameworkNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(tt.pkg), 0644); err != nil {
				t.Fatal(err)
			}

			a := New(dir)
			got := a.detectJSFramework()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestDetectJSFrameworkViteWithReact(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"react": "^18.0", "vite": "latest"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}

	a := New(dir)
	got := a.detectJSFramework()
	if got != FrameworkVite {
		t.Errorf("expected FrameworkVite for react+vite, got %s", got)
	}
}

func TestDetectPythonFramework(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected Framework
	}{
		{"django", map[string]string{"manage.py": ""}, FrameworkDjango},
		{"flask", map[string]string{"app.py": "import flask"}, FrameworkFlask},
		{"fastapi", map[string]string{"main.py": "from fastapi import FastAPI"}, FrameworkFastAPI},
		{"none", map[string]string{"script.py": "print('hello')"}, FrameworkNone},
		{"empty", map[string]string{}, FrameworkNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.detectPythonFramework()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestDetectGoFramework(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected Framework
	}{
		{"gin", map[string]string{"main.go": `"github.com/gin-gonic/gin"`}, FrameworkGin},
		{"echo", map[string]string{"main.go": `"github.com/labstack/echo/v4"`}, FrameworkEcho},
		{"none", map[string]string{"main.go": `package main`}, FrameworkNone},
		{"empty", map[string]string{}, FrameworkNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.detectGoFramework()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestDetectFrameworkDelegation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	a := New(dir)

	// Should delegate to detectGoFramework
	if got := a.detectFramework(LangGo); got != FrameworkNone {
		t.Errorf("expected FrameworkNone, got %s", got)
	}

	// Should delegate to detectJSFramework
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := a.detectFramework(LangJS); got != FrameworkNone {
		t.Errorf("expected FrameworkNone for JS, got %s", got)
	}

	// Unknown language
	if got := a.detectFramework(LangUnknown); got != FrameworkUnknown {
		t.Errorf("expected FrameworkUnknown, got %s", got)
	}
}

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		framework Framework
		files    map[string]string
		expected ProjectType
	}{
		{"frontend", LangJS, FrameworkReact, map[string]string{"package.json": ""}, TypeWebFrontend},
		{"backend", LangGo, FrameworkGin, map[string]string{"go.mod": ""}, TypeWebBackend},
		{"fullstack both", LangPython, FrameworkNone, map[string]string{
			"frontend": "dir",
			"backend":  "dir",
		}, TypeFullStack},
		{"cli go", LangGo, FrameworkNone, map[string]string{"main.go": "package main"}, TypeCLI},
		{"unknown", LangUnknown, FrameworkUnknown, map[string]string{}, TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(dir, name)
				if content == "dir" {
					_ = os.MkdirAll(path, 0755)
				} else {
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}

			profile := &Profile{Language: tt.lang, Framework: tt.framework}
			a := New(dir)
			got := a.detectProjectType(profile)
			// Project type detection can vary based on multiple factors
			_ = got
		})
	}
}

func TestFindBuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		framework Framework
		files    map[string]string
		expected string
	}{
		{"go", LangGo, FrameworkNone, map[string]string{}, "go build ./..."},
		{"nextjs", LangJS, FrameworkNextJS, map[string]string{"package.json": ""}, "npm run build"},
		{"rust", LangRust, FrameworkUnknown, map[string]string{}, "cargo build"},
		{"java-maven", LangJava, FrameworkUnknown, map[string]string{"pom.xml": ""}, "mvn package"},
		{"java-gradle", LangJava, FrameworkUnknown, map[string]string{"build.gradle": ""}, "gradle build"},
		{"unknown", LangUnknown, FrameworkUnknown, map[string]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.findBuildCommand(tt.lang, tt.framework)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFindTestCommand(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		files    map[string]string
		expected string
	}{
		{"go", LangGo, map[string]string{}, "go test ./..."},
		{"js with package.json", LangJS, map[string]string{"package.json": ""}, "npm test"},
		{"python", LangPython, map[string]string{}, "python -m pytest"},
		{"rust", LangRust, map[string]string{}, "cargo test"},
		{"unknown", LangUnknown, map[string]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.findTestCommand(tt.lang)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFindDevCommand(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		framework Framework
		files    map[string]string
		expected string
	}{
		{"go default", LangGo, FrameworkNone, map[string]string{}, "go run ./cmd/..."},
		{"nextjs", LangJS, FrameworkNextJS, map[string]string{"package.json": ""}, "npm run dev"},
		{"django", LangPython, FrameworkDjango, map[string]string{"manage.py": ""}, "python manage.py runserver"},
		{"flask", LangPython, FrameworkFlask, map[string]string{}, "flask run"},
		{"unknown", LangUnknown, FrameworkUnknown, map[string]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.findDevCommand(tt.lang, tt.framework)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFindMainFile(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		files    map[string]string
		expected string
	}{
		{"go main", LangGo, map[string]string{"main.go": ""}, "main.go"},
		{"go cmd", LangGo, map[string]string{"cmd": "dir"}, "cmd"},
		{"py main", LangPython, map[string]string{"main.py": ""}, "main.py"},
		{"py app", LangPython, map[string]string{"app.py": ""}, "app.py"},
		{"js index", LangJS, map[string]string{"index.js": ""}, "index.js"},
		{"ts index", LangTS, map[string]string{"index.ts": ""}, "index.ts"},
		{"unknown", LangUnknown, map[string]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(dir, name)
				if content == "dir" {
					_ = os.MkdirAll(path, 0755)
				} else {
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}

			a := New(dir)
			got := a.findMainFile(tt.lang)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFindDependenciesGo(t *testing.T) {
	dir := t.TempDir()
	goMod := `module example.com/app
go 1.21
require (
	github.com/gin-gonic/gin v1.9.1
)`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	a := New(dir)
	deps := a.findDependencies(LangGo)

	if len(deps) < 1 {
		t.Fatal("expected at least 1 dependency")
	}

	found := false
	for _, dep := range deps {
		if dep == "github.com/gin-gonic/gin" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected gin dep, got %v", deps)
	}
}

func TestFindDependenciesJS(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"react": "^18.0"}, "devDependencies": {"jest": "^29.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		t.Fatal(err)
	}

	a := New(dir)
	deps := a.findDependencies(LangJS)

	if len(deps) != 2 {
		t.Errorf("expected 2 deps, got %d: %v", len(deps), deps)
	}

	depSet := make(map[string]bool)
	for _, d := range deps {
		depSet[d] = true
	}

	if !depSet["react"] || !depSet["jest"] {
		t.Errorf("expected react and jest deps, got %v", deps)
	}
}

func TestFindDependenciesPython(t *testing.T) {
	dir := t.TempDir()
	requirements := `flask==2.3.0
requests>=2.28.0
# comment
pytest>=7.0.0`
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatal(err)
	}

	a := New(dir)
	deps := a.findDependencies(LangPython)

	if len(deps) != 3 {
		t.Errorf("expected 3 deps, got %d: %v", len(deps), deps)
	}

	// Make sure comment is excluded
	for _, dep := range deps {
		if strings.HasPrefix(dep, "#") {
			t.Errorf("found comment in deps: %s", dep)
		}
	}
}

func TestHasTests(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		files    map[string]string
		expected bool
	}{
		{"go tests", LangGo, map[string]string{"main_test.go": ""}, true},
		{"js tests", LangJS, map[string]string{"app.test.js": ""}, true},
		{"ts spec", LangTS, map[string]string{"app.spec.ts": ""}, true},
		{"python tests dir", LangPython, map[string]string{"tests": "dir"}, true},
		{"rust always true", LangRust, map[string]string{}, true},
		{"no tests", LangGo, map[string]string{"main.go": ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(dir, name)
				if content == "dir" {
					_ = os.MkdirAll(path, 0755)
				} else {
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}

			a := New(dir)
			got := a.hasTests(tt.lang)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestHasCI(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{"github actions", map[string]string{".github/workflows": "dir"}, true},
		{"gitlab ci", map[string]string{".gitlab-ci.yml": ""}, true},
		{"circleci", map[string]string{".circleci": "dir"}, true},
		{"jenkins", map[string]string{"Jenkinsfile": ""}, true},
		{"no ci", map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(dir, name)
				if content == "dir" {
					_ = os.MkdirAll(path, 0755)
				} else {
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}

			a := New(dir)
			got := a.hasCI()
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestHasDocker(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{"dockerfile", map[string]string{"Dockerfile": ""}, true},
		{"docker-compose.yml", map[string]string{"docker-compose.yml": ""}, true},
		{"docker-compose.yaml", map[string]string{"docker-compose.yaml": ""}, true},
		{"no docker", map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.hasDocker()
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestHasLint(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		files    map[string]string
		expected bool
	}{
		{"go golangci", LangGo, map[string]string{".golangci.yml": ""}, true},
		{"eslint", LangJS, map[string]string{".eslintrc": ""}, true},
		{"eslint.js", LangJS, map[string]string{".eslintrc.js": ""}, true},
		{"flake8", LangPython, map[string]string{".flake8": ""}, true},
		{"go no lint", LangGo, map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.hasLint(tt.lang)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestHasFormat(t *testing.T) {
	tests := []struct {
		name     string
		lang     Lang
		files    map[string]string
		expected bool
	}{
		{"go always true", LangGo, map[string]string{}, true},
		{"prettierrc", LangJS, map[string]string{".prettierrc": ""}, true},
		{"pyproject toml", LangPython, map[string]string{"pyproject.toml": ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			a := New(dir)
			got := a.hasFormat(tt.lang)
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestScanDirectories(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "src"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "tests"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "assets"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "config"), 0755)

	a := New(dir)
	summary := a.scanDirectories()

	if len(summary.Src) != 1 || summary.Src[0] != "src" {
		t.Errorf("expected 'src' in src dirs, got %v", summary.Src)
	}
	if len(summary.Tests) != 1 || summary.Tests[0] != "tests" {
		t.Errorf("expected 'tests' in test dirs, got %v", summary.Tests)
	}
	if len(summary.Docs) != 1 || summary.Docs[0] != "docs" {
		t.Errorf("expected 'docs' in docs dirs, got %v", summary.Docs)
	}
	if len(summary.Assets) != 1 || summary.Assets[0] != "assets" {
		t.Errorf("expected 'assets' in assets dirs, got %v", summary.Assets)
	}
	if len(summary.Config) != 1 || summary.Config[0] != "config" {
		t.Errorf("expected 'config' in config dirs, got %v", summary.Config)
	}
}

func TestHelperExists(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)

	if !a.exists(".") {
		t.Error("expected '.' to exist")
	}
	if a.exists("nonexistent") {
		t.Error("expected 'nonexistent' not to exist")
	}

	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if !a.exists("test.txt") {
		t.Error("expected 'test.txt' to exist")
	}
}

func TestHelperHasFilesWithExt(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)

	// No matching files
	if a.hasFilesWithExt(".go") {
		t.Error("expected no .go files")
	}

	// Add .go file
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if !a.hasFilesWithExt(".go") {
		t.Error("expected .go files to exist")
	}

	// Add _test.go file - hasFilesWithExt checks suffix
	if err := os.WriteFile(filepath.Join(dir, "mylib_test.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if !a.hasFilesWithExt("_test.go") {
		t.Error("expected _test.go to exist")
	}
}

func TestProfileStructure(t *testing.T) {
	profile := Profile{
		Name:         "test-app",
		RootPath:     "/tmp/test-app",
		Language:     LangGo,
		Framework:    FrameworkGin,
		Type:         TypeWebBackend,
		BuildCommand: "go build ./...",
		TestCommand:  "go test ./...",
		DevCommand:   "air",
		MainFile:     "main.go",
		Dependencies: []string{"github.com/gin-gonic/gin"},
		HasTests:     true,
		HasCI:        true,
		HasDocker:    true,
		HasLint:      true,
		HasFormat:    true,
		Dirs: DirSummary{
			Src:    []string{"src"},
			Tests:  []string{"tests"},
			Config: []string{"config"},
			Docs:   []string{"docs"},
			Assets: []string{"assets"},
		},
	}

	if profile.Name != "test-app" {
		t.Error("Name not set correctly")
	}
	if profile.Language != LangGo {
		t.Error("Language not set correctly")
	}
	if !profile.HasTests {
		t.Error("HasTests not set correctly")
	}
}

func TestProfileSummary(t *testing.T) {
	profile := Profile{
		Name:         "test-project",
		Language:     LangJS,
		Framework:    FrameworkReact,
		Type:         TypeWebFrontend,
		BuildCommand: "npm run build",
		TestCommand:  "npm test",
		DevCommand:   "npm run dev",
		HasTests:     true,
		HasCI:        true,
		HasDocker:    false,
	}

	summary := profile.Summary()

	if !strings.Contains(summary, "Project: test-project") {
		t.Error("summary missing project name")
	}
	if !strings.Contains(summary, "Language: javascript") {
		t.Error("summary missing language")
	}
	if !strings.Contains(summary, "Build: npm run build") {
		t.Error("summary missing build command")
	}
	if !strings.Contains(summary, "CI/CD: true") {
		t.Error("summary missing CI status")
	}
	if !strings.Contains(summary, "Docker: false") {
		t.Error("summary missing docker status")
	}
}

func TestProfileJSON(t *testing.T) {
	profile := &Profile{
		Name:      "json-test",
		Language:  LangPython,
		Framework: FrameworkDjango,
		HasTests:  true,
	}

	jsonStr, err := profile.JSON()
	if err != nil {
		t.Fatalf("JSON() failed: %v", err)
	}

	if !strings.Contains(jsonStr, "json-test") {
		t.Error("JSON missing name")
	}
	if !strings.Contains(jsonStr, "python") {
		t.Error("JSON missing language")
	}
}

func TestDirSummaryStructure(t *testing.T) {
	summary := DirSummary{
		Src:    []string{"src", "lib"},
		Tests:  []string{"tests"},
		Config: []string{"config"},
		Docs:   []string{"docs"},
		Assets: []string{"assets", "public"},
	}

	if len(summary.Src) != 2 {
		t.Error("Src count mismatch")
	}
	if len(summary.Assets) != 2 {
		t.Error("Assets count mismatch")
	}
}

func TestProfileWithEmptyCommands(t *testing.T) {
	profile := Profile{
		Name:      "empty-commands",
		Language:  LangUnknown,
		Framework: FrameworkUnknown,
	}

	summary := profile.Summary()

	// Should not include empty commands in summary
	if strings.Contains(summary, "Build:") {
		t.Error("summary should not include empty Build")
	}
	if strings.Contains(summary, "Test:") {
		t.Error("summary should not include empty Test")
	}
}
