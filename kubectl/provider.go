package kubectl

import (
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
			return &Config{
				Kubeconfig:  d.Get("kubeconfig").(string),
				Kubecontent: d.Get("kubecontent").(string),
				Kubecontext: d.Get("kubecontext").(string),
			}, nil
		},
	}
}
