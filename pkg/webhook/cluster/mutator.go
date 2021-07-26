/*
Copyright 2021.

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

package cluster

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kopilotv1alpha1 "github.com/smartxworks/kopilot/pkg/apis/kopilot/v1alpha1"
)

//+kubebuilder:webhook:path=/mutate-v1alpha1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=kopilot.smartx.com,resources=clusters,verbs=create;update,versions=v1alpha1,name=mutate.cluster.v1alpha1.kopilot.smartx.com,admissionReviewVersions={v1,v1beta1}

type Mutator struct {
	decoder *admission.Decoder
}

var _ admission.DecoderInjector = &Mutator{}

func NewMutator() *Mutator {
	return &Mutator{}
}

func (h *Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var cluster kopilotv1alpha1.Cluster
	if err := h.decoder.Decode(req, &cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Token == "" {
		cluster.Token = uuid.New().String()
	}

	marshaled, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}

func (h *Mutator) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}
