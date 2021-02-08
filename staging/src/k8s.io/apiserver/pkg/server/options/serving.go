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

package options

import (
	"context"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	cliflag "k8s.io/component-base/cli/flag"
)

// SecureServingOptions 保存 kube-scheduler HTTPS 服务相关参数
type SecureServingOptions struct {
	// 绑定的 IP 地址, 默认 0.0.0.0. 由 --bind-address 参数配置.
	// --bind-address: 用于设置监控 HTTPS 服务的 IP 地址 (默认值为 0.0.0.0)
	BindAddress net.IP
	// BindPort is ignored when Listener is set, will serve https even with 0.
	// BindPort 当 Listener 字段被设置时, 则忽略该字段. 即使设置为 0 也将服务 https
	// --secure-port: 用于设置要监控的 HTTPS 安全端口, 即使用身份验证和授权为 HTTPS 提供服务的端口 (默认值为 10295)
	BindPort int
	// BindNetwork is the type of network to bind to - defaults to "tcp", accepts "tcp",
	// "tcp4", and "tcp6".
	//
	// BindNetwork 指定将要绑定的网络类型, 默认 "tcp", 接收 "tcp", "tcp4" 以及 "tcp6".
	BindNetwork string
	// Required set to true means that BindPort cannot be zero.
	//
	// Required 设置为 true 意味着 BindPort 不可以为 0.
	Required bool
	// ExternalAddress is the address advertised, even if BindAddress is a loopback. By default this
	// is set to BindAddress if the later no loopback, or to the first host interface address.
	ExternalAddress net.IP

	// Listener is the secure server network listener.
	// either Listener or BindAddress/BindPort/BindNetwork is set,
	// if Listener is set, use it and omit BindAddress/BindPort/BindNetwork.
	//
	// Listener 是 HTTP 服务监听器, 无论设置了 Listener 还是 BindAddress/BindPort/BindNetwork, 只要设置了 Listener,
	// 那么将使用它并忽略 BindAddress/BindPort/BindNetwork.
	Listener net.Listener

	// ServerCert is the TLS cert info for serving secure traffic
	//
	// ServerCert 是用于提供安全流量的 TLS 证书信息.
	ServerCert GeneratableKeyCert
	// SNICertKeys are named CertKeys for serving secure traffic with SNI support.
	//
	// SNICertKeys 也称为 CertKeys, 用于通过 SNI 支持 HTTPS 通信.
	//
	// --tls-sni-cert-key: 用于设置 x509 的证书和密钥文件路径, 对于多个密钥/证书对, 可以多次使用 --tls-sni-cert-key 参数.
	// 例如, "example.cert,example.key" 或 "foo.crt, foo.key: *.foo.com, foo.com" (默认值为 [])
	SNICertKeys []cliflag.NamedCertKey
	// CipherSuites is the list of allowed cipher suites for the server.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	//
	// CipherSuites HTTPS 通信时使用的加密套件.
	//
	// --tls-cipher-suites: 用于设置 kube-apiserver 所使用的密码套件列表, 密码套件以逗号分隔. 如果未指定该参数,
	// 则使用默认的 Go 语言密码套件.
	CipherSuites []string
	// MinTLSVersion is the minimum TLS version supported.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	//
	// MinTLSVersion 表示支持的最小的 TLS 版本.
	//
	// --tls-min-version: 用于设置 kube-apiserver 支持的最低 TLS 版本, 可选地参数值有 VersionTLS10、VersionTLS11、
	// VersionTLS12
	MinTLSVersion string

	// HTTP2MaxStreamsPerConnection is the limit that the api server imposes on each client.
	// A value of zero means to use the default provided by golang's HTTP/2 support.
	//
	// --http2-max-streams-per-connection: 用于设置 kube-scheduler 为正处于 HTTP/2 连接中的客户端提供的最大流量限制.
	HTTP2MaxStreamsPerConnection int

	// PermitPortSharing controls if SO_REUSEPORT is used when binding the port, which allows
	// more than one instance to bind on the same address and port.
	//
	// --permit-port-sharing: 如果设置为 true, 则当绑定端口时将使用 SO_REUSEPORT 标志, 这将允许有多个实例绑定到同一个
	// 地址和端口. 默认为 false
	PermitPortSharing bool
}

type CertKey struct {
	// CertFile is a file containing a PEM-encoded certificate, and possibly the complete certificate chain
	//
	// --tls-cert-file: 用于设置 HTTPS 的 x509 证书文件所在的路径.
	CertFile string
	// KeyFile is a file containing a PEM-encoded private key for the certificate specified by CertFile
	//
	// --tls-private-key-file: 该参数指定的文件包含了与 --tls-cert-file 参数相匹配的默认 x509 密钥.
	KeyFile string
}

