// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apiterraform "github.com/gitpod-io/terraform-provider-ona/internal/api/go/tools/terraform"
	"github.com/hashicorp/terraform-plugin-codegen-spec/code"
	"github.com/hashicorp/terraform-plugin-codegen-spec/provider"
	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
	codeschema "github.com/hashicorp/terraform-plugin-codegen-spec/schema"
	"github.com/hashicorp/terraform-plugin-codegen-spec/spec"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const providerName = "ona"

// BuildSpecification builds a HashiCorp Provider Code Specification from proto Terraform annotations.
func BuildSpecification(files ...protoreflect.FileDescriptor) (spec.Specification, error) {
	var resources resource.Resources
	for _, file := range files {
		for i := range file.Messages().Len() {
			rs, err := resourceFromMessage(file.Messages().Get(i))
			if err != nil {
				return spec.Specification{}, err
			}
			resources = append(resources, rs...)
		}
	}

	result := spec.Specification{
		Provider:  &provider.Provider{Name: providerName},
		Resources: resources,
		Version:   spec.Version0_1,
	}
	if err := result.Validate(context.Background()); err != nil {
		return spec.Specification{}, fmt.Errorf("validate provider code specification: %w", err)
	}
	return result, nil
}

func MarshalSpecification(s spec.Specification) ([]byte, error) {
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal provider code specification: %w", err)
	}
	return append(out, '\n'), nil
}

func resourceFromMessage(message protoreflect.MessageDescriptor) (resource.Resources, error) {
	var resources resource.Resources
	if options, ok := message.Options().(*descriptorpb.MessageOptions); ok && proto.HasExtension(options, apiterraform.E_TerraformResource) {
		ext := proto.GetExtension(options, apiterraform.E_TerraformResource)
		annot, ok := ext.(*apiterraform.TerraformResource)
		if !ok {
			return nil, fmt.Errorf("%s Terraform resource annotation has type %T", message.FullName(), ext)
		}
		rs, err := buildResource(message, annot)
		if err != nil {
			return nil, err
		}
		resources = append(resources, rs)
	}

	for i := range message.Messages().Len() {
		nested, err := resourceFromMessage(message.Messages().Get(i))
		if err != nil {
			return nil, err
		}
		resources = append(resources, nested...)
	}
	return resources, nil
}

func buildResource(message protoreflect.MessageDescriptor, annot *apiterraform.TerraformResource) (resource.Resource, error) {
	name := resourceName(message, annot)
	importIDField := importIDFieldName(message, name, annot)
	if err := validateImportIDField(message, importIDField); err != nil {
		return resource.Resource{}, err
	}

	var attrs resource.Attributes
	for i := range message.Fields().Len() {
		field := message.Fields().Get(i)
		attr, ok, err := attributeFromField(message, field, name, importIDField)
		if err != nil {
			return resource.Resource{}, err
		}
		if ok {
			attrs = append(attrs, attr)
		}
	}
	if len(attrs) == 0 {
		return resource.Resource{}, fmt.Errorf("%s Terraform resource has no annotated fields", message.FullName())
	}

	return resource.Resource{
		Name: name,
		Schema: &resource.Schema{
			Attributes:  attrs,
			Description: stringPtr(resourceDescription(name, annot.GetDescription())),
		},
	}, nil
}

func validateImportIDField(message protoreflect.MessageDescriptor, importIDField string) error {
	if importIDField == "" {
		return nil
	}

	field := message.Fields().ByName(protoreflect.Name(importIDField))
	if field == nil {
		return fmt.Errorf("%s Terraform import_id_field %q does not match a proto field", message.FullName(), importIDField)
	}

	options, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || !proto.HasExtension(options, apiterraform.E_TerraformField) {
		return fmt.Errorf("%s Terraform import_id_field %q must reference a field annotated with terraform_field", message.FullName(), importIDField)
	}

	return nil
}

