package tls

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"net"

	"github.com/metalkube/kni-installer/pkg/asset"
	"github.com/metalkube/kni-installer/pkg/asset/installconfig"
	"github.com/pkg/errors"
)

// APIServerCertKey is the asset that generates the API server key/cert pair.
// [DEPRECATED]
type APIServerCertKey struct {
	SignedCertKey
}

var _ asset.Asset = (*APIServerCertKey)(nil)

// Dependencies returns the dependency of the the cert/key pair, which includes
// the parent CA, and install config if it depends on the install config for
// DNS names, etc.
func (a *APIServerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeCA{},
		&installconfig.InstallConfig{},
	}
}

// Generate generates the cert/key pair based on its dependencies.
func (a *APIServerCertKey) Generate(dependencies asset.Parents) error {
	kubeCA := &KubeCA{}
	installConfig := &installconfig.InstallConfig{}
	dependencies.Get(kubeCA, installConfig)

	apiServerAddress, err := cidrhost(installConfig.Config.Networking.ServiceCIDR.IPNet, 1)
	if err != nil {
		return errors.Wrap(err, "failed to get API Server address from InstallConfig")
	}

	cfg := &CertCfg{
		Subject:      pkix.Name{CommonName: "system:kube-apiserver", Organization: []string{"kube-master"}},
		KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		Validity:     ValidityTenYears,
		DNSNames: []string{
			apiAddress(installConfig.Config),
			"kubernetes", "kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
			"localhost",
		},
		IPAddresses: []net.IP{net.ParseIP(apiServerAddress), net.ParseIP("127.0.0.1")},
	}

	return a.SignedCertKey.Generate(cfg, kubeCA, "apiserver", AppendParent)
}

// Name returns the human-friendly name of the asset.
func (a *APIServerCertKey) Name() string {
	return "Certificate (kube-apiaserver)"
}

// KubeAPIServerToKubeletSignerCertKey is a key/cert pair that signs the kube-apiserver to kubelet client certs.
type KubeAPIServerToKubeletSignerCertKey struct {
	SelfSignedCertKey
}

var _ asset.WritableAsset = (*KubeAPIServerToKubeletSignerCertKey)(nil)

// Dependencies returns the dependency of the root-ca, which is empty.
func (c *KubeAPIServerToKubeletSignerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate generates the root-ca key and cert pair.
func (c *KubeAPIServerToKubeletSignerCertKey) Generate(parents asset.Parents) error {
	cfg := &CertCfg{
		Subject:   pkix.Name{CommonName: "kube-apiserver-to-kubelet-signer", OrganizationalUnit: []string{"openshift"}},
		KeyUsages: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		Validity:  ValidityOneYear,
		IsCA:      true,
	}

	return c.SelfSignedCertKey.Generate(cfg, "kube-apiserver-to-kubelet-signer")
}

// Name returns the human-friendly name of the asset.
func (c *KubeAPIServerToKubeletSignerCertKey) Name() string {
	return "Certificate (kube-apiserver-to-kubelet-signer)"
}

// KubeAPIServerToKubeletCABundle is the asset the generates the kube-apiserver-to-kubelet-ca-bundle,
// which contains all the individual client CAs.
type KubeAPIServerToKubeletCABundle struct {
	CertBundle
}

var _ asset.Asset = (*KubeAPIServerToKubeletCABundle)(nil)

// Dependencies returns the dependency of the cert bundle.
func (a *KubeAPIServerToKubeletCABundle) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerToKubeletSignerCertKey{},
	}
}

// Generate generates the cert bundle based on its dependencies.
func (a *KubeAPIServerToKubeletCABundle) Generate(deps asset.Parents) error {
	var certs []CertInterface
	for _, asset := range a.Dependencies() {
		deps.Get(asset)
		certs = append(certs, asset.(CertInterface))
	}
	return a.CertBundle.Generate("kube-apiserver-to-kubelet-ca-bundle", certs...)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerToKubeletCABundle) Name() string {
	return "Certificate (kube-apiserver-to-kubelet-ca-bundle)"
}

