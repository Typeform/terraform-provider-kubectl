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

func (k *KubectlConfig) ShouldCleanUp() bool {
	return k.toCleanup
}

func (k *KubectlConfig) Cleanup() error {

	if k.ShouldCleanUp() {
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

func NewKubectlConfig(m interface{}) (*KubectlConfig, error) {
	var err error

	cleanup := false
	kubecontent := m.(*Config).Kubecontent
	kubeconfig := m.(*Config).Kubeconfig
	kubecontext := m.(*Config).Kubecontext

	if kubeconfig == "" && kubecontent != "" {
		kubeconfig, err = createTempfile(kubecontent)
		if kubeconfig != "" {
			cleanup = true
		}
	}

	return &KubectlConfig{kubeconfig, kubecontent, kubecontext, cleanup}, err
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
