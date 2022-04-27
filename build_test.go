package condaenvupdate_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	condaenvupdate "github.com/paketo-buildpacks/conda-env-update"
	"github.com/paketo-buildpacks/conda-env-update/fakes"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir  string
		workingDir string
		cnbDir     string

		buffer *bytes.Buffer

		runner        *fakes.Runner
		planner       *fakes.Planner
		sbomGenerator *fakes.SBOMGenerator

		build        packit.BuildFunc
		buildContext packit.BuildContext
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		planner = &fakes.Planner{}
		runner = &fakes.Runner{}
		sbomGenerator = &fakes.SBOMGenerator{}

		runner.ShouldRunCall.Returns.Bool = true
		runner.ShouldRunCall.Returns.String = "some-sha"

		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		buffer = bytes.NewBuffer(nil)
		logger := scribe.NewEmitter(buffer)

		build = condaenvupdate.Build(planner, runner, sbomGenerator, logger, chronos.DefaultClock)
		buildContext = packit.BuildContext{
			BuildpackInfo: packit.BuildpackInfo{
				Name:        "Some Buildpack",
				Version:     "some-version",
				SBOMFormats: []string{sbom.CycloneDXFormat, sbom.SPDXFormat},
			},
			WorkingDir: workingDir,
			CNBPath:    cnbDir,
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name: "conda-environment",
					},
				},
			},
			Platform: packit.Platform{Path: "some-platform-path"},
			Layers:   packit.Layers{Path: layersDir},
			Stack:    "some-stack",
		}
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	it("returns a result that builds correctly", func() {
		result, err := build(buildContext)
		Expect(err).NotTo(HaveOccurred())

		layers := result.Layers
		Expect(layers).To(HaveLen(1))

		condaEnvLayer := layers[0]
		Expect(condaEnvLayer.Name).To(Equal("conda-env"))
		Expect(condaEnvLayer.Path).To(Equal(filepath.Join(layersDir, "conda-env")))

		Expect(condaEnvLayer.Build).To(BeFalse())
		Expect(condaEnvLayer.Launch).To(BeFalse())
		Expect(condaEnvLayer.Cache).To(BeFalse())

		Expect(condaEnvLayer.BuildEnv).To(BeEmpty())
		Expect(condaEnvLayer.LaunchEnv).To(BeEmpty())
		Expect(condaEnvLayer.ProcessLaunchEnv).To(BeEmpty())
		Expect(condaEnvLayer.SharedEnv).To(BeEmpty())

		Expect(condaEnvLayer.Metadata).To(HaveLen(1))
		Expect(condaEnvLayer.Metadata["lockfile-sha"]).To(Equal("some-sha"))

		Expect(condaEnvLayer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
			{
				Extension: sbom.Format(sbom.CycloneDXFormat).Extension(),
				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
			},
			{
				Extension: sbom.Format(sbom.SPDXFormat).Extension(),
				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
			},
		}))

		Expect(planner.MergeLayerTypesCall.CallCount).NotTo(BeZero())
		Expect(planner.MergeLayerTypesCall.Receives.String).To(Equal("conda-environment"))
		Expect(planner.MergeLayerTypesCall.Receives.BuildpackPlanEntrySlice).To(Equal([]packit.BuildpackPlanEntry{
			{
				Name: "conda-environment",
			},
		}))
		Expect(runner.ExecuteCall.Receives.CondaEnvPath).To(Equal(filepath.Join(layersDir, "conda-env")))
		Expect(runner.ExecuteCall.Receives.CondaCachePath).To(Equal(filepath.Join(layersDir, "conda-env-cache")))
		Expect(runner.ExecuteCall.Receives.WorkingDir).To(Equal(workingDir))

		Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
	})

	context("when the runner executes outputting a non-empty cache dir", func() {
		it.Before(func() {
			runner.ExecuteCall.Stub = func(_, c, _ string) error {
				Expect(os.Mkdir(c, os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(c, "some-file"), []byte{}, os.ModePerm)).To(Succeed())
				return nil
			}
		})

		it.After(func() {
			Expect(os.RemoveAll(filepath.Join(layersDir, "conda-env-cache"))).To(Succeed())
		})

		it("cache layer is exported", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			layers := result.Layers
			Expect(layers).To(HaveLen(2))

			condaEnvLayer := layers[0]
			Expect(condaEnvLayer.Name).To(Equal("conda-env"))

			cacheLayer := layers[1]
			Expect(cacheLayer.Name).To(Equal("conda-env-cache"))
			Expect(cacheLayer.Path).To(Equal(filepath.Join(layersDir, "conda-env-cache")))

			Expect(cacheLayer.Build).To(BeFalse())
			Expect(cacheLayer.Launch).To(BeFalse())
			Expect(cacheLayer.Cache).To(BeTrue())
		})
	})

	context("when a build plan entry requires conda-environment at launch", func() {
		it.Before(func() {
			planner.MergeLayerTypesCall.Returns.Launch = true
		})

		it("assigns the flag to the conda env layer", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			layers := result.Layers
			Expect(layers).To(HaveLen(1))

			condaEnvLayer := layers[0]
			Expect(condaEnvLayer.Name).To(Equal("conda-env"))

			Expect(condaEnvLayer.Build).To(BeFalse())
			Expect(condaEnvLayer.Launch).To(BeTrue())
			Expect(condaEnvLayer.Cache).To(BeFalse())

			Expect(planner.MergeLayerTypesCall.Receives.String).To(Equal("conda-environment"))
			Expect(planner.MergeLayerTypesCall.Receives.BuildpackPlanEntrySlice).To(Equal([]packit.BuildpackPlanEntry{
				{
					Name: "conda-environment",
				},
			}))
		})
	})

	context("when a build plan entry requires conda-environment at build", func() {
		it.Before(func() {
			planner.MergeLayerTypesCall.Returns.Build = true
		})

		it("assigns build and cache to the conda env layer", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			layers := result.Layers
			Expect(layers).To(HaveLen(1))

			condaEnvLayer := layers[0]
			Expect(condaEnvLayer.Name).To(Equal("conda-env"))

			Expect(condaEnvLayer.Build).To(BeTrue())
			Expect(condaEnvLayer.Launch).To(BeFalse())
			Expect(condaEnvLayer.Cache).To(BeTrue())

			Expect(planner.MergeLayerTypesCall.Receives.String).To(Equal("conda-environment"))
			Expect(planner.MergeLayerTypesCall.Receives.BuildpackPlanEntrySlice).To(Equal([]packit.BuildpackPlanEntry{
				{
					Name: "conda-environment",
				},
			}))
		})
	})

	context("cached packages should be reused", func() {
		it.Before(func() {
			runner.ShouldRunCall.Returns.Bool = false
			runner.ShouldRunCall.Returns.String = "cached-sha"

		})
		it("reuses cached conda env layer instead of running build process", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			layers := result.Layers
			Expect(layers).To(HaveLen(1))

			condaEnvLayer := layers[0]
			Expect(condaEnvLayer.Name).To(Equal("conda-env"))

			Expect(planner.MergeLayerTypesCall.Receives.String).To(Equal("conda-environment"))
			Expect(planner.MergeLayerTypesCall.Receives.BuildpackPlanEntrySlice).To(Equal([]packit.BuildpackPlanEntry{
				{
					Name: "conda-environment",
				},
			}))
			Expect(runner.ExecuteCall.CallCount).To(BeZero())
		})
	})

	context("failure cases", func() {
		context("conda layer cannot be fetched", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(layersDir, "conda-env.toml"), nil, 0000)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("conda cache layer cannot be fetched", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(layersDir, "conda-env-cache.toml"), nil, 0000)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("runner ShouldRun fails", func() {
			it.Before(func() {
				runner.ShouldRunCall.Returns.Error = errors.New("some-shouldrun-error")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError("some-shouldrun-error"))
			})
		})

		context("layer cannot be reset", func() {
			it.Before(func() {
				runner.ShouldRunCall.Returns.Bool = true
				Expect(os.Chmod(layersDir, 0500)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("error could not create directory")))
			})
		})

		context("install process fails to execute", func() {
			it.Before(func() {
				runner.ShouldRunCall.Returns.Bool = true
				runner.ExecuteCall.Returns.Error = errors.New("some execution error")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("some execution error")))
			})
		})

		context("when generating the SBOM returns an error", func() {
			it.Before(func() {
				buildContext.BuildpackInfo.SBOMFormats = []string{"random-format"}
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(`unsupported SBOM format: 'random-format'`))
			})
		})

		context("when formatting the SBOM returns an error", func() {
			it.Before(func() {
				sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("failed to generate SBOM")))
			})
		})
	})
}
