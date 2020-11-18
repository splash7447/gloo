package knative

import (
	"reflect"

	"github.com/solo-io/solo-kit/pkg/api/v1/resources/core"
	"github.com/solo-io/solo-kit/pkg/utils/kubeutils"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
)

type Ingress v1alpha1.Ingress

func (p *Ingress) GetMetadata() core.Metadata {
	return kubeutils.FromKubeMeta(p.ObjectMeta)
}

func (p *Ingress) SetMetadata(meta core.Metadata) {
	p.ObjectMeta = kubeutils.ToKubeMeta(meta)
}

func (p *Ingress) Equal(that interface{}) bool {
	return reflect.DeepEqual(p, that)
}

func (p *Ingress) Clone() *Ingress {
	ing := v1alpha1.Ingress(*p)
	copy := ing.DeepCopy()
	newIng := Ingress(*copy)
	return &newIng
}

// todo verify that this is the correct way to reproduce the bahavior of
// IsPublic() from https://github.com/knative/serving/blob/release-0.9/pkg/apis/networking/v1alpha1/ingress_lifecycle.go
// since it doesn't seem have been carried over to the networking package is later versions.
func (p *Ingress) IsPublic() bool {
	return p.Spec.DeprecatedVisibility == "" || p.Spec.DeprecatedVisibility == v1alpha1.IngressVisibilityExternalIP
}