type GeneratableKeyCert struct {
	// CertKey allows setting an explicit cert/key file to use.
	CertKey CertKey

	// CertDirectory specifies a directory to write generated certificates to if CertFile/KeyFile aren't explicitly set.
	// PairName is used to determine the filenames within CertDirectory.
	// If CertDirectory and PairName are not set, an in-memory certificate will be generated.
	//
	// --cert-dir: 用于设置 TLS 证书所在的目录. 如果提供了 --tls-cert-file 和 --tls-private-key-file, 则将忽略此参数.
	CertDirectory string
	// PairName is the name which will be used with CertDirectory to make a cert and key filenames.
	// It becomes CertDirectory/PairName.crt and CertDirectory/PairName.key
	PairName string

	// GeneratedCert holds an in-memory generated certificate if CertFile/KeyFile aren't explicitly set, and CertDirectory/PairName are not set.
	GeneratedCert dynamiccertificates.CertKeyContentProvider

	// FixtureDirectory is a directory that contains test fixture used to avoid regeneration of certs during tests.
	// The format is:
	// <host>_<ip>-<ip>_<alternateDNS>-<alternateDNS>.crt
	// <host>_<ip>-<ip>_<alternateDNS>-<alternateDNS>.key
	FixtureDirectory string
}

// NewSecureServingOptions 创建 SecureServingOptions 结构体, 并为其设置默认值, 该结构体用于配置
// 调度器 HTTPS 服务相关参数.
func NewSecureServingOptions() *SecureServingOptions {
	return &SecureServingOptions{
		BindAddress: net.ParseIP("0.0.0.0"),
		BindPort:    443,
		ServerCert: GeneratableKeyCert{
			PairName:      "apiserver",
			CertDirectory: "apiserver.local.config/certificates",
		},
	}
}

func (s *SecureServingOptions) DefaultExternalAddress() (net.IP, error) {
	if s.ExternalAddress != nil && !s.ExternalAddress.IsUnspecified() {
		return s.ExternalAddress, nil
	}
	return utilnet.ResolveBindAddress(s.BindAddress)
}

func (s *SecureServingOptions) Validate() []error {
	if s == nil {
		return nil
	}

	errors := []error{}

	if s.Required && s.BindPort < 1 || s.BindPort > 65535 {
		errors = append(errors, fmt.Errorf("--secure-port %v must be between 1 and 65535, inclusive. It cannot be turned off with 0", s.BindPort))
	} else if s.BindPort < 0 || s.BindPort > 65535 {
		errors = append(errors, fmt.Errorf("--secure-port %v must be between 0 and 65535, inclusive. 0 for turning off secure port", s.BindPort))
	}

	if (len(s.ServerCert.CertKey.CertFile) != 0 || len(s.ServerCert.CertKey.KeyFile) != 0) && s.ServerCert.GeneratedCert != nil {
		errors = append(errors, fmt.Errorf("cert/key file and in-memory certificate cannot both be set"))
	}

	return errors
}

