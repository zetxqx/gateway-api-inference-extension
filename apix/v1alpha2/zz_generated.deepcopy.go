//go:build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha2

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EndpointPickerConfig) DeepCopyInto(out *EndpointPickerConfig) {
	*out = *in
	if in.ExtensionRef != nil {
		in, out := &in.ExtensionRef, &out.ExtensionRef
		*out = new(Extension)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EndpointPickerConfig.
func (in *EndpointPickerConfig) DeepCopy() *EndpointPickerConfig {
	if in == nil {
		return nil
	}
	out := new(EndpointPickerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Extension) DeepCopyInto(out *Extension) {
	*out = *in
	in.ExtensionReference.DeepCopyInto(&out.ExtensionReference)
	in.ExtensionConnection.DeepCopyInto(&out.ExtensionConnection)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Extension.
func (in *Extension) DeepCopy() *Extension {
	if in == nil {
		return nil
	}
	out := new(Extension)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExtensionConnection) DeepCopyInto(out *ExtensionConnection) {
	*out = *in
	if in.FailureMode != nil {
		in, out := &in.FailureMode, &out.FailureMode
		*out = new(ExtensionFailureMode)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExtensionConnection.
func (in *ExtensionConnection) DeepCopy() *ExtensionConnection {
	if in == nil {
		return nil
	}
	out := new(ExtensionConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExtensionReference) DeepCopyInto(out *ExtensionReference) {
	*out = *in
	if in.Group != nil {
		in, out := &in.Group, &out.Group
		*out = new(Group)
		**out = **in
	}
	if in.Kind != nil {
		in, out := &in.Kind, &out.Kind
		*out = new(Kind)
		**out = **in
	}
	if in.PortNumber != nil {
		in, out := &in.PortNumber, &out.PortNumber
		*out = new(PortNumber)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExtensionReference.
func (in *ExtensionReference) DeepCopy() *ExtensionReference {
	if in == nil {
		return nil
	}
	out := new(ExtensionReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferenceObjective) DeepCopyInto(out *InferenceObjective) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferenceObjective.
func (in *InferenceObjective) DeepCopy() *InferenceObjective {
	if in == nil {
		return nil
	}
	out := new(InferenceObjective)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InferenceObjective) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferenceObjectiveList) DeepCopyInto(out *InferenceObjectiveList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]InferenceObjective, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferenceObjectiveList.
func (in *InferenceObjectiveList) DeepCopy() *InferenceObjectiveList {
	if in == nil {
		return nil
	}
	out := new(InferenceObjectiveList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InferenceObjectiveList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferenceObjectiveSpec) DeepCopyInto(out *InferenceObjectiveSpec) {
	*out = *in
	if in.Criticality != nil {
		in, out := &in.Criticality, &out.Criticality
		*out = new(Criticality)
		**out = **in
	}
	if in.TargetModels != nil {
		in, out := &in.TargetModels, &out.TargetModels
		*out = make([]TargetModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.PoolRef = in.PoolRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferenceObjectiveSpec.
func (in *InferenceObjectiveSpec) DeepCopy() *InferenceObjectiveSpec {
	if in == nil {
		return nil
	}
	out := new(InferenceObjectiveSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferenceObjectiveStatus) DeepCopyInto(out *InferenceObjectiveStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferenceObjectiveStatus.
func (in *InferenceObjectiveStatus) DeepCopy() *InferenceObjectiveStatus {
	if in == nil {
		return nil
	}
	out := new(InferenceObjectiveStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferencePool) DeepCopyInto(out *InferencePool) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferencePool.
func (in *InferencePool) DeepCopy() *InferencePool {
	if in == nil {
		return nil
	}
	out := new(InferencePool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InferencePool) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferencePoolList) DeepCopyInto(out *InferencePoolList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]InferencePool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferencePoolList.
func (in *InferencePoolList) DeepCopy() *InferencePoolList {
	if in == nil {
		return nil
	}
	out := new(InferencePoolList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InferencePoolList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferencePoolSpec) DeepCopyInto(out *InferencePoolSpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[LabelKey]LabelValue, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.EndpointPickerConfig.DeepCopyInto(&out.EndpointPickerConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferencePoolSpec.
func (in *InferencePoolSpec) DeepCopy() *InferencePoolSpec {
	if in == nil {
		return nil
	}
	out := new(InferencePoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InferencePoolStatus) DeepCopyInto(out *InferencePoolStatus) {
	*out = *in
	if in.Parents != nil {
		in, out := &in.Parents, &out.Parents
		*out = make([]PoolStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InferencePoolStatus.
func (in *InferencePoolStatus) DeepCopy() *InferencePoolStatus {
	if in == nil {
		return nil
	}
	out := new(InferencePoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ParentGatewayReference) DeepCopyInto(out *ParentGatewayReference) {
	*out = *in
	if in.Group != nil {
		in, out := &in.Group, &out.Group
		*out = new(Group)
		**out = **in
	}
	if in.Kind != nil {
		in, out := &in.Kind, &out.Kind
		*out = new(Kind)
		**out = **in
	}
	if in.Namespace != nil {
		in, out := &in.Namespace, &out.Namespace
		*out = new(Namespace)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ParentGatewayReference.
func (in *ParentGatewayReference) DeepCopy() *ParentGatewayReference {
	if in == nil {
		return nil
	}
	out := new(ParentGatewayReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PoolObjectReference) DeepCopyInto(out *PoolObjectReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PoolObjectReference.
func (in *PoolObjectReference) DeepCopy() *PoolObjectReference {
	if in == nil {
		return nil
	}
	out := new(PoolObjectReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PoolStatus) DeepCopyInto(out *PoolStatus) {
	*out = *in
	in.GatewayRef.DeepCopyInto(&out.GatewayRef)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PoolStatus.
func (in *PoolStatus) DeepCopy() *PoolStatus {
	if in == nil {
		return nil
	}
	out := new(PoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetModel) DeepCopyInto(out *TargetModel) {
	*out = *in
	if in.Weight != nil {
		in, out := &in.Weight, &out.Weight
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetModel.
func (in *TargetModel) DeepCopy() *TargetModel {
	if in == nil {
		return nil
	}
	out := new(TargetModel)
	in.DeepCopyInto(out)
	return out
}
