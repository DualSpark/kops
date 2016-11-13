package cloudup

import (
	"testing"

	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/kops/registry"
	"k8s.io/kops/upup/models"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awstasks"
	"k8s.io/kops/upup/pkg/fi/nodeup"
	"k8s.io/kops/upup/pkg/fi/fitasks"
)

func TestConfig_LimitYaml(t *testing.T) {

	channels := []string{"s3://s3-states/foo/addons/bootstrap-channel.yaml"}

	t.Logf("channels %v", channels)

	config := &nodeup.NodeUpConfig{Channels: channels}

	l := &Loader{}
	l.Init()


	l.Cluster = buildDefaultCluster(t)
	l.Cluster.Name = "foo"

	keyStore, err := registry.KeyStore(l.Cluster)
	if err != nil {
		t.Fatalf("error building keyStore for cluster: %v", err)
	}

	l.AddTypes(map[string]interface{}{
		"keypair":     &fitasks.Keypair{},
		"secret":      &fitasks.Secret{},
		"managedFile": &fitasks.ManagedFile{},
	})

	l.TemplateFunctions["CA"] = func() fi.CAStore {
		return keyStore
	}

	clusterTags, err := buildCloudupTags(l.Cluster)
	if err != nil {
		t.Fatalf("error building clusterTags for cluster: %v", err)
	}

	l.Tags = clusterTags

	// RenderNodeUpConfig returns the NodeUp config, in YAML format
	l.TemplateFunctions["RenderNodeUpConfig"] = func(ig *api.InstanceGroup) (string, error) {
		config.Channels = channels
		config.ClusterName = l.Cluster.Name
		yaml, err := api.ToYaml(config)
		if err != nil {
			t.Fatalf("error building ConfigBase for cluster: %v", err)
		}

		return string(yaml), nil
	}

	var groups []*api.InstanceGroup
	groups = append(groups, buildMinimalMasterInstanceGroup("us-mock-1a", "us-mock-1b", "us-mock-1c"))
	groups = append(groups, buildMinimalNodeInstanceGroup("us-mock-1a", "us-mock-1b"))

	l.TemplateFunctions["Masters"] = func() []*api.InstanceGroup {
		var groups []*api.InstanceGroup
		for _, ig := range groups {
			if !ig.IsMaster() {
				continue
			}
			groups = append(groups, ig)
		}
		return groups
	}

	l.TemplateFunctions["Region"] = func() string {
		return "foo"
	}
	l.TemplateFunctions["NodeSets"] = func() []*api.InstanceGroup {
		var groups []*api.InstanceGroup
		for _, ig := range groups {
			if ig.IsMaster() {
				continue
			}
			groups = append(groups, ig)
		}
		return groups
	}

	modelStore := models.NewAssetPath("")
	l.ModelStore = modelStore

	l.AddTypes(map[string]interface{}{
		// EC2
		"elasticIP":                   &awstasks.ElasticIP{},
		"instance":                    &awstasks.Instance{},
		"instanceElasticIPAttachment": &awstasks.InstanceElasticIPAttachment{},
		"instanceVolumeAttachment":    &awstasks.InstanceVolumeAttachment{},
		"ebsVolume":                   &awstasks.EBSVolume{},
		"sshKey":                      &awstasks.SSHKey{},

		// IAM
		"iamInstanceProfile":     &awstasks.IAMInstanceProfile{},
		"iamInstanceProfileRole": &awstasks.IAMInstanceProfileRole{},
		"iamRole":                &awstasks.IAMRole{},
		"iamRolePolicy":          &awstasks.IAMRolePolicy{},

		// VPC / Networking
		"dhcpOptions":           &awstasks.DHCPOptions{},
		"internetGateway":       &awstasks.InternetGateway{},
		"route":                 &awstasks.Route{},
		"routeTable":            &awstasks.RouteTable{},
		"routeTableAssociation": &awstasks.RouteTableAssociation{},
		"securityGroup":         &awstasks.SecurityGroup{},
		"securityGroupRule":     &awstasks.SecurityGroupRule{},
		"subnet":                &awstasks.Subnet{},
		"vpc":                   &awstasks.VPC{},
		"ngw":                   &awstasks.NatGateway{},
		"vpcDHDCPOptionsAssociation": &awstasks.VPCDHCPOptionsAssociation{},

		// ELB
		"loadBalancer":             &awstasks.LoadBalancer{},
		"loadBalancerAttachment":   &awstasks.LoadBalancerAttachment{},
		"loadBalancerHealthChecks": &awstasks.LoadBalancerHealthChecks{},

		// Autoscaling
		"autoscalingGroup":    &awstasks.AutoscalingGroup{},
		"launchConfiguration": &awstasks.LaunchConfiguration{},

		// Route53
		"dnsName": &awstasks.DNSName{},
		"dnsZone": &awstasks.DNSZone{},
	})

	taskMap, err := l.BuildTasks(modelStore, []string{"cloudup"})
	if err != nil {
		t.Fatalf("error building tasks: %v", err)
	}

	t.Logf("taskMap %v", taskMap)

}
