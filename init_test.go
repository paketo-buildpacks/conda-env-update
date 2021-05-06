package condaenvupdate_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitCondaEnvUpdate(t *testing.T) {
	suite := spec.New("conda-env-update", spec.Report(report.Terminal{}), spec.Parallel())
	suite("Build", testBuild)
	suite("CondaRunner", testCondaRunner)
	suite("Detect", testDetect)
	suite.Run(t)
}
