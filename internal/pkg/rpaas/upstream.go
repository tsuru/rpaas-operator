// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpaas

import (
	"context"
	"fmt"
	"strings"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
)

func (m *k8sRpaasManager) AddUpstream(ctx context.Context, instanceName string, upstream v1alpha1.AllowedUpstream) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	if upstream.Host == "" {
		return &ValidationError{Msg: "cannot add an upstream with empty host"}
	}

	for _, u := range instance.Spec.AllowedUpstreams {
		if u.Host == upstream.Host && u.Port == upstream.Port {
			return &ConflictError{Msg: fmt.Sprintf("upstream already present in instance: %s", instanceName)}
		}
	}
	instance.Spec.AllowedUpstreams = append(instance.Spec.AllowedUpstreams, upstream)

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) GetUpstreams(ctx context.Context, instanceName string) ([]v1alpha1.AllowedUpstream, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	return instance.Spec.AllowedUpstreams, nil
}

func (m *k8sRpaasManager) DeleteUpstream(ctx context.Context, instanceName string, upstream v1alpha1.AllowedUpstream) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	found := false
	upstreams := instance.Spec.AllowedUpstreams
	for i, u := range upstreams {
		if u.Port == upstream.Port && strings.Compare(u.Host, upstream.Host) == 0 {
			found = true
			upstreams = append(upstreams[:i], upstreams[i+1:]...)
			break
		}
	}
	if !found {
		return &NotFoundError{Msg: fmt.Sprintf("upstream not found inside list of allowed upstreams of %s", instanceName)}
	}

	instance.Spec.AllowedUpstreams = upstreams
	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) GetUpstreamOptions(ctx context.Context, instanceName string) ([]v1alpha1.UpstreamOptions, error) {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, err
	}

	return instance.Spec.UpstreamOptions, nil
}

