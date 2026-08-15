// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const tfplugindocsPackage = "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"

var (
	sectionKinds = map[string]string{
		"generating missing resource content":           "resource",
		"generating missing data source content":        "data source",
		"generating missing function content":           "function",
		"generating missing ephemeral resource content": "ephemeral resource",
		"generating missing action content":             "action",
		"generating missing list resource content":      "list resource",
		"generating missing state store content":        "state store",
		"generating missing provider content":           "provider",
	}
	generatedTemplatePattern = regexp.MustCompile(`^generating new template for (?:data-source |function )?("(?:[^"\\]|\\.)*")$`)
	fallbackTemplatePattern  = regexp.MustCompile(`^(resource|data-source|function|ephemeral resource|action|list resource|state store) ("(?:[^"\\]|\\.)*") fallback template exists, creating template$`)
	explicitKinds            = map[string]string{
		"resource":           "resource",
		"data-source":        "data source",
		"function":           "function",
		"ephemeral resource": "ephemeral resource",
		"action":             "action",
		"list resource":      "list resource",
		"state store":        "state store",
	}
)

type missingTemplate struct {
	Kind string
	Name string
}

type outputInspector struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	kind    string
	missing []missingTemplate
}

func (i *outputInspector) Write(p []byte) (int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	_, _ = i.buffer.Write(p)
	i.consumeLines(false)
	return len(p), nil
}

func (i *outputInspector) finish() []missingTemplate {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.consumeLines(true)
	missing := append([]missingTemplate(nil), i.missing...)
	sort.Slice(missing, func(a, b int) bool {
		if missing[a].Kind == missing[b].Kind {
			return missing[a].Name < missing[b].Name
		}
		return missing[a].Kind < missing[b].Kind
	})
	return missing
}

func (i *outputInspector) consumeLines(includePartial bool) {
	for {
		line, err := i.buffer.ReadString('\n')
		if errors.Is(err, io.EOF) && !includePartial {
			i.buffer.WriteString(line)
			return
		}
		if line != "" {
			i.inspectLine(strings.TrimSpace(line))
		}
		if errors.Is(err, io.EOF) {
			return
		}
	}
}

func (i *outputInspector) inspectLine(line string) {
	if kind, ok := sectionKinds[line]; ok {
		i.kind = kind
		return
	}
	if line == "rendering static website" {
		i.kind = ""
		return
	}

	if matches := fallbackTemplatePattern.FindStringSubmatch(line); len(matches) == 3 {
		i.addMissing(explicitKinds[matches[1]], matches[2])
		return
	}

	matches := generatedTemplatePattern.FindStringSubmatch(line)
	if len(matches) != 2 || i.kind == "" {
		return
	}
	i.addMissing(i.kind, matches[1])
}

func (i *outputInspector) addMissing(kind, quotedName string) {
	name, err := strconv.Unquote(quotedName)
	if err != nil {
		return
	}
	i.missing = append(i.missing, missingTemplate{Kind: kind, Name: name})
}

type commandRunner func(context.Context, io.Writer, io.Writer) error

func run(ctx context.Context, stdout, stderr io.Writer, runner commandRunner) error {
	inspector := &outputInspector{}
	if err := runner(ctx, io.MultiWriter(stdout, inspector), io.MultiWriter(stderr, inspector)); err != nil {
		return fmt.Errorf("tfplugindocs failed: %w", err)
	}

	missing := inspector.finish()
	if len(missing) == 0 {
		return nil
	}

	var message strings.Builder
	message.WriteString("tfplugindocs generated fallback templates; add checked-in templates for:")
	for _, template := range missing {
		fmt.Fprintf(&message, "\n- %s %q", template.Kind, template.Name)
	}
	return errors.New(message.String())
}

func tfplugindocsRunner(args []string) commandRunner {
	return func(ctx context.Context, stdout, stderr io.Writer) error {
		commandArgs := append([]string{"run", tfplugindocsPackage}, args...)
		command := exec.CommandContext(ctx, "go", commandArgs...)
		command.Stdout = stdout
		command.Stderr = stderr
		return command.Run()
	}
}

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr, tfplugindocsRunner(os.Args[1:])); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
