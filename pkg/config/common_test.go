// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/crossplane/upjet/v2/pkg/config/conversion"
	"github.com/crossplane/upjet/v2/pkg/registry"
)

func TestDefaultResource(t *testing.T) {
	identityConversion := conversion.NewIdentityConversionExpandPaths(conversion.AllVersions, conversion.AllVersions, nil)

	type args struct {
		name              string
		sch               *schema.Resource
		frameworkResource fwresource.Resource
		reg               *registry.Resource
		opts              []ResourceOption
	}

	cases := map[string]struct {
		reason string
		args   args
		want   *Resource
	}{
		"ThreeSectionsName": {
			reason: "It should return GVK properly for names with three sections",
			args: args{
				name: "aws_ec2_instance",
			},
			want: &Resource{
				Name:                           "aws_ec2_instance",
				ShortGroup:                     "ec2",
				Kind:                           "Instance",
				Version:                        "v1alpha1",
				ExternalName:                   NameAsIdentifier,
				References:                     map[string]Reference{},
				Sensitive:                      NopSensitive,
				UseAsync:                       true,
				SchemaElementOptions:           SchemaElementOptions{},
				ServerSideApplyMergeStrategies: ServerSideApplyMergeStrategies{},
				Conversions:                    []conversion.Conversion{identityConversion},
				OverrideFieldNames:             map[string]string{},
			},
		},
		"TwoSectionsName": {
			reason: "It should return GVK properly for names with three sections",
			args: args{
				name: "aws_instance",
			},
			want: &Resource{
				Name:                           "aws_instance",
				ShortGroup:                     "aws",
				Kind:                           "Instance",
				Version:                        "v1alpha1",
				ExternalName:                   NameAsIdentifier,
				References:                     map[string]Reference{},
				Sensitive:                      NopSensitive,
				UseAsync:                       true,
				SchemaElementOptions:           SchemaElementOptions{},
				ServerSideApplyMergeStrategies: ServerSideApplyMergeStrategies{},
				Conversions:                    []conversion.Conversion{identityConversion},
				OverrideFieldNames:             map[string]string{},
			},
		},
		"NameWithPrefixAcronym": {
			reason: "It should return prefix acronym in capital case",
			args: args{
				name: "aws_db_sql_server",
			},
			want: &Resource{
				Name:                           "aws_db_sql_server",
				ShortGroup:                     "db",
				Kind:                           "SQLServer",
				Version:                        "v1alpha1",
				ExternalName:                   NameAsIdentifier,
				References:                     map[string]Reference{},
				Sensitive:                      NopSensitive,
				UseAsync:                       true,
				SchemaElementOptions:           SchemaElementOptions{},
				ServerSideApplyMergeStrategies: ServerSideApplyMergeStrategies{},
				Conversions:                    []conversion.Conversion{identityConversion},
				OverrideFieldNames:             map[string]string{},
			},
		},
		"NameWithSuffixAcronym": {
			reason: "It should return suffix acronym in capital case",
			args: args{
				name: "aws_db_server_id",
			},
			want: &Resource{
				Name:                           "aws_db_server_id",
				ShortGroup:                     "db",
				Kind:                           "ServerID",
				Version:                        "v1alpha1",
				ExternalName:                   NameAsIdentifier,
				References:                     map[string]Reference{},
				Sensitive:                      NopSensitive,
				UseAsync:                       true,
				SchemaElementOptions:           SchemaElementOptions{},
				ServerSideApplyMergeStrategies: ServerSideApplyMergeStrategies{},
				Conversions:                    []conversion.Conversion{identityConversion},
				OverrideFieldNames:             map[string]string{},
			},
		},
		"NameWithMultipleAcronyms": {
			reason: "It should return both prefix & suffix acronyms in capital case",
			args: args{
				name: "aws_db_sql_server_id",
			},
			want: &Resource{
				Name:                           "aws_db_sql_server_id",
				ShortGroup:                     "db",
				Kind:                           "SQLServerID",
				Version:                        "v1alpha1",
				ExternalName:                   NameAsIdentifier,
				References:                     map[string]Reference{},
				Sensitive:                      NopSensitive,
				UseAsync:                       true,
				SchemaElementOptions:           SchemaElementOptions{},
				ServerSideApplyMergeStrategies: ServerSideApplyMergeStrategies{},
				Conversions:                    []conversion.Conversion{identityConversion},
				OverrideFieldNames:             map[string]string{},
			},
		},
	}

	// TODO(muvaf): Find a way to compare function pointers.
	ignoreUnexported := []cmp.Option{
		cmpopts.IgnoreFields(Sensitive{}, "fieldPaths", "AdditionalConnectionDetailsFn"),
		cmpopts.IgnoreFields(LateInitializer{}, "ignoredCanonicalFieldPaths", "conditionalIgnoredCanonicalFieldPaths"),
		cmpopts.IgnoreFields(ExternalName{}, "SetIdentifierArgumentFn", "GetExternalNameFn", "GetIDFn"),
		cmpopts.IgnoreUnexported(Resource{}),
		cmpopts.IgnoreUnexported(reflect.ValueOf(identityConversion).Elem().Interface()),
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := DefaultResource(tc.args.name, tc.args.sch, tc.args.frameworkResource, tc.args.reg, tc.args.opts...)
			if diff := cmp.Diff(tc.want, r, ignoreUnexported...); diff != "" {
				t.Errorf("\n%s\nDefaultResource(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMoveToStatus(t *testing.T) {
	type args struct {
		sch    *schema.Resource
		fields []string
	}
	type want struct {
		sch *schema.Resource
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"DoesNotExist": {
			args: args{
				fields: []string{"topD"},
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
						"topB": {Type: schema.TypeInt},
						"topC": {Type: schema.TypeString, Optional: true},
					},
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
						"topB": {Type: schema.TypeInt},
						"topC": {Type: schema.TypeString, Optional: true},
					},
				},
			},
		},
		"MissingFieldDoesNotAbortRemaining": {
			reason: "A fieldpath that cannot be resolved should be skipped without aborting the remaining fieldpaths",
			args: args{
				fields: []string{"topD", "topA"},
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString, Optional: true},
						"topB": {Type: schema.TypeInt},
					},
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Type:     schema.TypeString,
							Optional: false,
							Computed: true,
						},
						"topB": {Type: schema.TypeInt},
					},
				},
			},
		},
		"TopLevelBasicFields": {
			args: args{
				fields: []string{"topA", "topB"},
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
						"topB": {Type: schema.TypeInt},
						"topC": {Type: schema.TypeString, Optional: true},
					},
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Type:     schema.TypeString,
							Optional: false,
							Computed: true,
						},
						"topB": {
							Type:     schema.TypeInt,
							Optional: false,
							Computed: true,
						},
						"topC": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: false,
						},
					},
				},
			},
		},
		"ComplexFields": {
			args: args{
				fields: []string{"topA"},
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Type: schema.TypeMap,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"leafA": {
										Type: schema.TypeMap,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"leafB": {
													Type:     schema.TypeString,
													Computed: false,
													Optional: true,
												},
												"leafC": {
													Type:     schema.TypeString,
													Computed: false,
													Optional: true,
												},
											},
										},
									},
								},
							},
						},
						"topB": {Type: schema.TypeString},
					},
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Type:     schema.TypeMap,
							Computed: true,
							Optional: false,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"leafA": {
										Type:     schema.TypeMap,
										Computed: true,
										Optional: false,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"leafB": {
													Type:     schema.TypeString,
													Computed: true,
													Optional: false,
												},
												"leafC": {
													Type:     schema.TypeString,
													Computed: true,
													Optional: false,
												},
											},
										},
									},
								},
							},
						},
						"topB": {Type: schema.TypeString},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			MoveToStatus(tc.args.sch, tc.args.fields...)
			if diff := cmp.Diff(tc.want.sch, tc.args.sch); diff != "" {
				t.Errorf("\n%s\nMoveToStatus(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMoveToStatusSharedSchema(t *testing.T) {
	// Terraform providers commonly share schema pointers between resources.
	// For example, terraform-provider-aws constructs the tags schema once
	// (via sync.OnceValue) and reuses the same *schema.Schema in every
	// taggable resource, and some resources embed shared *schema.Resource
	// blocks through Elem. MoveToStatus must not mutate such shared schemas
	// in place; otherwise, moving a field to status in one resource silently
	// flips {Optional, Computed} for all other resources in the process.
	newFixtures := func() (sharedLeaf *schema.Schema, sharedNested *schema.Resource, r1, r2 *schema.Resource) {
		sharedLeaf = &schema.Schema{
			Type:     schema.TypeMap,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		}
		sharedNested = &schema.Resource{
			Schema: map[string]*schema.Schema{
				"tags":    sharedLeaf,
				"max_age": {Type: schema.TypeInt, Optional: true},
			},
		}
		newResource := func() *schema.Resource {
			return &schema.Resource{
				Schema: map[string]*schema.Schema{
					"tags": sharedLeaf,
					"rule": {
						Type:     schema.TypeList,
						Optional: true,
						Elem:     sharedNested,
					},
				},
			}
		}
		return sharedLeaf, sharedNested, newResource(), newResource()
	}

	assertMoved := func(t *testing.T, name string, s *schema.Schema) {
		t.Helper()
		if s == nil {
			t.Errorf("schema %q: not found", name)
			return
		}
		if s.Optional || !s.Computed {
			t.Errorf("schema %q: got {Optional: %t, Computed: %t}, want {Optional: false, Computed: true}", name, s.Optional, s.Computed)
		}
	}
	assertUntouched := func(t *testing.T, sharedLeaf *schema.Schema, sharedNested *schema.Resource, r2 *schema.Resource) {
		t.Helper()
		if !sharedLeaf.Optional || sharedLeaf.Computed {
			t.Errorf("shared leaf schema mutated in place: got {Optional: %t, Computed: %t}, want {Optional: true, Computed: false}", sharedLeaf.Optional, sharedLeaf.Computed)
		}
		for name, s := range sharedNested.Schema {
			if !s.Optional || s.Computed {
				t.Errorf("shared nested schema field %q mutated in place: got {Optional: %t, Computed: %t}, want {Optional: true, Computed: false}", name, s.Optional, s.Computed)
			}
		}
		for _, fp := range []string{"tags", "rule", "rule.tags", "rule.max_age"} {
			s := GetSchema(r2, fp)
			if s == nil {
				t.Errorf("unrelated resource: schema %q not found", fp)
				continue
			}
			if !s.Optional || s.Computed {
				t.Errorf("unrelated resource: schema %q mutated: got {Optional: %t, Computed: %t}, want {Optional: true, Computed: false}", fp, s.Optional, s.Computed)
			}
		}
	}

	t.Run("TopLevelSharedLeaf", func(t *testing.T) {
		sharedLeaf, sharedNested, r1, r2 := newFixtures()
		MoveToStatus(r1, "tags")
		assertMoved(t, "tags", r1.Schema["tags"])
		assertUntouched(t, sharedLeaf, sharedNested, r2)
	})

	t.Run("SharedNestedResourceViaElem", func(t *testing.T) {
		sharedLeaf, sharedNested, r1, r2 := newFixtures()
		MoveToStatus(r1, "rule")
		assertMoved(t, "rule", r1.Schema["rule"])
		assertMoved(t, "rule.tags", GetSchema(r1, "rule.tags"))
		assertMoved(t, "rule.max_age", GetSchema(r1, "rule.max_age"))
		if el, ok := r1.Schema["rule"].Elem.(*schema.Resource); ok && el == sharedNested {
			t.Errorf("rule.Elem still points at the shared nested resource; it must be copied before mutation")
		}
		assertUntouched(t, sharedLeaf, sharedNested, r2)
	})

	t.Run("LeafUnderSharedNestedResource", func(t *testing.T) {
		sharedLeaf, sharedNested, r1, r2 := newFixtures()
		MoveToStatus(r1, "rule.tags")
		assertMoved(t, "rule.tags", GetSchema(r1, "rule.tags"))
		if s := GetSchema(r1, "rule.max_age"); s == nil || !s.Optional || s.Computed {
			t.Errorf("sibling schema \"rule.max_age\" should not be moved to status")
		}
		assertUntouched(t, sharedLeaf, sharedNested, r2)
	})
}

func TestMarkAsRequired(t *testing.T) {
	type args struct {
		sch    *schema.Resource
		fields []string
	}
	type want struct {
		sch *schema.Resource
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"DoesNotExist": {
			args: args{
				fields: []string{"topD"},
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
						"topB": {Type: schema.TypeInt, Computed: true},
						"topC": {Type: schema.TypeString, Optional: true},
					},
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
						"topB": {Type: schema.TypeInt, Computed: true},
						"topC": {Type: schema.TypeString, Optional: true},
					},
				},
			},
		},
		"TopLevelBasicFields": {
			args: args{
				fields: []string{"topB", "topC"},
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
						"topB": {Type: schema.TypeInt, Computed: true},
						"topC": {Type: schema.TypeString, Optional: true},
					},
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
						"topB": {
							Type:     schema.TypeInt,
							Optional: false,
							Computed: false,
						},
						"topC": {
							Type:     schema.TypeString,
							Optional: false,
							Computed: false,
						},
					},
				},
			},
		},
		"ComplexFields": {
			args: args{
				fields: []string{"topA.leafA", "topA.leafA.leafC"},
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Type: schema.TypeMap,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"leafA": {
										Type: schema.TypeMap,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"leafB": {Type: schema.TypeString},
												"leafC": {Type: schema.TypeString},
											},
										},
									},
								},
							},
						},
						"topB": {Type: schema.TypeString},
					},
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Type: schema.TypeMap,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"leafA": {
										Type:     schema.TypeMap,
										Computed: false,
										Optional: false,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"leafB": {Type: schema.TypeString},
												"leafC": {
													Type:     schema.TypeString,
													Computed: false,
													Optional: false,
												},
											},
										},
									},
								},
							},
						},
						"topB": {Type: schema.TypeString},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			MarkAsRequired(tc.args.sch, tc.args.fields...)
			if diff := cmp.Diff(tc.want.sch, tc.args.sch); diff != "" {
				t.Errorf("\n%s\nMarkAsRequired(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetSchema(t *testing.T) {
	type args struct {
		sch       *schema.Resource
		fieldpath string
	}
	type want struct {
		sch *schema.Schema
	}
	schLeaf := &schema.Schema{
		Type: schema.TypeString,
	}
	schA := &schema.Schema{
		Type: schema.TypeMap,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"fieldA": schLeaf,
			},
		},
	}
	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"topA": schA,
		},
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"TopLevelField": {
			args: args{
				fieldpath: "topA",
				sch:       res,
			},
			want: want{
				sch: schA,
			},
		},
		"LeafField": {
			args: args{
				fieldpath: "topA.fieldA",
				sch:       res,
			},
			want: want{
				sch: schLeaf,
			},
		},
		"TopLevelFieldNotFound": {
			args: args{
				fieldpath: "topB",
				sch:       res,
			},
			want: want{
				sch: nil,
			},
		},
		"LeafFieldNotFound": {
			args: args{
				fieldpath: "topA.olala.omama",
				sch:       res,
			},
			want: want{
				sch: nil,
			},
		},
		"TopFieldIsNotMap": {
			args: args{
				fieldpath: "topA.topB",
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {Type: schema.TypeString},
					},
				},
			},
			want: want{
				sch: nil,
			},
		},
		"MiddleFieldIsNotResource": {
			args: args{
				fieldpath: "topA.topB.topC",
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"topB": {
										Elem: &schema.Schema{},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				sch: nil,
			},
		},
		"MiddleFieldIsNilResource": {
			reason: "A nil element resource in the middle of the path is reported as not found instead of panicking.",
			args: args{
				fieldpath: "topA.topB.topC",
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"topB": {
										Elem: (*schema.Resource)(nil),
									},
								},
							},
						},
					},
				},
			},
			want: want{
				sch: nil,
			},
		},
		"NilResourceSchema": {
			reason: "A nil Terraform resource schema is reported as not found instead of panicking.",
			args: args{
				fieldpath: "topA",
				sch:       nil,
			},
			want: want{
				sch: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sch := GetSchema(tc.args.sch, tc.args.fieldpath)
			if diff := cmp.Diff(tc.want.sch, sch); diff != "" {
				t.Errorf("\n%s\nGetSchema(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManipulateAllFieldsInSchema(t *testing.T) {
	type args struct {
		sch *schema.Resource
		op  func(sch *schema.Schema)
	}
	type want struct {
		sch *schema.Resource
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"SetEmptyDescription": {
			args: args{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Description: "topADescription",
							Type:        schema.TypeMap,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"leafA": {
										Description: "leafADescription",
										Type:        schema.TypeMap,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"leafB": {
													Description: "",
													Type:        schema.TypeString,
												},
												"leafC": {
													Description: "leafCDescription",
													Type:        schema.TypeString,
												},
											},
										},
									},
								},
							},
						},
						"topB": {Type: schema.TypeString},
					},
				},
				op: func(sch *schema.Schema) {
					sch.Description = ""
				},
			},
			want: want{
				sch: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"topA": {
							Description: "",
							Type:        schema.TypeMap,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"leafA": {
										Description: "",
										Type:        schema.TypeMap,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"leafB": {
													Description: "",
													Type:        schema.TypeString,
												},
												"leafC": {
													Description: "",
													Type:        schema.TypeString,
												},
											},
										},
									},
								},
							},
						},
						"topB": {Type: schema.TypeString, Description: ""},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ManipulateEveryField(tc.args.sch, tc.args.op)
			if diff := cmp.Diff(tc.want.sch, tc.args.sch); diff != "" {
				t.Errorf("\n%s\nMoveToStatus(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
