package main

import (
	"os"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/draft"
	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
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
