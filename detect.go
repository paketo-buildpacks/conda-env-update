package condaenvupdate

import (
	"path/filepath"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/fs"
)

// Detect returns a packit.DetectFunc that will be invoked during the
// detect phase of the buildpack lifecycle.
//
// Detection passes when there is an environment.yml or package-list.txt file
// in the app directory, and will contribute a Build Plan that provides
// conda-environment and requires conda.
func Detect() packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		envFile, err := fs.Exists(filepath.Join(context.WorkingDir, EnvironmentFileName))
		if err != nil {
			return packit.DetectResult{}, packit.Fail.WithMessage("failed trying to stat %s: %w", EnvironmentFileName, err)
		}
		lockFile, err := fs.Exists(filepath.Join(context.WorkingDir, LockfileName))
		if err != nil {
			return packit.DetectResult{}, packit.Fail.WithMessage("failed trying to stat %s: %w", LockfileName, err)
		}

		if !envFile && !lockFile {
			return packit.DetectResult{}, packit.Fail.WithMessage("no 'environment.yml' and 'package-list.txt' found")
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: CondaEnvPlanEntry},
				},
				Requires: []packit.BuildPlanRequirement{
					{
						Name: CondaPlanEntry,
						Metadata: map[string]interface{}{
							"build": true,
						},
					},
				},
			},
		}, nil
	}
}
