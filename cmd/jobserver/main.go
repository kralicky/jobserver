package main

import (
	"github.com/kralicky/jobserver/pkg/cli/jobserver"

	_ "github.com/kralicky/jobserver/pkg/cgroups/cgroupsv1"
	_ "github.com/kralicky/jobserver/pkg/cgroups/cgroupsv2"
	_ "github.com/kralicky/jobserver/pkg/logger"
)

func main() {
	jobserver.Execute()
}
