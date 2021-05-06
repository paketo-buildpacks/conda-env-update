package condaenvupdate_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
	condaenvupdate "github.com/paketo-community/conda-env-update"
	"github.com/paketo-community/conda-env-update/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testCondaRunner(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir     string
		condaLayerPath string
		condaCachePath string

		executable *fakes.Executable
		executions []pexec.Execution
		summer     *fakes.Summer
		runner     condaenvupdate.CondaRunner
		buffer     *bytes.Buffer
		logger     scribe.Logger
	)

	it.Before(func() {
		var err error

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())
		condaLayerPath = "a-conda-layer"
		condaCachePath = "a-conda-cache-path"

		executable = &fakes.Executable{}
		executions = []pexec.Execution{}
		executable.ExecuteCall.Stub = func(ex pexec.Execution) error {
			executions = append(executions, ex)
			return nil
		}

		summer = &fakes.Summer{}
		buffer = bytes.NewBuffer(nil)
		logger = scribe.NewLogger(buffer)
		runner = condaenvupdate.NewCondaRunner(executable, summer, logger)
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	context("ShouldRun", func() {
		it("returns true, with no sha, and no error when no lockfile is present", func() {
			run, sha, err := runner.ShouldRun(workingDir, map[string]interface{}{})
			Expect(run).To(BeTrue())
			Expect(sha).To(Equal(""))
			Expect(err).NotTo(HaveOccurred())
		})

		context("when there is an error checking if a lockfile is present", func() {
			it.Before(func() {
				Expect(os.Chmod(workingDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
			})

			it("returns false, with no sha, and an error", func() {
				run, sha, err := runner.ShouldRun(workingDir, map[string]interface{}{})
				Expect(run).To(BeFalse())
				Expect(sha).To(Equal(""))
				Expect(err).To(HaveOccurred())
			})
		})

		context("when a lockfile is present", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(workingDir, "package-list.txt"), nil, os.ModePerm)).To(Succeed())
			})
			context("and the lockfile sha is unchanged", func() {
				it("return false, with the existing sha, and no error", func() {
					summer.SumCall.Returns.String = "a-sha"
					Expect(os.WriteFile(filepath.Join(workingDir, "package-list.txt"), nil, os.ModePerm)).To(Succeed())

					metadata := map[string]interface{}{
						"lockfile-sha": "a-sha",
					}

					run, sha, err := runner.ShouldRun(workingDir, metadata)
					Expect(run).To(BeFalse())
					Expect(sha).To(Equal("a-sha"))
					Expect(err).NotTo(HaveOccurred())
				})
				context("and there is and error summing the lock file", func() {
					it.Before(func() {
						summer.SumCall.Returns.Error = errors.New("summing lockfile failed")
					})

					it("returns false, with no sha, and an error", func() {
						run, sha, err := runner.ShouldRun(workingDir, map[string]interface{}{})
						Expect(run).To(BeFalse())
						Expect(sha).To(Equal(""))
						Expect(err).To(MatchError("summing lockfile failed"))

					})
				})
			})

			it("returns true, with a new sha, and no error when the lockfile has changed", func() {
				summer.SumCall.Returns.String = "a-new-sha"
				metadata := map[string]interface{}{
					"lockfile-sha": "a-sha",
				}

				run, sha, err := runner.ShouldRun(workingDir, metadata)
				Expect(run).To(BeTrue())
				Expect(sha).To(Equal("a-new-sha"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	context("Execute", func() {
		context("when a vendor dir is present", func() {
			var vendorPath string
			it.Before(func() {
				vendorPath = filepath.Join(workingDir, "vendor")
				Expect(os.Mkdir(vendorPath, os.ModePerm))
			})

			it("runs conda create with additional vendor args and WITHOUT cache layer in env", func() {
				err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions[0].Args).To(Equal([]string{
					"create",
					"--file", filepath.Join(workingDir, "package-list.txt"),
					"--prefix", condaLayerPath,
					"--yes",
					"--quiet",
					"--channel", vendorPath,
					"--override-channels",
					"--offline",
				}))
				Expect(executions[0].Env).NotTo(ContainElement(fmt.Sprintf("CONDA_PKGS_DIRS=%s", condaCachePath)))
				Expect(executable.ExecuteCall.CallCount).To(Equal(1))
			})

		})

		context("when a lockfile exists", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(workingDir, condaenvupdate.LockfileName), nil, os.ModePerm)).To(Succeed())
			})
			it("runs conda create with the cache layer available in the environment", func() {
				err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions[0].Args).To(Equal([]string{
					"create",
					"--file", filepath.Join(workingDir, "package-list.txt"),
					"--prefix", condaLayerPath,
					"--yes",
					"--quiet",
				}))
				Expect(executions[0].Env).To(ContainElement(fmt.Sprintf("CONDA_PKGS_DIRS=%s", condaCachePath)))
				Expect(executable.ExecuteCall.CallCount).To(Equal(2))
				Expect(executions[1].Args).To(Equal([]string{
					"clean",
					"--packages",
					"--tarballs",
				}))
			})
		})

		context("when no vendor dir or lockfile exists", func() {
			it("runs conda env update with the cache layer available in the environment", func() {
				err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions[0].Args).To(Equal([]string{
					"env",
					"update",
					"--prefix", condaLayerPath,
					"--file", filepath.Join(workingDir, "environment.yml"),
				}))
				Expect(executions[0].Env).To(ContainElement(fmt.Sprintf("CONDA_PKGS_DIRS=%s", condaCachePath)))
				Expect(executable.ExecuteCall.CallCount).To(Equal(2))
				Expect(executions[1].Args).To(Equal([]string{
					"clean",
					"--packages",
					"--tarballs",
				}))
			})

			context("failure cases", func() {
				context("there is an error checking for vendor directory", func() {
					it.Before(func() {
						Expect(os.Chmod(workingDir, 0000)).To(Succeed())
					})

					it.After(func() {
						Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
					})

					it("returns an error", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError(ContainSubstring("permission denied")))
					})
				})

				context("when the conda env command fails to run", func() {
					it.Before(func() {
						executable.ExecuteCall.Stub = func(ex pexec.Execution) error {
							fmt.Fprintln(ex.Stdout, "conda error stdout")
							fmt.Fprintln(ex.Stderr, "conda error stderr")
							return errors.New("some conda failure")
						}
					})

					it("returns an error and logs the stdout and stderr output from the command", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError("failed to run conda command: some conda failure"))
						Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("Failed to run CONDA_PKGS_DIRS=%s conda env update --prefix a-conda-layer --file %s", condaCachePath, filepath.Join(workingDir, condaenvupdate.EnvironmentFileName))))
						Expect(buffer.String()).To(ContainSubstring("conda error stdout"))
						Expect(buffer.String()).To(ContainSubstring("conda error stderr"))
					})

					context("and there is a vendor directory", func() {
						var vendorPath string
						it.Before(func() {
							vendorPath = filepath.Join(workingDir, "vendor")
							Expect(os.Mkdir(vendorPath, os.ModePerm))
						})

						it("returns an error and logs the stdout and stderr output from the command", func() {
							err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
							Expect(err).To(MatchError("failed to run conda command: some conda failure"))

							args := []string{
								"create",
								"--file", filepath.Join(workingDir, "package-list.txt"),
								"--prefix", condaLayerPath,
								"--yes",
								"--quiet",
								"--channel", vendorPath,
								"--override-channels",
								"--offline",
							}
							Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("Failed to run conda %s", strings.Join(args, " "))))
							Expect(buffer.String()).To(ContainSubstring("conda error stdout"))
							Expect(buffer.String()).To(ContainSubstring("conda error stderr"))
						})
					})
				})

				context("when the conda clean command fails to run", func() {
					it.Before(func() {
						executable.ExecuteCall.Stub = func(ex pexec.Execution) error {
							for _, arg := range ex.Args {
								if arg == "clean" {
									fmt.Fprintln(ex.Stdout, "conda error stdout")
									fmt.Fprintln(ex.Stderr, "conda error stderr")
									return errors.New("some conda clean failure")
								}
							}
							return nil
						}
					})

					it("returns an error", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError("failed to run conda command: some conda clean failure"))
						Expect(buffer.String()).To(ContainSubstring("Failed to run conda clean --packages --tarballs"))
						Expect(buffer.String()).To(ContainSubstring("conda error stdout"))
						Expect(buffer.String()).To(ContainSubstring("conda error stderr"))
					})
				})
			})
		})
	})
}
