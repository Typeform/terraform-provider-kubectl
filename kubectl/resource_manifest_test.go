package kubectl_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"testing"
	"time"

	. "github.com/Typeform/terraform-provider-kubectl/kubectl"
	. "github.com/Typeform/terraform-provider-kubectl/kubectl/resource"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

const kubernetesResource = `---
apiVersion: v1
kind: Namespace
metadata:
  name: acceptance-test

---
apiVersion: v1
kind: Pod
metadata:
  namespace: acceptance-test
  name: rss-site
  labels:
    test: acceptance
spec:
  containers:
    - name: front-end
      image: nginx
      ports:
        - containerPort: 80
    - name: rss-reader
      image: nickchase/rss-php-nginx:v1
      ports:
        - containerPort: 88
`

const kubernetesResourceAfterUpdate = `---
apiVersion: v1
kind: Namespace
metadata:
  name: acceptance-test
`

const maxNumberOfAttempts = 10

var testAccProviders map[string]terraform.ResourceProvider
var testAccProvider *schema.Provider
var manifestResource, manifestResourceUpdate string
var kubeconfig, kubecontext string

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func init() {
	testAccProvider = Provider()

	re := regexp.MustCompile(`\r?\n`)
	content := re.ReplaceAllString(kubernetesResource, `\n`)
	contentUpdated := re.ReplaceAllString(kubernetesResourceAfterUpdate, `\n`)

	homedir := getEnv("HOME", "˜")
	kubeconfig := getEnv("TP_KUBECTL_KUBECONFIG", path.Join(homedir, ".kube/config"))
	kubecontext := getEnv("TP_KUBECTL_KUBECONTEXT", "minikube")

	manifestResource = fmt.Sprintf(`
	provider "kubectl" {
	  kubeconfig  = "%s"
	  kubecontext = "%s"
	}

	resource "kubectl_manifest" "config-auth" {
	  content = "%s"
	  name = "config-auth"
	}`, kubeconfig, kubecontext, content)

	manifestResourceUpdate = fmt.Sprintf(`
	provider "kubectl" {
	  kubeconfig  = "%s"
	  kubecontext = "%s"
	}

	resource "kubectl_manifest" "config-auth" {
	  content = "%s"
	  name = "config-auth"
	}`, kubeconfig, kubecontext, contentUpdated)

	testAccProviders = map[string]terraform.ResourceProvider{
		"kubectl": testAccProvider,
	}
}

func TestAccManifest(t *testing.T) {

	resource.Test(t, resource.TestCase{

		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckManifestResourceDestroyed("pod", "test=acceptance", "acceptance-test"),
		Steps: []resource.TestStep{

			resource.TestStep{
				Config: manifestResource,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNamespaceExists("acceptance-test"),
					testManifestResourcesExist("pod", "test=acceptance", "acceptance-test", 1),
					resource.TestCheckResourceAttr("kubectl_manifest.config-auth", "name", "config-auth"),
					resource.TestCheckResourceAttr("kubectl_manifest.config-auth", "resources.#", "2"),
				),
			},

			resource.TestStep{
				Config: manifestResourceUpdate,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNamespaceExists("acceptance-test"),
					testManifestResourcesExist("pod", "test=acceptance", "acceptance-test", 0),
					resource.TestCheckResourceAttr("kubectl_manifest.config-auth", "resources.#", "1"),
				),
			},
		},
	})
}

func testAccPreCheck(t *testing.T) {

}

func testAccCheckManifestResourceDestroyed(kind, label, namespace string) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {
		kubectlConfig, _ := NewKubectlConfig(testAccProvider.Meta())

		res, _ := resourceManifestRetrieveByLabel(kind, label, namespace, kubectlConfig)

		for attempts := 0; len(res.Items) != 0; attempts++ {
			res, _ = resourceManifestRetrieveByLabel(kind, label, namespace, kubectlConfig)

			if attempts > maxNumberOfAttempts {
				return fmt.Errorf("Resource of kind %s with label %s still does not exist after %d attempts of retrieving it\n", kind, label, attempts)
			}
			time.Sleep(10 * time.Second)
		}

		return nil
	}
}

func testAccCheckNamespaceExists(name string) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {

		return nil
	}
}

func testManifestResourcesExist(kind, label, namespace string, items int) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {

		_, ok := s.RootModule().Resources["kubectl_manifest.config-auth"]
		if !ok {
			return fmt.Errorf("Resource %s not found in terraform state", "kubectl_manifest.config-auth")
		}

		//id := rs.Primary.ID
		//attributes := rs.Primary.Attributes

		kubectlConfig, _ := NewKubectlConfig(testAccProvider.Meta())

		res, _ := resourceManifestRetrieveByLabel(kind, label, namespace, kubectlConfig)

		for attempts := 0; len(res.Items) != items; attempts++ {
			res, _ = resourceManifestRetrieveByLabel(kind, label, namespace, kubectlConfig)

			if attempts > maxNumberOfAttempts {
				return fmt.Errorf("Resource of kind %s with label %s still exists after %d attempts of deleting it\n", kind, label, attempts)
			}
			time.Sleep(10 * time.Second)
		}

		return nil
	}
}

func resourceManifestRetrieveByLabel(resType, label, namespace string, kubectlCLIConfig *KubectlConfig) (*KubectlResponse, error) {

	args := kubectlCLIConfig.RenderArgs("get", resType, "-l", label, "-o", "json")
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	retrieveCommand := NewCLICommand("kubectl", args...)

	stdout := &bytes.Buffer{}
	retrieveCommand.Stdout = stdout

	retrieveCommand.RunCommand()

	res := &KubectlResponse{}
	json.Unmarshal(stdout.Bytes(), &res)

	return res, nil
}