func (m *k8sRpaasManager) EnsureUpstreamOptions(ctx context.Context, instanceName string, args UpstreamOptionsArgs) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	// Common validations
	if err := validateUpstreamOptionsArgs(args); err != nil {
		return err
	}

	// Check if the bind exists in the instance binds
	bindExists := false
	for _, bind := range instance.Spec.Binds {
		if bind.Name == args.PrimaryBind {
			bindExists = true
			break
		}
	}
	if !bindExists {
		return &ValidationError{Msg: fmt.Sprintf("bind '%s' does not exist in instance binds", args.PrimaryBind)}
	}

	// Check if upstream options for this bind already exist
	upstreamExists := false
	upstreamIndex := -1
	for i, uo := range instance.Spec.UpstreamOptions {
		if uo.PrimaryBind == args.PrimaryBind {
			upstreamExists = true
			upstreamIndex = i
			break
		}
	}

	// Check if this bind is already referenced as a canary bind
	// If so, it cannot have its own canary binds
	isReferencedAsCanary := false
	for _, uo := range instance.Spec.UpstreamOptions {
		if upstreamExists && uo.PrimaryBind == args.PrimaryBind {
			continue // Skip the current upstream option being updated
		}
		for _, canaryBind := range uo.CanaryBinds {
			if canaryBind == args.PrimaryBind {
				isReferencedAsCanary = true
				break
			}
		}
		if isReferencedAsCanary {
			break
		}
	}

	if isReferencedAsCanary && len(args.CanaryBinds) > 0 {
		return &ValidationError{Msg: fmt.Sprintf("bind '%s' is referenced as a canary bind in another upstream option and cannot have its own canary binds", args.PrimaryBind)}
	}

	// Validate canary binds using shared function
	skipPrimaryBind := ""
	if upstreamExists {
		skipPrimaryBind = args.PrimaryBind
	}
	_, err = validateCanaryBinds(instance.Spec.UpstreamOptions, args, skipPrimaryBind)
	if err != nil {
		return err
	}

	// Traffic shaping validations using shared function
	if err := validateTrafficShapingOptions(args); err != nil {
		return err
	}

	// If this is a canary bind, validate weight rule: only one canary per group can have weight > 0
	if args.TrafficShapingPolicy.Weight > 0 && isReferencedAsCanary {
		operation := "add"
		if upstreamExists {
			operation = "update"
		}
		if err := m.validateCanaryWeightRule(instance.Spec.UpstreamOptions, args.PrimaryBind, args.TrafficShapingPolicy.Weight, operation); err != nil {
			return err
		}
	}

	// Load balance validations using shared function
	if err := validateLoadBalanceOptions(args); err != nil {
		return err
	}

	// For update operations: implement mutual exclusion - if one is provided, clear the other
	if upstreamExists {
		if strings.TrimSpace(args.TrafficShapingPolicy.HeaderValue) != "" {
			// HeaderValue provided, clear HeaderPattern
			args.TrafficShapingPolicy.HeaderPattern = ""
		} else if strings.TrimSpace(args.TrafficShapingPolicy.HeaderPattern) != "" {
			// HeaderPattern provided, clear HeaderValue
			args.TrafficShapingPolicy.HeaderValue = ""
		}
	}

	// Apply defaults to UpstreamOptions
	applyUpstreamOptionsDefaults(&args)

	upstreamOptions := v1alpha1.UpstreamOptions{
		PrimaryBind:          args.PrimaryBind,
		CanaryBinds:          args.CanaryBinds,
		TrafficShapingPolicy: args.TrafficShapingPolicy,
		LoadBalance:          args.LoadBalance,
		LoadBalanceHashKey:   args.LoadBalanceHashKey,
	}

	if upstreamExists {
		// Update existing upstream options
		instance.Spec.UpstreamOptions[upstreamIndex] = upstreamOptions
	} else {
		// Add new upstream options
		instance.Spec.UpstreamOptions = append(instance.Spec.UpstreamOptions, upstreamOptions)
	}

	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) DeleteUpstreamOptions(ctx context.Context, instanceName, primaryBind string) error {
	instance, err := m.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}
	originalInstance := instance.DeepCopy()

	if primaryBind == "" {
		return &ValidationError{Msg: "cannot delete upstream options with empty app"}
	}

	// Remove the primary bind from UpstreamOptions and also remove any references to it as a canary bind
	found := false
	upstreamOptions := make([]v1alpha1.UpstreamOptions, 0, len(instance.Spec.UpstreamOptions))

	for _, uo := range instance.Spec.UpstreamOptions {
		if uo.PrimaryBind == primaryBind {
			// Found the bind to delete, skip adding it to the new slice
			found = true
			continue
		}

		// For other upstream options, remove any references to the deleted bind in CanaryBinds
		updatedCanaryBinds := make([]string, 0, len(uo.CanaryBinds))
		for _, canaryBind := range uo.CanaryBinds {
			if canaryBind != primaryBind {
				updatedCanaryBinds = append(updatedCanaryBinds, canaryBind)
			}
		}

		// Create a copy of the upstream option with updated canary binds
		updatedUO := uo
		updatedUO.CanaryBinds = updatedCanaryBinds
		upstreamOptions = append(upstreamOptions, updatedUO)
	}

	if !found {
		return &NotFoundError{Msg: fmt.Sprintf("upstream options for bind '%s' not found in instance: %s", primaryBind, instanceName)}
	}

	instance.Spec.UpstreamOptions = upstreamOptions
	return m.patchInstance(ctx, originalInstance, instance)
}