// AddFlags 添加 HTTPS 服务相关参数
func (s *SecureServingOptions) AddFlags(fs *pflag.FlagSet) {
	if s == nil {
		return
	}

	// --bind-address: 用于设置监控 HTTPS 服务的 IP 地址 (默认值为 0.0.0.0)
	fs.IPVar(&s.BindAddress, "bind-address", s.BindAddress, ""+
		"The IP address on which to listen for the --secure-port port. The "+
		"associated interface(s) must be reachable by the rest of the cluster, and by CLI/web "+
		"clients. If blank or an unspecified address (0.0.0.0 or ::), all interfaces will be used.")

	desc := "The port on which to serve HTTPS with authentication and authorization."
	if s.Required {
		desc += " It cannot be switched off with 0."
	} else {
		desc += " If 0, don't serve HTTPS at all."
	}
	// --secure-port: 用于设置要监控的 HTTPS 安全端口, 即使用身份验证和授权为 HTTPS 提供服务的端口 (默认值为 10295)
	fs.IntVar(&s.BindPort, "secure-port", s.BindPort, desc)

	// --cert-dir: 用于设置 TLS 证书所在的目录. 如果提供了 --tls-cert-file 和 --tls-private-key-file, 则将忽略此参数.
	fs.StringVar(&s.ServerCert.CertDirectory, "cert-dir", s.ServerCert.CertDirectory, ""+
		"The directory where the TLS certs are located. "+
		"If --tls-cert-file and --tls-private-key-file are provided, this flag will be ignored.")

	// --tls-cert-file: 用于设置 HTTPS 的 x509 证书文件所在的路径.
	fs.StringVar(&s.ServerCert.CertKey.CertFile, "tls-cert-file", s.ServerCert.CertKey.CertFile, ""+
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert). If HTTPS serving is enabled, and --tls-cert-file and "+
		"--tls-private-key-file are not provided, a self-signed certificate and key "+
		"are generated for the public address and saved to the directory specified by --cert-dir.")

	// --tls-private-key-file: 该参数指定的文件包含了与 --tls-cert-file 参数相匹配的默认 x509 密钥.
	fs.StringVar(&s.ServerCert.CertKey.KeyFile, "tls-private-key-file", s.ServerCert.CertKey.KeyFile,
		"File containing the default x509 private key matching --tls-cert-file.")

	// 返回加密套件名称列表
	tlsCipherPreferredValues := cliflag.PreferredTLSCipherNames()
	tlsCipherInsecureValues := cliflag.InsecureTLSCipherNames()
	// --tls-cipher-suites: 用于设置 kube-apiserver 所使用的密码套件列表, 密码套件以逗号分隔. 如果未指定该参数,
	// 则使用默认的 Go 语言密码套件.
	fs.StringSliceVar(&s.CipherSuites, "tls-cipher-suites", s.CipherSuites,
		"Comma-separated list of cipher suites for the server. "+
			"If omitted, the default Go cipher suites will be used. \n"+
			"Preferred values: "+strings.Join(tlsCipherPreferredValues, ", ")+". \n"+
			"Insecure values: "+strings.Join(tlsCipherInsecureValues, ", ")+".")

	tlsPossibleVersions := cliflag.TLSPossibleVersions()
	// --tls-min-version: 用于设置 kube-apiserver 支持的最低 TLS 版本, 可选地参数值有 VersionTLS10、VersionTLS11、
	// VersionTLS12
	fs.StringVar(&s.MinTLSVersion, "tls-min-version", s.MinTLSVersion,
		"Minimum TLS version supported. "+
			"Possible values: "+strings.Join(tlsPossibleVersions, ", "))

	// --tls-sni-cert-key: 用于设置 x509 的证书和密钥文件路径, 对于多个密钥/证书对, 可以多次使用 --tls-sni-cert-key 参数.
	// 例如, "example.cert,example.key" 或 "foo.crt, foo.key: *.foo.com, foo.com" (默认值为 [])
	fs.Var(cliflag.NewNamedCertKeyArray(&s.SNICertKeys), "tls-sni-cert-key", ""+
		"A pair of x509 certificate and private key file paths, optionally suffixed with a list of "+
		"domain patterns which are fully qualified domain names, possibly with prefixed wildcard "+
		"segments. The domain patterns also allow IP addresses, but IPs should only be used if "+
		"the apiserver has visibility to the IP address requested by a client. "+
		"If no domain patterns are provided, the names of the certificate are "+
		"extracted. Non-wildcard matches trump over wildcard matches, explicit domain patterns "+
		"trump over extracted names. For multiple key/certificate pairs, use the "+
		"--tls-sni-cert-key multiple times. "+
		"Examples: \"example.crt,example.key\" or \"foo.crt,foo.key:*.foo.com,foo.com\".")

	// --http2-max-streams-per-connection: 用于设置 kube-scheduler 为正处于 HTTP/2 连接中的客户端提供的最大流量限制.
	fs.IntVar(&s.HTTP2MaxStreamsPerConnection, "http2-max-streams-per-connection", s.HTTP2MaxStreamsPerConnection, ""+
		"The limit that the server gives to clients for "+
		"the maximum number of streams in an HTTP/2 connection. "+
		"Zero means to use golang's default.")

	// --permit-port-sharing: 如果设置为 true, 则当绑定端口时将使用 SO_REUSEPORT 标志, 这将允许有多个实例绑定到同一个
	// 地址和端口. 默认为 false
	fs.BoolVar(&s.PermitPortSharing, "permit-port-sharing", s.PermitPortSharing,
		"If true, SO_REUSEPORT will be used when binding the port, which allows "+
			"more than one instance to bind on the same address and port. [default=false]")
}