func attributeFromField(message protoreflect.MessageDescriptor, field protoreflect.FieldDescriptor, resourceName string, importIDField string) (resource.Attribute, bool, error) {
	options, ok := field.Options().(*descriptorpb.FieldOptions)
	if !ok || !proto.HasExtension(options, apiterraform.E_TerraformField) {
		return resource.Attribute{}, false, nil
	}

	ext := proto.GetExtension(options, apiterraform.E_TerraformField)
	annot, ok := ext.(*apiterraform.TerraformField)
	if !ok {
		return resource.Attribute{}, false, fmt.Errorf("%s.%s Terraform field annotation has type %T", message.FullName(), field.Name(), ext)
	}

	name := string(field.Name())
	if annot.Name != "" {
		name = annot.Name
	} else if string(field.Name()) == importIDField {
		name = "id"
	}
	mode, err := fieldMode(annot.Mode)
	if err != nil {
		return resource.Attribute{}, false, fmt.Errorf("%s.%s: %w", message.FullName(), field.Name(), err)
	}
	description := fieldDescription(field, resourceName, name, importIDField, annot.Description)
	sensitive := annot.Sensitive

	switch field.Kind() {
	case protoreflect.StringKind:
		attr := resource.Attribute{
			Name: name,
			String: &resource.StringAttribute{
				ComputedOptionalRequired: mode,
				Description:              stringPtr(description),
				Sensitive:                boolPtrIfTrue(sensitive),
				PlanModifiers:            stringPlanModifiers(field, importIDField, annot),
			},
		}
		return attr, true, nil
	default:
		return resource.Attribute{}, false, fmt.Errorf("%s.%s has unsupported Terraform field kind %s", message.FullName(), field.Name(), field.Kind())
	}
}

func fieldMode(mode apiterraform.TerraformFieldMode) (codeschema.ComputedOptionalRequired, error) {
	switch mode {
	case apiterraform.TerraformFieldMode_TERRAFORM_FIELD_MODE_UNSPECIFIED:
		return codeschema.Computed, nil
	case apiterraform.TerraformFieldMode_TERRAFORM_FIELD_MODE_COMPUTED:
		return codeschema.Computed, nil
	case apiterraform.TerraformFieldMode_TERRAFORM_FIELD_MODE_OPTIONAL:
		return codeschema.Optional, nil
	case apiterraform.TerraformFieldMode_TERRAFORM_FIELD_MODE_REQUIRED:
		return codeschema.Required, nil
	case apiterraform.TerraformFieldMode_TERRAFORM_FIELD_MODE_COMPUTED_OPTIONAL:
		return codeschema.ComputedOptional, nil
	default:
		return "", fmt.Errorf("unsupported Terraform field mode %s", mode)
	}
}

func stringPlanModifiers(field protoreflect.FieldDescriptor, importIDField string, annot *apiterraform.TerraformField) codeschema.StringPlanModifiers {
	useStateForUnknown := string(field.Name()) == importIDField
	if annot.UseStateForUnknown != nil {
		useStateForUnknown = annot.GetUseStateForUnknown()
	}
	if !useStateForUnknown {
		return nil
	}
	return codeschema.StringPlanModifiers{
		{
			Custom: &codeschema.CustomPlanModifier{
				Imports: []code.Import{
					{Path: "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"},
				},
				SchemaDefinition: "stringplanmodifier.UseStateForUnknown()",
			},
		},
	}
}

func resourceName(message protoreflect.MessageDescriptor, annot *apiterraform.TerraformResource) string {
	if annot.Name != "" {
		return annot.Name
	}
	return snakeName(string(message.Name()))
}

func importIDFieldName(message protoreflect.MessageDescriptor, resourceName string, annot *apiterraform.TerraformResource) string {
	if annot.ImportIdField != "" {
		return annot.ImportIdField
	}

	messageName := snakeName(string(message.Name()))
	candidates := []string{
		resourceName + "_id",
		messageName + "_id",
		"id",
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if message.Fields().ByName(protoreflect.Name(candidate)) != nil {
			return candidate
		}
	}

	return ""
}

func resourceDescription(resourceName string, override string) string {
	if override != "" {
		return override
	}
	providerDisplayName := "Ona"
	return fmt.Sprintf("Represents %s %s %s.", article(providerDisplayName), providerDisplayName, humanName(resourceName))
}

func fieldDescription(field protoreflect.FieldDescriptor, resourceName string, attributeName string, importIDField string, override string) string {
	if override != "" {
		return override
	}
	if string(field.Name()) == importIDField {
		return fmt.Sprintf("%s ID. Use this value as the Terraform import ID.", titleName(resourceName))
	}
	return fmt.Sprintf("%s %s.", titleName(resourceName), humanName(attributeName))
}

func snakeName(name string) string {
	if name == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range name {
		if 'A' <= r && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func humanName(name string) string {
	return strings.ReplaceAll(name, "_", " ")
}

func titleName(name string) string {
	parts := strings.Split(humanName(name), " ")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func article(word string) string {
	if word == "" {
		return "a"
	}
	switch strings.ToLower(word[:1]) {
	case "a", "e", "i", "o", "u":
		return "an"
	default:
		return "a"
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func boolPtrIfTrue(value bool) *bool {
	if !value {
		return nil
	}
	return &value
}
