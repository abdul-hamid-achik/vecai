package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/abdul-hamid-achik/vecai/internal/llm"
	"github.com/abdul-hamid-achik/vecai/internal/tools"
)

const defaultMaxConcurrency = 4

type parallelExecutor struct {
	registry       *tools.Registry
	maxConcurrency int
}

func newParallelExecutor(registry *tools.Registry, maxConcurrency int) *parallelExecutor {
	if maxConcurrency <= 0 {
		maxConcurrency = defaultMaxConcurrency
	}
	return &parallelExecutor{
		registry:       registry,
		maxConcurrency: maxConcurrency,
	}
}

// executeParallel runs tool calls with sequential permission checks
// then concurrent execution. Results are returned in the same order as toolCalls.
func (pe *parallelExecutor) executeParallel(
	ctx context.Context,
	toolCalls []llm.ToolCall,
	checkPermission func(name string) (bool, error),
	onResult func(name, result string, err error),
) []toolResult {
	// Phase 1: Sequential permission checks
	approved := make([]bool, len(toolCalls))
	for i, tc := range toolCalls {
		allowed, _ := checkPermission(tc.Name)
		approved[i] = allowed
	}

	// Phase 2: Concurrent execution with semaphore
	results := make([]toolResult, len(toolCalls))
	sem := make(chan struct{}, pe.maxConcurrency)
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		if !approved[i] {
			results[i] = toolResult{Name: tc.Name, Result: "Permission denied", Error: true}
			continue
		}

		wg.Add(1)
		go func(idx int, call llm.ToolCall) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := pe.registry.Execute(ctx, call.Name, call.Input)
			if err != nil {
				results[idx] = toolResult{Name: call.Name, Result: "Error: " + err.Error(), Error: true}
			} else {
				results[idx] = toolResult{Name: call.Name, Result: result}
			}
			if onResult != nil {
				var execErr error
				if results[idx].Error {
					execErr = fmt.Errorf("%s", results[idx].Result)
				}
				onResult(call.Name, results[idx].Result, execErr)
			}
		}(i, tc)
	}
	wg.Wait()
	return results
}
