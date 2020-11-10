// Code generated by solo-kit. DO NOT EDIT.

package v1

import (
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"

	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
	"github.com/solo-io/solo-kit/pkg/errors"
	skstats "github.com/solo-io/solo-kit/pkg/stats"

	"github.com/solo-io/go-utils/contextutils"
	"github.com/solo-io/go-utils/errutils"
)

var (
	// Deprecated. See mApiResourcesIn
	mApiSnapshotIn = stats.Int64("api.gateway.solo.io/emitter/snap_in", "Deprecated. Use api.gateway.solo.io/emitter/resources_in. The number of snapshots in", "1")

	// metrics for emitter
	mApiResourcesIn    = stats.Int64("api.gateway.solo.io/emitter/resources_in", "The number of resource lists received on open watch channels", "1")
	mApiSnapshotOut    = stats.Int64("api.gateway.solo.io/emitter/snap_out", "The number of snapshots out", "1")
	mApiSnapshotMissed = stats.Int64("api.gateway.solo.io/emitter/snap_missed", "The number of snapshots missed", "1")

	// views for emitter
	// deprecated: see apiResourcesInView
	apisnapshotInView = &view.View{
		Name:        "api.gateway.solo.io/emitter/snap_in",
		Measure:     mApiSnapshotIn,
		Description: "Deprecated. Use api.gateway.solo.io/emitter/resources_in. The number of snapshots updates coming in.",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}

	apiResourcesInView = &view.View{
		Name:        "api.gateway.solo.io/emitter/resources_in",
		Measure:     mApiResourcesIn,
		Description: "The number of resource lists received on open watch channels",
		Aggregation: view.Count(),
		TagKeys: []tag.Key{
			skstats.NamespaceKey,
			skstats.ResourceKey,
		},
	}
	apisnapshotOutView = &view.View{
		Name:        "api.gateway.solo.io/emitter/snap_out",
		Measure:     mApiSnapshotOut,
		Description: "The number of snapshots updates going out",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
	apisnapshotMissedView = &view.View{
		Name:        "api.gateway.solo.io/emitter/snap_missed",
		Measure:     mApiSnapshotMissed,
		Description: "The number of snapshots updates going missed. this can happen in heavy load. missed snapshot will be re-tried after a second.",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
)

func init() {
	view.Register(
		apisnapshotInView,
		apisnapshotOutView,
		apisnapshotMissedView,
		apiResourcesInView,
	)
}

type ApiSnapshotEmitter interface {
	Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *ApiSnapshot, <-chan error, error)
}

type ApiEmitter interface {
	ApiSnapshotEmitter
	Register() error
	VirtualService() VirtualServiceClient
	RouteTable() RouteTableClient
	Gateway() GatewayClient
}

func NewApiEmitter(virtualServiceClient VirtualServiceClient, routeTableClient RouteTableClient, gatewayClient GatewayClient) ApiEmitter {
	return NewApiEmitterWithEmit(virtualServiceClient, routeTableClient, gatewayClient, make(chan struct{}))
}

func NewApiEmitterWithEmit(virtualServiceClient VirtualServiceClient, routeTableClient RouteTableClient, gatewayClient GatewayClient, emit <-chan struct{}) ApiEmitter {
	return &apiEmitter{
		virtualService: virtualServiceClient,
		routeTable:     routeTableClient,
		gateway:        gatewayClient,
		forceEmit:      emit,
	}
}

type apiEmitter struct {
	forceEmit      <-chan struct{}
	virtualService VirtualServiceClient
	routeTable     RouteTableClient
	gateway        GatewayClient
}

func (c *apiEmitter) Register() error {
	if err := c.virtualService.Register(); err != nil {
		return err
	}
	if err := c.routeTable.Register(); err != nil {
		return err
	}
	if err := c.gateway.Register(); err != nil {
		return err
	}
	return nil
}

func (c *apiEmitter) VirtualService() VirtualServiceClient {
	return c.virtualService
}

func (c *apiEmitter) RouteTable() RouteTableClient {
	return c.routeTable
}

func (c *apiEmitter) Gateway() GatewayClient {
	return c.gateway
}

func (c *apiEmitter) Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *ApiSnapshot, <-chan error, error) {

	if len(watchNamespaces) == 0 {
		watchNamespaces = []string{""}
	}

	for _, ns := range watchNamespaces {
		if ns == "" && len(watchNamespaces) > 1 {
			return nil, nil, errors.Errorf("the \"\" namespace is used to watch all namespaces. Snapshots can either be tracked for " +
				"specific namespaces or \"\" AllNamespaces, but not both.")
		}
	}

	errs := make(chan error)
	var done sync.WaitGroup
	ctx := opts.Ctx
	/* Create channel for VirtualService */
	type virtualServiceListWithNamespace struct {
		list      VirtualServiceList
		namespace string
	}
	virtualServiceChan := make(chan virtualServiceListWithNamespace)

	var initialVirtualServiceList VirtualServiceList
	/* Create channel for RouteTable */
	type routeTableListWithNamespace struct {
		list      RouteTableList
		namespace string
	}
	routeTableChan := make(chan routeTableListWithNamespace)

	var initialRouteTableList RouteTableList
	/* Create channel for Gateway */
	type gatewayListWithNamespace struct {
		list      GatewayList
		namespace string
	}
	gatewayChan := make(chan gatewayListWithNamespace)

	var initialGatewayList GatewayList

	currentSnapshot := ApiSnapshot{}

	for _, namespace := range watchNamespaces {
		/* Setup namespaced watch for VirtualService */
		{
			virtualServices, err := c.virtualService.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "initial VirtualService list")
			}
			initialVirtualServiceList = append(initialVirtualServiceList, virtualServices...)
		}
		virtualServiceNamespacesChan, virtualServiceErrs, err := c.virtualService.Watch(namespace, opts)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "starting VirtualService watch")
		}

		done.Add(1)
		go func(namespace string) {
			defer done.Done()
			errutils.AggregateErrs(ctx, errs, virtualServiceErrs, namespace+"-virtualServices")
		}(namespace)
		/* Setup namespaced watch for RouteTable */
		{
			routeTables, err := c.routeTable.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "initial RouteTable list")
			}
			initialRouteTableList = append(initialRouteTableList, routeTables...)
		}
		routeTableNamespacesChan, routeTableErrs, err := c.routeTable.Watch(namespace, opts)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "starting RouteTable watch")
		}

		done.Add(1)
		go func(namespace string) {
			defer done.Done()
			errutils.AggregateErrs(ctx, errs, routeTableErrs, namespace+"-routeTables")
		}(namespace)
		/* Setup namespaced watch for Gateway */
		{
			gateways, err := c.gateway.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "initial Gateway list")
			}
			initialGatewayList = append(initialGatewayList, gateways...)
		}
		gatewayNamespacesChan, gatewayErrs, err := c.gateway.Watch(namespace, opts)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "starting Gateway watch")
		}

		done.Add(1)
		go func(namespace string) {
			defer done.Done()
			errutils.AggregateErrs(ctx, errs, gatewayErrs, namespace+"-gateways")
		}(namespace)

		/* Watch for changes and update snapshot */
		go func(namespace string) {
			for {
				select {
				case <-ctx.Done():
					return
				case virtualServiceList, ok := <-virtualServiceNamespacesChan:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case virtualServiceChan <- virtualServiceListWithNamespace{list: virtualServiceList, namespace: namespace}:
					}
				case routeTableList, ok := <-routeTableNamespacesChan:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case routeTableChan <- routeTableListWithNamespace{list: routeTableList, namespace: namespace}:
					}
				case gatewayList, ok := <-gatewayNamespacesChan:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case gatewayChan <- gatewayListWithNamespace{list: gatewayList, namespace: namespace}:
					}
				}
			}
		}(namespace)
	}
	/* Initialize snapshot for VirtualServices */
	currentSnapshot.VirtualServices = initialVirtualServiceList.Sort()
	/* Initialize snapshot for RouteTables */
	currentSnapshot.RouteTables = initialRouteTableList.Sort()
	/* Initialize snapshot for Gateways */
	currentSnapshot.Gateways = initialGatewayList.Sort()

	snapshots := make(chan *ApiSnapshot)
	go func() {
		// sent initial snapshot to kick off the watch
		initialSnapshot := currentSnapshot.Clone()
		snapshots <- &initialSnapshot

		timer := time.NewTicker(time.Second * 1)
		previousHash, err := currentSnapshot.Hash(nil)
		if err != nil {
			contextutils.LoggerFrom(ctx).Panicw("error while hashing, this should never happen", zap.Error(err))
		}
		sync := func() {
			currentHash, err := currentSnapshot.Hash(nil)
			// this should never happen, so panic if it does
			if err != nil {
				contextutils.LoggerFrom(ctx).Panicw("error while hashing, this should never happen", zap.Error(err))
			}
			if previousHash == currentHash {
				return
			}

			sentSnapshot := currentSnapshot.Clone()
			select {
			case snapshots <- &sentSnapshot:
				stats.Record(ctx, mApiSnapshotOut.M(1))
				previousHash = currentHash
			default:
				stats.Record(ctx, mApiSnapshotMissed.M(1))
			}
		}
		virtualServicesByNamespace := make(map[string]VirtualServiceList)
		routeTablesByNamespace := make(map[string]RouteTableList)
		gatewaysByNamespace := make(map[string]GatewayList)

		for {
			record := func() { stats.Record(ctx, mApiSnapshotIn.M(1)) }
			defer func() {
				close(snapshots)
				// we must wait for done before closing the error chan,
				// to avoid sending on close channel.
				done.Wait()
				close(errs)
			}()

			select {
			case <-timer.C:
				sync()
			case <-ctx.Done():
				return
			case <-c.forceEmit:
				sentSnapshot := currentSnapshot.Clone()
				snapshots <- &sentSnapshot
			case virtualServiceNamespacedList, ok := <-virtualServiceChan:
				if !ok {
					return
				}
				record()

				namespace := virtualServiceNamespacedList.namespace

				skstats.IncrementResourceCount(
					ctx,
					namespace,
					"virtual_service",
					mApiResourcesIn,
				)

				// merge lists by namespace
				virtualServicesByNamespace[namespace] = virtualServiceNamespacedList.list
				var virtualServiceList VirtualServiceList
				for _, virtualServices := range virtualServicesByNamespace {
					virtualServiceList = append(virtualServiceList, virtualServices...)
				}
				currentSnapshot.VirtualServices = virtualServiceList.Sort()
			case routeTableNamespacedList, ok := <-routeTableChan:
				if !ok {
					return
				}
				record()

				namespace := routeTableNamespacedList.namespace

				skstats.IncrementResourceCount(
					ctx,
					namespace,
					"route_table",
					mApiResourcesIn,
				)

				// merge lists by namespace
				routeTablesByNamespace[namespace] = routeTableNamespacedList.list
				var routeTableList RouteTableList
				for _, routeTables := range routeTablesByNamespace {
					routeTableList = append(routeTableList, routeTables...)
				}
				currentSnapshot.RouteTables = routeTableList.Sort()
			case gatewayNamespacedList, ok := <-gatewayChan:
				if !ok {
					return
				}
				record()

				namespace := gatewayNamespacedList.namespace

				skstats.IncrementResourceCount(
					ctx,
					namespace,
					"gateway",
					mApiResourcesIn,
				)

				// merge lists by namespace
				gatewaysByNamespace[namespace] = gatewayNamespacedList.list
				var gatewayList GatewayList
				for _, gateways := range gatewaysByNamespace {
					gatewayList = append(gatewayList, gateways...)
				}
				currentSnapshot.Gateways = gatewayList.Sort()
			}
		}
	}()
	return snapshots, errs, nil
}
