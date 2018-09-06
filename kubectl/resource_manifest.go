package kubectl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Typeform/terraform-provider-kubectl/kubectl/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceManifest() *schema.Resource {
	return &schema.Resource{
		Create: withCLIConfig(resourceManifestCreate),
		Read:   withCLIConfig(resourceManifestRead),
		Update: withCLIConfig(resourceManifestUpdate),
		Delete: withCLIConfig(resourceManifestDelete),

		Schema: map[string]*schema.Schema{
			"content": &schema.Schema{
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
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

type resourceHandler func(d *schema.ResourceData, m interface{}, kubectlCLIConfig *KubectlConfig) error
type resourceHandlerWithCLI func(d *schema.ResourceData, m interface{}) error

func HashResource(v interface{}) int {

	resource := v.(map[string]interface{})
	uuid := resource["uid"]
	return schema.HashString(uuid)
}

func withCLIConfig(resHandler resourceHandler) func(d *schema.ResourceData, m interface{}) error {

	return func(d *schema.ResourceData, m interface{}) error {
		kubectlCLIConfig, err := NewKubectlConfig(m)
		if err != nil {
			return fmt.Errorf("error while processing kubeconfig file: %s", err)
		}
		defer kubectlCLIConfig.Cleanup()

		return resHandler(d, m, kubectlCLIConfig)
	}
}

// The steps involved in creating a resource are:
// 	1. Splitting the yaml document in multiple resources
//  2. Update the resources:
//		- it applies any resource present in the schema (re-applies if needed)
//		- for each resource it creates it retrieves it and updates the state with its uid
func resourceManifestCreate(d *schema.ResourceData, m interface{}, kubectlCLIConfig *KubectlConfig) error {

	var namespace string

	if nm, ok := d.GetOk("namespace"); ok {
		namespace = nm.(string)
	}
	manifestResources, err := resource.SplitYAMLDocument(d.Get("content").(string))
	if err != nil {
		return err
	}
	tfResources, err := updateResources(manifestResources, namespace, kubectlCLIConfig)
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
//		- for each resource it creates it retrieves it and updates the state with its uid
//  3. Deletes the resources which where present in the old state but are not anymore
//
func resourceManifestUpdate(d *schema.ResourceData, m interface{}, kubectlCLIConfig *KubectlConfig) error {

	if d.HasChange("content") {

		var namespace string

		if nm, ok := d.GetOk("namespace"); ok {
			namespace = nm.(string)
		}
		tfOldResources := d.Get("resources").(*schema.Set)

		manifestResources, err := resource.SplitYAMLDocument(d.Get("content").(string))
		if err != nil {
			return err
		}
		tfResources, err := updateResources(manifestResources, namespace, kubectlCLIConfig)
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
func resourceManifestDelete(d *schema.ResourceData, m interface{}, kubectlCLIConfig *KubectlConfig) error {

	toDelete := d.Get("resources").(*schema.Set)
	err := deleteResources(toDelete, kubectlCLIConfig)
	return err
}

// Reads the resources using the resource handle provided
func resourceManifestRead(d *schema.ResourceData, m interface{}, kubectlCLIConfig *KubectlConfig) error {

	tfResources := d.Get("resources").(*schema.Set)
	tfResourcesList := tfResources.List()

	kubectlResources := schema.NewSet(HashResource, []interface{}{})

	for _, tfResource := range tfResourcesList {

		resourceObj, ok := tfResource.(map[string]interface{})
		if !ok {
			fmt.Errorf("error converting resource map\n")
		}
		selflink, ok := resourceObj["selflink"].(string)
		if !ok {
			fmt.Errorf("error converting resource selflink into string\n")
		}

		resource, namespace, ok := resourceFromSelflink(selflink)
		if !ok {
			return fmt.Errorf("invalid resource id: %s", selflink)
		}

		args := []string{"get", "--ignore-not-found", resource}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}

		stdout := &bytes.Buffer{}

		args = kubectlCLIConfig.RenderArgs(args...)
		getCommand := NewCLICommand("kubectl", args...)
		getCommand.Stdout = stdout
		if err := getCommand.RunCommand(); err != nil {
			return err
		}
		if strings.TrimSpace(stdout.String()) != "" {
			kubectlResources.Add(tfResource)
		}
	}

	commonResources := setIntersection(tfResources, kubectlResources)

	err := d.Set("resources", commonResources)
	if err != nil {
		return err
	}
	if commonResources.Len() < 1 {
		d.SetId("")
	}
	return nil
}

func deleteResources(manifestResources *schema.Set, kubectlCLIConfig *KubectlConfig) error {

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
		args := []string{"delete", resourceHandle}

		args = kubectlCLIConfig.RenderArgs(args...)
		if namespace != "" {
			args = append(args, "-n", namespace)
		}

		deleteCommand := NewCLICommand("kubectl", args...)
		deleteCommand.Stdin = strings.NewReader(selflink)

		err := deleteCommand.RunCommand()
		if err != nil {
			return err
		}
	}

	return nil
}

func updateResources(manifestResources []string, namespace string, kubectlCLIConfig *KubectlConfig) (*schema.Set, error) {

	tfResources := schema.NewSet(HashResource, []interface{}{})

	for _, manifestResource := range manifestResources {
		args := kubectlCLIConfig.RenderArgs("apply", "-f", "-")
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		applyCommand := NewCLICommand("kubectl", args...)
		applyCommand.Stdin = strings.NewReader(manifestResource)
		if err := applyCommand.RunCommand(); err != nil {
			return nil, err
		}

		stdout := &bytes.Buffer{}
		args = kubectlCLIConfig.RenderArgs("get", "-f", "-", "-o", "json")
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		getCommand := NewCLICommand("kubectl", args...)
		getCommand.Stdin = strings.NewReader(manifestResource)
		getCommand.Stdout = stdout

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
			return nil, fmt.Errorf("could not parse self-link from response %s", stdout.String())
		}
		uid := data.Items[0].Metadata.UID
		if uid == "" {
			return nil, fmt.Errorf("could not parse uid from response %s", stdout.String())
		}

		manifestResourceBase64 := base64.StdEncoding.EncodeToString([]byte(manifestResource))
		tfResources.Add(map[string]interface{}{"uid": uid, "selflink": selflink, "content": manifestResourceBase64})
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
