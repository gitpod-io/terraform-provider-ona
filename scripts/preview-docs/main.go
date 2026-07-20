package main

import (
	"flag"
	"fmt"
	"html"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/fsnotify/fsnotify"
)

type page struct {
	Section string
	Title   string
	Source  string
	Link    string
}

type watchPaths []string

func (paths *watchPaths) String() string {
	return strings.Join(*paths, ",")
}

func (paths *watchPaths) Set(path string) error {
	if path == "" {
		return fmt.Errorf("watch path must not be empty")
	}
	*paths = append(*paths, path)
	return nil
}

func main() {
	providerDir, err := providerDir()
	if err != nil {
		exit(err)
	}

	host := flag.String("host", envDefault("HOST", "127.0.0.1"), "host to bind")
	port := flag.Int("port", envIntDefault("PORT", 8808), "port to bind")
	siteDir := flag.String("site-dir", envDefault("SITE_DIR", filepath.Join(providerDir, ".tmp", "docs-preview")), "rendered site directory")
	skipGenerate := flag.Bool("skip-generate", os.Getenv("SKIP_GENERATE") == "1", "render existing docs without regenerating them")
	noServe := flag.Bool("no-serve", os.Getenv("NO_SERVE") == "1", "render the preview site and exit without starting a server")
	var watchedPaths watchPaths
	flag.Var(&watchedPaths, "watch", "path to watch for preview updates; repeat this flag for multiple paths")
	flag.Parse()

	if !*skipGenerate {
		if err := generateDocs(providerDir); err != nil {
			exit(err)
		}
	}
	if err := validateDocs(providerDir); err != nil {
		exit(err)
	}
	if err := renderSite(providerDir, *siteDir); err != nil {
		exit(err)
	}

	if *noServe {
		fmt.Printf("Rendered Terraform provider docs preview to %s\n", *siteDir)
		return
	}
	if len(watchedPaths) > 0 {
		paths, err := resolveWatchPaths(providerDir, watchedPaths)
		if err != nil {
			exit(err)
		}
		go func() {
			if err := watchDocs(paths, func() error {
				return renderSite(providerDir, *siteDir)
			}); err != nil {
				fmt.Fprintf(os.Stderr, "watch docs preview: %v\n", err)
			}
		}()
		fmt.Printf("Watching docs paths: %s\n", strings.Join(paths, ", "))
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("Terraform provider docs preview: http://%s/\n", addr)
	fmt.Println("Press Ctrl-C to stop.")
	if err := http.ListenAndServe(addr, http.FileServer(http.Dir(*siteDir))); err != nil {
		exit(fmt.Errorf("serve docs preview: %w", err))
	}
}

func resolveWatchPaths(providerDir string, paths []string) ([]string, error) {
	result := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(providerDir, path)
		}
		path = filepath.Clean(path)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("watch path %s: %w", path, err)
		}
		if !info.IsDir() {
			path = filepath.Dir(path)
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result, nil
}

func watchDocs(paths []string, render func() error) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create file watcher: %w", err)
	}
	defer watcher.Close()

	for _, path := range paths {
		if err := addWatchDirectories(watcher, path); err != nil {
			return err
		}
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := addWatchDirectories(watcher, event.Name); err != nil {
						return err
					}
				}
			}
			if event.Has(fsnotify.Write | fsnotify.Create | fsnotify.Remove | fsnotify.Rename) {
				if err := render(); err != nil {
					fmt.Fprintf(os.Stderr, "rerender docs preview: %v\n", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watch docs: %w", err)
		}
	}
}

func addWatchDirectories(watcher *fsnotify.Watcher, path string) error {
	return filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if err := watcher.Add(path); err != nil {
			return fmt.Errorf("watch directory %s: %w", path, err)
		}
		return nil
	})
}

func providerDir() (string, error) {
	_, currentFile, _, ok := runtimeCaller()
	if !ok {
		return "", fmt.Errorf("resolve current file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..")), nil
}

var runtimeCaller = func() (uintptr, string, int, bool) {
	return caller(0)
}

