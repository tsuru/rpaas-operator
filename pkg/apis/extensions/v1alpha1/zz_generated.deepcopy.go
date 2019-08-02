// +build !ignore_autogenerated

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	nginxv1alpha1 "github.com/tsuru/nginx-operator/pkg/apis/nginx/v1alpha1"
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigRef) DeepCopyInto(out *ConfigRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigRef.
func (in *ConfigRef) DeepCopy() *ConfigRef {
	if in == nil {
		return nil
	}
	out := new(ConfigRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Location) DeepCopyInto(out *Location) {
	*out = *in
	if in.Content != nil {
		in, out := &in.Content, &out.Content
		*out = new(Value)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Location.
func (in *Location) DeepCopy() *Location {
	if in == nil {
		return nil
	}
	out := new(Location)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxConfig) DeepCopyInto(out *NginxConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxConfig.
func (in *NginxConfig) DeepCopy() *NginxConfig {
	if in == nil {
		return nil
	}
	out := new(NginxConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RpaasInstance) DeepCopyInto(out *RpaasInstance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RpaasInstance.
func (in *RpaasInstance) DeepCopy() *RpaasInstance {
	if in == nil {
		return nil
	}
	out := new(RpaasInstance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RpaasInstance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RpaasInstanceList) DeepCopyInto(out *RpaasInstanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RpaasInstance, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RpaasInstanceList.
func (in *RpaasInstanceList) DeepCopy() *RpaasInstanceList {
	if in == nil {
		return nil
	}
	out := new(RpaasInstanceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RpaasInstanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RpaasInstanceSpec) DeepCopyInto(out *RpaasInstanceSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Blocks != nil {
		in, out := &in.Blocks, &out.Blocks
		*out = make(map[BlockType]ConfigRef, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Locations != nil {
		in, out := &in.Locations, &out.Locations
		*out = make([]Location, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Certificates != nil {
		in, out := &in.Certificates, &out.Certificates
		*out = new(nginxv1alpha1.TLSSecret)
		(*in).DeepCopyInto(*out)
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(nginxv1alpha1.NginxService)
		(*in).DeepCopyInto(*out)
	}
	if in.ExtraFiles != nil {
		in, out := &in.ExtraFiles, &out.ExtraFiles
		*out = new(nginxv1alpha1.FilesRef)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RpaasInstanceSpec.
func (in *RpaasInstanceSpec) DeepCopy() *RpaasInstanceSpec {
	if in == nil {
		return nil
	}
	out := new(RpaasInstanceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RpaasInstanceStatus) DeepCopyInto(out *RpaasInstanceStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RpaasInstanceStatus.
func (in *RpaasInstanceStatus) DeepCopy() *RpaasInstanceStatus {
	if in == nil {
		return nil
	}
	out := new(RpaasInstanceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RpaasPlan) DeepCopyInto(out *RpaasPlan) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RpaasPlan.
func (in *RpaasPlan) DeepCopy() *RpaasPlan {
	if in == nil {
		return nil
	}
	out := new(RpaasPlan)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RpaasPlan) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RpaasPlanList) DeepCopyInto(out *RpaasPlanList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RpaasPlan, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RpaasPlanList.
func (in *RpaasPlanList) DeepCopy() *RpaasPlanList {
	if in == nil {
		return nil
	}
	out := new(RpaasPlanList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RpaasPlanList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RpaasPlanSpec) DeepCopyInto(out *RpaasPlanSpec) {
	*out = *in
	out.Config = in.Config
	if in.Template != nil {
		in, out := &in.Template, &out.Template
		*out = new(Value)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RpaasPlanSpec.
func (in *RpaasPlanSpec) DeepCopy() *RpaasPlanSpec {
	if in == nil {
		return nil
	}
	out := new(RpaasPlanSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Value) DeepCopyInto(out *Value) {
	*out = *in
	if in.ValueFrom != nil {
		in, out := &in.ValueFrom, &out.ValueFrom
		*out = new(ValueSource)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Value.
func (in *Value) DeepCopy() *Value {
	if in == nil {
		return nil
	}
	out := new(Value)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValueSource) DeepCopyInto(out *ValueSource) {
	*out = *in
	if in.ConfigMapKeyRef != nil {
		in, out := &in.ConfigMapKeyRef, &out.ConfigMapKeyRef
		*out = new(v1.ConfigMapKeySelector)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValueSource.
func (in *ValueSource) DeepCopy() *ValueSource {
	if in == nil {
		return nil
	}
	out := new(ValueSource)
	in.DeepCopyInto(out)
	return out
}
