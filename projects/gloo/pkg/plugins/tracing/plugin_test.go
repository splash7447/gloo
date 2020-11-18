package tracing

import (
	envoytrace "github.com/envoyproxy/go-control-plane/envoy/config/trace/v3"
	"github.com/golang/protobuf/ptypes"
	envoytrace_gloo "github.com/solo-io/gloo/projects/gloo/pkg/api/external/envoy/config/trace/v3"
	"github.com/solo-io/solo-kit/pkg/api/v1/resources/core"

	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoytracing "github.com/envoyproxy/go-control-plane/envoy/type/tracing/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/gogo/protobuf/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/hcm"
	"github.com/solo-io/gloo/projects/gloo/pkg/api/v1/options/tracing"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins"
)

var _ = Describe("Plugin", func() {

	It("should update listener properly", func() {
		pluginParams := plugins.Params{
			Snapshot: nil,
		}
		p := NewPlugin()
		cfg := &envoyhttp.HttpConnectionManager{}
		hcmSettings := &hcm.HttpConnectionManagerSettings{
			Tracing: &tracing.ListenerTracingSettings{
				RequestHeadersForTags: []string{"header1", "header2"},
				Verbose:               true,
				TracePercentages: &tracing.TracePercentages{
					ClientSamplePercentage:  &types.FloatValue{Value: 10},
					RandomSamplePercentage:  &types.FloatValue{Value: 20},
					OverallSamplePercentage: &types.FloatValue{Value: 30},
				},
				ProviderConfig: nil,
			},
		}
		err := p.ProcessHcmSettings(pluginParams.Snapshot, cfg, hcmSettings)
		Expect(err).To(BeNil())
		expected := &envoyhttp.HttpConnectionManager{
			Tracing: &envoyhttp.HttpConnectionManager_Tracing{
				CustomTags: []*envoytracing.CustomTag{
					{
						Tag: "header1",
						Type: &envoytracing.CustomTag_RequestHeader{
							RequestHeader: &envoytracing.CustomTag_Header{
								Name: "header1",
							},
						},
					},
					{
						Tag: "header2",
						Type: &envoytracing.CustomTag_RequestHeader{
							RequestHeader: &envoytracing.CustomTag_Header{
								Name: "header2",
							},
						},
					},
				},
				ClientSampling:  &envoy_type.Percent{Value: 10},
				RandomSampling:  &envoy_type.Percent{Value: 20},
				OverallSampling: &envoy_type.Percent{Value: 30},
				Verbose:         true,
				Provider:        nil,
			},
		}
		Expect(cfg).To(Equal(expected))
	})

	It("should update listener properly - with defaults", func() {
		pluginParams := plugins.Params{
			Snapshot: nil,
		}
		p := NewPlugin()
		cfg := &envoyhttp.HttpConnectionManager{}
		hcmSettings := &hcm.HttpConnectionManagerSettings{
			Tracing: &tracing.ListenerTracingSettings{},
		}
		err := p.ProcessHcmSettings(pluginParams.Snapshot, cfg, hcmSettings)
		Expect(err).To(BeNil())
		expected := &envoyhttp.HttpConnectionManager{
			Tracing: &envoyhttp.HttpConnectionManager_Tracing{
				ClientSampling:  &envoy_type.Percent{Value: 100},
				RandomSampling:  &envoy_type.Percent{Value: 100},
				OverallSampling: &envoy_type.Percent{Value: 100},
				Verbose:         false,
				Provider:        nil,
			},
		}
		Expect(cfg).To(Equal(expected))
	})

	Context("should handle tracing provider config", func() {

		It("when provider config is nil", func() {
			pluginParams := plugins.Params{
				Snapshot: nil,
			}
			p := NewPlugin()
			cfg := &envoyhttp.HttpConnectionManager{}
			hcmSettings := &hcm.HttpConnectionManagerSettings{
				Tracing: &tracing.ListenerTracingSettings{
					ProviderConfig: nil,
				},
			}
			err := p.ProcessHcmSettings(pluginParams.Snapshot, cfg, hcmSettings)
			Expect(err).To(BeNil())
			Expect(cfg.Tracing.Provider).To(BeNil())
		})

		It("when provider config references invalid upstream", func() {
			pluginParams := plugins.Params{
				Snapshot: &v1.ApiSnapshot{
					Upstreams: v1.UpstreamList{
						// No valid upstreams
					},
				},
			}
			p := NewPlugin()
			cfg := &envoyhttp.HttpConnectionManager{}
			hcmSettings := &hcm.HttpConnectionManagerSettings{
				Tracing: &tracing.ListenerTracingSettings{
					ProviderConfig: &tracing.ListenerTracingSettings_ZipkinConfig{
						ZipkinConfig: &envoytrace_gloo.ZipkinConfig{
							CollectorUpstreamRef: &core.ResourceRef{
								Name:      "invalid-name",
								Namespace: "invalid-namespace",
							},
						},
					},
				},
			}
			err := p.ProcessHcmSettings(pluginParams.Snapshot, cfg, hcmSettings)
			Expect(err).NotTo(BeNil())
		})

		It("when provider config references valid upstream", func() {
			us := v1.NewUpstream("default", "valid")
			pluginParams := plugins.Params{
				Snapshot: &v1.ApiSnapshot{
					Upstreams: v1.UpstreamList{us},
				},
			}
			p := NewPlugin()
			cfg := &envoyhttp.HttpConnectionManager{}
			hcmSettings := &hcm.HttpConnectionManagerSettings{
				Tracing: &tracing.ListenerTracingSettings{
					ProviderConfig: &tracing.ListenerTracingSettings_ZipkinConfig{
						ZipkinConfig: &envoytrace_gloo.ZipkinConfig{
							CollectorUpstreamRef: &core.ResourceRef{
								Name:      "valid",
								Namespace: "default",
							},
							CollectorEndpoint:        "/api/v2/spans",
							CollectorEndpointVersion: envoytrace_gloo.ZipkinConfig_HTTP_JSON,
							SharedSpanContext:        nil,
							TraceId_128Bit:           false,
						},
					},
				},
			}
			err := p.ProcessHcmSettings(pluginParams.Snapshot, cfg, hcmSettings)
			Expect(err).To(BeNil())

			expectedZipkinConfig := &envoytrace.ZipkinConfig{
				CollectorCluster:         "valid_default",
				CollectorEndpoint:        "/api/v2/spans",
				CollectorEndpointVersion: envoytrace.ZipkinConfig_HTTP_JSON,
				SharedSpanContext:        nil,
				TraceId_128Bit:           false,
			}
			expectedZipkinConfigMarshalled, _ := ptypes.MarshalAny(expectedZipkinConfig)

			expectedEnvoyTracingProvider := &envoytrace.Tracing_Http{
				Name: "envoy.tracers.zipkin",
				ConfigType: &envoytrace.Tracing_Http_TypedConfig{
					TypedConfig: expectedZipkinConfigMarshalled,
				},
			}

			Expect(cfg.Tracing.Provider.GetName()).To(Equal(expectedEnvoyTracingProvider.GetName()))
			Expect(cfg.Tracing.Provider.GetTypedConfig()).To(Equal(expectedEnvoyTracingProvider.GetTypedConfig()))
		})

	})

	It("should update routes properly", func() {
		p := NewPlugin()
		in := &v1.Route{}
		out := &envoyroute.Route{}
		err := p.ProcessRoute(plugins.RouteParams{}, in, out)
		Expect(err).NotTo(HaveOccurred())

		inFull := &v1.Route{
			Options: &v1.RouteOptions{
				Tracing: &tracing.RouteTracingSettings{
					RouteDescriptor: "hello",
				},
			},
		}
		outFull := &envoyroute.Route{}
		err = p.ProcessRoute(plugins.RouteParams{}, inFull, outFull)
		Expect(err).NotTo(HaveOccurred())
		Expect(outFull.Decorator.Operation).To(Equal("hello"))
		Expect(outFull.Tracing.ClientSampling.Numerator / 10000).To(Equal(uint32(100)))
		Expect(outFull.Tracing.RandomSampling.Numerator / 10000).To(Equal(uint32(100)))
		Expect(outFull.Tracing.OverallSampling.Numerator / 10000).To(Equal(uint32(100)))
	})

	It("should update routes properly - with defaults", func() {
		p := NewPlugin()
		in := &v1.Route{}
		out := &envoyroute.Route{}
		err := p.ProcessRoute(plugins.RouteParams{}, in, out)
		Expect(err).NotTo(HaveOccurred())

		inFull := &v1.Route{
			Options: &v1.RouteOptions{
				Tracing: &tracing.RouteTracingSettings{
					RouteDescriptor: "hello",
					TracePercentages: &tracing.TracePercentages{
						ClientSamplePercentage:  &types.FloatValue{Value: 10},
						RandomSamplePercentage:  &types.FloatValue{Value: 20},
						OverallSamplePercentage: &types.FloatValue{Value: 30},
					},
				},
			},
		}
		outFull := &envoyroute.Route{}
		err = p.ProcessRoute(plugins.RouteParams{}, inFull, outFull)
		Expect(err).NotTo(HaveOccurred())
		Expect(outFull.Decorator.Operation).To(Equal("hello"))
		Expect(outFull.Tracing.ClientSampling.Numerator / 10000).To(Equal(uint32(10)))
		Expect(outFull.Tracing.RandomSampling.Numerator / 10000).To(Equal(uint32(20)))
		Expect(outFull.Tracing.OverallSampling.Numerator / 10000).To(Equal(uint32(30)))
	})

})
