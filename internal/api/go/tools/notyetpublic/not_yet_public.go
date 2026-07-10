package notyetpublic

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func Service(opts *descriptorpb.ServiceOptions) (*NotYetPublic, bool) {
	if opts == nil {
		return nil, false
	}
	return get(opts, E_NotYetPublicService)
}

func Method(opts *descriptorpb.MethodOptions) (*NotYetPublic, bool) {
	if opts == nil {
		return nil, false
	}
	return get(opts, E_NotYetPublicMethod)
}

func Message(opts *descriptorpb.MessageOptions) (*NotYetPublic, bool) {
	if opts == nil {
		return nil, false
	}
	return get(opts, E_NotYetPublicMessage)
}

func Enum(opts *descriptorpb.EnumOptions) (*NotYetPublic, bool) {
	if opts == nil {
		return nil, false
	}
	return get(opts, E_NotYetPublicEnum)
}

func Field(opts *descriptorpb.FieldOptions) (*NotYetPublic, bool) {
	if opts == nil {
		return nil, false
	}
	return get(opts, E_NotYetPublicField)
}

func get(opts proto.Message, ext protoreflect.ExtensionType) (*NotYetPublic, bool) {
	if !proto.HasExtension(opts, ext) {
		return nil, false
	}
	value, ok := proto.GetExtension(opts, ext).(*NotYetPublic)
	return value, ok && value != nil
}
