package main

import (
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	condaenvupdate "github.com/paketo-buildpacks/conda-env-update"
)

func main() {
	logger := scribe.NewLogger(os.Stdout)
	summer := fs.NewChecksumCalculator()
	condaRunner := condaenvupdate.NewCondaRunner(pexec.NewExecutable("conda"), summer, logger)
	planner := draft.NewPlanner()

	packit.Run(condaenvupdate.Detect(),
		condaenvupdate.Build(planner, condaRunner, logger, chronos.DefaultClock),
	)
}
