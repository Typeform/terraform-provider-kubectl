package kubectl

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Typeform/terraform-provider-kubectl/kubectl/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceManifest() *schema.Resource {
	return &schema.Resource{
		Create: resourceManifestCreate,
		Read:   resourceManifestRead,
		Exists: resourceManifestExists,
		Update: resourceManifestUpdate,
		Delete: resourceManifestDelete,

		Schema: map[string]*schema.Schema{
			"content": &schema.Schema{
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				StateFunc: func(val interface{}) string {
					contentHash := sha256.Sum256([]byte(val.(string)))
					return fmt.Sprintf("%x", contentHash)
				},
			},
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"namespace": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"resources": {
				Type:     schema.TypeSet,
				Computed: true,
				Set:      HashResource,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"selflink": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"uid": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"content": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func HashResource(v interface{}) int {

	resource := v.(map[string]interface{})
	uuid := resource["uid"]

	return schema.HashString(uuid)
}

// The steps involved in creating a resource are:
// 	1. Splitting the yaml manifest document in multiple resources
//  2. Update the resources:
//		For each resource:
//		1. it applies the manifest (re-applies if needed)
//		2. fetches the newly created resource
//		3. parses response to get `selflink` and `uid`
//		4. encodes the content in base64
//		5. adds the created resources to the terraform state
func resourceManifestCreate(d *schema.ResourceData, m interface{}) error {
	var namespace string

	config := m.(*Config)

	kubectlCLIConfig, err := NewKubectlConfig(config)
	if err != nil {
		return fmt.Errorf("error while processing kubeconfig file: %s", err)
	}
	defer kubectlCLIConfig.Cleanup()

	if nm, ok := d.GetOk("namespace"); ok {
		namespace = nm.(string)
	}
	manifestResources, err := resource.SplitYAMLDocument(
		d.Get("content").(string))
	if err != nil {
		return err
	}
	tfResources, err := updateResources(
		manifestResources, namespace, kubectlCLIConfig)
	if err != nil {
		return err
	}
	err = d.Set("resources", tfResources)
	if err != nil {
		return err
	}
	err = d.Set("namespace", namespace)
	d.SetId(d.Get("name").(string))
	return nil
}

// The steps involved in updating resources are:
// 	1. Checking if the content has changed
//  2. Update the resources:
//		- it applies any resource present in the schema (re-applies if needed)
//		- for each resource it creates it retrieves it and updates the state
//		  with its uid
//  3. Deletes the resources which where present in the old state but are not
//     anymore
//
func resourceManifestUpdate(d *schema.ResourceData, m interface{}) error {

	config := m.(*Config)

	kubectlCLIConfig, err := NewKubectlConfig(config)
	if err != nil {
		return fmt.Errorf("error while processing kubeconfig file: %s", err)
	}
	defer kubectlCLIConfig.Cleanup()

	if d.HasChange("content") {

		var namespace string

		if nm, ok := d.GetOk("namespace"); ok {
			namespace = nm.(string)
		}
		tfOldResources := d.Get("resources").(*schema.Set)

		manifestResources, err := resource.SplitYAMLDocument(
			d.Get("content").(string))
		if err != nil {
			return err
		}
		tfResources, err := updateResources(
			manifestResources, namespace, kubectlCLIConfig)
		if err != nil {
			return err
		}

		toDelete := setDifference(tfOldResources, tfResources)
		err = deleteResources(toDelete, kubectlCLIConfig)
		if err != nil {
			return err
		}

		err = d.Set("resources", tfResources)
		if err != nil {
			return err
		}

	}
	return nil
}

// Simply deletes all the resources in the manifest one by one
//	1. gets the resources from the terraform state
//
//	for each of the retrieved resources:
//	- delete the resource
func resourceManifestDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)

	kubectlCLIConfig, err := NewKubectlConfig(config)
	if err != nil {
		return fmt.Errorf("error while processing kubeconfig file: %s", err)
	}
	defer kubectlCLIConfig.Cleanup()

	toDelete := d.Get("resources").(*schema.Set)
	err = deleteResources(toDelete, kubectlCLIConfig)
	return err
}

// Reads the resources using the resource handle provided
//
// for each resource in the terraform state:
// - tries to match the resource with a live k8s resource
// - builds the intersection beetween state and live resources
// - if the intersaction is empty sets resource id to "" (will force create)
func resourceManifestRead(d *schema.ResourceData, m interface{}) error {

	config := m.(*Config)

	kubectlCLIConfig, err := NewKubectlConfig(config)
	if err != nil {
		return fmt.Errorf("error while processing kubeconfig file: %s", err)
	}
	defer kubectlCLIConfig.Cleanup()

	log.Printf("[DEBUG] start refreshing object %s", d.Get("name").(string))

	commonResources, errs := getTfResourcesFromK8s(kubectlCLIConfig, d)

	// TODO: make sure to differentiate the kind of errors and abort only when they
	//       mean that the resource does not exist
	if len(errs) != 0 {
		for _, k8sErr := range errs {
			log.Printf("[DEBUG] Error while refreshing resources from K8s %s", k8sErr)
		}
	}

	err = d.Set("resources", commonResources)
	if err != nil {
		log.Printf("[DEBUG] Error while refreshing resources %s", err)
		return err
	}
	if commonResources.Len() < 1 {
		d.SetId("")
	}
	log.Printf("[DEBUG] done refreshing object %s", d.Get("name").(string))

	return nil
}

func getTfResourcesFromK8s(kubectlCLIConfig *KubectlConfig, d *schema.ResourceData) (
	*schema.Set, []error) {

	errs := make([]error, 0)

	tfResources := d.Get("resources").(*schema.Set)
	tfResourcesList := tfResources.List()
	kubectlResources := schema.NewSet(HashResource, []interface{}{})

	var wg sync.WaitGroup
	errChan := make(chan error, len(tfResourcesList))
	resChan := make(chan interface{}, len(tfResourcesList))

	var closeOnce sync.Once
	closeChannels := func() {
		close(errChan)
		close(resChan)
	}

	defer func() {
		closeOnce.Do(closeChannels)
	}()

	for _, tfResource := range tfResourcesList {
		wg.Add(1)
		go func(tfResource interface{}) {
			defer wg.Done()
			readResource(kubectlCLIConfig, tfResource, resChan, errChan)
		}(tfResource)
	}

	wg.Wait()
	closeOnce.Do(closeChannels)

	for routineErr := range errChan {
		errs = append(errs, routineErr)
	}
	for routineResource := range resChan {
		kubectlResources.Add(routineResource)
	}

	commonResources := setIntersection(tfResources, kubectlResources)
	return commonResources, errs
}

func readResource(kubectlCLIConfig *KubectlConfig, tfResource interface{},
	resChan chan<- interface{}, errChan chan<- error) {

	resourceObj, ok := tfResource.(map[string]interface{})
	if !ok {
		errChan <- fmt.Errorf("error converting resource map\n")
		return
	}
	selflink, ok := resourceObj["selflink"].(string)
	if !ok {
		errChan <- fmt.Errorf("error converting resource selflink into string\n")
		return
	}
	resourceHandle, namespace, ok := resourceFromSelflink(selflink)
	if !ok {
		errChan <- fmt.Errorf("invalid resource id: %s", selflink)
		return
	}
	log.Printf("[DEBUG] start refreshing resource %s in namespace %s",
		resourceHandle, namespace)

	stdout := &bytes.Buffer{}
	commandFactory := &CLICommandFactory{KubectlConfig: kubectlCLIConfig}
	getCommand := commandFactory.CreateGetByHandleCommand(
		resourceHandle, namespace, stdout)

	if err := getCommand.RunCommand(); err != nil {
		errChan <- err
		return
	}

	if strings.TrimSpace(stdout.String()) != "" {
		resChan <- tfResource
	}
	log.Printf("[DEBUG] end refreshing resource %s in namespace %s",
		resourceHandle, namespace)
}

// Tries to fetch at least one of the resources contained in the state.
//
// Works as a read, but just returns successfully if any resource present in the
// terraform state is also present in k8s
func resourceManifestExists(d *schema.ResourceData, m interface{}) (bool, error) {

	config := m.(*Config)

	kubectlCLIConfig, err := NewKubectlConfig(config)
	if err != nil {
		return false, fmt.Errorf(
			"error while processing kubeconfig file: %s", err,
		)
	}
	defer kubectlCLIConfig.Cleanup()

	tfResources := d.Get("resources").(*schema.Set)
	tfResourcesList := tfResources.List()

	for _, tfResource := range tfResourcesList {

		resourceObj, ok := tfResource.(map[string]interface{})
		if !ok {
			log.Printf("error converting resource map\n")
			continue
		}
		selflink, ok := resourceObj["selflink"].(string)
		if !ok {
			log.Printf("error converting resource selflink into string\n")
			continue
		}
		resourceHandle, namespace, ok := resourceFromSelflink(selflink)
		if !ok {
			log.Printf("invalid resource id: %s", selflink)
			continue
		}

		stdout := &bytes.Buffer{}
		commandFactory := &CLICommandFactory{KubectlConfig: kubectlCLIConfig}
		getCommand := commandFactory.CreateGetByHandleCommand(resourceHandle,
			namespace, stdout)

		if err := getCommand.RunCommand(); err != nil {
			log.Printf("error executing run command: %s", err)
			continue
		}
		if strings.TrimSpace(stdout.String()) != "" {
			return true, nil
		}
	}

	return false, nil
}

func deleteResources(manifestResources *schema.Set,
	kubectlCLIConfig *KubectlConfig) error {

	manifestResourcesList := manifestResources.List()

	for _, resource := range manifestResourcesList {

		tfResource, ok := resource.(map[string]interface{})
		if !ok {
			return errors.New("Error while converting resource into resource map")
		}
		selflink, ok := tfResource["selflink"].(string)
		if !ok {
			return errors.New("Error extracting selflink from resource map")
		}

		resourceHandle, namespace, ok := resourceFromSelflink(selflink)
		if !ok {
			return fmt.Errorf("invalid resource id: %s", selflink)
		}
		commandFactory := &CLICommandFactory{KubectlConfig: kubectlCLIConfig}
		deleteCommand := commandFactory.CreateDeleteByHandleCommand(
			resourceHandle, namespace)

		err := deleteCommand.RunCommand()
		if err != nil {
			return err
		}
	}

	return nil
}

func updateResources(manifestResources []string, namespace string,
	kubectlCLIConfig *KubectlConfig) (*schema.Set, error) {

	tfResources := schema.NewSet(HashResource, []interface{}{})
	commandFactory := &CLICommandFactory{KubectlConfig: kubectlCLIConfig}

	for _, manifestResource := range manifestResources {
		applyCommand := commandFactory.CreateApplyManifestCommand(
			manifestResource, namespace)

		if err := applyCommand.RunCommand(); err != nil {
			return nil, err
		}

		stdout := &bytes.Buffer{}
		getCommand := commandFactory.CreateGetByManifestCommand(
			manifestResource, namespace, stdout)

		if err := getCommand.RunCommand(); err != nil {
			return nil, err
		}

		var data resource.KubectlResponse
		if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
			return nil, fmt.Errorf("decoding response: %v", err)
		}

		if len(data.Items) > 1 {
			return nil, fmt.Errorf("Expecting a single resource, found multiple")
		}
		selflink := data.Items[0].Metadata.Selflink
		if selflink == "" {
			return nil, fmt.Errorf("could not parse self-link from response %s",
				stdout.String(),
			)
		}
		uid := data.Items[0].Metadata.UID
		if uid == "" {
			return nil, fmt.Errorf("could not parse uid from response %s",
				stdout.String(),
			)
		}

		manifestResourceBase64 := base64.StdEncoding.EncodeToString(
			[]byte(manifestResource))
		tfResources.Add(map[string]interface{}{"uid": uid, "selflink": selflink,
			"content": manifestResourceBase64})
	}

	return tfResources, nil
}

func resourceFromSelflink(s string) (resource, namespace string, ok bool) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	resource = parts[len(parts)-2] + "/" + parts[len(parts)-1]

	for i, part := range parts {
		if part == "namespaces" && len(parts) > i+1 {
			namespace = parts[i+1]
			break
		}
	}
	return resource, namespace, true
}

func setIntersection(set1, set2 *schema.Set) *schema.Set {
	intersection := schema.NewSet(HashResource, []interface{}{})
	set1Elems := set1.List()

	for _, elem := range set1Elems {
		if set2.Contains(elem) {
			intersection.Add(elem)
		}
	}
	return intersection
}

func setDifference(set1, set2 *schema.Set) *schema.Set {
	difference := schema.NewSet(HashResource, []interface{}{})
	set1Elems := set1.List()

	for _, elem := range set1Elems {
		if !set2.Contains(elem) {
			difference.Add(elem)
		}
	}
	return difference
}
