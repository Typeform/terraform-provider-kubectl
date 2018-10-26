package kubectl_test

import (
	"bytes"
	"os"
	"regexp"
	"strings"

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
				kubectlConfig.InitializeConfiguration()
				kubectlConfig.Cleanup()

				Expect(err).To(BeNil())
			})

			It("Should use kubeconfig as file path", func() {
				Expect(kubectlConfig.Kubeconfig).To(Equal(filepath))
				Expect(kubectlConfig.Kubecontent).To(Equal(""))
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
				kubectlConfig.InitializeConfiguration()

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
		})
	})

})

var _ = Describe("CLICommandFactory", func() {

	Describe("Creating various commands", func() {

		Context("When kubeconfig parameter is set", func() {

			expectedGetByHandle := "kubectl --kubeconfig /home/user/.kube/config get --ignore-not-found=true /v2/myresourceHandle -n test"
			expectedGetByManifest := "kubectl --kubeconfig /home/user/.kube/config get -f - -o json -n test"
			expectedStdin := "---\napiVersion: v1\nkind: Namespace\n  metadata:\n  name: acceptance-test"
			expectedDeleteByHandle := "kubectl --kubeconfig /home/user/.kube/config delete --ignore-not-found=true /v2/myResource -n test"
			expectedApplyManifest := "kubectl --kubeconfig /home/user/.kube/config apply -f - -n test"
			var (
				filepath       string
				config         *Config
				kubectlConfig  *KubectlConfig
				commandFactory *CLICommandFactory
				err            error
			)

			BeforeEach(func() {
				filepath = "/home/user/.kube/config"
				config = &Config{Kubeconfig: filepath}
				kubectlConfig, err = NewKubectlConfig(config)
				commandFactory = &CLICommandFactory{KubectlConfig: kubectlConfig}

				Expect(err).To(BeNil())
			})

			It("Should create a valid get by handle command", func() {
				stdout := &bytes.Buffer{}
				getCommand := commandFactory.CreateGetByHandleCommand(
					"/v2/myresourceHandle", "test", stdout)

				resultingCommand := strings.Join(getCommand.Args, " ")

				Expect(resultingCommand).To(Equal(expectedGetByHandle))
			})

			It("Should create a valid get by manifest command", func() {
				stdout := &bytes.Buffer{}
				getCommand := commandFactory.CreateGetByManifestCommand(
					expectedStdin, "test", stdout)

				resultingCommand := strings.Join(getCommand.Args, " ")

				buf := new(bytes.Buffer)
				buf.ReadFrom(getCommand.Stdin)

				Expect(resultingCommand).To(Equal(expectedGetByManifest))
				Expect(buf.String()).To(Equal(expectedStdin))

			})

			It("Should create a valid delete by handle command", func() {
				deleteCommand := commandFactory.CreateDeleteByHandleCommand(
					"/v2/myResource", "test")
				resultingCommand := strings.Join(deleteCommand.Args, " ")

				Expect(resultingCommand).To(Equal(expectedDeleteByHandle))
			})

			It("Should create a valid apply command", func() {
				applyCommand := commandFactory.CreateApplyManifestCommand(
					expectedStdin, "test")

				resultingCommand := strings.Join(applyCommand.Args, " ")

				buf := new(bytes.Buffer)
				buf.ReadFrom(applyCommand.Stdin)

				Expect(resultingCommand).To(Equal(expectedApplyManifest))
				Expect(buf.String()).To(Equal(expectedStdin))
			})
		})

	})

})
