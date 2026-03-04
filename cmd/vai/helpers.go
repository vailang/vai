package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vailang/vai/internal/config"
	"github.com/vailang/vai/internal/prompter"
)

// loadProject finds vai.toml, loads the config, and returns the config,
// the absolute config path, and the project base directory.
func loadProject() (cfg *config.Config, cfgPath string, baseDir string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", "", err
	}
	cfgPath, err = config.FindConfig(cwd)
	if err != nil {
		return nil, "", "", fmt.Errorf("no vai.toml found (run 'vai init <name>' to create one)")
	}
	cfg, err = config.LoadConfig(cfgPath)
	if err != nil {
		return nil, "", "", err
	}
	cfgPath, _ = filepath.Abs(cfgPath)
	baseDir = filepath.Dir(cfgPath)
	return cfg, cfgPath, baseDir, nil
}

// prompterOpts configures a prompter run.
type prompterOpts struct {
	cfg          *config.Config
	baseDir      string
	userPrompt   string
	systemPrompt string // optional
	verbose      bool
	autoYes      bool
	noChangeMsg  string // message when no changes (terminal mode)
	successMsg   string // message after flush (terminal mode)
}

// runPrompterFlow runs the prompter, displays results, confirms with the
// user, and flushes changes to disk.
func runPrompterFlow(ctx context.Context, opts prompterOpts) (*prompter.Result, error) {
	p, err := prompter.New(opts.cfg, opts.baseDir)
	if err != nil {
		return nil, err
	}
	if opts.systemPrompt != "" {
		p.SetSystemPrompt(opts.systemPrompt)
	}

	var (
		wg           sync.WaitGroup
		promptEvents chan prompter.Event
	)
	if !jsonFlag {
		promptEvents = make(chan prompter.Event, 32)
		p.SetEvents(promptEvents)
		display := prompter.NewDisplay(opts.verbose)
		wg.Add(1)
		go func() {
			defer wg.Done()
			display.Consume(promptEvents)
		}()
	}

	result, runErr := p.Run(ctx, opts.userPrompt)
	if promptEvents != nil {
		close(promptEvents)
		wg.Wait()
	}
	if runErr != nil {
		return nil, runErr
	}

	if len(result.Changes) == 0 {
		if jsonFlag {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return result, enc.Encode(map[string]any{
				"changes": []any{},
				"summary": result.Summary,
			})
		}
		msg := opts.noChangeMsg
		if msg == "" {
			msg = result.Summary
		}
		fmt.Println(msg)
		return result, nil
	}

	// Display changes.
	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return result, enc.Encode(map[string]any{
			"changes":    result.Changes,
			"summary":    result.Summary,
			"tokens_in":  result.TokensIn,
			"tokens_out": result.TokensOut,
		})
	}

	prompter.DisplayChanges(result.Changes, os.Stdout)

	// Confirm unless autoYes.
	if !opts.autoYes {
		if !prompter.Confirm(os.Stdin, os.Stdout) {
			fmt.Println("Aborted.")
			return result, nil
		}
	}

	// Flush changes to disk.
	if err := result.Flush(opts.baseDir); err != nil {
		return result, fmt.Errorf("writing changes: %w", err)
	}

	if opts.successMsg != "" {
		fmt.Println(opts.successMsg)
	}

	return result, nil
}
