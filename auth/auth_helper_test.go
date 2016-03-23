package dockerauth

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/wercker/wercker/util"
)

type AuthHelperSuite struct {
	*util.TestSuite
}

func (a *AuthHelperSuite) TestNormalizeRegistry() {
	quay := "https://quay.io/v1/"
	dock := "https://registry.hub.docker.com/v1/"
	a.Equal(quay, NormalizeRegistry("https://quay.io"))
	a.Equal(quay, NormalizeRegistry("https://quay.io/v1"))
	a.Equal(quay, NormalizeRegistry("http://quay.io/v1"))
	a.Equal(quay, NormalizeRegistry("https://quay.io/v1/"))
	a.Equal(quay, NormalizeRegistry("quay.io"))

	a.Equal(dock, NormalizeRegistry(""))
	a.Equal(dock, NormalizeRegistry("https://registry.hub.docker.com"))
	a.Equal(dock, NormalizeRegistry("http://registry.hub.docker.com"))
	a.Equal(dock, NormalizeRegistry("registry.hub.docker.com"))
	a.Equal("https://quay.io/v2/", NormalizeRegistry("quay.io/v2/"))
	a.Equal("https://registry-1.docker.io/v2/", NormalizeRegistry("registry-1.docker.io"))
}

func TestExampleTestSuite(t *testing.T) {
	suiteTester := &AuthHelperSuite{&util.TestSuite{}}
	suite.Run(t, suiteTester)
}
