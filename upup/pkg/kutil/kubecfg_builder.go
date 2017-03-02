/*
Copyright 2016 The Kubernetes Authors.

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

package kutil

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/client-go/tools/clientcmd"
	k8sapi "k8s.io/client-go/tools/clientcmd/api"
)

// KubeconfigBuilder builds a kubecfg file
// This logic previously lives in the bash scripts (create-kubeconfig in cluster/common.sh)
type KubeconfigBuilder struct {
	KubeMasterIP string

	Context   string
	Namespace string

	KubeBearerToken string
	KubeUser        string
	KubePassword    string

	CACert     []byte
	ClientCert []byte
	ClientKey  []byte

	configAccess clientcmd.ConfigAccess
}

// Create new KubeconfigBuilder
func NewKubeconfigBuilder() *KubeconfigBuilder {
	c := &KubeconfigBuilder{}
	c.configAccess = clientcmd.NewDefaultPathOptions()
	return c
}

func (b *KubeconfigBuilder) DeleteKubeConfig() error {
	config, err := b.configAccess.GetStartingConfig()
	if err != nil {
		return fmt.Errorf("error loading kubeconfig: %v", err)
	}

	if config == nil || k8sapi.IsConfigEmpty(config) {
		glog.V(2).Infof("kubeconfig is empty")
		return nil
	}

	delete(config.Clusters, b.Context)
	delete(config.AuthInfos, b.Context)
	delete(config.AuthInfos, fmt.Sprintf("%s-basic-auth", b.Context))
	delete(config.Contexts, b.Context)

	if config.CurrentContext == b.Context {
		config.CurrentContext = ""
	}

	if err := clientcmd.ModifyConfig(b.configAccess, *config, false); err != nil {
		return fmt.Errorf("error writing kubeconfig: %v", err)
	}

	fmt.Printf("Deleted kubectl config for %s\n", b.Context)
	return nil
}

// Create new Rest Client
func (c *KubeconfigBuilder) BuildRestConfig() (*restclient.Config, error) {
	restConfig := &restclient.Config{
		Host: "https://" + c.KubeMasterIP,
	}
	restConfig.CAData = c.CACert
	restConfig.CertData = c.ClientCert
	restConfig.KeyData = c.ClientKey

	// username/password or bearer token may be set, but not both
	if c.KubeBearerToken != "" {
		restConfig.BearerToken = c.KubeBearerToken
	} else {
		restConfig.Username = c.KubeUser
		restConfig.Password = c.KubePassword
	}

	return restConfig, nil
}

// Write out a new kubeconfig
func (b *KubeconfigBuilder) WriteKubecfg() error {
	config, err := b.configAccess.GetStartingConfig()
	if err != nil {
		return fmt.Errorf("error reading kubeconfig: %v", err)
	}

	if config == nil {
		config = &k8sapi.Config{}
	}

	{
		cluster := config.Clusters[b.Context]
		if cluster == nil {
			cluster = k8sapi.NewCluster()
		}
		cluster.Server = "https://" + b.KubeMasterIP

		if b.CACert == nil {
			cluster.InsecureSkipTLSVerify = true
			cluster.CertificateAuthority = ""
			cluster.CertificateAuthorityData = nil
		} else {
			cluster.InsecureSkipTLSVerify = false
			cluster.CertificateAuthority = ""
			cluster.CertificateAuthorityData = b.CACert
		}

		if config.Clusters == nil {
			config.Clusters = make(map[string]*k8sapi.Cluster)
		}
		config.Clusters[b.Context] = cluster
	}

	{
		authInfo := config.AuthInfos[b.Context]
		if authInfo == nil {
			authInfo = k8sapi.NewAuthInfo()
		}

		if b.KubeBearerToken != "" {
			authInfo.Token = b.KubeBearerToken
		} else if b.KubeUser != "" && b.KubePassword != "" {
			authInfo.Username = b.KubeUser
			authInfo.Password = b.KubePassword
		}

		if b.ClientCert != nil && b.ClientKey != nil {
			authInfo.ClientCertificate = ""
			authInfo.ClientCertificateData = b.ClientCert
			authInfo.ClientKey = ""
			authInfo.ClientKeyData = b.ClientKey
		}

		if config.AuthInfos == nil {
			config.AuthInfos = make(map[string]*k8sapi.AuthInfo)
		}
		config.AuthInfos[b.Context] = authInfo
	}

	// If we have a bearer token, also create a credential entry with basic auth
	// so that it is easy to discover the basic auth password for your cluster
	// to use in a web browser.
	if b.KubeUser != "" && b.KubePassword != "" {
		name := b.Context + "-basic-auth"
		authInfo := config.AuthInfos[name]
		if authInfo == nil {
			authInfo = k8sapi.NewAuthInfo()
		}

		authInfo.Username = b.KubeUser
		authInfo.Password = b.KubePassword

		if config.AuthInfos == nil {
			config.AuthInfos = make(map[string]*k8sapi.AuthInfo)
		}
		config.AuthInfos[name] = authInfo
	}

	{
		context := config.Contexts[b.Context]
		if context == nil {
			context = k8sapi.NewContext()
		}

		context.Cluster = b.Context
		context.AuthInfo = b.Context

		if b.Namespace != "" {
			context.Namespace = b.Namespace
		}

		if config.Contexts == nil {
			config.Contexts = make(map[string]*k8sapi.Context)
		}
		config.Contexts[b.Context] = context
	}

	config.CurrentContext = b.Context

	if err := clientcmd.ModifyConfig(b.configAccess, *config, true); err != nil {
		return err
	}

	fmt.Printf("Kops has set your kubectl context to %s\n", b.Context)
	return nil
}
