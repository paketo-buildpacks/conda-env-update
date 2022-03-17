package condaenvupdate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	condaenvupdate "github.com/paketo-buildpacks/conda-env-update"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir string
		detect     packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		detect = condaenvupdate.Detect()
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	context("when there is an environment.yml in the working dir", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "environment.yml"), nil, 0644)).To(Succeed())
		})

		it("detects", func() {
			result, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Plan).To(Equal(packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{
						Name: "conda-environment",
					},
				},
				Requires: []packit.BuildPlanRequirement{
					{
						Name: "conda",
						Metadata: map[string]interface{}{
							"build": true,
						},
					},
				},
			}))
		})
	})

	context("when there is an package-list.txt in the working dir", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "package-list.txt"), nil, 0644)).To(Succeed())
		})

		it("detects", func() {
			result, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Plan).To(Equal(packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{
						Name: "conda-environment",
					},
				},
				Requires: []packit.BuildPlanRequirement{
					{
						Name: "conda",
						Metadata: map[string]interface{}{
							"build": true,
						},
					},
				},
			}))
		})
	})

	context("when no environment.yml or package-list.txt is present in the working dir", func() {
		it("fails to detect", func() {
			_, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).To(MatchError(packit.Fail))
		})
	})

	context("failure cases", func() {
		context("when the file cannot be stat'd", func() {
			it.Before(func() {
				Expect(os.Chmod(workingDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := detect(packit.DetectContext{
					WorkingDir: workingDir,
				})
				Expect(err).To(MatchError(ContainSubstring("failed trying to stat environment.yml:")))
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})
	})
}
