// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	tf "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/upjet/v2/pkg/config"
	"github.com/crossplane/upjet/v2/pkg/resource/fake"
	"github.com/crossplane/upjet/v2/pkg/terraform"
)

var (
	zl      = zap.New(zap.UseDevMode(true))
	logTest = logging.NewLogrLogger(zl.WithName("provider-aws"))
	ots     = NewOperationStore(logTest)
	timeout = time.Duration(1200000000000)
	cfg     = &config.Resource{
		TerraformResource: &schema.Resource{
			Timeouts: &schema.ResourceTimeout{
				Create: &timeout,
				Read:   &timeout,
				Update: &timeout,
				Delete: &timeout,
			},
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"id": {
					Type:     schema.TypeString,
					Computed: true,
					Required: false,
				},
				"map": {
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				"list": {
					Type: schema.TypeList,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
			},
		},
		ExternalName: config.IdentifierFromProvider,
		Sensitive: config.Sensitive{AdditionalConnectionDetailsFn: func(attr map[string]any) (map[string][]byte, error) {
			return nil, nil
		}},
	}
	obj = fake.Terraformed{
		Parameterizable: fake.Parameterizable{
			Parameters: map[string]any{
				"name": "example",
				"map": map[string]any{
					"key": "value",
				},
				"list": []any{"elem1", "elem2"},
			},
		},
		Observable: fake.Observable{
			Observation: map[string]any{},
		},
	}
)

func prepareTerraformPluginSDKExternal(r Resource, cfg *config.Resource) *terraformPluginSDKExternal {
	schemaBlock := cfg.TerraformResource.CoreConfigSchema()
	rawConfig, err := schema.JSONMapToStateValue(map[string]any{"name": "example"}, schemaBlock)
	if err != nil {
		panic(err)
	}
	return &terraformPluginSDKExternal{
		ts:             terraform.Setup{},
		resourceSchema: r,
		config:         cfg,
		params: map[string]any{
			"name": "example",
		},
		rawConfig: rawConfig,
		logger:    logTest,
		opTracker: NewAsyncTracker(),
	}
}

type mockResource struct {
	ApplyFn                 func(ctx context.Context, s *tf.InstanceState, d *tf.InstanceDiff, meta interface{}) (*tf.InstanceState, diag.Diagnostics)
	RefreshWithoutUpgradeFn func(ctx context.Context, s *tf.InstanceState, meta interface{}) (*tf.InstanceState, diag.Diagnostics)
}

func (m mockResource) Apply(ctx context.Context, s *tf.InstanceState, d *tf.InstanceDiff, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
	return m.ApplyFn(ctx, s, d, meta)
}

func (m mockResource) RefreshWithoutUpgrade(ctx context.Context, s *tf.InstanceState, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
	return m.RefreshWithoutUpgradeFn(ctx, s, meta)
}

func TestTerraformPluginSDKConnect(t *testing.T) {
	type args struct {
		setupFn terraform.SetupFn
		cfg     *config.Resource
		ots     *OperationTrackerStore
		obj     fake.Terraformed
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"Successful": {
			args: args{
				setupFn: func(_ context.Context, _ client.Client, _ xpresource.Managed) (terraform.Setup, error) {
					return terraform.Setup{}, nil
				},
				cfg: cfg,
				obj: obj,
				ots: ots,
			},
		},
		"HCL": {
			args: args{
				setupFn: func(_ context.Context, _ client.Client, _ xpresource.Managed) (terraform.Setup, error) {
					return terraform.Setup{}, nil
				},
				cfg: cfg,
				obj: fake.Terraformed{
					Parameterizable: fake.Parameterizable{
						Parameters: map[string]any{
							"name": "      ${jsonencode({\n          type = \"object\"\n        })}",
							"map": map[string]any{
								"key": "value",
							},
							"list": []any{"elem1", "elem2"},
						},
					},
					Observable: fake.Observable{
						Observation: map[string]any{},
					},
				},
				ots: ots,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewTerraformPluginSDKConnector(nil, tc.args.setupFn, tc.args.cfg, tc.args.ots, WithTerraformPluginSDKLogger(logTest))
			_, err := c.Connect(t.Context(), &tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConnect(...): -want error, +got error:\n", diff)
			}
		})
	}
}

func TestTerraformPluginSDKObserve(t *testing.T) {
	type args struct {
		r   Resource
		cfg *config.Resource
		obj fake.Terraformed
	}
	type want struct {
		obs managed.ExternalObservation
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"NotExists": {
			args: args{
				r: mockResource{
					RefreshWithoutUpgradeFn: func(ctx context.Context, s *tf.InstanceState, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
						return nil, nil
					},
				},
				cfg: cfg,
				obj: obj,
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceUpToDate:        false,
					ResourceLateInitialized: false,
					ConnectionDetails:       nil,
					Diff:                    "",
				},
			},
		},
		"UpToDate": {
			args: args{
				r: mockResource{
					RefreshWithoutUpgradeFn: func(ctx context.Context, s *tf.InstanceState, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
						return &tf.InstanceState{ID: "example-id", Attributes: map[string]string{"name": "example"}}, nil
					},
				},
				cfg: cfg,
				obj: obj,
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        true,
					ResourceLateInitialized: true,
					ConnectionDetails:       nil,
					Diff:                    "",
				},
			},
		},
		"InitProvider": {
			args: args{
				r: mockResource{
					RefreshWithoutUpgradeFn: func(ctx context.Context, s *tf.InstanceState, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
						return &tf.InstanceState{ID: "example-id", Attributes: map[string]string{"name": "example2"}}, nil
					},
				},
				cfg: cfg,
				obj: fake.Terraformed{
					Parameterizable: fake.Parameterizable{
						Parameters: map[string]any{
							"name": "example",
							"map": map[string]any{
								"key": "value",
							},
							"list": []any{"elem1", "elem2"},
						},
						InitParameters: map[string]any{
							"list": []any{"elem1", "elem2", "elem3"},
						},
					},
					Observable: fake.Observable{
						Observation: map[string]any{},
					},
				},
			},
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          true,
					ResourceUpToDate:        false,
					ResourceLateInitialized: true,
					ConnectionDetails:       nil,
					Diff:                    "",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			terraformPluginSDKExternal := prepareTerraformPluginSDKExternal(tc.args.r, tc.args.cfg)
			observation, err := terraformPluginSDKExternal.Observe(t.Context(), &tc.args.obj)
			if diff := cmp.Diff(tc.want.obs, observation); diff != "" {
				t.Errorf("\n%s\nObserve(...): -want observation, +got observation:\n", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConnect(...): -want error, +got error:\n", diff)
			}
		})
	}
}

