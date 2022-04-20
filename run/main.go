package main

import (
	"os"

	condaenvupdate "github.com/paketo-buildpacks/conda-env-update"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

type Generator struct{}

func (f Generator) Generate(dir string) (sbom.SBOM, error) {
	return sbom.Generate(dir)
}

func main() {
	logger := scribe.NewEmitter(os.Stdout).WithLevel(os.Getenv("BP_LOG_LEVEL"))

	packit.Run(
		condaenvupdate.Detect(),
		condaenvupdate.Build(
			draft.NewPlanner(),
			condaenvupdate.NewCondaRunner(pexec.NewExecutable("conda"), fs.NewChecksumCalculator(), logger),
			Generator{},
			logger,
			chronos.DefaultClock),
	)
}
