# Kubectl Terraform Provider

The kubectl Kubectl provider enables Terraform to deploy Kubernetes resources. Unlike the [official Kubernetes provider][kubernetes-provider] it handles raw manifests, leveraging `kubectl` directly to allow developers to work with any Kubernetes resource natively.

## Usage

Use `go get` to install the provider:

```
go get -u github.com/Typeform/terraform-provider-kubectl
```

Register the plugin in `~/.terraformrc`:

```hcl
providers {
  kubectl = "/$GOPATH/bin/terraform-provider-kubectl"
}
```

The provider takes optional configuration to specify a `kubeconfig` file:

```hcl
provider "kubectl" {
  kubeconfig     = "/path/to/kubeconfig"
  kubecontext    = <context within kubeconfig> #optional
}

or

provider "kubectl" {
  kubecontent = <base64 encoded kubeconfig>
  kubecontext    = <context within kubeconfig> #optional
}
```

The k8s Terraform provider introduces a single Terraform resource, a `k8s_manifest`. The resource contains a `content` field, which contains a raw manifest.

```hcl
variable "replicas" {
  type    = "string"
  default = 3
}

data "template_file" "nginx-deployment" {
  template = "${file("manifests/nginx-deployment.yaml")}"

  vars {
    replicas = "${var.replicas}"
  }
}

resource "k8s_manifest" "nginx-deployment" {
  content = "${data.template_file.nginx-deployment.rendered}"
}
```

In this case `manifests/nginx-deployment.yaml` is a templated deployment manifest.

```yaml
apiVersion: apps/v1beta2
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.7.9
        ports:
        - containerPort: 80
```

The Kubernetes resources can then be managed through Terraform.

```terminal
$ terraform apply
# ...
Apply complete! Resources: 1 added, 1 changed, 0 destroyed.
$ kubectl get deployments
NAME               DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
nginx-deployment   3         3         3            3           1m
$ terraform apply -var 'replicas=5'
# ...
Apply complete! Resources: 0 added, 1 changed, 0 destroyed.
$ kubectl get deployments
NAME               DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
nginx-deployment   5         5         5            3           3m
$ terraform destroy -force
# ...
Destroy complete! Resources: 2 destroyed.
$ kubectl get deployments
No resources found.
```

[kubernetes-provider]: https://www.terraform.io/docs/providers/kubernetes/index.html
