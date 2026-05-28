package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"autodev/internal/core"
)

// RegisterFileTools adds basic file manipulation tools to the registry.
func RegisterFileTools(reg *Registry, workDir string) {
	// write_file
	reg.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "write_file",
			Description: "Write content to a file. If the file exists, it will be overwritten. The path should be absolute or relative to the working directory.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]string{"type": "string", "description": "File path"},
					"content": map[string]string{"type": "string", "description": "File content"},
				},
				"required": []string{"path", "content"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		path, ok := args["path"].(string)
		if !ok {
			return nil, ErrMissingPath
		}
		
		// Resolve path if relative
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}

		// Create parent directories if needed
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}

		content, _ := args["content"].(string)
		
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return nil, err
		}

		return map[string]string{"status": "success", "path": path}, nil
	})
	
	// read_file
	reg.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "read_file",
			Description: "Read the content of a file.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]string{"type": "string", "description": "File path"},
				},
				"required": []string{"path"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		path, ok := args["path"].(string)
		if !ok {
			return nil, ErrMissingPath
		}

		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		return map[string]string{"path": path, "content": string(content)}, nil
	})

	// list_files
	reg.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "list_files",
			Description: "List files and directories in a given path. Supports optional pattern filtering.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]string{"type": "string", "description": "Directory path to list"},
					"pattern": map[string]string{"type": "string", "description": "Optional glob pattern to filter files (e.g., '*.go')"},
					"recursive": map[string]any{"type": "boolean", "description": "Whether to list recursively (default: false)"},
				},
				"required": []string{"path"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		path, ok := args["path"].(string)
		if !ok {
			return nil, ErrMissingPath
		}

		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}

		pattern, _ := args["pattern"].(string)
		recursive, _ := args["recursive"].(bool)

		var fileList []map[string]any
		if recursive {
			var err error
			fileList, err = listRecursive(path, pattern)
			if err != nil {
				return nil, fmt.Errorf("recursive list failed: %w", err)
			}
		} else {
			for _, entry := range entries {
				if pattern != "" {
					matched, _ := filepath.Match(pattern, entry.Name())
					if !matched {
						continue
					}
				}
				info, _ := entry.Info()
				fileList = append(fileList, map[string]any{
					"name":    entry.Name(),
					"is_dir":  entry.IsDir(),
					"size":    info.Size(),
				})
			}
		}

		return map[string]any{"path": path, "files": fileList, "count": len(fileList)}, nil
	})

	// search_files
	reg.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "search_files",
			Description: "Search for files matching a pattern or files containing specific text content.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":      map[string]string{"type": "string", "description": "Directory to search in"},
					"name_pattern": map[string]string{"type": "string", "description": "Filename pattern to match (glob)"},
					"content_query": map[string]string{"type": "string", "description": "Text to search for within files"},
					"max_depth": map[string]any{"type": "integer", "description": "Max directory depth to search (default: 5)"},
				},
				"required": []string{"path"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		path, ok := args["path"].(string)
		if !ok {
			return nil, ErrMissingPath
		}

		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}

		namePattern, _ := args["name_pattern"].(string)
		contentQuery, _ := args["content_query"].(string)
		maxDepth, _ := args["max_depth"].(int)
		if maxDepth <= 0 {
			maxDepth = 5
		}

	var results []map[string]any
	walkErr := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		depth := strings.Count(strings.TrimPrefix(p, path), string(os.PathSeparator))
		if depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if namePattern != "" {
			matched, _ := filepath.Match(namePattern, info.Name())
			if !matched {
				return nil
			}
		}

		if contentQuery != "" {
			content, readErr := os.ReadFile(p)
			if readErr != nil {
				return nil
			}
			if !strings.Contains(string(content), contentQuery) {
				return nil
			}
		}

		relPath, _ := filepath.Rel(path, p)
		results = append(results, map[string]any{
			"path": relPath,
			"size": info.Size(),
		})

		return nil
	})

	if walkErr != nil {
		return nil, fmt.Errorf("file search failed: %w", walkErr)
	}

	return map[string]any{"path": path, "results": results, "count": len(results)}, nil
	})

	// execute_command
	reg.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "execute_command",
			Description: "Execute a shell command and return its output. Use with caution.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]string{"type": "string", "description": "The command and arguments to execute"},
					"working_dir": map[string]string{"type": "string", "description": "Optional working directory for the command"},
				},
				"required": []string{"command"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		command, ok := args["command"].(string)
		if !ok {
			return nil, ErrMissingCommand
		}

		workingDir := workDir
		if wd, ok := args["working_dir"].(string); ok && wd != "" {
			if !filepath.IsAbs(wd) {
				wd = filepath.Join(workDir, wd)
			}
			workingDir = wd
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = workingDir

		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		return map[string]any{
			"command":    command,
			"exit_code":  exitCode,
			"output":     string(output),
			"working_dir": workingDir,
		}, nil
	})
}

type ToolResult struct {
	Status string `json:"status"`
	Message string `json:"message,omitempty"`
}

var ErrMissingPath = &ToolError{"missing 'path' argument"}
var ErrMissingCommand = &ToolError{"missing 'command' argument"}

type ToolError struct {
	msg string
}

func (e *ToolError) Error() string {
	return e.msg
}

func listRecursive(dir, pattern string) ([]map[string]any, error) {
	var fileList []map[string]any
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if pattern != "" {
			matched, _ := filepath.Match(pattern, info.Name())
			if !matched {
				return nil
			}
		}
		rel, _ := filepath.Rel(dir, p)
		fileList = append(fileList, map[string]any{
			"name":   info.Name(),
			"path":   rel,
			"is_dir": info.IsDir(),
			"size":   info.Size(),
		})
		return nil
	})
	return fileList, err
}
