package kubectl

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
)

type Config struct {
	Kubeconfig  string
	Kubecontent string
	Kubecontext string
}

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"kubeconfig": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"kubecontent": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"kubecontext": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"kubectl_manifest": resourceManifest(),
		},
		ConfigureFunc: func(d *schema.ResourceData) (interface{}, error) {
			config := &Config{
				Kubeconfig:  d.Get("kubeconfig").(string),
				Kubecontent: d.Get("kubecontent").(string),
				Kubecontext: d.Get("kubecontext").(string),
			}
			kubectlCLIConfig, err := NewKubectlConfig(config)
			if err != nil {
				return nil, fmt.Errorf(
					"error while processing kubeconfig file: %s", err,
				)
			}
			defer kubectlCLIConfig.Cleanup()
			return kubectlCLIConfig, nil
		},
	}
}
