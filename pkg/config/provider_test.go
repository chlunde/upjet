// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const testProviderSchema = `{
  "format_version": "1.0",
  "provider_schemas": {
    "registry.terraform.io/hashicorp/test": {
      "provider": {
        "version": 0,
        "block": {}
      },
      "resource_schemas": {
        "test_resource": {
          "version": 0,
          "block": {
            "attributes": {
              "name": {
                "type": "string",
                "optional": true
              }
            }
          }
        }
      }
    }
  }
}`

func TestNewProviderClearsSchemaFuncAfterMaterialization(t *testing.T) {
	tp := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"test_resource": {
				SchemaFunc: func() map[string]*schema.Schema {
					return map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					}
				},
			},
		},
	}
	p := NewProvider([]byte(testProviderSchema), "test", "github.com/upbound/provider-test", nil,
		WithIncludeList(nil),
		WithTerraformPluginSDKIncludeList([]string{"test_resource"}),
		WithTerraformProvider(tp))

	r := p.Resources["test_resource"].TerraformResource
	if r == nil {
		t.Fatal("no Terraform resource registered for \"test_resource\"")
	}
	if r.Schema == nil {
		t.Error("Resource.Schema was not materialized from Resource.SchemaFunc")
	}
	if r.SchemaFunc != nil {
		t.Error("Resource.SchemaFunc was not cleared after its schema was materialized into Resource.Schema")
	}
	// The substantive assertion: configuration applied to Resource.Schema
	// must be visible through Resource.SchemaMap, which the SDK's read and
	// apply paths use. SchemaMap prefers SchemaFunc over Schema, so a stale
	// SchemaFunc would hide the mutation.
	r.Schema["injected"] = &schema.Schema{
		Type:     schema.TypeString,
		Computed: true,
	}
	if _, ok := r.SchemaMap()["injected"]; !ok {
		t.Error("mutation of Resource.Schema is not visible through Resource.SchemaMap; Resource.SchemaFunc is likely still set and shadowing Resource.Schema")
	}
	// The SDK considers a Resource with both Schema and SchemaFunc set
	// invalid.
	if err := r.InternalValidate(nil, true); err != nil {
		t.Errorf("Resource.InternalValidate failed: %v", err)
	}
}
