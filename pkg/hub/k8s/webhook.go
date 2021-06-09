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

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kopilotv1alpha1 "github.com/smartxworks/kopilot/pkg/hub/k8s/apis/kopilot/v1alpha1"
)

func ServeWebhook() error {
	scheme := runtime.NewScheme()
	utilruntime.Must(kopilotv1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		return fmt.Errorf("build manager: %s", err)
	}

	mgr.GetWebhookServer().Register("/mutate-v1alpha1-cluster", &webhook.Admission{
		Handler: &ClusterDefaulter{},
	})
	mgr.GetWebhookServer().Register("/validate-v1alpha1-cluster", &webhook.Admission{
		Handler: &ClusterValidator{},
	})

	return mgr.Start(ctrl.SetupSignalHandler())
}

//+kubebuilder:webhook:path=/mutate-v1alpha1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=kopilot.smartx.com,resources=clusters,verbs=create;update,versions=v1alpha1,name=mutate.cluster.v1alpha1.kopilot.smartx.com,admissionReviewVersions={v1,v1beta1}

type ClusterDefaulter struct {
	decoder *admission.Decoder
}

var _ admission.DecoderInjector = &ClusterDefaulter{}

func (h *ClusterDefaulter) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *ClusterDefaulter) Handle(ctx context.Context, req admission.Request) admission.Response {
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

//+kubebuilder:webhook:path=/validate-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=kopilot.smartx.com,resources=clusters,verbs=create;update,versions=v1alpha1,name=validate.cluster.v1alpha1.kopilot.smartx.com,admissionReviewVersions={v1,v1beta1}

type ClusterValidator struct {
	decoder *admission.Decoder
}

var _ admission.DecoderInjector = &ClusterValidator{}

func (h *ClusterValidator) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *ClusterValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var cluster kopilotv1alpha1.Cluster
	if err := h.decoder.Decode(req, &cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var errs field.ErrorList
	if cluster.Token == "" {
		errs = append(errs, field.Required(field.NewPath("token"), ""))
	}

	if len(errs) > 0 {
		return webhook.Denied(errs.ToAggregate().Error())
	}
	return admission.Allowed("")
}
