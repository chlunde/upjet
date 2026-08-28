// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	tf "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/crossplane/upjet/v2/pkg/config"
	"github.com/crossplane/upjet/v2/pkg/resource/fake"
)

// theSecret is the value that must never leave the provider through the change
// log. Every test below asserts on it rather than on a placeholder.
const theSecret = "hunter2-do-not-log"

func sdkConfigWithSchema(s map[string]*schema.Schema) *config.Resource {
	return &config.Resource{
		TerraformResource: &schema.Resource{Schema: s},
		ExternalName:      config.IdentifierFromProvider,
		Sensitive: config.Sensitive{AdditionalConnectionDetailsFn: func(_ map[string]any) (map[string][]byte, error) {
			return nil, nil
		}},
	}
}

// updateWithDiff runs the terraform plugin SDK external client's Update with
// the given instance diff already in place, as Observe would have left it, and
// returns the AdditionalDetails of the result together with the error, if any:
// the change logger records the details of failed operations too.
func updateWithDiff(t *testing.T, cfg *config.Resource, d *tf.InstanceDiff) (managed.AdditionalDetails, error) {
	t.Helper()
	r := mockResource{
		ApplyFn: func(_ context.Context, _ *tf.InstanceState, _ *tf.InstanceDiff, _ interface{}) (*tf.InstanceState, diag.Diagnostics) {
			return &tf.InstanceState{ID: "example-id"}, nil
		},
	}
	e := prepareTerraformPluginSDKExternal(r, cfg)
	e.instanceDiff = d
	o := fake.Terraformed{
		Parameterizable: fake.Parameterizable{Parameters: map[string]any{"name": "example"}},
		Observable:      fake.Observable{Observation: map[string]any{}},
	}
	u, err := e.Update(t.Context(), &o)
	return u.AdditionalDetails, err
}

func attrDiff(from, to string) *tf.ResourceAttrDiff {
	return &tf.ResourceAttrDiff{Old: from, New: to}
}

