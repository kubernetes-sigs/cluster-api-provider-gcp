package bootstrap

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"
	gceconfigv1 "sigs.k8s.io/cluster-api-provider-gcp/pkg/apis/gceproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud/google/machinesetup"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/cert"
	apierrors "sigs.k8s.io/cluster-api/pkg/errors"
	"sigs.k8s.io/cluster-api/pkg/kubeadm"
)

type GCEClientKubeadm interface {
	TokenCreate(params kubeadm.TokenCreateParams) (string, error)
}

type BashMetadataBuilder struct {
	machineSetupConfigGetter machinesetup.GCEClientMachineSetupConfigGetter
	kubeadm                  GCEClientKubeadm
	certificateAuthority     *cert.CertificateAuthority
}

type MetadataParams struct {
	MachineSetupConfigGetter machinesetup.GCEClientMachineSetupConfigGetter
	Kubeadm                  GCEClientKubeadm
	CertificateAuthority     *cert.CertificateAuthority
}

func NewBashMetadataBuilder(params MetadataParams) (MetadataBuilder, error) {
	b := &BashMetadataBuilder{
		kubeadm:                  params.Kubeadm,
		machineSetupConfigGetter: params.MachineSetupConfigGetter,
		certificateAuthority:     params.CertificateAuthority,
	}
	if b.kubeadm == nil {
		b.kubeadm = kubeadm.New()
	}

	return b, nil
}

func (b *BashMetadataBuilder) BuildMetadata(cluster *clusterv1.Cluster, machine *clusterv1.Machine, clusterConfig *gceconfigv1.GCEClusterProviderSpec, configParams *machinesetup.ConfigParams) (*compute.Metadata, error) {
	if b.machineSetupConfigGetter == nil {
		return nil, errors.New("a valid machineSetupConfigGetter is required")
	}

	var metadataMap map[string]string
	if machine.Spec.Versions.Kubelet == "" {
		return nil, errors.New("invalid master configuration: missing Machine.Spec.Versions.Kubelet")
	}
	machineSetupConfigs, err := b.machineSetupConfigGetter.GetMachineSetupConfig()
	if err != nil {
		return nil, err
	}
	machineSetupMetadata, err := machineSetupConfigs.GetMetadata(configParams)
	if err != nil {
		return nil, err
	}
	if IsMaster(configParams.Roles) {
		if machine.Spec.Versions.ControlPlane == "" {
			return nil, apierrors.InvalidMachineConfiguration(
				"invalid master configuration: missing Machine.Spec.Versions.ControlPlane")
		}
		var err error
		metadataMap, err = masterMetadata(cluster, machine, clusterConfig.Project, &machineSetupMetadata)
		if err != nil {
			return nil, err
		}
		ca := b.certificateAuthority
		if ca != nil {
			metadataMap["ca-cert"] = base64.StdEncoding.EncodeToString(ca.Certificate)
			metadataMap["ca-key"] = base64.StdEncoding.EncodeToString(ca.PrivateKey)
		}
	} else {
		var err error
		kubeadmToken, err := b.getKubeadmToken()
		if err != nil {
			return nil, err
		}
		metadataMap, err = nodeMetadata(kubeadmToken, cluster, machine, clusterConfig.Project, &machineSetupMetadata)
		if err != nil {
			return nil, err
		}
	}

	{
		var b strings.Builder

		project := clusterConfig.Project

		clusterName := cluster.Name
		nodeTag := clusterName + "-worker"

		network := "default"
		subnetwork := "kubernetes"

		fmt.Fprintf(&b, "[global]\n")
		fmt.Fprintf(&b, "project-id = %s\n", project)
		fmt.Fprintf(&b, "network-name = %s\n", network)
		fmt.Fprintf(&b, "subnetwork-name = %s\n", subnetwork)
		fmt.Fprintf(&b, "node-tags = %s\n", nodeTag)

		metadataMap["cloud-config"] = b.String()
	}

	var metadataItems []*compute.MetadataItems
	for k, v := range metadataMap {
		v := v // rebind scope to avoid loop aliasing below
		metadataItems = append(metadataItems, &compute.MetadataItems{
			Key:   k,
			Value: &v,
		})
	}
	metadata := compute.Metadata{
		Items: metadataItems,
	}
	return &metadata, nil
}

func (b *BashMetadataBuilder) getKubeadmToken() (string, error) {
	tokenParams := kubeadm.TokenCreateParams{
		Ttl: time.Duration(10) * time.Minute,
	}
	output, err := b.kubeadm.TokenCreate(tokenParams)
	if err != nil {
		glog.Errorf("unable to create token: %v [%s]", err, output)
		return "", err
	}
	return strings.TrimSpace(output), err
}
