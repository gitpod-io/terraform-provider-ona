package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/testing/replayproxy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var opts replayproxy.Options
	listenAddr := flag.String("listen", "127.0.0.1:0", "address the replay proxy listens on")
	flag.StringVar(&opts.Mode, "mode", replayproxy.ModeReplay, "proxy mode: record or replay")
	flag.StringVar(&opts.FixtureDir, "fixture-dir", "", "fixture directory")
	flag.StringVar(&opts.Scenario, "scenario", "", "scenario name written during record mode")
	flag.StringVar(&opts.ExpectedLanguage, "language", replayproxy.LanguageGo, "expected SDK language: go, typescript, or python")
	flag.StringVar(&opts.UpstreamBaseURL, "upstream", "", "upstream management-plane base URL for record mode")
	flag.StringVar(&opts.PublicURL, "public-url", "", "base URL SDK examples should use for this proxy")
	validateOnly := flag.Bool("validate-fixture", false, "validate the fixture and exit")
	flag.Parse()

	if *validateOnly {
		return replayproxy.ValidateFixture(opts.FixtureDir)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return replayproxy.ListenAndServe(ctx, *listenAddr, opts)
}
