package condaenvupdate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface Executable --output fakes/executable.go

// Executable defines the interface for invoking an executable.
type Executable interface {
	Execute(pexec.Execution) error
}

// Summer defines the interface for computing a SHA256 for a set of files
// and/or directories.
//
//go:generate faux --interface Summer --output fakes/summer.go
type Summer interface {
	Sum(arg ...string) (string, error)
}

// CondaRunner implements the Runner interface.
type CondaRunner struct {
	logger     scribe.Emitter
	executable Executable
	summer     Summer
}

// NewCondaRunner creates an instance of CondaRunner given an Executable, a Summer, and a Logger.
func NewCondaRunner(executable Executable, summer Summer, logger scribe.Emitter) CondaRunner {
	return CondaRunner{
		executable: executable,
		summer:     summer,
		logger:     logger,
	}
}

// ShouldRun determines whether the conda environment setup command needs to be
// run, given the path to the app directory and the metadata from the
// preexisting conda-env layer. It returns true if the conda environment setup
// command must be run during this build, the SHA256 of the package-list.txt in
// the app directory, and an error. If there is no package-list.txt, the sha
// returned is an empty string.
func (c CondaRunner) ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error) {
	lockfilePath := filepath.Join(workingDir, LockfileName)
	_, err = os.Stat(lockfilePath)

	if errors.Is(err, os.ErrNotExist) {
		return true, "", nil
	}

	if err != nil {
		return false, "", err
	}

	updatedLockfileSha, err := c.summer.Sum(lockfilePath)
	if err != nil {
		return false, "", err
	}

	if updatedLockfileSha == metadata[LockfileShaName] {
		return false, updatedLockfileSha, nil
	}

	return true, updatedLockfileSha, nil
}

func (c CondaRunner) loadChannelContent(args []string) error {
	searchArgs := append([]string{"search", "--quiet"}, args...)
	c.logger.Subprocess("Running 'conda %s'", strings.Join(searchArgs, " "))

	err := c.executable.Execute(pexec.Execution{
		Args:   searchArgs,
		Stdout: c.logger.ActionWriter,
		Stderr: c.logger.ActionWriter,
	})
	if err != nil {
		return fmt.Errorf("failed to run conda command: %w", err)
	}
	return err
}

// Execute runs the conda environment setup command and cleans up unnecessary
// artifacts. If a vendor directory is present, it uses vendored packages and
// installs them in offline mode. If a packages-list.txt file is present, it creates a
// new environment based on the packages list. Otherwise, it updates the
// existing packages to their latest versions.
//
// For more information about the commands used, see:
// https://docs.conda.io/projects/conda/en/latest/commands/create.html
// https://docs.conda.io/projects/conda/en/latest/commands/update.html
// https://docs.conda.io/projects/conda/en/latest/commands/clean.html
func (c CondaRunner) Execute(condaLayerPath string, condaCachePath string, workingDir string) error {
	vendorDirExists, err := fs.Exists(filepath.Join(workingDir, "vendor"))
	if err != nil {
		return err
	}

	lockfileExists, err := fs.Exists(filepath.Join(workingDir, LockfileName))
	if err != nil {
		return err
	}

	historyFile := filepath.Join(condaLayerPath, "conda-meta", "history")

	args := []string{
		"create",
		"--file", filepath.Join(workingDir, LockfileName),
		"--prefix", condaLayerPath,
		"--yes",
		"--quiet",
	}

	if vendorDirExists {

		vendorArgs := []string{
			"--channel", filepath.Join(workingDir, "vendor"),
			"--override-channels",
			"--offline",
		}
		args = append(args, vendorArgs...)

		// Workaround the vendor channel content does not seem to be loaded
		err = c.loadChannelContent(vendorArgs)
		if err != nil {
			return err
		}

		c.logger.Subprocess("Running 'conda %s'", strings.Join(args, " "))

		err = c.executable.Execute(pexec.Execution{
			Args:   args,
			Stdout: c.logger.ActionWriter,
			Stderr: c.logger.ActionWriter,
		})
		if err != nil {
			return fmt.Errorf("failed to run conda command: %w", err)
		}

		c.logger.Subprocess("Removing %s", historyFile)
		err = os.RemoveAll(historyFile)
		if err != nil {
			return err
		}

		return nil
	}

	if !lockfileExists {
		args = []string{
			"env",
			"update",
			"--prefix", condaLayerPath,
			"--file", filepath.Join(workingDir, EnvironmentFileName),
		}
	}

	c.logger.Subprocess("Running 'CONDA_PKGS_DIRS=%s conda %s'", condaCachePath, strings.Join(args, " "))

	err = c.executable.Execute(pexec.Execution{
		Args:   args,
		Env:    append(os.Environ(), fmt.Sprintf("CONDA_PKGS_DIRS=%s", condaCachePath)),
		Stdout: c.logger.ActionWriter,
		Stderr: c.logger.ActionWriter,
	})

	if err != nil {
		return fmt.Errorf("failed to run conda command: %w", err)
	}

	args = []string{
		"clean",
		"--packages",
		"--tarballs",
	}

	c.logger.Subprocess("Running 'conda %s'", strings.Join(args, " "))

	err = c.executable.Execute(pexec.Execution{
		Args:   args,
		Stdout: c.logger.ActionWriter,
		Stderr: c.logger.ActionWriter,
	})
	if err != nil {
		return fmt.Errorf("failed to run conda command: %w", err)
	}

	c.logger.Subprocess("Removing %s", historyFile)
	err = os.RemoveAll(historyFile)
	if err != nil {
		return err
	}

	return nil
}
