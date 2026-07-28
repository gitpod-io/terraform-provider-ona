// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfvalue

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestStableComputedString(t *testing.T) {
	t.Parallel()

	attribute := StableComputedString("description")
	type Expectation struct {
		Computed            bool
		MarkdownDescription string
		PlanModifiers       []string
	}
	modifiers := make([]string, 0, len(attribute.PlanModifiers))
	for _, modifier := range attribute.PlanModifiers {
		modifiers = append(modifiers, fmt.Sprintf("%T", modifier))
	}
	got := Expectation{Computed: attribute.Computed, MarkdownDescription: attribute.MarkdownDescription, PlanModifiers: modifiers}
	expected := Expectation{Computed: true, MarkdownDescription: "description", PlanModifiers: []string{"stringplanmodifier.useStateForUnknownModifier"}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("StableComputedString() mismatch (-want +got):\n%s", diff)
	}
}

func TestComputedDataSourceString(t *testing.T) {
	t.Parallel()

	attribute := ComputedDataSourceString("description")
	type Expectation struct {
		Computed            bool
		MarkdownDescription string
	}
	got := Expectation{Computed: attribute.Computed, MarkdownDescription: attribute.MarkdownDescription}
	expected := Expectation{Computed: true, MarkdownDescription: "description"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ComputedDataSourceString() mismatch (-want +got):\n%s", diff)
	}
}