// KubeAPIServerToKubeletClientCertKey is the asset that generates the kube-apiserver to kubelet client key/cert pair.
type KubeAPIServerToKubeletClientCertKey struct {
	SignedCertKey
}

var _ asset.Asset = (*KubeAPIServerToKubeletClientCertKey)(nil)

// Dependencies returns the dependency of the the cert/key pair
func (a *KubeAPIServerToKubeletClientCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerToKubeletSignerCertKey{},
	}
}

// Generate generates the cert/key pair based on its dependencies.
func (a *KubeAPIServerToKubeletClientCertKey) Generate(dependencies asset.Parents) error {
	ca := &KubeAPIServerToKubeletSignerCertKey{}
	dependencies.Get(ca)

	cfg := &CertCfg{
		Subject:      pkix.Name{CommonName: "system:kube-apiserver", Organization: []string{"kube-master"}},
		KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Validity:     ValidityOneYear,
	}

	return a.SignedCertKey.Generate(cfg, ca, "kube-apiserver-to-kubelet-client", DoNotAppendParent)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerToKubeletClientCertKey) Name() string {
	return "Certificate (kube-apiserver-to-kubelet-client)"
}

// KubeAPIServerLocalhostSignerCertKey is a key/cert pair that signs the kube-apiserver server cert for SNI localhost.
type KubeAPIServerLocalhostSignerCertKey struct {
	SelfSignedCertKey
}

var _ asset.WritableAsset = (*KubeAPIServerLocalhostSignerCertKey)(nil)

// Dependencies returns the dependency of the root-ca, which is empty.
func (c *KubeAPIServerLocalhostSignerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate generates the root-ca key and cert pair.
func (c *KubeAPIServerLocalhostSignerCertKey) Generate(parents asset.Parents) error {
	cfg := &CertCfg{
		Subject:   pkix.Name{CommonName: "kube-apiserver-localhost-signer", OrganizationalUnit: []string{"openshift"}},
		KeyUsages: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		Validity:  ValidityTenYears,
		IsCA:      true,
	}

	return c.SelfSignedCertKey.Generate(cfg, "kube-apiserver-localhost-signer")
}

// Name returns the human-friendly name of the asset.
func (c *KubeAPIServerLocalhostSignerCertKey) Name() string {
	return "Certificate (kube-apiserver-localhost-signer)"
}

// KubeAPIServerLocalhostCABundle is the asset the generates the kube-apiserver-localhost-ca-bundle,
// which contains all the individual client CAs.
type KubeAPIServerLocalhostCABundle struct {
	CertBundle
}

var _ asset.Asset = (*KubeAPIServerLocalhostCABundle)(nil)

// Dependencies returns the dependency of the cert bundle.
func (a *KubeAPIServerLocalhostCABundle) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerLocalhostSignerCertKey{},
	}
}

// Generate generates the cert bundle based on its dependencies.
func (a *KubeAPIServerLocalhostCABundle) Generate(deps asset.Parents) error {
	var certs []CertInterface
	for _, asset := range a.Dependencies() {
		deps.Get(asset)
		certs = append(certs, asset.(CertInterface))
	}
	return a.CertBundle.Generate("kube-apiserver-localhost-ca-bundle", certs...)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerLocalhostCABundle) Name() string {
	return "Certificate (kube-apiserver-localhost-ca-bundle)"
}

// KubeAPIServerLocalhostServerCertKey is the asset that generates the kube-apiserver serving key/cert pair for SNI localhost.
type KubeAPIServerLocalhostServerCertKey struct {
	SignedCertKey
}

var _ asset.Asset = (*KubeAPIServerLocalhostServerCertKey)(nil)

// Dependencies returns the dependency of the the cert/key pair
func (a *KubeAPIServerLocalhostServerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerLocalhostSignerCertKey{},
	}
}

