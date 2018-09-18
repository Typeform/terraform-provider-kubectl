package kubectl

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type KubectlConfig struct {
	Kubeconfig  string
	Kubecontent string
	Kubecontext string
	toCleanup   bool
}

func (k *KubectlConfig) Cleanup() error {

	if k.toCleanup {
		if err := deleteFile(k.Kubeconfig); err != nil {
			errorString := `Cleanup operation on temporary file failed with error %s.
			Plese make sure (if it exists) that the file %s is manually removed.`

			return fmt.Errorf(errorString, err, k.Kubeconfig)
		}
	}
	return nil
}

func (k *KubectlConfig) RenderArgs(args ...string) []string {

	if k.Kubeconfig != "" {
		args = append([]string{"--kubeconfig", k.Kubeconfig}, args...)
	}
	if k.Kubecontext != "" {
		args = append([]string{"--context", k.Kubecontext}, args...)
	}
	return args
}

func (k *KubectlConfig) InitializeConfiguration() error {
	var kubeconfig string
	var err error

	if k.Kubecontent != "" {
		kubeconfig, err = createTempfile(k.Kubecontent)
		k.toCleanup = true
	}
	if kubeconfig != "" {
		k.Kubeconfig = kubeconfig
	}
	return err
}

func NewKubectlConfig(m interface{}) (*KubectlConfig, error) {
	var err error

	kubecontent := m.(*Config).Kubecontent
	kubeconfig := m.(*Config).Kubeconfig
	kubecontext := m.(*Config).Kubecontext

	kubectlConfig := &KubectlConfig{
		Kubeconfig:  kubeconfig,
		Kubecontent: kubecontent,
		Kubecontext: kubecontext,
		toCleanup:   false,
	}

	err = kubectlConfig.InitializeConfiguration()
	return kubectlConfig, err
}

type CLICommand struct {
	*exec.Cmd
}

func NewCLICommand(name string, args ...string) *CLICommand {

	cmd := exec.Command(name, args...)
	return &CLICommand{cmd}
}

func (c *CLICommand) RunCommand() error {
	stderr := &bytes.Buffer{}
	c.Cmd.Stderr = stderr
	if err := c.Cmd.Run(); err != nil {
		cmdStr := c.Cmd.Path + " " + strings.Join(c.Cmd.Args, " ")
		if stderr.Len() == 0 {
			return fmt.Errorf("%s: %v", cmdStr, err)
		}
		return fmt.Errorf("%s %v: %s", cmdStr, err, stderr.Bytes())
	}
	return nil
}

type CLICommandFactory struct {
	KubectlConfig *KubectlConfig
}

func (c *CLICommandFactory) CreateGetByHandleCommand(
	resourceHandle, namespace string, stdout *bytes.Buffer) *CLICommand {

	args := []string{"get", "--ignore-not-found", resourceHandle}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	args = c.KubectlConfig.RenderArgs(args...)
	getCommand := NewCLICommand("kubectl", args...)
	getCommand.Stdout = stdout
	return getCommand
}

func (c *CLICommandFactory) CreateGetByManifestCommand(
	resourceManifest, namespace string, stdout *bytes.Buffer) *CLICommand {

	args := c.KubectlConfig.RenderArgs("get", "-f", "-", "-o", "json")
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	getCommand := NewCLICommand("kubectl", args...)
	getCommand.Stdin = strings.NewReader(resourceManifest)
	getCommand.Stdout = stdout
	return getCommand
}

func (c *CLICommandFactory) CreateApplyManifestCommand(
	manifestResource, namespace string) *CLICommand {

	args := c.KubectlConfig.RenderArgs("apply", "-f", "-")
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	applyCommand := NewCLICommand("kubectl", args...)
	applyCommand.Stdin = strings.NewReader(manifestResource)
	return applyCommand
}

func (c *CLICommandFactory) CreateDeleteByHandleCommand(
	resourceHandle, namespace string) *CLICommand {

	args := []string{"delete", resourceHandle}

	args = c.KubectlConfig.RenderArgs(args...)
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	deleteCommand := NewCLICommand("kubectl", args...)
	deleteCommand.Stdin = strings.NewReader(resourceHandle)
	return deleteCommand
}