func caller(skip int) (uintptr, string, int, bool) {
	return runtime.Caller(skip + 1)
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envIntDefault(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func generateDocs(providerDir string) error {
	if err := run(providerDir, nil, "go", "generate", "."); err != nil {
		return err
	}
	env := os.Environ()
	env = append(env, "GOWORK=off")
	return run(filepath.Join(providerDir, "tools"), env, "go", "generate", "./...")
}

func validateDocs(providerDir string) error {
	env := os.Environ()
	env = append(env, "GOWORK=off")
	return run(
		filepath.Join(providerDir, "tools"),
		env,
		"go",
		"run",
		"github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs",
		"validate",
		"--provider-dir",
		"..",
		"--provider-name",
		"ona",
	)
}

func run(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func renderSite(providerDir, siteDir string) error {
	docsDir := filepath.Join(providerDir, "docs")
	if _, err := os.Stat(docsDir); err != nil {
		return fmt.Errorf("read docs directory: %w", err)
	}
	if err := os.RemoveAll(siteDir); err != nil {
		return fmt.Errorf("clear preview directory: %w", err)
	}
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		return fmt.Errorf("create preview directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "style.css"), []byte(stylesheet), 0o644); err != nil {
		return fmt.Errorf("write stylesheet: %w", err)
	}

	pages, err := docPages(docsDir)
	if err != nil {
		return err
	}
	for _, p := range pages {
		sourcePath := filepath.Join(docsDir, p.Source)
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", sourcePath, err)
		}
		body := renderMarkdown(string(content))
		target := filepath.Join(siteDir, strings.TrimSuffix(p.Link, ".html")+".html")
		if p.Link == "index.html" {
			target = filepath.Join(siteDir, "index.html")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create page directory: %w", err)
		}
		document := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <link rel="stylesheet" href="/style.css">
</head>
<body>
  <div class="layout">
    %s
    <main class="content">
      %s
    </main>
  </div>
</body>
</html>
`, html.EscapeString(p.Title), renderNav(pages, p.Source), body)
		if err := os.WriteFile(target, []byte(document), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}
	return nil
}

func docPages(docsDir string) ([]page, error) {
	sections := []struct {
		name  string
		paths []string
	}{
		{name: "Provider", paths: []string{"index.md"}},
		{name: "Guides", paths: globDocs(docsDir, "guides", "*.md")},
		{name: "Resources", paths: globDocs(docsDir, "resources", "*.md")},
		{name: "Data Sources", paths: globDocs(docsDir, "data-sources", "*.md")},
		{name: "Ephemeral Resources", paths: globDocs(docsDir, "ephemeral-resources", "*.md")},
		{name: "List Resources", paths: globDocs(docsDir, "list-resources", "*.md")},
	}

	var pages []page
	for _, section := range sections {
		for _, source := range section.paths {
			sourcePath := filepath.Join(docsDir, source)
			if _, err := os.Stat(sourcePath); err != nil {
				continue
			}
			title, err := pageTitle(sourcePath)
			if err != nil {
				return nil, err
			}
			pages = append(pages, page{
				Section: section.name,
				Title:   title,
				Source:  filepath.ToSlash(source),
				Link:    linkPath(source),
			})
		}
	}
	return pages, nil
}

func globDocs(docsDir string, parts ...string) []string {
	pattern := filepath.Join(append([]string{docsDir}, parts...)...)
	matches, _ := filepath.Glob(pattern)
	sort.Strings(matches)
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(docsDir, match)
		if err == nil {
			result = append(result, rel)
		}
	}
	return result
}

func pageTitle(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	for _, line := range strings.Split(stripFrontmatter(string(content)), "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), nil
		}
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return titleWords(name), nil
}

func titleWords(value string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range value {
		if unicode.IsSpace(r) {
			capitalizeNext = true
			result.WriteRune(r)
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func linkPath(source string) string {
	source = filepath.ToSlash(source)
	if source == "index.md" {
		return "index.html"
	}
	return strings.TrimSuffix(source, ".md") + ".html"
}

func renderNav(pages []page, activeSource string) string {
	var out strings.Builder
	out.WriteString(`<nav class="sidebar"><div class="brand">Ona Provider</div>`)
	currentSection := ""
	for i, p := range pages {
		if p.Section != currentSection {
			if i > 0 {
				out.WriteString("</ul>")
			}
			currentSection = p.Section
			out.WriteString("<h2>")
			out.WriteString(html.EscapeString(p.Section))
			out.WriteString("</h2><ul>")
		}
		active := ""
		if p.Source == activeSource {
			active = " active"
		}
		out.WriteString(`<li><a class="`)
		out.WriteString(active)
		out.WriteString(`" href="/`)
		out.WriteString(html.EscapeString(p.Link))
		out.WriteString(`">`)
		out.WriteString(html.EscapeString(p.Title))
		out.WriteString(`</a></li>`)
	}
	if len(pages) > 0 {
		out.WriteString("</ul>")
	}
	out.WriteString("</nav>")
	return out.String()
}

func stripFrontmatter(markdown string) string {
	if !strings.HasPrefix(markdown, "---\n") {
		return markdown
	}
	end := strings.Index(markdown[4:], "\n---\n")
	if end == -1 {
		return markdown
	}
	return strings.TrimLeft(markdown[end+9:], "\n")
}

var headingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
var linkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
var codePattern = regexp.MustCompile("`([^`]+)`")
var anchorPattern = regexp.MustCompile(`[^a-z0-9]+`)

func renderMarkdown(markdown string) string {
	lines := strings.Split(stripFrontmatter(markdown), "\n")
	var out strings.Builder
	var paragraph []string
	listOpen := false
	codeOpen := false
	commentOpen := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		out.WriteString("<p>")
		out.WriteString(inlineMarkdown(strings.Join(paragraph, " ")))
		out.WriteString("</p>\n")
		paragraph = nil
	}
	closeList := func() {
		if listOpen {
			out.WriteString("</ul>\n")
			listOpen = false
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			flushParagraph()
			closeList()
			if codeOpen {
				out.WriteString("</code></pre>\n")
				codeOpen = false
				continue
			}
			language := html.EscapeString(strings.TrimSpace(strings.TrimPrefix(line, "```")))
			className := ""
			if language != "" {
				className = ` class="language-` + language + `"`
			}
			out.WriteString("<pre><code")
			out.WriteString(className)
			out.WriteString(">")
			codeOpen = true
			continue
		}
		if codeOpen {
			out.WriteString(html.EscapeString(line))
			out.WriteByte('\n')
			continue
		}
		if strings.HasPrefix(line, "<!--") {
			commentOpen = !strings.HasSuffix(line, "-->")
			continue
		}
		if commentOpen {
			commentOpen = !strings.HasSuffix(line, "-->")
			continue
		}
		if strings.HasPrefix(line, "<a id=") {
			flushParagraph()
			closeList()
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if matches := headingPattern.FindStringSubmatch(line); matches != nil {
			flushParagraph()
			closeList()
			level := len(matches[1])
			text := strings.TrimSpace(matches[2])
			anchor := anchorPattern.ReplaceAllString(strings.ToLower(text), "-")
			anchor = strings.Trim(anchor, "-")
			fmt.Fprintf(&out, `<h%d id="%s">%s</h%d>`+"\n", level, html.EscapeString(anchor), inlineMarkdown(text), level)
			continue
		}
		if strings.HasPrefix(line, "- ") {
			flushParagraph()
			if !listOpen {
				out.WriteString("<ul>\n")
				listOpen = true
			}
			out.WriteString("<li>")
			out.WriteString(inlineMarkdown(strings.TrimSpace(strings.TrimPrefix(line, "- "))))
			out.WriteString("</li>\n")
			continue
		}
		if strings.TrimSpace(line) == "" {
			flushParagraph()
			closeList()
			continue
		}
		paragraph = append(paragraph, strings.TrimSpace(line))
	}

	flushParagraph()
	closeList()
	if codeOpen {
		out.WriteString("</code></pre>\n")
	}
	return out.String()
}

func inlineMarkdown(text string) string {
	escaped := html.EscapeString(text)
	escaped = codePattern.ReplaceAllString(escaped, `<code>$1</code>`)
	escaped = linkPattern.ReplaceAllString(escaped, `<a href="$2">$1</a>`)
	return escaped
}

func exit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

const stylesheet = `
:root {
  color-scheme: light;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  color: #1f2933;
  background: #f7f8fa;
}
body { margin: 0; }
a { color: #2457a6; text-decoration: none; }
a:hover { text-decoration: underline; }
.layout {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  min-height: 100vh;
}
.sidebar {
  background: #ffffff;
  border-right: 1px solid #d8dde6;
  padding: 24px;
}
.brand {
  font-size: 18px;
  font-weight: 700;
  margin-bottom: 24px;
}
.sidebar h2 {
  color: #637083;
  font-size: 12px;
  letter-spacing: 0;
  margin: 24px 0 8px;
  text-transform: uppercase;
}
.sidebar ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
.sidebar li { margin: 2px 0; }
.sidebar a {
  border-radius: 6px;
  color: #334155;
  display: block;
  padding: 7px 8px;
}
.sidebar a.active {
  background: #edf2ff;
  color: #123f8c;
  font-weight: 600;
}
.content {
  max-width: 920px;
  padding: 40px 56px 80px;
}
h1 {
  font-size: 32px;
  line-height: 1.2;
  margin: 0 0 24px;
}
h2 {
  border-top: 1px solid #d8dde6;
  font-size: 22px;
  margin: 36px 0 16px;
  padding-top: 28px;
}
h3 {
  font-size: 18px;
  margin: 28px 0 12px;
}
p, li {
  font-size: 15px;
  line-height: 1.6;
}
code {
  background: #eef1f5;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.92em;
  padding: 2px 4px;
}
pre {
  background: #18202f;
  border-radius: 8px;
  color: #edf2f7;
  overflow-x: auto;
  padding: 16px;
}
pre code {
  background: transparent;
  color: inherit;
  padding: 0;
}
@media (max-width: 860px) {
  .layout { grid-template-columns: 1fr; }
  .sidebar {
    border-bottom: 1px solid #d8dde6;
    border-right: 0;
  }
  .content { padding: 28px 20px 56px; }
}
`
