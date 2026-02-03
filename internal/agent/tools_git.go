package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// GIT TOOLS
// ═══════════════════════════════════════════════════════════════════════════════

// GitStatusTool shows git working tree status
type GitStatusTool struct {
	BaseTool
	workingDir string
}

// NewGitStatusTool creates a git status tool
func NewGitStatusTool(workingDir string) *GitStatusTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"short": map[string]interface{}{
				"type":        "boolean",
				"description": "Use short format output (default: false)",
			},
		},
	}

	return &GitStatusTool{
		BaseTool: NewBaseTool(
			"git_status",
			"Show git working tree status. Returns staged, unstaged, and untracked files.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitStatusTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	cmdArgs := []string{"status", "--porcelain=v1"}

	short := false
	if s, ok := args["short"].(bool); ok {
		short = s
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git status failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Parse porcelain output
	var staged, unstaged, untracked []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		file := strings.TrimSpace(line[3:])

		if x == '?' {
			untracked = append(untracked, file)
		} else {
			if x != ' ' {
				staged = append(staged, file)
			}
			if y != ' ' {
				unstaged = append(unstaged, file)
			}
		}
	}

	// Get branch info
	branchCmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	if t.workingDir != "" {
		branchCmd.Dir = t.workingDir
	}
	branchOut, _ := branchCmd.Output()
	branch := strings.TrimSpace(string(branchOut))

	result := map[string]interface{}{
		"branch":    branch,
		"staged":    staged,
		"unstaged":  unstaged,
		"untracked": untracked,
		"clean":     len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0,
		"success":   true,
	}

	if short {
		result["output"] = string(output)
	}

	return result, nil
}

// GitDiffTool shows git diff
type GitDiffTool struct {
	BaseTool
	workingDir string
}

// NewGitDiffTool creates a git diff tool
func NewGitDiffTool(workingDir string) *GitDiffTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"staged": map[string]interface{}{
				"type":        "boolean",
				"description": "Show staged changes (--cached)",
			},
			"file": map[string]interface{}{
				"type":        "string",
				"description": "Specific file to diff",
			},
			"commit": map[string]interface{}{
				"type":        "string",
				"description": "Compare with specific commit (e.g., HEAD~1, main)",
			},
			"stat": map[string]interface{}{
				"type":        "boolean",
				"description": "Show diffstat only (file changes summary)",
			},
		},
	}

	return &GitDiffTool{
		BaseTool: NewBaseTool(
			"git_diff",
			"Show changes between commits, commit and working tree, etc.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitDiffTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	cmdArgs := []string{"diff", "--no-color"}

	if staged, ok := args["staged"].(bool); ok && staged {
		cmdArgs = append(cmdArgs, "--cached")
	}

	if stat, ok := args["stat"].(bool); ok && stat {
		cmdArgs = append(cmdArgs, "--stat")
	}

	if commit, ok := args["commit"].(string); ok && commit != "" {
		cmdArgs = append(cmdArgs, commit)
	}

	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git diff failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Count changed files from stat
	diffOutput := string(output)
	changedFiles := 0
	additions := 0
	deletions := 0

	// Parse stat output if present
	lines := strings.Split(diffOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, " | ") {
			changedFiles++
		}
		if strings.Contains(line, "insertion") || strings.Contains(line, "deletion") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if strings.HasPrefix(p, "insertion") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &additions)
				}
				if strings.HasPrefix(p, "deletion") && i > 0 {
					fmt.Sscanf(parts[i-1], "%d", &deletions)
				}
			}
		}
	}

	return map[string]interface{}{
		"diff":      diffOutput,
		"files":     changedFiles,
		"additions": additions,
		"deletions": deletions,
		"success":   true,
	}, nil
}

// GitCommitTool commits changes
type GitCommitTool struct {
	BaseTool
	workingDir string
}