// TestTerraformPluginSDKObserveNotFound is a regression test
// for the "value is not an object" panic (e.g., in provider-aws).
// When Observe calls schema.Diff with an InstanceState whose RawPlan is
// the zero cty.Value, the AWS provider's setTagsAll() interceptor calls
// diff.GetRawPlan().GetAttr("tags"), and go-cty's Value.GetAttr panics with
// "value is not an object" on the nil cty.Value.
// The same applies to RawConfig.
func TestTerraformPluginSDKObserveNotFound(t *testing.T) {
	tagsTimeout := timeout
	cases := map[string]struct {
		customizeDiff schema.CustomizeDiffFunc
		description   string
	}{
		"RawConfig": {
			customizeDiff: func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
				// Calling d.GetRawConfig().GetAttr on a zero RawConfig will panic.
				_ = d.GetRawConfig().GetAttr("name")
				return nil
			},
			description: "Observed a panic when reading from InstanceState.RawConfig",
		},
		"RawPlan": {
			customizeDiff: func(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
				// Calling d.GetRawPlan().GetAttr on a zero RawPlan will panic.
				_ = d.GetRawPlan().GetAttr("tags")
				return nil
			},
			description: "Observed a panic when reading from InstanceState.RawPlan",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfgWithTags := &config.Resource{
				TerraformResource: &schema.Resource{
					Timeouts: &schema.ResourceTimeout{
						Create: &tagsTimeout,
						Read:   &tagsTimeout,
						Update: &tagsTimeout,
						Delete: &tagsTimeout,
					},
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"tags": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"tags_all": {
							Type:     schema.TypeMap,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
					CustomizeDiff: tc.customizeDiff,
				},
				ExternalName: config.IdentifierFromProvider,
				Sensitive: config.Sensitive{AdditionalConnectionDetailsFn: func(_ map[string]any) (map[string][]byte, error) {
					return nil, nil
				}},
			}

			notFound := mockResource{
				RefreshWithoutUpgradeFn: func(_ context.Context, _ *tf.InstanceState, _ interface{}) (*tf.InstanceState, diag.Diagnostics) {
					// Force into the state where the op tracker cache exists
					// but resource is not found.
					return nil, nil
				},
			}

			ext := prepareTerraformPluginSDKExternal(notFound, cfgWithTags)
			ext.opTracker.SetTfState(&tf.InstanceState{
				ID:         "example-id",
				Attributes: map[string]string{"name": "example"},
			})

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: %v", tc.description, r)
				}
			}()

			_, err := ext.Observe(t.Context(), &obj)
			if err != nil {
				t.Fatalf("Observe(...) returned an unexpected error: %v", err)
			}
		})
	}
}

