package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gitpod-io/gitpod-next/api/public-clients/go/sdk"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
)

func main() {
	if err := run(); err != nil {
		var authErr *sdk.AuthenticationRequiredError
		if errors.Is(err, sdk.ErrMissingAPIKey) {
			log.Fatalf("set %s to run this example", sdk.APIKeyEnvVar)
		}
		if errors.As(err, &authErr) {
			log.Fatalf("%v\nAuthenticate with github.com in Settings > Git authentications, then rerun this example.", authErr)
		}
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()

	logger := sdk.NewDebugLogger(os.Stderr)
	ona, err := sdk.NewFromEnv(sdk.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("create SDK: %w", err)
	}

	contextURL := "https://github.com/gitpod-io/template-golang-cli"
	env, err := ona.Environments().Create(ctx, sdk.CreateEnvironmentOptions{
		ContextURL: contextURL,
		Name:       "ona sdk Codex agent example",
	})
	if err != nil {
		return fmt.Errorf("create environment from %s: %w", contextURL, err)
	}
	environmentID := env.ID()
	defer func() {
		deleteOpts := sdk.DeleteEnvironmentOptions{}
		stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Minute)
		if err := ona.Environments().Stop(stopCtx, environmentID); err != nil {
			log.Printf("stop environment %s during cleanup: %v", environmentID, err)
			deleteOpts.Force = true
		}
		stopCancel()

		deleteCtx, deleteCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
		defer deleteCancel()
		if err := ona.Environments().Delete(deleteCtx, environmentID, deleteOpts); err != nil {
			log.Printf("delete environment %s during cleanup: %v", environmentID, err)
		}
	}()

	workspaceDir := environmentWorkspaceDir(env)
	if workspaceDir == "" {
		return fmt.Errorf("environment %s did not report a workspace directory", environmentID)
	}

	initialPrompt := fmt.Sprintf(`In the repository workspace at %s:

1. Inspect the repository layout.
2. Create a file named ona-codex-agent-notes.md in that workspace.
3. In that file, summarize what the repository contains and one small improvement you would make.
4. Finish without asking for confirmation.`, workspaceDir)

	agent, err := env.StartCodex(ctx, sdk.EnvironmentCodexOptions{
		Name:   "Create an SDK example note",
		Prompt: initialPrompt,
	})
	if err != nil {
		if agent != nil && agent.ID() != "" {
			fmt.Fprintf(os.Stderr, "agent_execution_id: %s\n", agent.ID())
			return fmt.Errorf("start Codex in environment %s with agent execution %s: %w", environmentID, agent.ID(), err)
		}
		return fmt.Errorf("start Codex in environment %s: %w", environmentID, err)
	}

	fmt.Fprintf(os.Stderr, "environment: %s\n", environmentID)
	fmt.Fprintf(os.Stderr, "agent_execution_id: %s\n", agent.ID())

	stream, err := agent.MessageStream(ctx)
	if err != nil {
		return fmt.Errorf("open Codex message stream %s: %w", agent.ID(), err)
	}
	streamDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(os.Stdout, stream)
		streamDone <- err
	}()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			message := scanner.Text()
			if strings.TrimSpace(message) == "" {
				continue
			}
			if err := agent.SendMessage(ctx, message); err != nil {
				log.Printf("send stdin message to Codex %s: %v", agent.ID(), err)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("read stdin for Codex %s: %v", agent.ID(), err)
		}
	}()

	lastLine := ""
	result, err := agent.WatchResult(ctx, func(_ context.Context, exec *v1.AgentExecution) error {
		line := sdk.AgentStatusLine(exec)
		if line == lastLine {
			return nil
		}
		fmt.Fprintf(os.Stderr, "agent update: %s\n", line)
		lastLine = line
		return nil
	})
	if err != nil {
		_ = stream.Close()
		return fmt.Errorf("watch Codex result %s: %w", agent.ID(), err)
	}
	_ = stream.Close()
	if err := <-streamDone; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.ErrClosedPipe) {
		return fmt.Errorf("read Codex message stream %s: %w", agent.ID(), err)
	}

	fmt.Fprintf(os.Stderr, "Codex finished: %s\n", sdk.AgentStatusLine(result))
	if result.GetStatus().GetPhase() == v1.AgentExecution_PHASE_WAITING_FOR_INPUT {
		fmt.Fprintln(os.Stderr, "Codex is waiting for user input.")
		return nil
	}

	changes, err := env.GitChanges(ctx, sdk.GitChangesOptions{Unified: 3})
	if err != nil {
		return fmt.Errorf("read git changes for environment %s: %w", environmentID, err)
	}

	status := changes.Status
	fmt.Fprintf(os.Stderr, "repository: %s\n", status.GetCloneUrl())
	fmt.Fprintf(os.Stderr, "branch: %s\n", status.GetBranch())
	fmt.Fprintf(os.Stderr, "changed_files: %d\n", status.GetTotalChangedFiles())
	for _, changed := range changes.Files {
		file := changed.File
		diff := changed.Diff
		fmt.Fprintf(os.Stderr, "%s %s hunks=%d binary=%t\n", file.GetChangeType().String(), file.GetPath(), len(diff.GetHunks()), diff.GetIsBinary())
	}
	return nil
}

func environmentWorkspaceDir(env *sdk.Environment) string {
	if env == nil || env.Proto() == nil {
		return ""
	}
	status := env.Proto().GetStatus()
	if workspaceDir := status.GetDevcontainer().GetRemoteWorkspaceFolder(); workspaceDir != "" {
		return workspaceDir
	}
	return status.GetContent().GetContentLocationInMachine()
}