// Generate generates the cert/key pair based on its dependencies.
func (a *KubeAPIServerLocalhostServerCertKey) Generate(dependencies asset.Parents) error {
	ca := &KubeAPIServerLocalhostSignerCertKey{}
	dependencies.Get(ca)

	cfg := &CertCfg{
		Subject:      pkix.Name{CommonName: "system:kube-apiserver", Organization: []string{"kube-master"}},
		KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		Validity:     ValidityOneDay,
		DNSNames: []string{
			"localhost",
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	return a.SignedCertKey.Generate(cfg, ca, "kube-apiserver-localhost-server", AppendParent)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerLocalhostServerCertKey) Name() string {
	return "Certificate (kube-apiserver-localhost-server)"
}

// KubeAPIServerServiceNetworkSignerCertKey is a key/cert pair that signs the kube-apiserver server cert for SNI service network.
type KubeAPIServerServiceNetworkSignerCertKey struct {
	SelfSignedCertKey
}

var _ asset.WritableAsset = (*KubeAPIServerServiceNetworkSignerCertKey)(nil)

// Dependencies returns the dependency of the root-ca, which is empty.
func (c *KubeAPIServerServiceNetworkSignerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate generates the root-ca key and cert pair.
func (c *KubeAPIServerServiceNetworkSignerCertKey) Generate(parents asset.Parents) error {
	cfg := &CertCfg{
		Subject:   pkix.Name{CommonName: "kube-apiserver-service-network-signer", OrganizationalUnit: []string{"openshift"}},
		KeyUsages: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		Validity:  ValidityTenYears,
		IsCA:      true,
	}

	return c.SelfSignedCertKey.Generate(cfg, "kube-apiserver-service-network-signer")
}

// Name returns the human-friendly name of the asset.
func (c *KubeAPIServerServiceNetworkSignerCertKey) Name() string {
	return "Certificate (kube-apiserver-service-network-signer)"
}

// KubeAPIServerServiceNetworkCABundle is the asset the generates the kube-apiserver-service-network-ca-bundle,
// which contains all the individual client CAs.
type KubeAPIServerServiceNetworkCABundle struct {
	CertBundle
}

var _ asset.Asset = (*KubeAPIServerServiceNetworkCABundle)(nil)

// Dependencies returns the dependency of the cert bundle.
func (a *KubeAPIServerServiceNetworkCABundle) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerServiceNetworkSignerCertKey{},
	}
}

// Generate generates the cert bundle based on its dependencies.
func (a *KubeAPIServerServiceNetworkCABundle) Generate(deps asset.Parents) error {
	var certs []CertInterface
	for _, asset := range a.Dependencies() {
		deps.Get(asset)
		certs = append(certs, asset.(CertInterface))
	}
	return a.CertBundle.Generate("kube-apiserver-service-network-ca-bundle", certs...)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerServiceNetworkCABundle) Name() string {
	return "Certificate (kube-apiserver-service-network-ca-bundle)"
}

// KubeAPIServerServiceNetworkServerCertKey is the asset that generates the kube-apiserver serving key/cert pair for SNI service network.
type KubeAPIServerServiceNetworkServerCertKey struct {
	SignedCertKey
}

var _ asset.Asset = (*KubeAPIServerServiceNetworkServerCertKey)(nil)

// Dependencies returns the dependency of the the cert/key pair
func (a *KubeAPIServerServiceNetworkServerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerServiceNetworkSignerCertKey{},
		&installconfig.InstallConfig{},
	}
}

// Generate generates the cert/key pair based on its dependencies.
func (a *KubeAPIServerServiceNetworkServerCertKey) Generate(dependencies asset.Parents) error {
	ca := &KubeAPIServerServiceNetworkSignerCertKey{}
	installConfig := &installconfig.InstallConfig{}
	dependencies.Get(ca, installConfig)
	serviceAddress, err := cidrhost(installConfig.Config.Networking.ServiceCIDR.IPNet, 1)
	if err != nil {
		return errors.Wrap(err, "failed to get service address for kube-apiserver from InstallConfig")
	}

	cfg := &CertCfg{
		Subject:      pkix.Name{CommonName: "system:kube-apiserver", Organization: []string{"kube-master"}},
		KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		Validity:     ValidityOneDay,
		DNSNames: []string{
			"kubernetes", "kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
		},
		IPAddresses: []net.IP{net.ParseIP(serviceAddress)},
	}

	return a.SignedCertKey.Generate(cfg, ca, "kube-apiserver-service-network-server", AppendParent)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerServiceNetworkServerCertKey) Name() string {
	return "Certificate (kube-apiserver-service-network-server)"
}

