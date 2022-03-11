 // +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package acrcrd

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AvailableCapacityReservation.
func (in *AvailableCapacityReservation) DeepCopy() *AvailableCapacityReservation {
	if in == nil {
		return nil
	}
	out := new(AvailableCapacityReservation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AvailableCapacityReservation) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AvailableCapacityReservationList) DeepCopyInto(out *AvailableCapacityReservationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AvailableCapacityReservation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AvailableCapacityReservationList.
func (in *AvailableCapacityReservationList) DeepCopy() *AvailableCapacityReservationList {
	if in == nil {
		return nil
	}
	out := new(AvailableCapacityReservationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AvailableCapacityReservationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