func TestTerraformPluginSDKUpdateAdditionalDetails(t *testing.T) {
	baseSchema := map[string]*schema.Schema{
		"name":     {Type: schema.TypeString, Required: true},
		"id":       {Type: schema.TypeString, Computed: true},
		"password": {Type: schema.TypeString, Optional: true, Sensitive: true},
		"token":    {Type: schema.TypeString, Optional: true},
		"tags":     {Type: schema.TypeMap, Elem: &schema.Schema{Type: schema.TypeString}},
		"subnets":  {Type: schema.TypeSet, Elem: &schema.Schema{Type: schema.TypeString}},
		"disk": {
			Type: schema.TypeList,
			Elem: &schema.Resource{Schema: map[string]*schema.Schema{
				"size":    {Type: schema.TypeInt, Optional: true},
				"kms_key": {Type: schema.TypeString, Optional: true, ForceNew: true},
			}},
		},
	}

	// A config whose upjet sensitive field path model - which is separate from
	// the Terraform schema's Sensitive flag - marks "token" sensitive.
	upjetSensitiveCfg := sdkConfigWithSchema(baseSchema)
	upjetSensitiveCfg.Sensitive.AddFieldPath("token", "spec.forProvider.tokenSecretRef")

	cases := map[string]struct {
		reason  string
		cfg     *config.Resource
		diff    *tf.InstanceDiff
		want    managed.AdditionalDetails
		wantErr bool
	}{
		"NoDiff": {
			reason: "A nil diff must not add anything to the change log entry.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff:   nil,
			want:   nil,
		},
		"EmptyDiff": {
			reason: "An empty diff must not add anything to the change log entry.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff:   &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{}},
			want:   nil,
		},
		"SingleAttribute": {
			reason: "A single changed attribute is reported by name.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"name": attrDiff("old", "new"),
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "name",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"SeveralAttributes": {
			reason: "Several changed attributes are reported sorted; collection elements collapse onto a single wildcarded path.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"name":            attrDiff("old", "new"),
				"tags.Owner":      attrDiff("a", "b"),
				"tags.Team":       attrDiff("c", "d"),
				"tags.%":          attrDiff("2", "2"),
				"disk.0.size":     attrDiff("10", "20"),
				"disk.1.size":     attrDiff("30", "40"),
				"subnets.1234567": attrDiff("", "subnet-1"),
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "disk.*.size, name, subnets.*, tags, tags.*",
				DetailsKeyChangedAttributeCount: "5",
			},
		},
		"RequiresReplace": {
			reason:  "upjet refuses an update that would replace the external resource; the details name the attribute that blocked it.",
			wantErr: true,
			cfg:     sdkConfigWithSchema(baseSchema),
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"disk.0.kms_key": {Old: "a", New: "b", RequiresNew: true},
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "disk.*.kms_key (requiresReplace)",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"SchemaSensitiveAttribute": {
			reason: "An attribute the Terraform schema marks sensitive is named and marked, but neither its old nor its new value is reported.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"password": attrDiff(theSecret, theSecret+"-new"),
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "password (sensitive)",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"DiffSensitiveAttribute": {
			reason: "An attribute the diff itself marks sensitive is marked even though the schema does not say so.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"token": {Old: theSecret, New: theSecret + "-new", Sensitive: true},
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "token (sensitive)",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"UpjetSensitiveFieldPath": {
			reason: "upjet's own sensitive field path model marks an attribute the Terraform schema and the diff both consider ordinary.",
			cfg:    upjetSensitiveCfg,
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"token": attrDiff(theSecret, theSecret+"-new"),
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "token (sensitive)",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"MapKeysAreNotReported": {
			reason: "Map keys are user data, not schema identifiers, and must never be reported.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"tags." + theSecret: attrDiff("a", "b"),
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "tags.*",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"UnknownAttributeIsDropped": {
			reason: "A key the resource's Terraform schema does not declare is dropped rather than guessed at.",
			cfg:    sdkConfigWithSchema(baseSchema),
			diff: &tf.InstanceDiff{Attributes: map[string]*tf.ResourceAttrDiff{
				"not_in_the_schema": attrDiff("a", "b"),
				"name":              attrDiff("old", "new"),
			}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "name",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := updateWithDiff(t, tc.cfg, tc.diff)
			if tc.wantErr != (err != nil) {
				t.Fatalf("Update(...): wantErr %v, got %v", tc.wantErr, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nUpdate(...).AdditionalDetails: -want, +got:\n%s", tc.reason, diff)
			}
			assertNoSecretLeak(t, got)
		})
	}
}

// assertNoSecretLeak is the check that matters most: whatever the helper
// decides to report, no attribute value may appear in it.
func assertNoSecretLeak(t *testing.T, d managed.AdditionalDetails) {
	t.Helper()
	for k, v := range d {
		if strings.Contains(k, theSecret) || strings.Contains(v, theSecret) {
			t.Errorf("sensitive value leaked into AdditionalDetails[%q] = %q", k, v)
		}
	}
}

func TestTerraformPluginSDKUpdateAdditionalDetailsTruncation(t *testing.T) {
	const total = 100
	s := map[string]*schema.Schema{
		// prepareTerraformPluginSDKExternal builds its raw config from this.
		"name": {Type: schema.TypeString, Required: true},
	}
	attrs := map[string]*tf.ResourceAttrDiff{}
	for i := 0; i < total; i++ {
		k := fmt.Sprintf("attribute_%03d", i)
		s[k] = &schema.Schema{Type: schema.TypeString, Optional: true}
		attrs[k] = attrDiff("old", "new")
	}

	got, err := updateWithDiff(t, sdkConfigWithSchema(s), &tf.InstanceDiff{Attributes: attrs})
	if err != nil {
		t.Fatalf("Update(...): unexpected error: %v", err)
	}

	if got[DetailsKeyChangedAttributeCount] != fmt.Sprint(total) {
		t.Errorf("the total number of changed attributes must survive truncation: want %d, got %q", total, got[DetailsKeyChangedAttributeCount])
	}
	rendered := got[DetailsKeyChangedAttributes]
	if len(rendered) > maxChangedAttributesBytes {
		t.Errorf("rendered value is %d bytes, which exceeds the %d byte cap", len(rendered), maxChangedAttributesBytes)
	}
	omitted := total - maxChangedAttributes
	wantSuffix := fmt.Sprintf(", ... (%d more omitted)", omitted)
	if !strings.HasSuffix(rendered, wantSuffix) {
		t.Errorf("truncation must be visible in the rendered value: want suffix %q, got %q", wantSuffix, rendered)
	}
	if n := strings.Count(strings.TrimSuffix(rendered, wantSuffix), ", ") + 1; n != maxChangedAttributes {
		t.Errorf("want %d rendered attributes, got %d: %q", maxChangedAttributes, n, rendered)
	}
	// Truncation keeps the sorted prefix, so the first and the last rendered
	// entries are deterministic.
	if !strings.HasPrefix(rendered, "attribute_000, attribute_001, ") {
		t.Errorf("want the sorted prefix of the changed attribute set, got %q", rendered)
	}
}

func TestTPFUpdateAdditionalDetails(t *testing.T) {
	sensitiveSchema := rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"name": rschema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"id": rschema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"password": rschema.StringAttribute{
				Required:  true,
				Sensitive: true,
			},
			"tags": rschema.MapAttribute{
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}

	cases := map[string]struct {
		reason  string
		schema  rschema.Schema
		current map[string]any
		planned map[string]any
		want    managed.AdditionalDetails
	}{
		"NoDiff": {
			reason:  "A plan that matches the observed state must not add anything to the change log entry.",
			schema:  newBaseSchema(),
			current: map[string]any{"id": "example-id", "name": "example", "map": map[string]any{"key": "value"}, "list": []any{"elem1"}},
			planned: map[string]any{"id": "example-id", "name": "example", "map": map[string]any{"key": "value"}, "list": []any{"elem1"}},
			want:    nil,
		},
		"SingleAttribute": {
			reason:  "A single changed attribute is reported by name.",
			schema:  newBaseSchema(),
			current: map[string]any{"id": "example-id", "name": "example", "map": map[string]any{"key": "value"}, "list": []any{"elem1"}},
			planned: map[string]any{"id": "example-id", "name": "updated", "map": map[string]any{"key": "value"}, "list": []any{"elem1"}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "name",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"SeveralAttributes": {
			reason:  "Several changed attributes are reported sorted, with collection element keys wildcarded.",
			schema:  newBaseSchema(),
			current: map[string]any{"id": "example-id", "name": "example", "map": map[string]any{"key": "value"}, "list": []any{"elem1"}},
			planned: map[string]any{"id": "example-id", "name": "updated", "map": map[string]any{"key": "changed"}, "list": []any{"elem2"}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "list.*, map.*, name",
				DetailsKeyChangedAttributeCount: "3",
			},
		},
		"SensitiveAttribute": {
			reason:  "A sensitive attribute is named and marked, but neither its old nor its new value is reported.",
			schema:  sensitiveSchema,
			current: map[string]any{"id": "example-id", "name": "example", "password": theSecret, "tags": map[string]any{"k": "v"}},
			planned: map[string]any{"id": "example-id", "name": "example", "password": theSecret + "-new", "tags": map[string]any{"k": "v"}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "password (sensitive)",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
		"MapKeysAreNotReported": {
			reason:  "Map keys are user data, not schema identifiers, and must never be reported.",
			schema:  sensitiveSchema,
			current: map[string]any{"id": "example-id", "name": "example", "password": "x", "tags": map[string]any{theSecret: "v"}},
			planned: map[string]any{"id": "example-id", "name": "example", "password": "x", "tags": map[string]any{theSecret: "changed"}},
			want: managed.AdditionalDetails{
				DetailsKeyChangedAttributes:     "tags.*",
				DetailsKeyChangedAttributeCount: "1",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := tc.schema
			r := &mockTPFResource{
				SchemaMethod: func(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
					response.Schema = s
				},
			}
			cfg := newBaseUpjetConfig()
			cfg.TerraformPluginFrameworkResource = r
			e := prepareTPFExternalWithTestConfig(testConfiguration{
				r:               r,
				cfg:             cfg,
				obj:             newBaseObject(),
				params:          tc.planned,
				currentStateMap: tc.current,
				plannedStateMap: tc.planned,
				newStateMap:     tc.planned,
			})

			// Observe is what computes the plan and, with it, the changed
			// attribute set that Update reports.
			if _, err := e.Observe(t.Context(), &fake.Terraformed{
				Parameterizable: fake.Parameterizable{Parameters: tc.planned},
				Observable:      fake.Observable{Observation: map[string]any{}},
			}); err != nil {
				t.Fatalf("Observe(...): unexpected error: %v", err)
			}

			o := newBaseObject()
			u, err := e.Update(t.Context(), &o)
			if err != nil {
				t.Fatalf("Update(...): unexpected error: %v", err)
			}
			if diff := cmp.Diff(tc.want, u.AdditionalDetails); diff != "" {
				t.Errorf("%s\nUpdate(...).AdditionalDetails: -want, +got:\n%s", tc.reason, diff)
			}
			assertNoSecretLeak(t, u.AdditionalDetails)
		})
	}
}
