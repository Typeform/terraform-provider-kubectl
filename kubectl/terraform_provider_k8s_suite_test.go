package kubectl_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTerraformProviderK8s(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TerraformProviderK8s Suite")
}