// KubeAPIServerLBSignerCertKey is a key/cert pair that signs the kube-apiserver server cert for SNI load balancer.
type KubeAPIServerLBSignerCertKey struct {
	SelfSignedCertKey
}

var _ asset.WritableAsset = (*KubeAPIServerLBSignerCertKey)(nil)

// Dependencies returns the dependency of the root-ca, which is empty.
func (c *KubeAPIServerLBSignerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate generates the root-ca key and cert pair.
func (c *KubeAPIServerLBSignerCertKey) Generate(parents asset.Parents) error {
	cfg := &CertCfg{
		Subject:   pkix.Name{CommonName: "kube-apiserver-lb-signer", OrganizationalUnit: []string{"openshift"}},
		KeyUsages: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		Validity:  ValidityTenYears,
		IsCA:      true,
	}

	return c.SelfSignedCertKey.Generate(cfg, "kube-apiserver-lb-signer")
}

// Name returns the human-friendly name of the asset.
func (c *KubeAPIServerLBSignerCertKey) Name() string {
	return "Certificate (kube-apiserver-lb-signer)"
}

// KubeAPIServerLBCABundle is the asset the generates the kube-apiserver-lb-ca-bundle,
// which contains all the individual client CAs.
type KubeAPIServerLBCABundle struct {
	CertBundle
}

var _ asset.Asset = (*KubeAPIServerLBCABundle)(nil)

// Dependencies returns the dependency of the cert bundle.
func (a *KubeAPIServerLBCABundle) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerLBSignerCertKey{},
	}
}

// Generate generates the cert bundle based on its dependencies.
func (a *KubeAPIServerLBCABundle) Generate(deps asset.Parents) error {
	var certs []CertInterface
	for _, asset := range a.Dependencies() {
		deps.Get(asset)
		certs = append(certs, asset.(CertInterface))
	}
	return a.CertBundle.Generate("kube-apiserver-lb-ca-bundle", certs...)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerLBCABundle) Name() string {
	return "Certificate (kube-apiserver-lb-ca-bundle)"
}

// KubeAPIServerLBServerCertKey is the asset that generates the kube-apiserver serving key/cert pair for SNI load balancer.
type KubeAPIServerLBServerCertKey struct {
	SignedCertKey
}

var _ asset.Asset = (*KubeAPIServerLBServerCertKey)(nil)

// Dependencies returns the dependency of the the cert/key pair
func (a *KubeAPIServerLBServerCertKey) Dependencies() []asset.Asset {
	return []asset.Asset{
		&KubeAPIServerLBSignerCertKey{},
		&installconfig.InstallConfig{},
	}
}

// Generate generates the cert/key pair based on its dependencies.
func (a *KubeAPIServerLBServerCertKey) Generate(dependencies asset.Parents) error {
	ca := &KubeAPIServerLBSignerCertKey{}
	installConfig := &installconfig.InstallConfig{}
	dependencies.Get(ca, installConfig)

	cfg := &CertCfg{
		Subject:      pkix.Name{CommonName: "system:kube-apiserver", Organization: []string{"kube-master"}},
		KeyUsages:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		Validity:     ValidityOneDay,
		DNSNames: []string{
			apiAddress(installConfig.Config),
		},
	}

	return a.SignedCertKey.Generate(cfg, ca, "kube-apiserver-lb-server", AppendParent)
}

// Name returns the human-friendly name of the asset.
func (a *KubeAPIServerLBServerCertKey) Name() string {
	return "Certificate (kube-apiserver-lb-server)"
}