func TestTerraformPluginSDKCreate(t *testing.T) {
	type args struct {
		r   Resource
		cfg *config.Resource
		obj fake.Terraformed
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"Unsuccessful": {
			args: args{
				r: mockResource{
					ApplyFn: func(ctx context.Context, s *tf.InstanceState, d *tf.InstanceDiff, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
						return nil, nil
					},
				},
				cfg: cfg,
				obj: obj,
			},
			want: want{
				err: errors.New("failed to read the ID of the new resource"),
			},
		},
		"Successful": {
			args: args{
				r: mockResource{
					ApplyFn: func(ctx context.Context, s *tf.InstanceState, d *tf.InstanceDiff, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
						return &tf.InstanceState{ID: "example-id"}, nil
					},
				},
				cfg: cfg,
				obj: obj,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			terraformPluginSDKExternal := prepareTerraformPluginSDKExternal(tc.args.r, tc.args.cfg)
			_, err := terraformPluginSDKExternal.Create(t.Context(), &tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConnect(...): -want error, +got error:\n", diff)
			}
		})
	}
}

func TestTerraformPluginSDKUpdate(t *testing.T) {
	type args struct {
		r   Resource
		cfg *config.Resource
		obj fake.Terraformed
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"Successful": {
			args: args{
				r: mockResource{
					ApplyFn: func(ctx context.Context, s *tf.InstanceState, d *tf.InstanceDiff, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
						return &tf.InstanceState{ID: "example-id"}, nil
					},
				},
				cfg: cfg,
				obj: obj,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			terraformPluginSDKExternal := prepareTerraformPluginSDKExternal(tc.args.r, tc.args.cfg)
			_, err := terraformPluginSDKExternal.Update(t.Context(), &tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConnect(...): -want error, +got error:\n", diff)
			}
		})
	}
}

func TestTerraformPluginSDKDelete(t *testing.T) {
	type args struct {
		r   Resource
		cfg *config.Resource
		obj fake.Terraformed
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		args
		want
	}{
		"Successful": {
			args: args{
				r: mockResource{
					ApplyFn: func(ctx context.Context, s *tf.InstanceState, d *tf.InstanceDiff, meta interface{}) (*tf.InstanceState, diag.Diagnostics) {
						return &tf.InstanceState{ID: "example-id"}, nil
					},
				},
				cfg: cfg,
				obj: obj,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			terraformPluginSDKExternal := prepareTerraformPluginSDKExternal(tc.args.r, tc.args.cfg)
			_, err := terraformPluginSDKExternal.Delete(t.Context(), &tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nConnect(...): -want error, +got error:\n", diff)
			}
		})
	}
}

