package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/vecai/internal/config"
)

// handleModelsCommand handles the "models" subcommand
func handleModelsCommand(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return modelsHelp()
	}

	switch args[0] {
	case "list":
		return modelsList(cfg)
	case "test":
		return modelsTest(cfg)
	case "pull":
		return modelsPull(cfg)
	case "help", "--help", "-h":
		return modelsHelp()
	default:
		return fmt.Errorf("unknown models subcommand: %s. Use 'vecai models help' for usage", args[0])
	}
}

// modelsHelp shows help for the models subcommand
func modelsHelp() error {
	fmt.Print(`vecai models - Manage Ollama models

Usage:
  vecai models list       Show configured model tiers
  vecai models test       Benchmark each configured tier
  vecai models pull       Pull all configured models from Ollama
  vecai models help       Show this help

Examples:
  vecai models list       # Show current tier configuration
  vecai models test       # Test response time for each tier
  vecai models pull       # Download all models needed by vecai
`)
	return nil
}

// modelsList shows the configured model tiers
func modelsList(cfg *config.Config) error {
	fmt.Println("Configured Model Tiers:")
	fmt.Println()
	fmt.Printf("  fast:   %s\n", cfg.Ollama.ModelFast)
	fmt.Printf("  smart:  %s\n", cfg.Ollama.ModelSmart)
	fmt.Printf("  genius: %s\n", cfg.Ollama.ModelGenius)
	fmt.Println()
	fmt.Printf("Default tier: %s\n", cfg.DefaultTier)
	fmt.Printf("Ollama URL:   %s\n", cfg.Ollama.BaseURL)
	fmt.Printf("Keep alive:   %s\n", cfg.Ollama.KeepAlive)
	fmt.Println()

	// Check which models are available locally
	fmt.Println("Local availability:")
	models := []struct {
		tier  string
		model string
	}{
		{"fast", cfg.Ollama.ModelFast},
		{"smart", cfg.Ollama.ModelSmart},
		{"genius", cfg.Ollama.ModelGenius},
	}

	for _, m := range models {
		available := checkModelAvailable(cfg.Ollama.BaseURL, m.model)
		status := "not found"
		if available {
			status = "available"
		}
		fmt.Printf("  %s (%s): %s\n", m.tier, m.model, status)
	}

	return nil
}

// modelsTest benchmarks each configured tier
func modelsTest(cfg *config.Config) error {
	fmt.Println("Testing model tiers (simple prompt: 'What is 2+2?')...")
	fmt.Println()

	models := []struct {
		tier  string
		model string
	}{
		{"fast", cfg.Ollama.ModelFast},
		{"smart", cfg.Ollama.ModelSmart},
		{"genius", cfg.Ollama.ModelGenius},
	}

	for _, m := range models {
		fmt.Printf("Testing %s (%s)... ", m.tier, m.model)

		start := time.Now()
		resp, err := testModel(cfg.Ollama.BaseURL, m.model, "What is 2+2? Answer with just the number.")
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}

		// Truncate response for display
		respTrimmed := strings.TrimSpace(resp)
		if len(respTrimmed) > 50 {
			respTrimmed = respTrimmed[:47] + "..."
		}

		fmt.Printf("%.2fs - \"%s\"\n", elapsed.Seconds(), respTrimmed)
	}

	return nil
}

// modelsPull pulls all configured models from Ollama
func modelsPull(cfg *config.Config) error {
	models := []string{
		cfg.Ollama.ModelFast,
		cfg.Ollama.ModelSmart,
		cfg.Ollama.ModelGenius,
	}

	// Deduplicate in case same model is used for multiple tiers
	seen := make(map[string]bool)
	var unique []string
	for _, m := range models {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}

	fmt.Printf("Pulling %d models from Ollama...\n\n", len(unique))

	for _, model := range unique {
		fmt.Printf("Pulling %s...\n", model)

		cmd := exec.Command("ollama", "pull", model)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("  ERROR: Failed to pull %s: %v\n", model, err)
			continue
		}

		fmt.Printf("  Done: %s\n\n", model)
	}

	fmt.Println("All models pulled successfully!")
	return nil
}

// checkModelAvailable checks if a model is available locally in Ollama
func checkModelAvailable(baseURL, model string) bool {
	resp, err := http.Get(baseURL + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	// Check if model is in the list (handle both "model" and "model:tag" formats)
	for _, m := range result.Models {
		if m.Name == model || strings.HasPrefix(m.Name, model+":") || model == strings.Split(m.Name, ":")[0] {
			return true
		}
	}

	return false
}

// testModel sends a test prompt to Ollama and returns the response
func testModel(baseURL, model, prompt string) (string, error) {
	reqBody := map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}

	reqBytes, _ := json.Marshal(reqBody)
	resp, err := http.Post(baseURL+"/api/generate", "application/json", strings.NewReader(string(reqBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Response, nil
}
