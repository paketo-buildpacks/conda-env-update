package condaenvupdate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface Executable --output fakes/executable.go

// Executable defines the interface for invoking an executable.
type Executable interface {
	Execute(pexec.Execution) error
}

//go:generate faux --interface Summer --output fakes/summer.go
// Summer defines the interface for computing a SHA256 for a set of files
// and/or directories.
type Summer interface {
	Sum(arg ...string) (string, error)
}

// CondaRunner implements the Runner interface.
type CondaRunner struct {
	logger     scribe.Logger
	executable Executable
	summer     Summer
}

// NewCondaRunner creates an instance of CondaRunner given an Executable, a Summer, and a Logger.
func NewCondaRunner(executable Executable, summer Summer, logger scribe.Logger) CondaRunner {
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
	vendorDirExists, err := fileExists(filepath.Join(workingDir, "vendor"))
	if err != nil {
		return err
	}

	lockfileExists, err := fileExists(filepath.Join(workingDir, LockfileName))
	if err != nil {
		return err
	}

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

		c.logger.Subprocess("Running conda %s", strings.Join(args, " "))

		buffer := bytes.NewBuffer(nil)
		err = c.executable.Execute(pexec.Execution{
			Args:   args,
			Stdout: buffer,
			Stderr: buffer,
		})
		if err != nil {
			c.logger.Action("Failed to run conda %s", strings.Join(args, " "))
			c.logger.Detail(buffer.String())
			return fmt.Errorf("failed to run conda command: %w", err)
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

	c.logger.Subprocess("Running CONDA_PKGS_DIRS=%s conda %s", condaCachePath, strings.Join(args, " "))

	buffer := bytes.NewBuffer(nil)
	err = c.executable.Execute(pexec.Execution{
		Args:   args,
		Env:    append(os.Environ(), fmt.Sprintf("CONDA_PKGS_DIRS=%s", condaCachePath)),
		Stdout: buffer,
		Stderr: buffer,
	})

	if err != nil {
		c.logger.Action("Failed to run CONDA_PKGS_DIRS=%s conda %s", condaCachePath, strings.Join(args, " "))
		c.logger.Detail(buffer.String())
		return fmt.Errorf("failed to run conda command: %w", err)
	}

	args = []string{
		"clean",
		"--packages",
		"--tarballs",
	}

	c.logger.Subprocess("Running conda %s", strings.Join(args, " "))

	buffer = bytes.NewBuffer(nil)
	err = c.executable.Execute(pexec.Execution{
		Args:   args,
		Stdout: buffer,
		Stderr: buffer,
	})
	if err != nil {
		c.logger.Action("Failed to run conda %s", strings.Join(args, " "))
		c.logger.Detail(buffer.String())
		return fmt.Errorf("failed to run conda command: %w", err)
	}

	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