// renameSecretConversion is a config.TerraformConversion that renames the
// "secret" attribute on the way back from the Terraform layer. Connection
// details are derived from sensitive paths expressed against the native
// Terraform schema, so they have to be computed before any conversion runs.
// Registering this conversion in a test makes that ordering observable: if the
// connection details are computed after the conversion, the "secret" path no
// longer resolves and no details are produced at all.
type renameSecretConversion struct{}

func (renameSecretConversion) Convert(params map[string]any, _ *config.Resource, mode config.Mode) (map[string]any, error) {
	if mode != config.FromTerraform {
		return params, nil
	}
	if v, ok := params["secret"]; ok {
		delete(params, "secret")
		params["secret_converted"] = v
	}
	return params, nil
}

// connectionDetailsConfig returns a resource configuration with a sensitive
// "secret" attribute and a conversion that renames it.
func connectionDetailsConfig() *config.Resource {
	return &config.Resource{
		TerraformResource: &schema.Resource{
			Timeouts: &schema.ResourceTimeout{
				Create: &timeout,
				Read:   &timeout,
				Update: &timeout,
				Delete: &timeout,
			},
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"id": {
					Type:     schema.TypeString,
					Computed: true,
				},
				"secret": {
					Type:      schema.TypeString,
					Computed:  true,
					Sensitive: true,
				},
			},
		},
		ExternalName: config.IdentifierFromProvider,
		Sensitive: config.Sensitive{AdditionalConnectionDetailsFn: func(_ map[string]any) (map[string][]byte, error) {
			return nil, nil
		}},
		TerraformConversions: []config.TerraformConversion{renameSecretConversion{}},
	}
}

func connectionDetailsObject() fake.Terraformed {
	return fake.Terraformed{
		Parameterizable: fake.Parameterizable{
			Parameters: map[string]any{
				"name": "example",
			},
		},
		Observable: fake.Observable{
			Observation: map[string]any{},
		},
		MetadataProvider: fake.MetadataProvider{
			ConnectionDetailsMapping: map[string]string{
				"secret": "spec.forProvider.secretSecretRef",
			},
		},
	}
}

// TestTerraformPluginSDKUpdateConnectionDetails asserts that Update publishes
// the connection details of the updated resource, and that they are computed
// from the pre-conversion state map, exactly like Observe does.
func TestTerraformPluginSDKUpdateConnectionDetails(t *testing.T) {
	newState := &tf.InstanceState{
		ID: "example-id",
		Attributes: map[string]string{
			"id":     "example-id",
			"name":   "example",
			"secret": "s3cret",
		},
	}
	r := mockResource{
		ApplyFn: func(_ context.Context, _ *tf.InstanceState, _ *tf.InstanceDiff, _ interface{}) (*tf.InstanceState, diag.Diagnostics) {
			return newState, nil
		},
		RefreshWithoutUpgradeFn: func(_ context.Context, _ *tf.InstanceState, _ interface{}) (*tf.InstanceState, diag.Diagnostics) {
			return newState, nil
		},
	}

	updateObj := connectionDetailsObject()
	update, err := prepareTerraformPluginSDKExternal(r, connectionDetailsConfig()).Update(t.Context(), &updateObj)
	if err != nil {
		t.Fatalf("Update(...): unexpected error: %v", err)
	}
	want := managed.ConnectionDetails{"attribute.secret": []byte("s3cret")}
	if diff := cmp.Diff(want, update.ConnectionDetails); diff != "" {
		t.Errorf("\n%s\nUpdate(...): -want connection details, +got connection details:\n", diff)
	}

	observeObj := connectionDetailsObject()
	observation, err := prepareTerraformPluginSDKExternal(r, connectionDetailsConfig()).Observe(t.Context(), &observeObj)
	if err != nil {
		t.Fatalf("Observe(...): unexpected error: %v", err)
	}
	if diff := cmp.Diff(observation.ConnectionDetails, update.ConnectionDetails); diff != "" {
		t.Errorf("\n%s\nUpdate(...): -want Observe connection details, +got Update connection details:\n", diff)
	}
}
