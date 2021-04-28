/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
)

type HelmExecutor interface {
	InstallRelease(path, ns string) error
	DeleteRelease(path, ns string) error
}

// CmdHelmExecutor is HelmExecutor implementation using os/exec.Cmd
type CmdHelmExecutor struct {
	kubeconfig string
}

// HelmChart stores info about chart in filesystem
type HelmChart struct {
	name      string
	path      string
	namespace string
}

// InstallRelease calls "helm install" for chart with set args
// and creates namespace if not created
func (c *CmdHelmExecutor) InstallRelease(ch *HelmChart, args string) error {
	cmdStr := fmt.Sprintf("helm install "+
		"--kubeconfig %s "+
		"-n %s --create-namespace "+
		"%s "+"%s "+args,
		c.kubeconfig, ch.namespace, ch.name, ch.path)

	return execCmdStr(cmdStr)
}

// DeleteRelease call "helm delete" for chart
func (c *CmdHelmExecutor) DeleteRelease(ch *HelmChart) error {
	cmdStr := fmt.Sprintf("helm delete "+
		"--kubeconfig %s "+
		"-n %s "+"%s",
		c.kubeconfig, ch.namespace, ch.name)

	return execCmdStr(cmdStr)
}

func execCmdStr(cmdStr string) error {
	cmdStrSplit := strings.Split(cmdStr, " ")
	return execCmdObj(exec.Command(cmdStrSplit[0], cmdStrSplit[1:]...))
}

func execCmdObj(cmd *exec.Cmd) error {
	var stdout, stderr bytes.Buffer

	e2elog.Logf("Exec: %s", cmd.Args)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	e2elog.Logf("Stdout: %s", stdout.String())
	e2elog.Logf("Stderr: %s", stderr.String())

	return err
}
