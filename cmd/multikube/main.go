package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/amimof/multikube/pkg/clientconfig"
	"github.com/amimof/multikube/pkg/api"
	"github.com/amimof/multikube/pkg/middleware"
	"github.com/amimof/multikube/pkg/proxy"
	"github.com/amimof/multikube/pkg/server"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"io/ioutil"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"time"
)

var (
	// VERSION of the app. Is set when project is built and should never be set manually
	VERSION string
	// COMMIT is the Git commit currently used when compiling. Is set when project is built and should never be set manually
	COMMIT string
	// BRANCH is the Git branch currently used when compiling. Is set when project is built and should never be set manually
	BRANCH string
	// GOVERSION used to compile. Is set when project is built and should never be set manually
	GOVERSION string

	enabledListeners []string
	cleanupTimeout   time.Duration
	maxHeaderSize    uint64

	socketPath string

	host         string
	port         int
	listenLimit  int
	keepAlive    time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration

	oidcPollInterval       time.Duration
	oidcIssuerURL          string
	oidcUsernameClaim      string
	oidcCaFile             string
	oidcInsecureSkipVerify bool
	tlsHost                string
	tlsPort                int
	tlsListenLimit         int
	tlsKeepAlive           time.Duration
	tlsReadTimeout         time.Duration
	tlsWriteTimeout        time.Duration
	tlsCertificate         string
	tlsCertificateKey      string
	tlsCACertificate       string

	externalHost string

	metricsHost string
	metricsPort int

	apiPort int
	apiHost string

	rs256PublicKey string

	kubeconfigPath string
)

func init() {
	pflag.StringVar(&socketPath, "socket-path", "/var/run/multikube.sock", "the unix socket to listen on")
	pflag.StringVar(&host, "host", "localhost", "The host address on which to listen for the --port port")
	pflag.StringVar(&externalHost, "external-host", "localhost", "The external hostname that clients connect to")
	pflag.StringVar(&tlsHost, "tls-host", "localhost", "The host address on which to listen for the --tls-port port")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&rs256PublicKey, "rs256-public-key", "", "the RS256 public key used to validate the signature of client JWT's")
	pflag.StringVar(&kubeconfigPath, "kubeconfig", "/etc/multikube/kubeconfig", "absolute path to a kubeconfig file")
	pflag.StringVar(&metricsHost, "metrics-host", "localhost", "The host address on which to listen for the --metrics-port port")
	pflag.StringVar(&apiHost, "api-host", "localhost", "The host address on which to listen for the --api-port port")
	pflag.StringVar(&oidcIssuerURL, "oidc-issuer-url", "", "The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT)")
	pflag.StringVar(&oidcUsernameClaim, "oidc-username-claim", "sub", " The OpenID claim to use as the user name. Note that claims other than the default is not guaranteed to be unique and immutable")
	pflag.StringVar(&oidcCaFile, "oidc-ca-file", "", "the certificate authority file to be used for verifyign the OpenID server")
	pflag.StringSliceVar(&enabledListeners, "scheme", []string{"https"}, "the listeners to enable, this can be repeated and defaults to the schemes in the swagger spec")

	pflag.IntVar(&port, "port", 8080, "the port to listen on for insecure connections, defaults to 8080")
	pflag.IntVar(&tlsPort, "tls-port", 8443, "the port to listen on for secure connections, defaults to 8443")
	pflag.IntVar(&metricsPort, "metrics-port", 8888, "the port to listen on for Prometheus metrics, defaults to 8888")
	pflag.IntVar(&apiPort, "api-port", 8081, "the port to listen on for Api calls, defaults to 8081")
	pflag.IntVar(&listenLimit, "listen-limit", 0, "limit the number of outstanding requests")
	pflag.IntVar(&tlsListenLimit, "tls-listen-limit", 0, "limit the number of outstanding requests")
	pflag.Uint64Var(&maxHeaderSize, "max-header-size", 1000000, "controls the maximum number of bytes the server will read parsing the request header's keys and values, including the request line. It does not limit the size of the request body")

	pflag.DurationVar(&cleanupTimeout, "cleanup-timeout", 10*time.Second, "grace period for which to wait before shutting down the server")
	pflag.DurationVar(&keepAlive, "keep-alive", 3*time.Minute, "sets the TCP keep-alive timeouts on accepted connections. It prunes dead TCP connections ( e.g. closing laptop mid-download)")
	pflag.DurationVar(&readTimeout, "read-timeout", 30*time.Second, "maximum duration before timing out read of the request")
	pflag.DurationVar(&writeTimeout, "write-timeout", 30*time.Second, "maximum duration before timing out write of the response")
	pflag.DurationVar(&tlsKeepAlive, "tls-keep-alive", 3*time.Minute, "sets the TCP keep-alive timeouts on accepted connections. It prunes dead TCP connections ( e.g. closing laptop mid-download)")
	pflag.DurationVar(&tlsReadTimeout, "tls-read-timeout", 30*time.Second, "maximum duration before timing out read of the request")
	pflag.DurationVar(&tlsWriteTimeout, "tls-write-timeout", 30*time.Second, "maximum duration before timing out write of the response")
	pflag.DurationVar(&oidcPollInterval, "oidc-poll-interval", 2*time.Second, "maximum duration between intervals in which the oidc issuer url (--oidc-issuer-url) is polled")

	pflag.BoolVar(&oidcInsecureSkipVerify, "oidc-insecure-skip-verify", false, "")
}

