// Copyright 2021 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

// RenderCustomValues looks the supported fields up, tries to execute them as
// Go template passing the instance as context and updates the instance itself.
//
// Supported fields:
// - rpaasinstance.spec.service.annotations
// - rpaasinstance.spec.ingress.annotations
// - rpaasinstance.spec.podTemplate.affinity.podAffinity
// - rpaasinstance.spec.podTemplate.affinity.podAntiAffinity
// - rpaasinstance.spec.podTemplate.topologySpreadConstraints
func RenderCustomValues(instance *v1alpha1.RpaasInstance) error {
	if instance == nil {
		return nil
	}

	if err := renderServiceCustomAnnotations(instance); err != nil {
		return err
	}

	if err := renderIngressCustomAnnotations(instance); err != nil {
		return err
	}

	if err := renderTopologySpreadConstraints(instance); err != nil {
		return err
	}

	if err := renderAffinityCustomValues(instance); err != nil {
		return err
	}

	return renderPodSpec(instance)
}

func renderServiceCustomAnnotations(instance *v1alpha1.RpaasInstance) error {
	if instance.Spec.Service == nil {
		return nil
	}

	for k, v := range instance.Spec.Service.Annotations {
		renderedValue, err := renderTemplate(instance, v)
		if err != nil {
			return err
		}

		instance.Spec.Service.Annotations[k] = renderedValue
	}

	return nil
}

func renderIngressCustomAnnotations(instance *v1alpha1.RpaasInstance) error {
	if instance.Spec.Ingress == nil {
		return nil
	}

	for k, v := range instance.Spec.Ingress.Annotations {
		renderedValue, err := renderTemplate(instance, v)
		if err != nil {
			return err
		}

		instance.Spec.Ingress.Annotations[k] = renderedValue
	}

	return nil
}

func renderTopologySpreadConstraints(instance *v1alpha1.RpaasInstance) error {
	if len(instance.Spec.PodTemplate.TopologySpreadConstraints) < 1 {
		return nil
	}
	for i := range instance.Spec.PodTemplate.TopologySpreadConstraints {
		if err := renderLabelSelector(instance, instance.Spec.PodTemplate.TopologySpreadConstraints[i].LabelSelector); err != nil {
			return err
		}
	}
	return nil
}

func renderAffinityCustomValues(instance *v1alpha1.RpaasInstance) error {
	if instance.Spec.PodTemplate.Affinity == nil {
		return nil
	}

	if podAffinity := instance.Spec.PodTemplate.Affinity.PodAffinity; podAffinity != nil {
		if err := renderPodAffinityTerms(instance, podAffinity.RequiredDuringSchedulingIgnoredDuringExecution); err != nil {
			return err
		}

		if err := renderWeightedPodAffinityTerms(instance, podAffinity.PreferredDuringSchedulingIgnoredDuringExecution); err != nil {
			return err
		}
	}

	if podAntiAffinity := instance.Spec.PodTemplate.Affinity.PodAntiAffinity; podAntiAffinity != nil {
		if err := renderPodAffinityTerms(instance, podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution); err != nil {
			return err
		}

		if err := renderWeightedPodAffinityTerms(instance, podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution); err != nil {
			return err
		}
	}

	return nil
}

func renderPodAffinityTerms(instance *v1alpha1.RpaasInstance, terms []corev1.PodAffinityTerm) error {
	for i := range terms {
		if err := renderLabelSelector(instance, terms[i].LabelSelector); err != nil {
			return err
		}
	}

	return nil
}

func renderWeightedPodAffinityTerms(instance *v1alpha1.RpaasInstance, terms []corev1.WeightedPodAffinityTerm) error {
	for i := range terms {
		if err := renderLabelSelector(instance, terms[i].PodAffinityTerm.LabelSelector); err != nil {
			return err
		}
	}

	return nil
}

func renderLabelSelector(instance *v1alpha1.RpaasInstance, ls *metav1.LabelSelector) error {
	if ls == nil {
		return nil
	}

	for key, value := range ls.MatchLabels {
		renderedValue, err := renderTemplate(instance, value)
		if err != nil {
			return err
		}

		ls.MatchLabels[key] = renderedValue
	}

	for j := range ls.MatchExpressions {
		for k, value := range ls.MatchExpressions[j].Values {
			renderedValue, err := renderTemplate(instance, value)
			if err != nil {
				return err
			}

			ls.MatchExpressions[j].Values[k] = renderedValue
		}
	}

	return nil
}

var internalTemplate = template.New("rpaasv2.internal").Funcs(sprig.GenericFuncMap())

func renderTemplate(instance *v1alpha1.RpaasInstance, templateStr string) (string, error) {
	t, err := internalTemplate.Clone()
	if err != nil {
		return "", err
	}

	tmpl, err := t.Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	if err = tmpl.Execute(&buffer, instance); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

func renderPodSpec(instance *v1alpha1.RpaasInstance) error {
	if instance.Spec.PodTemplate.InitContainers != nil {
		if err := renderContainerSpec(instance, instance.Spec.PodTemplate.InitContainers); err != nil {
			return err
		}
	}

	if instance.Spec.PodTemplate.Containers != nil {
		if err := renderContainerSpec(instance, instance.Spec.PodTemplate.Containers); err != nil {
			return err
		}
	}

	return nil
}

func renderContainerSpec(instance *v1alpha1.RpaasInstance, containers []corev1.Container) error {
	for i := range containers {
		for j, arg := range containers[i].Args {
			renderedArg, err := renderTemplate(instance, arg)
			if err != nil {
				return err
			}
			containers[i].Args[j] = renderedArg
		}

		for j, cmd := range containers[i].Command {
			renderedCmd, err := renderTemplate(instance, cmd)
			if err != nil {
				return err
			}
			containers[i].Command[j] = renderedCmd
		}

		for j, env := range containers[i].Env {
			if env.Value != "" {
				renderedValue, err := renderTemplate(instance, env.Value)
				if err != nil {
					return err
				}
				containers[i].Env[j].Value = renderedValue
			}
		}

	}

	return nil
}
