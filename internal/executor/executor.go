package executor

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Executor handles command execution with logging and error handling
type Executor struct {
	workDir string
	env     map[string]string
}

// New creates a new executor
func New(workDir string, env map[string]string) *Executor {
	if env == nil {
		env = make(map[string]string)
	}
	return &Executor{
		workDir: workDir,
		env:     env,
	}
}

// Run executes a command in the work directory with configured environment
func (e *Executor) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = e.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment
	cmd.Env = e.buildEnv()

	log.Printf("→ Running: %s %s", name, strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// RunSilent executes a command and returns output without logging to stdout
func (e *Executor) RunSilent(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = e.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment
	cmd.Env = e.buildEnv()

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %s %s: %w\nStderr: %s", name, strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

// RunShell executes a shell command with pipes and redirects
func (e *Executor) RunShell(script string) error {
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = e.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set environment
	cmd.Env = e.buildEnv()

	log.Printf("→ Running shell script")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell script failed: %w", err)
	}
	return nil
}

// SetEnv sets an environment variable
func (e *Executor) SetEnv(key, value string) {
	e.env[key] = value
}

// GetEnv gets an environment variable
func (e *Executor) GetEnv(key string) string {
	if val, ok := e.env[key]; ok {
		return val
	}
	return os.Getenv(key)
}

// buildEnv constructs the full environment for command execution
func (e *Executor) buildEnv() []string {
	env := os.Environ()
	envMap := make(map[string]string)

	// Parse current environment
	for _, pair := range env {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Overlay custom environment
	for k, v := range e.env {
		envMap[k] = v
	}

	// Convert back to string slice
	var result []string
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

// CheckCommand verifies that a required command exists
func CheckCommand(name string) error {
	_, err := exec.LookPath(name)
	return err
}