func main() {

	showver := pflag.Bool("version", false, "Print version")

	pflag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage:\n")
		fmt.Fprint(os.Stderr, "  multikube [OPTIONS]\n\n")

		title := "Kubernetes multi-cluster manager"
		fmt.Fprint(os.Stderr, title+"\n\n")
		desc := "Manages multiple Kubernetes clusters and provides a single API to clients"
		if desc != "" {
			fmt.Fprintf(os.Stderr, desc+"\n\n")
		}
		fmt.Fprintln(os.Stderr, pflag.CommandLine.FlagUsages())
	}

	// parse the CLI flags
	pflag.Parse()

	// Show version if requested
	if *showver {
		fmt.Printf("Version: %s\nCommit: %s\nBranch: %s\nGoVersion: %s\n", VERSION, COMMIT, BRANCH, GOVERSION)
		return
	}

	// Only allow one of the flags rs256-public-key and oidc-issuer-url
	if rs256PublicKey != "" && oidcIssuerURL != "" {
		log.Fatalf("Both flags `--rs256-public-key` and `--oidc-issue-url` cannot be set")
	}

	// Read provided kubeconfig file
	c, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Fatal(err)
	}

	// Create the proxy
	p := proxy.NewProxyFrom(c)

	// Setup default middlewares
	middlewares := []middleware.Middleware{
		middleware.WithEmpty,
		middleware.WithLogging,
		middleware.WithMetrics,
		middleware.WithJWT,
		middleware.WithCtxRoot,
		middleware.WithHeader,
	}

	// Add JWK validation middleware if issuer url is provided on cmd line
	if oidcIssuerURL != "" {
		p.Config.OIDCIssuerURL = oidcIssuerURL
		p.Config.OIDCPollInterval = oidcPollInterval
		p.Config.OIDCUsernameClaim = oidcUsernameClaim
		p.Config.OIDCInsecureSkipVerify = oidcInsecureSkipVerify
		if oidcCaFile != "" {
			p.Config.OIDCCa = readCert(oidcCaFile)
		}
		// Start polling OIDC Provider
		stop := p.Config.GetJWKSFromURL()
		defer stop()
		middlewares = append(middlewares, middleware.WithJWKValidation)
	}

	// // Add x509 public key validation if cert provided on cmd line
	if rs256PublicKey != "" {
		p.Config.RS256PublicKey = readCert(rs256PublicKey)
		p.Config.OIDCUsernameClaim = oidcUsernameClaim
		middlewares = append(middlewares, middleware.WithX509Validation)
	}

	// Create middleware
	m := p.Use(middlewares...)

	// Create the server
	s := &server.Server{
		EnabledListeners:  enabledListeners,
		CleanupTimeout:    cleanupTimeout,
		MaxHeaderSize:     maxHeaderSize,
		SocketPath:        socketPath,
		Host:              host,
		Port:              port,
		ListenLimit:       listenLimit,
		KeepAlive:         keepAlive,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		TLSHost:           tlsHost,
		TLSPort:           tlsPort,
		TLSCertificate:    tlsCertificate,
		TLSCertificateKey: tlsCertificateKey,
		TLSCACertificate:  tlsCACertificate,
		TLSListenLimit:    tlsListenLimit,
		TLSKeepAlive:      tlsKeepAlive,
		TLSReadTimeout:    tlsReadTimeout,
		TLSWriteTimeout:   tlsWriteTimeout,
		Handler:           m(nil, p),
	}


	// Client Config Service

	// Read provided tls certificate file.
	certData, err := ioutil.ReadFile(tlsCertificate)
	if err != nil {
		log.Fatal(err)
	}

	ccs := &clientconfig.ConfigService{
		KubeConfig: c,
		ExternalHost: externalHost,
		CertificateAuthorityData: certData,
	}


	// Multikube API Server

	api := api.NewApi()
	api.Register("clientconfig", ccs)



	// Setup api server middlewares
	apimiddlewares := []middleware.Middleware{
		middleware.WithEmpty,
		middleware.WithLogging,
		middleware.WithMetrics,
		middleware.WithJWT,
		middleware.WithHeader,
	}
	am := api.Use(apimiddlewares...)

	mkapis := server.NewServer()
	mkapis.Port = apiPort
	mkapis.Host = apiHost
	mkapis.Name = "Multikube Api"
	mkapis.Handler = am(nil,api)


	// Metrics server
	ms := server.NewServer()
	ms.Port = metricsPort
	ms.Host = metricsHost
	ms.Name = "metrics"

	// Setup opentracing
	cfg := config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
		},
	}
	tracer, closer, err := cfg.New("multikube", config.Logger(jaeger.StdLogger))
	if err != nil {
		log.Fatal(err)
	}
	opentracing.SetGlobalTracer(tracer)
	defer closer.Close()

	ms.Handler = promhttp.Handler()
	go ms.Serve()


	go mkapis.Serve()

	// Listen and serve!
	err = s.Serve()
	if err != nil {
		log.Fatal(err)
	}

}

// Reads an x509 certificate from the filesystem and returns an instance of x509.Certiticate. Returns nil on errors
func readCert(p string) *x509.Certificate {
	signer, err := ioutil.ReadFile(p)
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(signer)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	return cert
}
