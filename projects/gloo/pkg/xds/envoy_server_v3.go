// Copyright 2018 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// Package server provides an implementation of a streaming xDS server.
package xds

import (
	"context"
	"errors"

	envoy_service_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_service_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	envoy_service_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	envoy_service_route_v3 "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/server"
)

// Server is a collection of handlers for streaming discovery requests.
type EnvoyServerV3 interface {
	envoy_service_endpoint_v3.EndpointDiscoveryServiceServer
	envoy_service_cluster_v3.ClusterDiscoveryServiceServer
	envoy_service_route_v3.RouteDiscoveryServiceServer
	envoy_service_listener_v3.ListenerDiscoveryServiceServer
	envoy_service_discovery_v3.AggregatedDiscoveryServiceServer
}

type envoyServerV3 struct {
	server.Server
}

// NewServer creates handlers from a config watcher and an optional logger.
func NewEnvoyServerV3(genericServer server.Server) EnvoyServerV3 {
	return &envoyServerV3{Server: genericServer}
}

func (s *envoyServerV3) StreamAggregatedResources(
	stream envoy_service_discovery_v3.AggregatedDiscoveryService_StreamAggregatedResourcesServer,
) error {
	return s.Server.StreamV3(stream, cache.AnyType)
}

func (s *envoyServerV3) StreamEndpoints(
	stream envoy_service_endpoint_v3.EndpointDiscoveryService_StreamEndpointsServer,
) error {
	return s.Server.StreamV3(stream, EndpointType)
}

func (s *envoyServerV3) StreamClusters(
	stream envoy_service_cluster_v3.ClusterDiscoveryService_StreamClustersServer,
) error {
	return s.Server.StreamV3(stream, ClusterType)
}

func (s *envoyServerV3) StreamRoutes(
	stream envoy_service_route_v3.RouteDiscoveryService_StreamRoutesServer,
) error {
	return s.Server.StreamV3(stream, RouteType)
}

func (s *envoyServerV3) StreamListeners(
	stream envoy_service_listener_v3.ListenerDiscoveryService_StreamListenersServer,
) error {
	return s.Server.StreamV3(stream, ListenerType)
}

func (s *envoyServerV3) FetchEndpoints(
	ctx context.Context,
	req *envoy_service_discovery_v3.DiscoveryRequest,
) (*envoy_service_discovery_v3.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = EndpointType
	return s.Server.FetchV3(ctx, req)
}

func (s *envoyServerV3) FetchClusters(
	ctx context.Context,
	req *envoy_service_discovery_v3.DiscoveryRequest,
) (*envoy_service_discovery_v3.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = ClusterType
	return s.Server.FetchV3(ctx, req)
}

func (s *envoyServerV3) FetchRoutes(
	ctx context.Context,
	req *envoy_service_discovery_v3.DiscoveryRequest,
) (*envoy_service_discovery_v3.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = RouteType
	return s.Server.FetchV3(ctx, req)
}

func (s *envoyServerV3) FetchListeners(
	ctx context.Context,
	req *envoy_service_discovery_v3.DiscoveryRequest,
) (*envoy_service_discovery_v3.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = ListenerType
	return s.Server.FetchV3(ctx, req)
}

func (s *envoyServerV3) DeltaEndpoints(envoy_service_endpoint_v3.EndpointDiscoveryService_DeltaEndpointsServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV3) DeltaClusters(envoy_service_cluster_v3.ClusterDiscoveryService_DeltaClustersServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV3) DeltaRoutes(envoy_service_route_v3.RouteDiscoveryService_DeltaRoutesServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV3) DeltaListeners(envoy_service_listener_v3.ListenerDiscoveryService_DeltaListenersServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV3) DeltaAggregatedResources(
	envoy_service_discovery_v3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer,
) error {
	return errors.New("not implemented")
}