func (m *k8sRpaasManager) validateCanaryWeightRule(upstreamOptions []v1alpha1.UpstreamOptions, currentPrimaryBind string, currentWeight int, operation string) error {
	// Find all canary binds that reference the same parent as currentPrimaryBind
	var parentPrimaryBind string
	var canaryGroup []string

	// Find the parent bind for currentPrimaryBind
	for _, uo := range upstreamOptions {
		for _, canaryBind := range uo.CanaryBinds {
			if canaryBind == currentPrimaryBind {
				parentPrimaryBind = uo.PrimaryBind
				break
			}
		}
		if parentPrimaryBind != "" {
			break
		}
	}

	if parentPrimaryBind == "" {
		// This shouldn't happen if validation is correct, but just in case
		return nil
	}

	// Find all canary binds in the same group (same parent bind)
	for _, uo := range upstreamOptions {
		if uo.PrimaryBind == parentPrimaryBind {
			canaryGroup = uo.CanaryBinds
			break
		}
	}

	// Count existing weights in the canary group
	weightsCount := 0
	for _, canaryBind := range canaryGroup {
		// Skip the current bind being processed for update operations
		if operation == "update" && canaryBind == currentPrimaryBind {
			continue
		}

		// Find the upstream options for this canary bind
		for _, uo := range upstreamOptions {
			if uo.PrimaryBind == canaryBind && uo.TrafficShapingPolicy.Weight > 0 {
				weightsCount++
				break
			}
		}
	}

	// For add/update operations, check if adding this weight would violate the rule
	if currentWeight > 0 && weightsCount >= 1 {
		return &ValidationError{Msg: fmt.Sprintf("only one canary bind per group can have weight > 0, but found existing weight in canary group for parent '%s'", parentPrimaryBind)}
	}

	return nil
}

func applyTrafficShapingPolicyDefaults(policy *v1alpha1.TrafficShapingPolicy) {
	// Set WeightTotal based on Weight if not set and Weight is greater than 0
	if policy.Weight > 0 && policy.WeightTotal == 0 {
		if policy.Weight <= 100 {
			// For weights <= 100, use 100 as total (standard percentage)
			// This allows weight = 100 to mean 100% (100/100)
			policy.WeightTotal = 100
		} else {
			// For weights > 100, calculate weightTotal so that weight represents ~10% of total
			// This ensures weight/weightTotal gives a reasonable percentage
			policy.WeightTotal = policy.Weight * 10
		}
	}
}

func hasTrafficShapingPolicy(policy v1alpha1.TrafficShapingPolicy) bool {
	return policy.Weight > 0 ||
		strings.TrimSpace(policy.Header) != "" ||
		strings.TrimSpace(policy.Cookie) != ""
}

func applyUpstreamOptionsDefaults(args *UpstreamOptionsArgs) {
	// Set default load balance algorithm to round_robin if not specified
	if args.LoadBalance == "" {
		args.LoadBalance = v1alpha1.LoadBalanceRoundRobin
	}

	// Apply traffic shaping policy defaults
	applyTrafficShapingPolicyDefaults(&args.TrafficShapingPolicy)
}

// validateUpstreamOptionsArgs performs common validation for upstream options
func validateUpstreamOptionsArgs(args UpstreamOptionsArgs) error {
	if args.PrimaryBind == "" {
		return &ValidationError{Msg: "cannot process upstream options with empty app"}
	}

	// Validate canary binds count - only one canary per upstream is allowed
	if len(args.CanaryBinds) > 1 {
		return &ValidationError{Msg: fmt.Sprintf("only one canary bind is allowed per upstream, but %d were provided", len(args.CanaryBinds))}
	}

	return nil
}

// validateCanaryBinds validates canary bind references and relationships
func validateCanaryBinds(upstreamOptions []v1alpha1.UpstreamOptions, args UpstreamOptionsArgs, skipPrimaryBind string) ([]string, error) {
	canaryBindsWithWeight := []string{}
	for _, canaryBind := range args.CanaryBinds {
		canaryBindExists := false
		for _, uo := range upstreamOptions {
			// For update operations, skip the upstream being updated
			if skipPrimaryBind != "" && uo.PrimaryBind == skipPrimaryBind {
				continue
			}
			if uo.PrimaryBind == canaryBind {
				canaryBindExists = true
				// Check if this canary bind has weight defined
				if uo.TrafficShapingPolicy.Weight > 0 {
					canaryBindsWithWeight = append(canaryBindsWithWeight, canaryBind)
				}
				break
			}
		}
		if !canaryBindExists {
			return nil, &ValidationError{Msg: fmt.Sprintf("canary bind '%s' must reference an existing bind from another upstream option", canaryBind)}
		}

		// Validate that a bind cannot be used as canary if it's already referenced as canary elsewhere
		for _, uo := range upstreamOptions {
			if skipPrimaryBind != "" && uo.PrimaryBind == skipPrimaryBind {
				continue // Skip the current upstream being updated
			}
			for _, existingCanary := range uo.CanaryBinds {
				if existingCanary == canaryBind {
					return nil, &ValidationError{Msg: fmt.Sprintf("bind '%s' is already used as canary bind in upstream '%s' and cannot be used as canary in multiple upstreams", canaryBind, uo.PrimaryBind)}
				}
			}
		}

		// Validate that a bind cannot be used as canary if it has its own canary binds (prevent chaining)
		for _, uo := range upstreamOptions {
			if skipPrimaryBind != "" && uo.PrimaryBind == skipPrimaryBind {
				continue // Skip the current upstream being updated
			}
			if uo.PrimaryBind == canaryBind && len(uo.CanaryBinds) > 0 {
				return nil, &ValidationError{Msg: fmt.Sprintf("bind '%s' cannot be used as canary because it has its own canary binds, which would create a chain", canaryBind)}
			}
		}
	}

	// Validate that only one canary bind in the group has weight > 0
	if len(canaryBindsWithWeight) > 1 {
		return nil, &ValidationError{Msg: fmt.Sprintf("only one canary bind per group can have weight > 0, but found weight in multiple canary binds: %v", canaryBindsWithWeight)}
	}

	return canaryBindsWithWeight, nil
}

