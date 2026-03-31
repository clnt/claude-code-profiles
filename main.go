package main

import "github.com/clnt/claude-code-profiles/cmd"

var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
