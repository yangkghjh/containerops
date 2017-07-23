/*
Copyright 2016 - 2017 Huawei Technologies Co., Ltd. All rights reserved.

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

package module

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	. "github.com/logrusorgru/aurora"
	homeDir "github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v2"

	"github.com/Huawei/containerops/common"
	"github.com/Huawei/containerops/common/utils"
	"github.com/Huawei/containerops/singular/module/service"
)

// JSON export deployment data
func (d *Deployment) JSON() ([]byte, error) {
	return json.Marshal(&d)
}

//
func (d *Deployment) YAML() ([]byte, error) {
	return yaml.Marshal(&d)
}

//
func (d *Deployment) URIs() (namespace, repository, name string, err error) {
	array := strings.Split(d.URI, "/")

	if len(array) != 3 {
		return "", "", "", fmt.Errorf("Invalid deployment URI: %s", d.URI)
	}

	namespace, repository, name = array[0], array[1], array[2]

	return namespace, repository, name, nil
}

// TODO filter the log print with different color.
func (d *Deployment) Log(log string) {
	d.Logs = append(d.Logs, fmt.Sprintf("[%s] %s", time.Now().String(), log))

	if d.Verbose == true {
		if d.Timestamp == true {
			fmt.Println(Cyan(fmt.Sprintf("[%s] %s", time.Now().String(), strings.TrimSpace(log))))
		} else {
			fmt.Println(Cyan(log))
		}
	}
}

func (d *Deployment) Output(key, value string) {
	if d.Outputs == nil {
		d.Outputs = map[string]interface{}{}
	}

	d.Outputs[key] = value
}

// ParseFromFile
func (d *Deployment) ParseFromFile(t string, verbose, timestamp bool) error {
	if data, err := ioutil.ReadFile(t); err != nil {
		d.Log(fmt.Sprintf("Read deployment template file %s error: %s", t, err.Error()))
		return err
	} else {
		if err := yaml.Unmarshal(data, &d); err != nil {
			d.Log(fmt.Sprintf("Unmarshal the template file error: %s", err.Error()))
			return err
		}

		d.Verbose, d.Timestamp = verbose, timestamp

		if err := d.InitConfigPath(""); err != nil {
			return err
		}
	}

	return nil
}

func (d *Deployment) InitConfigPath(path string) error {
	if path == "" {
		home, _ := homeDir.Dir()
		d.Config = fmt.Sprintf("%s/.containerops/singular", home)
	}

	if utils.IsDirExist(d.Config) == false {
		if err := os.MkdirAll(d.Config, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

// Check Sequence: CheckServiceAuth -> TODO Check Other?
func (d *Deployment) Check() error {
	if err := d.CheckServiceAuth(); err != nil {
		return fmt.Errorf("check template or configuration error: %s ", err.Error())
	}

	return nil
}

// CheckServiceAuth
func (d *Deployment) CheckServiceAuth() error {
	if d.Service.Provider == "" || d.Service.Token == "" {
		if common.Singular.Provider == "" || common.Singular.Token == "" {
			return fmt.Errorf("Should provide infra service and auth token in %s", "deploy template, or configuration file")
		} else {
			d.Service.Provider, d.Service.Token = common.Singular.Provider, common.Singular.Token
		}
	}

	return nil
}

// Check SSH private and public key files
func (d *Deployment) CheckSSHKey() error {
	if utils.IsFileExist(d.Tools.SSH.Public) == false || utils.IsFileExist(d.Tools.SSH.Private) {
		return fmt.Errorf("Should provide SSH public and private key files in deploy process")
	}

	return nil
}

// Deploy Sequence: Preparing SSH Key files -> Preparing VM -> Preparing SSL root Key files -> Deploy Etcd
//   -> Deploy flannel -> Deploy k8s Master -> Deploy k8s node -> TODO Deploy other...
func (d *Deployment) Deploy() error {
	// Preparing SSH Keys
	if d.Tools.SSH.Public == "" || d.Tools.SSH.Private == "" {
		if public, private, fingerprint, err := CreateSSHKeyFiles(d.Config); err != nil {
			return err
		} else {
			d.Log(fmt.Sprintf(
				"Generate SSH key files successfully, fingerprint is %s\nPublic key file @ %s\nPrivate key file @ %s",
				fingerprint, public, private))
			d.Tools.SSH.Public, d.Tools.SSH.Private, d.Tools.SSH.Fingerprint = public, private, fingerprint
		}
	}

	switch d.Service.Provider {
	case "digitalocean":
		do := new(service.DigitalOcean)
		do.Token = d.Service.Token
		do.Region, do.Size, do.Image = d.Service.Region, d.Service.Size, d.Service.Image

		// Init DigitalOcean API client.
		do.InitClient()

		// Upload ssh public key
		if err := do.UpdateSSHKey(d.Tools.SSH.Public); err != nil {
			return err
		}

		// Create DigitalOcean Droplets
		if err := do.CreateDroplet(d.Nodes, d.Tools.SSH.Fingerprint); err != nil {
			return err
		}

		i := 0
		for key, _ := range do.Droplets {
			d.Log(fmt.Sprintf("Node %d created successfully, IP: %s", i, key))
			d.Output(fmt.Sprintf("NODE_%d", i), key)
			i += 1
		}

		fmt.Println(Cyan("Waiting 60 seconds for preparing droplets..."))
		time.Sleep(60 * time.Second)

		// Generate CA Root files
		if roots, err := GenerateCARootFiles(d.Config); err != nil {
			return err
		} else {
			d.Log("CA Root files generated successfully")

			for key, value := range roots {
				d.Output(key, value)
			}

			for ip, _ := range do.Droplets {
				if err := UploadCARootFiles(d.Config, roots, ip); err != nil {
					return err
				}
			}

		}

		for _, infra := range d.Infras {
			switch infra.Name {
			case "etcd":
				if err := d.DeployEtcd(infra); err != nil {
					return err
				}
			case "flannel":
				if err := d.DeployFlannel(infra); err != nil {
					return err
				}
			case "docker":
				if err := d.DeployDocker(infra); err != nil {
					return err
				}
			case "kubernetes":
				if err := d.DeployKubernetes(infra); err != nil {
					return err
				}
			default:
				return fmt.Errorf("Unsupport infrastruction software: %s", infra)
			}

		}

	default:
		return fmt.Errorf("Unsupport service provide: %s", d.Service.Provider)

	}

	return nil
}

// DeployEtcd is function deployment etcd cluster.
// Notes:
//   1. Only count master nodes in etcd deploy process.
//   2.
func (d *Deployment) DeployEtcd(infra Infra) error {
	if infra.Nodes.Master > d.Nodes {
		return fmt.Errorf("Deploy %s nodes more than %d", infra.Name, d.Nodes)
	}

	etcdNodes := map[string]string{}
	etcdEndpoints, etcdAdminEndpoints := []string{}, []string{}

	for i := 0; i < infra.Nodes.Master; i++ {
		etcdNodes[fmt.Sprintf("etcd-node-%d", i)] = d.Outputs[fmt.Sprintf("NODE_%d", i)].(string)
		etcdEndpoints = append(etcdEndpoints,
			fmt.Sprintf("https://%s:2379", d.Outputs[fmt.Sprintf("NODE_%d", i)].(string)))
		etcdAdminEndpoints = append(etcdAdminEndpoints,
			fmt.Sprintf("%s=https://%s:2380", fmt.Sprintf("etcd-node-%d", i),
				d.Outputs[fmt.Sprintf("NODE_%d", i)].(string)))
	}

	d.Output("EtcdEndpoints", strings.Join(etcdEndpoints, ","))

	if err := GenerateEtcdFiles(d.Config, etcdNodes, strings.Join(etcdAdminEndpoints, ","), infra.Version); err != nil {
		return err
	} else {
		if err := UploadEtcdCAFiles(d.Config, etcdNodes); err != nil {
			return err
		}

		for _, c := range infra.Components {
			if err := d.DownloadBinaryFile(c.Binary, c.URL, etcdNodes); err != nil {
				return err
			}
		}

		if err := StartEtcdCluster(d.Tools.SSH.Private, etcdNodes); err != nil {
			return err
		}

	}

	return nil
}

func (d *Deployment) DownloadBinaryFile(file, url string, nodes map[string]string) error {
	for _, ip := range nodes {
		downloadCmd := fmt.Sprintf("curl %s -o /usr/local/bin/%s", url, file)
		chmodCmd := fmt.Sprintf("chmod +x /usr/local/bin/%s", file)

		if err := utils.SSHCommand("root", d.Tools.SSH.Private, ip, 22, downloadCmd, os.Stdout, os.Stderr); err != nil {
			return err
		}

		if err := utils.SSHCommand("root", d.Tools.SSH.Private, ip, 22, chmodCmd, os.Stdout, os.Stderr); err != nil {
			return err
		}

	}

	return nil
}

func (d *Deployment) DeployFlannel(infra Infra) error {
	flanneldNodes := map[string]string{}
	for i := 0; i < infra.Nodes.Master; i++ {
		flanneldNodes[fmt.Sprintf("flanneld-node-%d", i)] = d.Outputs[fmt.Sprintf("NODE_%d", i)].(string)
	}

	if err := GenerateFlanneldFiles(d.Config, flanneldNodes, d.Outputs["EtcdEndpoints"].(string), infra.Version); err != nil {
		return err
	} else {
		if err := UploadFlanneldCAFiles(d.Config, flanneldNodes); err != nil {
			return err
		}

		for i, c := range infra.Components {
			if err := d.DownloadBinaryFile(c.Binary, c.URL, flanneldNodes); err != nil {
				return err
			}

			if c.Before != "" && i == 0 {
				if err := BeforeFlanneldExecute(d.Tools.SSH.Private, d.Outputs[fmt.Sprintf("NODE_%d", i)].(string), c.Before, d.Outputs["EtcdEndpoints"].(string)); err != nil {
					return err
				}
			}
		}

		if err := StartFlanneldCluster(d.Tools.SSH.Private, flanneldNodes); err != nil {
			return err
		}
	}

	return nil
}

func (d *Deployment) DeployDocker(infra Infra) error {
	dockerNodes := map[string]string{}
	for i := 0; i < infra.Nodes.Master; i++ {
		dockerNodes[fmt.Sprintf("docker-node-%d", i)] = d.Outputs[fmt.Sprintf("NODE_%d", i)].(string)
	}

	if err := GenerateDockerFiles(d.Config, dockerNodes, infra.Version); err != nil {
		return err
	} else {
		if err := UploadDockerCAFiles(d.Config, dockerNodes); err != nil {
			return err
		}

		for i, c := range infra.Components {
			if err := d.DownloadBinaryFile(c.Binary, c.URL, dockerNodes); err != nil {
				return err
			}

			if c.Before != "" {
				if err := BeforeDockerExecute(d.Tools.SSH.Private, d.Outputs[fmt.Sprintf("NODE_%d", i)].(string), c.Before); err != nil {
					return err
				}
			}
		}

		for i, _ := range dockerNodes {
			if err := StartDockerDaemon(d.Tools.SSH.Private, d.Outputs[fmt.Sprintf("NODE_%d", i)].(string)); err != nil {
				return err
			}
		}

		for i, c := range infra.Components {
			if c.After != "" {
				if err := AfterDockerExecute(d.Tools.SSH.Private, d.Outputs[fmt.Sprintf("NODE_%d", i)].(string), c.After); err != nil {
					return err
				}
			}
		}

	}

	return nil
}

func (d *Deployment) DeployKubernetes(infra Infra) error {
	return nil
}