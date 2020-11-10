// Code generated by solo-kit. DO NOT EDIT.

package v1

import (
	"sync"
	"time"

	github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative "github.com/solo-io/gloo/projects/clusteringress/pkg/api/external/knative"

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
	// Deprecated. See mTranslatorResourcesIn
	mTranslatorSnapshotIn = stats.Int64("translator.clusteringress.gloo.solo.io/emitter/snap_in", "Deprecated. Use translator.clusteringress.gloo.solo.io/emitter/resources_in. The number of snapshots in", "1")

	// metrics for emitter
	mTranslatorResourcesIn    = stats.Int64("translator.clusteringress.gloo.solo.io/emitter/resources_in", "The number of resource lists received on open watch channels", "1")
	mTranslatorSnapshotOut    = stats.Int64("translator.clusteringress.gloo.solo.io/emitter/snap_out", "The number of snapshots out", "1")
	mTranslatorSnapshotMissed = stats.Int64("translator.clusteringress.gloo.solo.io/emitter/snap_missed", "The number of snapshots missed", "1")

	// views for emitter
	// deprecated: see translatorResourcesInView
	translatorsnapshotInView = &view.View{
		Name:        "translator.clusteringress.gloo.solo.io/emitter/snap_in",
		Measure:     mTranslatorSnapshotIn,
		Description: "Deprecated. Use translator.clusteringress.gloo.solo.io/emitter/resources_in. The number of snapshots updates coming in.",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}

	translatorResourcesInView = &view.View{
		Name:        "translator.clusteringress.gloo.solo.io/emitter/resources_in",
		Measure:     mTranslatorResourcesIn,
		Description: "The number of resource lists received on open watch channels",
		Aggregation: view.Count(),
		TagKeys: []tag.Key{
			skstats.NamespaceKey,
			skstats.ResourceKey,
		},
	}
	translatorsnapshotOutView = &view.View{
		Name:        "translator.clusteringress.gloo.solo.io/emitter/snap_out",
		Measure:     mTranslatorSnapshotOut,
		Description: "The number of snapshots updates going out",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
	translatorsnapshotMissedView = &view.View{
		Name:        "translator.clusteringress.gloo.solo.io/emitter/snap_missed",
		Measure:     mTranslatorSnapshotMissed,
		Description: "The number of snapshots updates going missed. this can happen in heavy load. missed snapshot will be re-tried after a second.",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{},
	}
)

func init() {
	view.Register(
		translatorsnapshotInView,
		translatorsnapshotOutView,
		translatorsnapshotMissedView,
		translatorResourcesInView,
	)
}

type TranslatorSnapshotEmitter interface {
	Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *TranslatorSnapshot, <-chan error, error)
}

type TranslatorEmitter interface {
	TranslatorSnapshotEmitter
	Register() error
	ClusterIngress() github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressClient
}

func NewTranslatorEmitter(clusterIngressClient github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressClient) TranslatorEmitter {
	return NewTranslatorEmitterWithEmit(clusterIngressClient, make(chan struct{}))
}

func NewTranslatorEmitterWithEmit(clusterIngressClient github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressClient, emit <-chan struct{}) TranslatorEmitter {
	return &translatorEmitter{
		clusterIngress: clusterIngressClient,
		forceEmit:      emit,
	}
}

type translatorEmitter struct {
	forceEmit      <-chan struct{}
	clusterIngress github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressClient
}

func (c *translatorEmitter) Register() error {
	if err := c.clusterIngress.Register(); err != nil {
		return err
	}
	return nil
}

func (c *translatorEmitter) ClusterIngress() github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressClient {
	return c.clusterIngress
}

func (c *translatorEmitter) Snapshots(watchNamespaces []string, opts clients.WatchOpts) (<-chan *TranslatorSnapshot, <-chan error, error) {

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
	/* Create channel for ClusterIngress */
	type clusterIngressListWithNamespace struct {
		list      github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressList
		namespace string
	}
	clusterIngressChan := make(chan clusterIngressListWithNamespace)

	var initialClusterIngressList github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressList

	currentSnapshot := TranslatorSnapshot{}

	for _, namespace := range watchNamespaces {
		/* Setup namespaced watch for ClusterIngress */
		{
			clusteringresses, err := c.clusterIngress.List(namespace, clients.ListOpts{Ctx: opts.Ctx, Selector: opts.Selector})
			if err != nil {
				return nil, nil, errors.Wrapf(err, "initial ClusterIngress list")
			}
			initialClusterIngressList = append(initialClusterIngressList, clusteringresses...)
		}
		clusterIngressNamespacesChan, clusterIngressErrs, err := c.clusterIngress.Watch(namespace, opts)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "starting ClusterIngress watch")
		}

		done.Add(1)
		go func(namespace string) {
			defer done.Done()
			errutils.AggregateErrs(ctx, errs, clusterIngressErrs, namespace+"-clusteringresses")
		}(namespace)

		/* Watch for changes and update snapshot */
		go func(namespace string) {
			for {
				select {
				case <-ctx.Done():
					return
				case clusterIngressList, ok := <-clusterIngressNamespacesChan:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case clusterIngressChan <- clusterIngressListWithNamespace{list: clusterIngressList, namespace: namespace}:
					}
				}
			}
		}(namespace)
	}
	/* Initialize snapshot for Clusteringresses */
	currentSnapshot.Clusteringresses = initialClusterIngressList.Sort()

	snapshots := make(chan *TranslatorSnapshot)
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
				stats.Record(ctx, mTranslatorSnapshotOut.M(1))
				previousHash = currentHash
			default:
				stats.Record(ctx, mTranslatorSnapshotMissed.M(1))
			}
		}
		clusteringressesByNamespace := make(map[string]github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressList)

		for {
			record := func() { stats.Record(ctx, mTranslatorSnapshotIn.M(1)) }
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
			case clusterIngressNamespacedList, ok := <-clusterIngressChan:
				if !ok {
					return
				}
				record()

				namespace := clusterIngressNamespacedList.namespace

				skstats.IncrementResourceCount(
					ctx,
					namespace,
					"cluster_ingress",
					mTranslatorResourcesIn,
				)

				// merge lists by namespace
				clusteringressesByNamespace[namespace] = clusterIngressNamespacedList.list
				var clusterIngressList github_com_solo_io_gloo_projects_clusteringress_pkg_api_external_knative.ClusterIngressList
				for _, clusteringresses := range clusteringressesByNamespace {
					clusterIngressList = append(clusterIngressList, clusteringresses...)
				}
				currentSnapshot.Clusteringresses = clusterIngressList.Sort()
			}
		}
	}()
	return snapshots, errs, nil
}
