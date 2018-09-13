package kubectl_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
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

	manifestResourceTemplate := (`
	provider "kubectl" {
	  kubeconfig  = "%s"
	  kubecontext = "%s"
	}

	resource "kubectl_manifest" "rss-site" {
	  content = "%s"
	  name = "rss-site"
	}`)

	manifestResource = fmt.Sprintf(manifestResourceTemplate, kubeconfig,
		kubecontext, content)
	manifestResourceUpdate = fmt.Sprintf(manifestResourceTemplate, kubeconfig,
		kubecontext, contentUpdated)

	testAccProviders = map[string]terraform.ResourceProvider{
		"kubectl": testAccProvider,
	}
}

func TestAccManifest(t *testing.T) {

	resource.Test(t, resource.TestCase{

		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		CheckDestroy: testAccCheckManifestResourceDestroyed("pod",
			"test=acceptance", "acceptance-test",
		),
		Steps: []resource.TestStep{

			resource.TestStep{
				Config: manifestResource,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceExistsWithName("Namespace",
						"acceptance-test", "",
					),
					testManifestResourceItemsExistWithLabel("pod",
						"test=acceptance", "acceptance-test", 1,
					),
					resource.TestCheckResourceAttr("kubectl_manifest.rss-site",
						"name", "rss-site",
					),
					resource.TestCheckResourceAttr("kubectl_manifest.rss-site",
						"resources.#", "2",
					),
				),
			},

			resource.TestStep{
				Config: manifestResourceUpdate,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceExistsWithName("Namespace",
						"acceptance-test", "",
					),
					testManifestResourceItemsExistWithLabel("pod", "test=acceptance", "acceptance-test", 0),
					resource.TestCheckResourceAttr("kubectl_manifest.rss-site",
						"resources.#", "1",
					),
				),
			},
		},
	})
}

func testAccPreCheck(t *testing.T) {

}

func testAccCheckManifestResourceDestroyed(kind, label, namespace string,
) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {
		kubectlConfig, _ := testAccProvider.Meta().(*KubectlConfig)

		res, _ := resourceManifestRetrieveByLabel(kind, label, namespace,
			kubectlConfig,
		)

		var attempts int
		for attempts = 0; attempts <= maxNumberOfAttempts; attempts++ {
			res, _ = resourceManifestRetrieveByLabel(kind, label, namespace,
				kubectlConfig,
			)

			if len(res.Items) == 0 {
				break
			}
			time.Sleep(10 * time.Second)
		}
		if attempts > maxNumberOfAttempts {
			return fmt.Errorf("Resource of kind %s with label %s still does not exist after %d attempts of retrieving it\n", kind, label, attempts)
		}
		return nil
	}
}

func testAccCheckResourceExistsWithName(kind, name, namespace string,
) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {

		kubectlConfig, _ := testAccProvider.Meta().(*KubectlConfig)
		res, _ := resourceManifestRetrieveByName(kind, name, namespace,
			kubectlConfig,
		)

		var attempts int

		for attempts = 0; attempts <= maxNumberOfAttempts; attempts++ {
			res, _ = resourceManifestRetrieveByName(kind, name, namespace,
				kubectlConfig,
			)
			if res.Kind == kind {
				break
			}
			time.Sleep(10 * time.Second)
		}

		if attempts > maxNumberOfAttempts {
			return fmt.Errorf("Resource of kind %s with name %s still exists after %d attempts of retrieving it\n", kind, name, attempts)
		}
		return nil
	}
}

func testManifestResourceItemsExistWithLabel(kind, label, namespace string,
	items int) resource.TestCheckFunc {

	return func(s *terraform.State) (err error) {

		rs, ok := s.RootModule().Resources["kubectl_manifest.rss-site"]
		if !ok {
			return fmt.Errorf("Resource %s not found in terraform state",
				"kubectl_manifest.rss-site")
		}

		id := rs.Primary.ID
		if id != "rss-site" {
			return fmt.Errorf("Resource should have id: %s", "rss-site")
		}

		attributes := rs.Primary.Attributes

		kubectlConfig, _ := testAccProvider.Meta().(*KubectlConfig)

		res := &KubectlResponse{}
		var attempts int

		for attempts = 0; attempts <= maxNumberOfAttempts; attempts++ {

			res, _ = resourceManifestRetrieveByLabel(kind, label, namespace,
				kubectlConfig,
			)

			if len(res.Items) == items {
				break
			}
			time.Sleep(10 * time.Second)
		}

		if attempts > maxNumberOfAttempts {
			return fmt.Errorf("Resource of kind %s with label %s still exists after %d attempts of deleting it\n", kind, label, attempts)
		} else if items > 0 {
			setId := strconv.Itoa(schema.HashString(res.Items[0].Metadata.UID))

			tfUID := attributes["resources."+setId+".uid"]
			k8sUID := res.Items[0].Metadata.UID

			tfSelflink := attributes["resources."+setId+".selflink"]
			k8sSelflink := res.Items[0].Metadata.Selflink

			if tfUID != k8sUID {
				return fmt.Errorf("Expected uid %s [TF STATE} found %s [K8S]", tfUID, k8sUID)
			}
			if tfSelflink != k8sSelflink {
				return fmt.Errorf("Expected selflink %s [TF STATE} found %s [K8S]", tfSelflink, k8sSelflink)
			}
		}
		return nil
	}
}

func resourceManifestRetrieveByName(resType, resName, namespace string,
	kubectlCLIConfig *KubectlConfig) (*KubectlResponse, error) {

	args := kubectlCLIConfig.RenderArgs("get", resType, resName, "-o", "json")
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

func resourceManifestRetrieveByLabel(resType, label, namespace string,
	kubectlCLIConfig *KubectlConfig) (*KubectlResponse, error) {

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