// ApplyTo fills up serving information in the server configuration.
func (s *SecureServingOptions) ApplyTo(config **server.SecureServingInfo) error {
	if s == nil {
		return nil
	}
	if s.BindPort <= 0 && s.Listener == nil {
		return nil
	}

	if s.Listener == nil {
		var err error
		addr := net.JoinHostPort(s.BindAddress.String(), strconv.Itoa(s.BindPort))

		c := net.ListenConfig{}

		if s.PermitPortSharing {
			c.Control = permitPortReuse
		}

		s.Listener, s.BindPort, err = CreateListener(s.BindNetwork, addr, c)
		if err != nil {
			return fmt.Errorf("failed to create listener: %v", err)
		}
	} else {
		if _, ok := s.Listener.Addr().(*net.TCPAddr); !ok {
			return fmt.Errorf("failed to parse ip and port from listener")
		}
		s.BindPort = s.Listener.Addr().(*net.TCPAddr).Port
		s.BindAddress = s.Listener.Addr().(*net.TCPAddr).IP
	}

	*config = &server.SecureServingInfo{
		Listener:                     s.Listener,
		HTTP2MaxStreamsPerConnection: s.HTTP2MaxStreamsPerConnection,
	}
	c := *config

	serverCertFile, serverKeyFile := s.ServerCert.CertKey.CertFile, s.ServerCert.CertKey.KeyFile
	// load main cert
	if len(serverCertFile) != 0 || len(serverKeyFile) != 0 {
		var err error
		c.Cert, err = dynamiccertificates.NewDynamicServingContentFromFiles("serving-cert", serverCertFile, serverKeyFile)
		if err != nil {
			return err
		}
	} else if s.ServerCert.GeneratedCert != nil {
		c.Cert = s.ServerCert.GeneratedCert
	}

	if len(s.CipherSuites) != 0 {
		cipherSuites, err := cliflag.TLSCipherSuites(s.CipherSuites)
		if err != nil {
			return err
		}
		c.CipherSuites = cipherSuites
	}

	var err error
	c.MinTLSVersion, err = cliflag.TLSVersion(s.MinTLSVersion)
	if err != nil {
		return err
	}

	// load SNI certs
	namedTLSCerts := make([]dynamiccertificates.SNICertKeyContentProvider, 0, len(s.SNICertKeys))
	for _, nck := range s.SNICertKeys {
		tlsCert, err := dynamiccertificates.NewDynamicSNIContentFromFiles("sni-serving-cert", nck.CertFile, nck.KeyFile, nck.Names...)
		namedTLSCerts = append(namedTLSCerts, tlsCert)
		if err != nil {
			return fmt.Errorf("failed to load SNI cert and key: %v", err)
		}
	}
	c.SNICerts = namedTLSCerts

	return nil
}

func (s *SecureServingOptions) MaybeDefaultWithSelfSignedCerts(publicAddress string, alternateDNS []string, alternateIPs []net.IP) error {
	if s == nil || (s.BindPort == 0 && s.Listener == nil) {
		return nil
	}
	keyCert := &s.ServerCert.CertKey
	if len(keyCert.CertFile) != 0 || len(keyCert.KeyFile) != 0 {
		return nil
	}

	canReadCertAndKey := false
	if len(s.ServerCert.CertDirectory) > 0 {
		if len(s.ServerCert.PairName) == 0 {
			return fmt.Errorf("PairName is required if CertDirectory is set")
		}
		keyCert.CertFile = path.Join(s.ServerCert.CertDirectory, s.ServerCert.PairName+".crt")
		keyCert.KeyFile = path.Join(s.ServerCert.CertDirectory, s.ServerCert.PairName+".key")
		if canRead, err := certutil.CanReadCertAndKey(keyCert.CertFile, keyCert.KeyFile); err != nil {
			return err
		} else {
			canReadCertAndKey = canRead
		}
	}

	if !canReadCertAndKey {
		// add either the bind address or localhost to the valid alternates
		if s.BindAddress.IsUnspecified() {
			alternateDNS = append(alternateDNS, "localhost")
		} else {
			alternateIPs = append(alternateIPs, s.BindAddress)
		}

		if cert, key, err := certutil.GenerateSelfSignedCertKeyWithFixtures(publicAddress, alternateIPs, alternateDNS, s.ServerCert.FixtureDirectory); err != nil {
			return fmt.Errorf("unable to generate self signed cert: %v", err)
		} else if len(keyCert.CertFile) > 0 && len(keyCert.KeyFile) > 0 {
			if err := certutil.WriteCert(keyCert.CertFile, cert); err != nil {
				return err
			}
			if err := keyutil.WriteKey(keyCert.KeyFile, key); err != nil {
				return err
			}
			klog.Infof("Generated self-signed cert (%s, %s)", keyCert.CertFile, keyCert.KeyFile)
		} else {
			s.ServerCert.GeneratedCert, err = dynamiccertificates.NewStaticCertKeyContent("Generated self signed cert", cert, key)
			if err != nil {
				return err
			}
			klog.Infof("Generated self-signed cert in-memory")
		}
	}

	return nil
}

func CreateListener(network, addr string, config net.ListenConfig) (net.Listener, int, error) {
	if len(network) == 0 {
		network = "tcp"
	}

	ln, err := config.Listen(context.TODO(), network, addr)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to listen on %v: %v", addr, err)
	}

	// get port
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		ln.Close()
		return nil, 0, fmt.Errorf("invalid listen address: %q", ln.Addr().String())
	}

	return ln, tcpAddr.Port, nil
}