// NewGitCommitTool creates a git commit tool
func NewGitCommitTool(workingDir string) *GitCommitTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Commit message (required)",
			},
			"files": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Specific files to commit (default: all staged)",
			},
			"all": map[string]interface{}{
				"type":        "boolean",
				"description": "Stage all modified files before commit (-a)",
			},
		},
		"required": []string{"message"},
	}

	return &GitCommitTool{
		BaseTool: NewBaseTool(
			"git_commit",
			"Record changes to the repository. Stages files and commits with message.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitCommitTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("message parameter is required")
	}

	// Stage files if specified
	if files, ok := args["files"].([]interface{}); ok && len(files) > 0 {
		addArgs := []string{"add"}
		for _, f := range files {
			if s, ok := f.(string); ok {
				addArgs = append(addArgs, s)
			}
		}
		addCmd := exec.CommandContext(ctx, "git", addArgs...)
		if t.workingDir != "" {
			addCmd.Dir = t.workingDir
		}
		if out, err := addCmd.CombinedOutput(); err != nil {
			return map[string]interface{}{
				"error":   fmt.Sprintf("git add failed: %v", err),
				"output":  string(out),
				"success": false,
			}, nil
		}
	}

	// Build commit command
	cmdArgs := []string{"commit", "-m", message}

	if all, ok := args["all"].(bool); ok && all {
		cmdArgs = append(cmdArgs, "-a")
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git commit failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Get commit hash
	hashCmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	if t.workingDir != "" {
		hashCmd.Dir = t.workingDir
	}
	hashOut, _ := hashCmd.Output()
	hash := strings.TrimSpace(string(hashOut))

	return map[string]interface{}{
		"message": message,
		"hash":    hash,
		"output":  string(output),
		"success": true,
	}, nil
}

// GitPushTool pushes commits to remote
type GitPushTool struct {
	BaseTool
	workingDir string
}

// NewGitPushTool creates a git push tool
func NewGitPushTool(workingDir string) *GitPushTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"remote": map[string]interface{}{
				"type":        "string",
				"description": "Remote name (default: origin)",
			},
			"branch": map[string]interface{}{
				"type":        "string",
				"description": "Branch name (default: current branch)",
			},
			"set_upstream": map[string]interface{}{
				"type":        "boolean",
				"description": "Set upstream tracking (-u)",
			},
		},
	}

	return &GitPushTool{
		BaseTool: NewBaseTool(
			"git_push",
			"Push commits to remote repository.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitPushTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	remote := "origin"
	if r, ok := args["remote"].(string); ok && r != "" {
		remote = r
	}

	cmdArgs := []string{"push"}

	if setUpstream, ok := args["set_upstream"].(bool); ok && setUpstream {
		cmdArgs = append(cmdArgs, "-u")
	}

	cmdArgs = append(cmdArgs, remote)

	if branch, ok := args["branch"].(string); ok && branch != "" {
		cmdArgs = append(cmdArgs, branch)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git push failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"remote":  remote,
		"output":  string(output),
		"success": true,
	}, nil
}

// GitLogTool shows commit history
type GitLogTool struct {
	BaseTool
	workingDir string
}

// NewGitLogTool creates a git log tool
func NewGitLogTool(workingDir string) *GitLogTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of commits to show (default: 10)",
			},
			"oneline": map[string]interface{}{
				"type":        "boolean",
				"description": "Show one line per commit (default: true)",
			},
			"file": map[string]interface{}{
				"type":        "string",
				"description": "Show commits affecting specific file",
			},
		},
	}

	return &GitLogTool{
		BaseTool: NewBaseTool(
			"git_log",
			"Show commit history.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitLogTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	count := 10
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	oneline := true
	if o, ok := args["oneline"].(bool); ok {
		oneline = o
	}

	cmdArgs := []string{"log", fmt.Sprintf("-n%d", count)}

	if oneline {
		cmdArgs = append(cmdArgs, "--oneline")
	} else {
		cmdArgs = append(cmdArgs, "--format=%H|%an|%ar|%s")
	}

	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git log failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	// Parse commits
	var commits []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if oneline {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) >= 2 {
				commits = append(commits, map[string]interface{}{
					"hash":    parts[0],
					"message": parts[1],
				})
			}
		} else {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) >= 4 {
				commits = append(commits, map[string]interface{}{
					"hash":    parts[0],
					"author":  parts[1],
					"date":    parts[2],
					"message": parts[3],
				})
			}
		}
	}

	return map[string]interface{}{
		"commits": commits,
		"count":   len(commits),
		"success": true,
	}, nil
}

// GitAddTool stages files
type GitAddTool struct {
	BaseTool
	workingDir string
}

// NewGitAddTool creates a git add tool
func NewGitAddTool(workingDir string) *GitAddTool {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"files": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Files to stage (use ['.'] for all)",
			},
			"all": map[string]interface{}{
				"type":        "boolean",
				"description": "Stage all changes (-A)",
			},
		},
	}

	return &GitAddTool{
		BaseTool: NewBaseTool(
			"git_add",
			"Stage files for commit.",
			params,
		),
		workingDir: workingDir,
	}
}

func (t *GitAddTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	cmdArgs := []string{"add"}

	if all, ok := args["all"].(bool); ok && all {
		cmdArgs = append(cmdArgs, "-A")
	} else if files, ok := args["files"].([]interface{}); ok && len(files) > 0 {
		for _, f := range files {
			if s, ok := f.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	} else {
		return map[string]interface{}{
			"error":   "either 'files' or 'all: true' is required",
			"success": false,
		}, nil
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   fmt.Sprintf("git add failed: %v", err),
			"output":  string(output),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"output":  string(output),
		"success": true,
	}, nil
}
