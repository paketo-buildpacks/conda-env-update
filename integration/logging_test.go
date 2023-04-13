package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/occam"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testLogging(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect

		pack   occam.Pack
		docker occam.Docker

		name   string
		source string

		imagesMap map[string]interface{}
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()

		var err error
		name, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())

		imagesMap = map[string]interface{}{}
	})

	it.After(func() {
		for imageID := range imagesMap {
			Expect(docker.Image.Remove.Execute(imageID)).To(Succeed())
		}
		Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("when app is vendored", func() {
		it("has correct logging output", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "vendored_app"))
			Expect(err).NotTo(HaveOccurred())

			image, logs, err := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					minicondaBuildpack,
					buildpack,
					buildPlanBuildpack,
				).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred(), logs.String())

			imagesMap[image.ID] = nil

			Expect(logs).To(ContainLines(
				MatchRegexp(fmt.Sprintf(`%s \d+\.\d+\.\d+`, buildpackInfo.Buildpack.Name)),
				"  Executing build process",
				fmt.Sprintf(
					"    Running 'conda create --file /workspace/package-list.txt --prefix /layers/%s/conda-env --yes --quiet --channel /workspace/vendor --override-channels --offline'",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"),
				),
			))
			Expect(logs).To(ContainLines(
				fmt.Sprintf(
					"    Removing /layers/%s/conda-env/conda-meta/history",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"),
				),
				MatchRegexp(`      Completed in (\d+m)?(\d+)(\.\d+)?(ms|s)`),
				"",
			))
		})
	})

	context("when app has lockfile", func() {
		it("has correct logging output on initial build and rebuild", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "with_lock_file"))
			Expect(err).NotTo(HaveOccurred())

			image, logs, err := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					minicondaBuildpack,
					buildpack,
					buildPlanBuildpack,
				).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred(), logs.String())

			imagesMap[image.ID] = nil

			Expect(logs).To(ContainLines(
				MatchRegexp(fmt.Sprintf(`%s \d+\.\d+\.\d+`, buildpackInfo.Buildpack.Name)),
				"  Executing build process",
				fmt.Sprintf(
					"    Running 'CONDA_PKGS_DIRS=/layers/%s/conda-env-cache conda create --file /workspace/package-list.txt --prefix /layers/%s/conda-env --yes --quiet'",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"),
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"),
				),
			))
			Expect(logs).To(ContainLines("    Running 'conda clean --packages --tarballs'"))
			Expect(logs).To(ContainLines(
				fmt.Sprintf(
					"    Removing /layers/%s/conda-env/conda-meta/history",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"),
				),
				MatchRegexp(`      Completed in (\d+m)?(\d+)(\.\d+)?(ms|s)`),
				"",
			))

			secondImage, logs, err := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					minicondaBuildpack,
					buildpack,
					buildPlanBuildpack,
				).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred(), logs.String())

			imagesMap[secondImage.ID] = nil

			Expect(logs).To(ContainLines(
				MatchRegexp(fmt.Sprintf(`%s \d+\.\d+\.\d+`, buildpackInfo.Buildpack.Name)),
				fmt.Sprintf("  Reusing cached layer /layers/%s/conda-env", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				"",
			))
		})
	})

	context("when app doesn't have vendor folder and lockfile", func() {
		it("has correct logging output", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "default_app"))
			Expect(err).NotTo(HaveOccurred())

			image, logs, err := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					minicondaBuildpack,
					buildpack,
					buildPlanBuildpack,
				).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred(), logs.String())

			imagesMap[image.ID] = nil

			Expect(logs).To(ContainLines(
				MatchRegexp(fmt.Sprintf(`%s \d+\.\d+\.\d+`, buildpackInfo.Buildpack.Name)),
				"  Executing build process",
				fmt.Sprintf("    Running 'CONDA_PKGS_DIRS=/layers/%s/conda-env-cache conda env update --prefix /layers/%s/conda-env --file /workspace/environment.yml'",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
			))
			Expect(logs).To(ContainLines("    Running 'conda clean --packages --tarballs'"))
			Expect(logs).To(ContainLines(
				fmt.Sprintf(
					"    Removing /layers/%s/conda-env/conda-meta/history",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"),
				),
				MatchRegexp(`      Completed in (\d+m)?(\d+)(\.\d+)?(ms|s)`),
				"",
			))
		})
	})
}
