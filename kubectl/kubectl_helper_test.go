package kubectl_test

import (
	"os"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/Typeform/terraform-provider-kubectl/kubectl"
)

var _ = Describe("KubectlHelper", func() {

	Describe("Initializing from parameters", func() {

		Context("When kubeconfig parameter is set", func() {

			var (
				filepath      string
				config        *Config
				kubectlConfig *KubectlConfig
				err           error
			)

			BeforeEach(func() {
				filepath = "/home/user/.kube/config"
				config = &Config{Kubeconfig: filepath}
				kubectlConfig, err = NewKubectlConfig(config)

				Expect(err).To(BeNil())
			})

			It("Should use kubeconfig as file path", func() {
				Expect(kubectlConfig.Kubeconfig).To(Equal(filepath))
				Expect(kubectlConfig.Kubecontent).To(Equal(""))
			})

			It("Should need clean up", func() {
				Expect(kubectlConfig.ShouldCleanUp()).To(BeFalse())
			})
		})

		Context("When only kubecontent parameter is set", func() {

			var (
				config        *Config
				kubectlConfig *KubectlConfig
				err           error
			)

			BeforeEach(func() {
				base64Content := "VGhpcyBpcyBhIHRlc3QgY29udGVudA==" //"This is a test content"
				config = &Config{Kubecontent: base64Content}
				kubectlConfig, err = NewKubectlConfig(config)

				Expect(err).To(BeNil())
			})

			It("Should use kubeconfig as file path", func() {
				r, _ := regexp.Compile("kubeconfig_*")
				Expect(r.FindString(kubectlConfig.Kubeconfig)).To(Equal("kubeconfig_"))

				_, err := os.Stat(kubectlConfig.Kubeconfig)
				Expect(err).To(BeNil())

				kubectlConfig.Cleanup()

				_, err = os.Stat(kubectlConfig.Kubeconfig)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})

			It("Needs cleaning up", func() {
				Expect(kubectlConfig.ShouldCleanUp()).To(BeTrue())
			})

		})

		Context("When neither parameters are set", func() {

			var (
				config        *Config
				kubectlConfig *KubectlConfig
				err           error
			)

			BeforeEach(func() {
				config = &Config{}
				kubectlConfig, err = NewKubectlConfig(config)

				Expect(err).To(BeNil())
			})

			It("should not set any kubeconfig path", func() {
				Expect(kubectlConfig.Kubeconfig).To(Equal(""))
			})

			It("Should not clean up", func() {
				Expect(kubectlConfig.ShouldCleanUp()).To(BeFalse())
			})
		})
	})

})
