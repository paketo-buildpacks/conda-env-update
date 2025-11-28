package condaenvupdate_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	condaenvupdate "github.com/paketo-buildpacks/conda-env-update"
	"github.com/paketo-buildpacks/conda-env-update/fakes"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
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
		logger     scribe.Emitter
	)

	it.Before(func() {
		workingDir = t.TempDir()
		layersDir := t.TempDir()

		condaLayerPath = filepath.Join(layersDir, "a-conda-layer")
		condaCachePath = filepath.Join(layersDir, "a-conda-cache-path")

		executable = &fakes.Executable{}
		executions = []pexec.Execution{}
		executable.ExecuteCall.Stub = func(ex pexec.Execution) error {
			executions = append(executions, ex)
			Expect(os.MkdirAll(filepath.Join(condaLayerPath, "conda-meta"), os.ModePerm)).To(Succeed())
			// For reasons currently unknown, the search call triggers a permission issue in the tests
			Expect(os.Chmod(filepath.Join(condaLayerPath, "conda-meta"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(condaLayerPath, "conda-meta", "history"), []byte("some content"), os.ModePerm)).To(Succeed())
			_, err := fmt.Fprintln(ex.Stdout, "stdout output")
			Expect(err).NotTo(HaveOccurred())
			_, err = fmt.Fprintln(ex.Stderr, "stderr output")
			Expect(err).NotTo(HaveOccurred())
			return nil
		}

		summer = &fakes.Summer{}
		buffer = bytes.NewBuffer(nil)
		logger = scribe.NewEmitter(buffer)
		runner = condaenvupdate.NewCondaRunner(executable, summer, logger)
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

				Expect(executions[1].Args).To(Equal(args))
				Expect(executions[1].Env).NotTo(ContainElement(fmt.Sprintf("CONDA_PKGS_DIRS=%s", condaCachePath)))
				Expect(executable.ExecuteCall.CallCount).To(Equal(2))
				Expect(buffer.String()).To(ContainLines(
					fmt.Sprintf("    Running 'conda %s'", strings.Join(args, " ")),
					"      stdout output",
					"      stderr output",
				))

				historyFilepath := filepath.Join(condaLayerPath, "conda-meta", "history")
				Expect(historyFilepath).NotTo(BeAnExistingFile())
			})

			context("failure cases", func() {
				context("when there is an error running the conda command", func() {
					it.Before(func() {
						executable.ExecuteCall.Stub = func(ex pexec.Execution) error {
							_, err := fmt.Fprintln(ex.Stdout, "conda error stdout")
							Expect(err).NotTo(HaveOccurred())
							_, err = fmt.Fprintln(ex.Stderr, "conda error stderr")
							Expect(err).NotTo(HaveOccurred())
							return errors.New("some conda failure")
						}
					})

					it("returns an error with stdout/stderr output", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError("failed to run conda command: some conda failure"))

						args := []string{
							"search",
							"--quiet",
							"--channel", vendorPath,
							"--override-channels",
							"--offline",
						}
						Expect(buffer.String()).To(ContainLines(
							fmt.Sprintf("    Running 'conda %s'", strings.Join(args, " ")),
							"      conda error stdout",
							"      conda error stderr",
						))
					})
				})

				context("when the removing the history file fails with error", func() {
					it.Before(func() {
						executable.ExecuteCall.Stub = func(_ pexec.Execution) error {
							Expect(os.MkdirAll(filepath.Join(condaLayerPath, "conda-meta"), os.ModePerm)).To(Succeed())
							// For reasons currently unknown, the search call triggers a permission issue in the tests
							Expect(os.Chmod(filepath.Join(condaLayerPath, "conda-meta"), os.ModePerm)).To(Succeed())
							Expect(os.WriteFile(filepath.Join(condaLayerPath, "conda-meta", "history"), []byte("some content"), os.ModePerm)).To(Succeed())
							Expect(os.Chmod(filepath.Join(condaLayerPath, "conda-meta"), 0)).To(Succeed())
							return nil
						}
					})

					it("returns the error", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError(ContainSubstring("%s: permission denied", filepath.Join(condaLayerPath, "conda-meta"))))
						// Required for the automatic t.TempDir cleanup
						Expect(os.Chmod(filepath.Join(condaLayerPath, "conda-meta"), os.ModePerm)).To(Succeed())
					})
				})
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

				historyFilepath := filepath.Join(condaLayerPath, "conda-meta", "history")
				Expect(historyFilepath).NotTo(BeAnExistingFile())
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

				historyFilepath := filepath.Join(condaLayerPath, "conda-meta", "history")
				Expect(historyFilepath).NotTo(BeAnExistingFile())
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
							_, err := fmt.Fprintln(ex.Stdout, "conda error stdout")
							Expect(err).NotTo(HaveOccurred())
							_, err = fmt.Fprintln(ex.Stderr, "conda error stderr")
							Expect(err).NotTo(HaveOccurred())
							return errors.New("some conda failure")
						}
					})

					it("returns an error and logs the stdout and stderr output from the command", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError("failed to run conda command: some conda failure"))
						Expect(buffer.String()).To(ContainLines(
							fmt.Sprintf(
								"    Running 'CONDA_PKGS_DIRS=%s conda env update --prefix %s --file %s'",
								condaCachePath,
								condaLayerPath,
								filepath.Join(workingDir, condaenvupdate.EnvironmentFileName),
							),
							"      conda error stdout",
							"      conda error stderr",
						))
					})
				})

				context("when the conda clean command fails to run", func() {
					it.Before(func() {
						executable.ExecuteCall.Stub = func(ex pexec.Execution) error {
							for _, arg := range ex.Args {
								if arg == "clean" {
									_, err := fmt.Fprintln(ex.Stdout, "conda error stdout")
							                Expect(err).NotTo(HaveOccurred())
									_, err = fmt.Fprintln(ex.Stderr, "conda error stderr")
							                Expect(err).NotTo(HaveOccurred())
									return errors.New("some conda clean failure")
								}
							}
							return nil
						}
					})

					it("returns an error", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError("failed to run conda command: some conda clean failure"))
						Expect(buffer.String()).To(ContainLines(
							"    Running 'conda clean --packages --tarballs'",
							"      conda error stdout",
							"      conda error stderr",
						))
					})
				})

				context("when the removing the history file fails with error", func() {
					it.Before(func() {
						// Use the clean operation to create the error environment as it is the last thing that runs before
						// the history file is removed
						executable.ExecuteCall.Stub = func(ex pexec.Execution) error {
							for _, arg := range ex.Args {
								if arg == "clean" {
									Expect(os.MkdirAll(filepath.Join(condaLayerPath, "conda-meta"), os.ModePerm)).To(Succeed())
									Expect(os.WriteFile(filepath.Join(condaLayerPath, "conda-meta", "history"), []byte("some content"), os.ModePerm)).To(Succeed())
									Expect(os.Chmod(filepath.Join(condaLayerPath, "conda-meta"), 0)).To(Succeed())
								}
							}
							return nil
						}
					})

					it("returns the error", func() {
						err := runner.Execute(condaLayerPath, condaCachePath, workingDir)
						Expect(err).To(MatchError(ContainSubstring("%s: permission denied", filepath.Join(condaLayerPath, "conda-meta"))))
						// Required for the automatic t.TempDir cleanup
						Expect(os.Chmod(filepath.Join(condaLayerPath, "conda-meta"), os.ModePerm)).To(Succeed())
					})
				})
			})
		})
	})
}
