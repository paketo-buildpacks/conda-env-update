package condaenvupdate

const (
	// CondaEnvLayer is the name of the layer into which conda environment is installed.
	CondaEnvLayer = "conda-env"

	// CondaEnvCache is the name of the layer that is used as the conda package directory.
	CondaEnvCache = "conda-env-cache"

	// CondaEnvPlanEntry is the name of the Build Plan requirement that this buildpack provides.
	CondaEnvPlanEntry = "conda-environment"

	// CondaPlanEntry is the name of the Build Plan requirement for the miniconda
	// dependency that this buildpack requires.
	CondaPlanEntry = "conda"

	// LockfileShaName is the key in the Layer Content Metadata used to determine if layer
	// can be reused.
	LockfileShaName = "lockfile-sha"

	// LockfileName is the name of the export file from which the buildpack reinstalls packages
	// See https://docs.conda.io/projects/conda/en/latest/commands/list.html
	LockfileName = "package-list.txt"

	// EnvironmentFileName is the name of the conda environment file.
	EnvironmentFileName = "environment.yml"
)
