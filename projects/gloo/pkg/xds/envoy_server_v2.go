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

	envoy_service_discovery_v2 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/server"
)

// Server is a collection of handlers for streaming discovery requests.
type EnvoyServerV2 interface {
	v2.EndpointDiscoveryServiceServer
	v2.ClusterDiscoveryServiceServer
	v2.RouteDiscoveryServiceServer
	v2.ListenerDiscoveryServiceServer
	envoy_service_discovery_v2.AggregatedDiscoveryServiceServer
}

type envoyServerV2 struct {
	server.Server
}

// NewServer creates handlers from a config watcher and an optional logger.
func NewEnvoyServerV2(genericServer server.Server) EnvoyServerV2 {
	return &envoyServerV2{Server: genericServer}
}

func (s *envoyServerV2) StreamAggregatedResources(
	stream envoy_service_discovery_v2.AggregatedDiscoveryService_StreamAggregatedResourcesServer,
) error {
	return s.Server.StreamV2(stream, cache.AnyType)
}

func (s *envoyServerV2) StreamEndpoints(stream v2.EndpointDiscoveryService_StreamEndpointsServer) error {
	return s.Server.StreamV2(stream, EndpointType)
}

func (s *envoyServerV2) StreamClusters(stream v2.ClusterDiscoveryService_StreamClustersServer) error {
	return s.Server.StreamV2(stream, ClusterType)
}

func (s *envoyServerV2) StreamRoutes(stream v2.RouteDiscoveryService_StreamRoutesServer) error {
	return s.Server.StreamV2(stream, RouteType)
}

func (s *envoyServerV2) StreamListeners(stream v2.ListenerDiscoveryService_StreamListenersServer) error {
	return s.Server.StreamV2(stream, ListenerType)
}

func (s *envoyServerV2) FetchEndpoints(ctx context.Context, req *v2.DiscoveryRequest) (*v2.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = EndpointType
	return s.Server.FetchV2(ctx, req)
}

func (s *envoyServerV2) FetchClusters(ctx context.Context, req *v2.DiscoveryRequest) (*v2.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = ClusterType
	return s.Server.FetchV2(ctx, req)
}

func (s *envoyServerV2) FetchRoutes(ctx context.Context, req *v2.DiscoveryRequest) (*v2.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = RouteType
	return s.Server.FetchV2(ctx, req)
}

func (s *envoyServerV2) FetchListeners(ctx context.Context, req *v2.DiscoveryRequest) (*v2.DiscoveryResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.Unavailable, "empty request")
	}
	req.TypeUrl = ListenerType
	return s.Server.FetchV2(ctx, req)
}

func (s *envoyServerV2) DeltaClusters(_ v2.ClusterDiscoveryService_DeltaClustersServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV2) DeltaRoutes(_ v2.RouteDiscoveryService_DeltaRoutesServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV2) DeltaEndpoints(v2.EndpointDiscoveryService_DeltaEndpointsServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV2) DeltaListeners(v2.ListenerDiscoveryService_DeltaListenersServer) error {
	return errors.New("not implemented")
}

func (s *envoyServerV2) DeltaAggregatedResources(
	envoy_service_discovery_v2.AggregatedDiscoveryService_DeltaAggregatedResourcesServer,
) error {
	return errors.New("not implemented")
}