// validateTrafficShapingOptions validates traffic shaping policy rules
func validateTrafficShapingOptions(args UpstreamOptionsArgs) error {
	// Primary upstream cannot have any traffic shaping policy when it has canary binds
	if len(args.CanaryBinds) > 0 && hasTrafficShapingPolicy(args.TrafficShapingPolicy) {
		return &ValidationError{Msg: fmt.Sprintf("primary upstream '%s' cannot have traffic shaping policy when it has canary binds", args.PrimaryBind)}
	}

	// Validate that when header is specified, at least one of header-value or header-pattern must be provided
	if strings.TrimSpace(args.TrafficShapingPolicy.Header) != "" {
		headerValue := strings.TrimSpace(args.TrafficShapingPolicy.HeaderValue)
		headerPattern := strings.TrimSpace(args.TrafficShapingPolicy.HeaderPattern)
		if headerValue == "" && headerPattern == "" {
			return &ValidationError{Msg: "when header is specified, either header-value or header-pattern must be provided"}
		}
	}

	// Validate that header-value and header-pattern are mutually exclusive
	if strings.TrimSpace(args.TrafficShapingPolicy.HeaderValue) != "" && strings.TrimSpace(args.TrafficShapingPolicy.HeaderPattern) != "" {
		return &ValidationError{Msg: "header-value and header-pattern are mutually exclusive, please specify only one"}
	}

	return nil
}

// validateLoadBalanceOptions validates load balance algorithm and hash key
func validateLoadBalanceOptions(args UpstreamOptionsArgs) error {
	// Validate that LoadBalance algorithm is one of the supported values
	if args.LoadBalance != "" {
		validAlgorithms := []v1alpha1.LoadBalanceAlgorithm{
			v1alpha1.LoadBalanceRoundRobin,
			v1alpha1.LoadBalanceConsistentHash,
			v1alpha1.LoadBalanceEWMA,
		}

		isValid := false
		for _, valid := range validAlgorithms {
			if args.LoadBalance == valid {
				isValid = true
				break
			}
		}

		if !isValid {
			return &ValidationError{Msg: fmt.Sprintf("invalid loadBalance algorithm: %s. Valid values are: round_robin, chash, ewma", args.LoadBalance)}
		}
	}

	// Validate that LoadBalanceHashKey is required when LoadBalance is "chash"
	if args.LoadBalance == v1alpha1.LoadBalanceConsistentHash && strings.TrimSpace(args.LoadBalanceHashKey) == "" {
		return &ValidationError{Msg: "loadBalanceHashKey is required when loadBalance is \"chash\""}
	}

	// Validate that LoadBalanceHashKey is not provided for non-chash algorithms
	if args.LoadBalance != v1alpha1.LoadBalanceConsistentHash && strings.TrimSpace(args.LoadBalanceHashKey) != "" {
		return &ValidationError{Msg: "loadBalanceHashKey is only valid when loadBalance is \"chash\""}
	}

	return nil
}
