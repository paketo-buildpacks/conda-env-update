package condaenvupdate

import (
	"os"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/scribe"
)

//go:generate faux --interface Planner --output fakes/planner.go

// Planner defines the interface for using the build and launch requirements from incoming Buildpack Plan entries
// to determine whether a given layer should be available at build- and launch-time.
type Planner interface {
	MergeLayerTypes(string, []packit.BuildpackPlanEntry) (launch bool, build bool)
}

//go:generate faux --interface Runner --output fakes/runner.go

// Runner defines the interface for setting up the conda environment.
type Runner interface {
	Execute(condaEnvPath string, condaCachePath string, workingDir string) error
	ShouldRun(workingDir string, metadata map[string]interface{}) (bool, string, error)
}

// Build will return a packit.BuildFunc that will be invoked during the build
// phase of the buildpack lifecycle.
//
// Build updates the conda environment and stores the result in a layer. It may
// reuse the environment layer from a previous build, depending on conditions
// determined by the runner.
func Build(planner Planner, runner Runner, logger scribe.Logger, clock chronos.Clock) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		condaLayer, err := context.Layers.Get(CondaEnvLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		condaCacheLayer, err := context.Layers.Get(CondaEnvCache)
		if err != nil {
			return packit.BuildResult{}, err
		}

		run, sha, err := runner.ShouldRun(context.WorkingDir, condaLayer.Metadata)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if run {
			condaLayer, err = condaLayer.Reset()
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Process("Executing build process")

			duration, err := clock.Measure(func() error {
				return runner.Execute(condaLayer.Path, condaCacheLayer.Path, context.WorkingDir)
			})
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Action("Completed in %s", duration.Round(time.Millisecond))
			logger.Break()

			condaLayer.Metadata = map[string]interface{}{
				"built_at":     clock.Now().Format(time.RFC3339Nano),
				"lockfile-sha": sha,
			}
		} else {
			logger.Process("Reusing cached layer %s", condaLayer.Path)
			logger.Break()
		}

		condaLayer.Launch, condaLayer.Build = planner.MergeLayerTypes(CondaEnvPlanEntry, context.Plan.Entries)
		condaLayer.Cache = condaLayer.Build
		condaCacheLayer.Cache = true

		layers := []packit.Layer{condaLayer}
		if _, err := os.Stat(condaCacheLayer.Path); err == nil {
			if !fs.IsEmptyDir(condaCacheLayer.Path) {
				layers = append(layers, condaCacheLayer)
			}
		}

		return packit.BuildResult{
			Layers: layers,
		}, nil
	}
}
