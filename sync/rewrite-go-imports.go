//go:build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("root", "", "directory containing Go files")
	oldPrefix := flag.String("old", "", "old import path prefix")
	newPrefix := flag.String("new", "", "new import path prefix")
	flag.Parse()

	if *root == "" || *oldPrefix == "" || *newPrefix == "" {
		return fmt.Errorf("usage: go run sync/rewrite-go-imports.go -root <dir> -old <old-prefix> -new <new-prefix>")
	}

	return filepath.WalkDir(*root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		if err := rewriteFile(path, *oldPrefix, *newPrefix); err != nil {
			return fmt.Errorf("rewrite %s: %w", path, err)
		}
		return nil
	})
}

func rewriteFile(path, oldPrefix, newPrefix string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	changed := false
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return err
		}
		if importPath != oldPrefix && !strings.HasPrefix(importPath, oldPrefix+"/") {
			continue
		}
		spec.Path.Value = strconv.Quote(newPrefix + strings.TrimPrefix(importPath, oldPrefix))
		changed = true
	}
	if !changed {
		return nil
	}

	var out bytes.Buffer
	if err := format.Node(&out, fset, file); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}
