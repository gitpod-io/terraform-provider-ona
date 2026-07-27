package main

const providerSource = "registry.terraform.io/gitpod-io/ona"
const defaultOnaHost = "https://app.gitpod.io"

type config struct {
	host                 string
	token                string
	queryFile            string
	workdir              string
	providerDir          string
	terraform            string
	terraformParallelism int
	skipValidate         bool
}
