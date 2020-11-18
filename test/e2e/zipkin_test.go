package e2e_test

import (
	"context"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/solo-io/gloo/pkg/utils"

	gatewayv1 "github.com/solo-io/gloo/projects/gateway/pkg/api/v1"
	gatewaydefaults "github.com/solo-io/gloo/projects/gateway/pkg/defaults"
	envoytrace_gloo "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/config/trace/v3"
	gloov1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/hcm"
	static_plugin_gloo "github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/static"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/tracing"
	gloohelpers "github.com/solo-io/gloo/test/helpers"
	"github.com/solo-io/gloo/test/v1helpers"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
	"github.com/solo-io/solo-kit/pkg/api/v1/resources/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/solo-io/gloo/test/services"

	"github.com/solo-io/gloo/projects/gloo/pkg/defaults"
)

var _ = Describe("Zipkin config loading", func() {
	var (
		cancel        context.CancelFunc
		envoyInstance *services.EnvoyInstance
		zipkinServer  *http.Server
	)

	BeforeEach(func() {
		_, cancel = context.WithCancel(context.Background())
		defaults.HttpPort = services.NextBindPort()
		defaults.HttpsPort = services.NextBindPort()

		var err error
		envoyInstance, err = envoyFactory.NewEnvoyInstance()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if envoyInstance != nil {
			_ = envoyInstance.Clean()
		}
		cancel()
	})

	startZipkinServer := func(address string, handler http.Handler) {
		zipkinServer = &http.Server{
			Addr:    address,
			Handler: handler,
		}
		go func() {
			zipkinServer.ListenAndServe()
		}()
	}

	stopZipkinServer := func() {
		if zipkinServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			zipkinServer.Shutdown(ctx)
		}
	}

	It("should send trace msgs to the zipkin server", func() {
		err := envoyInstance.RunWithConfig(int(defaults.HttpPort), "./envoyconfigs/zipkin-envoy-conf.yaml")
		Expect(err).NotTo(HaveOccurred())

		// Start a dummy server listening on 9411 for Zipkin requests
		apiHit := make(chan bool, 1)
		zipkinHandler := http.NewServeMux()
		zipkinHandler.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/api/v2/spans")) // Zipkin json collector API
			fmt.Fprintf(w, "Dummy Zipkin Collector received request on - %q", html.EscapeString(r.URL.Path))
			apiHit <- true
		}))
		startZipkinServer(":9411", zipkinHandler)

		testRequest := createRequestWithTracingEnabled("127.0.0.1", 11082)
		Eventually(testRequest, 15, 1).Should(ContainSubstring(`<title>Envoy Admin</title>`))

		truez := true
		Eventually(apiHit, 5*time.Second).Should(Receive(&truez))

		stopZipkinServer()
	})

	It("should fail to load bad config", func() {
		err := envoyInstance.RunWithConfig(int(defaults.HttpPort), "./envoyconfigs/zipkin-envoy-invalid-conf.yaml")
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(And(ContainSubstring("can't unmarshal"), ContainSubstring(`unknown field "invalid_field"`))))
	})

	Context("dynamic tracing", func() {

		var (
			ctx            context.Context
			cancel         context.CancelFunc
			testClients    services.TestClients
			writeNamespace string
			testUs         *v1helpers.TestUpstream
			zipkinUs       *gloov1.Upstream
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
			defaults.HttpPort = services.NextBindPort()
			defaults.HttpsPort = services.NextBindPort()

			// run gloo
			writeNamespace = "gloo-system"
			ro := &services.RunOptions{
				NsToWrite: writeNamespace,
				NsToWatch: []string{"default", writeNamespace},
				WhatToRun: services.What{
					DisableFds: true,
					DisableUds: true,
				},
			}
			testClients = services.RunGlooGatewayUdsFds(ctx, ro)

			// write gateways and wait for them to be created
			err := gloohelpers.WriteDefaultGateways(writeNamespace, testClients.GatewayClient)
			Expect(err).NotTo(HaveOccurred(), "Should be able to write default gateways")
			Eventually(func() (gatewayv1.GatewayList, error) {
				return testClients.GatewayClient.List(writeNamespace, clients.ListOpts{})
			}, "10s", "0.1s").Should(HaveLen(2), "Gateways should be present")

			// run envoy
			err = envoyInstance.RunWithRole(writeNamespace+"~"+gatewaydefaults.GatewayProxyName, testClients.GlooPort)
			Expect(err).NotTo(HaveOccurred())

			// create test upstream
			// this is the upstream that will handle requests
			testUs = v1helpers.NewTestHttpUpstream(ctx, envoyInstance.LocalAddr())
			_, err = testClients.UpstreamClient.Write(testUs.Upstream, clients.WriteOpts{})
			Expect(err).NotTo(HaveOccurred())

			// create zipkin upstream
			zipkinUs = &gloov1.Upstream{
				Metadata: core.Metadata{
					Name:      "zipkin",
					Namespace: "default",
				},
				UpstreamType: &gloov1.Upstream_Static{
					Static: &static_plugin_gloo.UpstreamSpec{
						Hosts: []*static_plugin_gloo.Host{
							{
								Addr: envoyInstance.LocalAddr(),
								Port: 9411,
							},
						},
					},
				},
			}
			_, err = testClients.UpstreamClient.Write(zipkinUs, clients.WriteOpts{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			envoyInstance.Clean()
			cancel()
		})

		It("should not send trace msgs with nil provider", func() {
			gatewayClient := testClients.GatewayClient
			gw, err := gatewayClient.Read(writeNamespace, gatewaydefaults.GatewayProxyName, clients.ReadOpts{})
			Expect(err).NotTo(HaveOccurred())

			tracingWithoutProvider := &tracing.ListenerTracingSettings{
				ProviderConfig: nil,
			}

			httpGateway := gw.GetHttpGateway()
			setTracingOnGateway(httpGateway, tracingWithoutProvider)
			_, err = gatewayClient.Write(gw, clients.WriteOpts{Ctx: ctx, OverwriteExisting: true})
			Expect(err).NotTo(HaveOccurred())

			// write a virtual service so we have a proxy to our test upstream
			testVs := getTrivialVirtualServiceForUpstream("gloo-system", testUs.Upstream.Metadata.Ref())
			_, err = testClients.VirtualServiceClient.Write(testVs, clients.WriteOpts{})
			Expect(err).NotTo(HaveOccurred())

			// ensure the proxy and virtual service are created
			Eventually(func() (*gloov1.Proxy, error) {
				return testClients.ProxyClient.Read(writeNamespace, gatewaydefaults.GatewayProxyName, clients.ReadOpts{})
			}, "5s", "0.1s").ShouldNot(BeNil())

			Eventually(func() (*gatewayv1.VirtualService, error) {
				return testClients.VirtualServiceClient.Read(testVs.Metadata.GetNamespace(), testVs.Metadata.GetName(), clients.ReadOpts{})
			}, "5s", "0.1s").ShouldNot(BeNil())

			// ensure the upstream is reachable
			TestUpstreamReachable := func() {
				v1helpers.TestUpstreamReachable(defaults.HttpPort, testUs, nil)
			}
			TestUpstreamReachable()

			// Start a dummy server listening on 9411 for Zipkin requests
			apiHit := make(chan bool, 1)
			zipkinHandler := http.NewServeMux()
			zipkinHandler.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v2/spans")) // Zipkin json collector API
				fmt.Fprintf(w, "Dummy Zipkin Collector received request on - %q", html.EscapeString(r.URL.Path))
				apiHit <- true
			}))
			startZipkinServer(envoyInstance.LocalAddr()+":9411", zipkinHandler)

			// ensure we can reach out test upstream with a request
			testRequest := createRequestWithTracingEnabled("127.0.0.1", defaults.HttpPort)
			Eventually(testRequest, 15*time.Second, 1*time.Second).Should(BeEmpty())

			// we haven't configured tracing, so we don't expect the zipkin server to receive an api hit
			Eventually(apiHit, 5*time.Second).Should(Not(Receive()))

			stopZipkinServer()
		})

		It("should send trace msgs with zipkin provider", func() {
			gatewayClient := testClients.GatewayClient
			gw, err := gatewayClient.Read(writeNamespace, gatewaydefaults.GatewayProxyName, clients.ReadOpts{})
			Expect(err).NotTo(HaveOccurred())

			// configure zipkin, and write tracing configuration to gateway
			zipkinTracing := &tracing.ListenerTracingSettings{
				ProviderConfig: &tracing.ListenerTracingSettings_ZipkinConfig{
					ZipkinConfig: &envoytrace_gloo.ZipkinConfig{
						CollectorUpstreamRef:     utils.ResourceRefPtr(zipkinUs.Metadata.Ref()),
						CollectorEndpoint:        "/api/v2/spans",
						CollectorEndpointVersion: envoytrace_gloo.ZipkinConfig_HTTP_JSON,
					},
				},
			}

			httpGateway := gw.GetHttpGateway()
			setTracingOnGateway(httpGateway, zipkinTracing)
			_, err = gatewayClient.Write(gw, clients.WriteOpts{Ctx: ctx, OverwriteExisting: true})
			Expect(err).NotTo(HaveOccurred())

			// write a virtual service so we have a proxy to our test upstream
			testVs := getTrivialVirtualServiceForUpstream("gloo-system", testUs.Upstream.Metadata.Ref())
			_, err = testClients.VirtualServiceClient.Write(testVs, clients.WriteOpts{})
			Expect(err).NotTo(HaveOccurred())

			// ensure the proxy and virtual service are created
			Eventually(func() (*gloov1.Proxy, error) {
				return testClients.ProxyClient.Read(writeNamespace, gatewaydefaults.GatewayProxyName, clients.ReadOpts{})
			}, "5s", "0.1s").ShouldNot(BeNil())
			Eventually(func() (*gatewayv1.VirtualService, error) {
				return testClients.VirtualServiceClient.Read(testVs.Metadata.GetNamespace(), testVs.Metadata.GetName(), clients.ReadOpts{})
			}, "5s", "0.1s").ShouldNot(BeNil())

			// ensure the upstream is reachable
			TestUpstreamReachable := func() {
				v1helpers.TestUpstreamReachable(defaults.HttpPort, testUs, nil)
			}
			TestUpstreamReachable()

			// Start a dummy server listening on 9411 for Zipkin requests
			apiHit := make(chan bool, 1)
			zipkinHandler := http.NewServeMux()
			zipkinHandler.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/api/v2/spans")) // Zipkin json collector API
				fmt.Fprintf(w, "Dummy Zipkin Collector received request on - %q", html.EscapeString(r.URL.Path))
				apiHit <- true
			}))
			startZipkinServer(envoyInstance.LocalAddr()+":9411", zipkinHandler)

			// ensure we can reach out test upstream with a request
			testRequest := createRequestWithTracingEnabled("127.0.0.1", defaults.HttpPort)
			Eventually(testRequest, 15*time.Second, 1*time.Second).Should(BeEmpty())

			// ensure the zipkin server received tracing from the test upstream
			expectedZipkinApiHit := true
			Eventually(apiHit, 5*time.Second).Should(Receive(&expectedZipkinApiHit))

			stopZipkinServer()
		})

		It("should error with misconfigured zipkin provider", func() {
			gatewayClient := testClients.GatewayClient
			gw, err := gatewayClient.Read(writeNamespace, gatewaydefaults.GatewayProxyName, clients.ReadOpts{})
			Expect(err).NotTo(HaveOccurred())

			// configure zipkin with an invalid CollectorUpstreamRef
			invalidZipkinTracing := &tracing.ListenerTracingSettings{
				ProviderConfig: &tracing.ListenerTracingSettings_ZipkinConfig{
					ZipkinConfig: &envoytrace_gloo.ZipkinConfig{
						CollectorUpstreamRef:     nil,
						CollectorEndpoint:        "/api/v2/spans",
						CollectorEndpointVersion: envoytrace_gloo.ZipkinConfig_HTTP_JSON,
					},
				},
			}

			httpGateway := gw.GetHttpGateway()
			setTracingOnGateway(httpGateway, invalidZipkinTracing)
			_, err = gatewayClient.Write(gw, clients.WriteOpts{Ctx: ctx, OverwriteExisting: true})
			Expect(err).NotTo(HaveOccurred())

			// write a virtual service so we have a proxy to our test upstream
			testVs := getTrivialVirtualServiceForUpstream("gloo-system", testUs.Upstream.Metadata.Ref())
			_, err = testClients.VirtualServiceClient.Write(testVs, clients.WriteOpts{})
			Expect(err).NotTo(HaveOccurred())

			// ensure the proxy creation returns an error
			Eventually(func() error {
				_, err := testClients.ProxyClient.Read(writeNamespace, gatewaydefaults.GatewayProxyName, clients.ReadOpts{})
				if err != nil {
					return err
				}
				return nil
			}, "5s", "0.1s").Should(Not(HaveOccurred()))
		})
	})
})

func createRequestWithTracingEnabled(address string, port uint32) func() (string, error) {
	return func() (string, error) {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/", address, port), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

		// Set a random trace ID
		req.Header.Set("x-client-trace-id", "test-trace-id-1234567890")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		return string(body), err
	}
}

func setTracingOnGateway(httpGateway *gatewayv1.HttpGateway, tracing *tracing.ListenerTracingSettings) {
	if httpGateway != nil {
		httpGateway.Options = &gloov1.HttpListenerOptions{
			HttpConnectionManagerSettings: &hcm.HttpConnectionManagerSettings{
				Tracing: tracing,
			},
		}
	}
}
