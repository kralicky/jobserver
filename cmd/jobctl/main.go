package main

import (
	"github.com/kralicky/jobserver/pkg/cli/jobctl"
	_ "github.com/kralicky/jobserver/pkg/logger"
)

func main() {
	jobctl.Execute()
}
