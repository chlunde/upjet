// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLateInitialize(t *testing.T) {
	type args struct {
		desiredObject  any
		observedObject any
		opts           []GenericLateInitializerOption
	}

	testKeyDesiredField := "test-key-desiredField"
	testStringDesiredField := "test-string-desiredField"
	testKeyObservedField := "test-key-observedField"
	testStringObservedField := "test-string-observedField"
	testInt64ObservedField := 1
	testStringEmpty := ""

	type nestedStruct1 struct {
		F1 *string
		F2 []*string
	}

	type nestedStruct2 struct {
		F1 *int
		F2 []*int
	}

	type nestedStruct3 struct {
		F1 *string
		F2 *string
	}

	type nestedStruct4 struct {
		F1 *string `json:"f_1,omitempty"`
	}

	type nestedStruct5 struct {
		F1 [][]string
	}

	type nestedStruct6 struct {
		F1 []string `json:"f_1,omitempty"`
	}

	type nestedStruct7 struct {
		F1 map[string]string
	}

	type nestedStruct8 struct {
		F1 map[string]*string
	}

	type nestedStruct9 struct {
		F1 map[string][]string `json:"f_1,omitempty"`
	}

	type nestedStruct10 struct {
		F1 *string
		F2 *nestedStruct1
	}

	tests := map[string]struct {
		args         args
		wantModified bool
		wantErr      bool
		wantCRObject any
	}{
		"TypeMismatch": {
			args: args{
				desiredObject: &nestedStruct1{
					F1: &testStringDesiredField,
					F2: []*string{
						&testStringDesiredField,
					},
				},
				observedObject: &nestedStruct2{
					F1: &testInt64ObservedField,
				},
			},
			wantErr: true,
		},
		"NilCRObject": {
			args: args{
				observedObject: &struct{}{},
			},
		},
		"NilResponseObject": {
			args: args{
				desiredObject: &struct{}{},
			},
			wantCRObject: &struct{}{},
		},
		"TestNonStructCRObject": {
			args: args{
				desiredObject:  &testStringDesiredField,
				observedObject: &struct{}{},
			},
			wantErr: true,
		},
		"TestNonStructResponseObject": {
			args: args{
				desiredObject:  &struct{}{},
				observedObject: &testStringObservedField,
			},
			wantErr: true,
		},
		"TestEmptyStructCRAndResponseObjects": {
			args: args{
				desiredObject:  &struct{}{},
				observedObject: &struct{}{},
			},
			wantCRObject: &struct{}{},
		},
		"TestInitializedCRStringField": {
			args: args{
				desiredObject: &struct {
					F1 *string
				}{
					F1: &testStringDesiredField,
				},
				observedObject: &struct {
					F1 *string
				}{
					F1: &testStringObservedField,
				},
			},
			wantModified: false,
			wantCRObject: &struct {
				F1 *string
			}{
				F1: &testStringDesiredField,
			},
		},
		"TestUninitializedCRStringField": {
			args: args{
				desiredObject: &struct {
					F1 *string
				}{
					F1: nil,
				},
				observedObject: &struct {
					F1 *string
				}{
					F1: &testStringObservedField,
				},
			},
			wantModified: true,
			wantCRObject: &struct {
				F1 *string
			}{
				F1: &testStringObservedField,
			},
		},
		"TestInitializedCRNestedFields": {
			args: args{
				desiredObject: &struct {
					C1 *nestedStruct1
				}{
					C1: &nestedStruct1{
						F1: &testStringDesiredField,
						F2: []*string{
							&testStringDesiredField,
						},
					},
				},
				observedObject: &struct {
					C1 *nestedStruct1
				}{
					C1: &nestedStruct1{
						F1: &testStringObservedField,
						F2: []*string{
							&testStringObservedField,
						},
					},
				},
			},
			wantModified: false,
			wantCRObject: &struct {
				C1 *nestedStruct1
			}{
				C1: &nestedStruct1{
					F1: &testStringDesiredField,
					F2: []*string{
						&testStringDesiredField,
					},
				},
			},
		},
		"TestUninitializedCRNestedFields": {
			args: args{
				desiredObject: &struct {
					C1 *nestedStruct1
				}{},
				observedObject: &struct {
					C1 *nestedStruct1
				}{
					C1: &nestedStruct1{
						F1: &testStringObservedField,
						F2: []*string{
							&testStringObservedField,
						},
					},
				},
			},
			wantModified: true,
			wantCRObject: &struct {
				C1 *nestedStruct1
			}{
				C1: &nestedStruct1{
					F1: &testStringObservedField,
					F2: []*string{
						&testStringObservedField,
					},
				},
			},
		},
		"TestNilObservedNestedFields": {
			args: args{
				desiredObject: &struct {
					C1 *nestedStruct10
				}{},
				observedObject: &struct {
					C1 *nestedStruct10
				}{
					C1: &nestedStruct10{
						F1: &testStringDesiredField,
					},
				},
			},
			wantModified: true,
			wantCRObject: &struct {
				C1 *nestedStruct10
			}{
				C1: &nestedStruct10{
					F1: &testStringDesiredField,
				},
			},
		},
		"TestFieldKindMismatch": {
			args: args{
				desiredObject: &nestedStruct1{
					F1: nil,
				},
				observedObject: &nestedStruct2{
					F1: &testInt64ObservedField,
				},
			},
			wantErr: true,
		},
		"TestNestedFieldKindMismatch": {
			args: args{
				desiredObject: &struct {
					C1 *nestedStruct1
				}{
					C1: &nestedStruct1{
						F1: nil,
					},
				},
				observedObject: &struct {
					C1 *nestedStruct2
				}{
					C1: &nestedStruct2{
						F1: &testInt64ObservedField,
					},
				},
			},
			wantErr: true,
		},
		"TestSliceItemKindMismatch": {
			args: args{
				desiredObject: &nestedStruct1{},
				observedObject: &nestedStruct3{
					F1: &testStringObservedField,
					F2: &testStringObservedField,
				},
			},
			wantErr: true,
		},
		"TestInitializedSliceOfStringField": {
			args: args{
				desiredObject: &nestedStruct6{
					F1: []string{
						testStringDesiredField,
					},
				},
				observedObject: &nestedStruct6{
					F1: []string{
						testStringObservedField,
					},
				},
			},
			wantModified: false,
			wantCRObject: &nestedStruct6{
				F1: []string{
					testStringDesiredField,
				},
			},
		},
		"TestUninitializedSliceOfStringField": {
			args: args{
				desiredObject: &nestedStruct6{},
				observedObject: &nestedStruct6{
					F1: []string{
						testStringObservedField,
					},
				},
			},
			wantModified: true,
			wantCRObject: &nestedStruct6{
				F1: []string{
					testStringObservedField,
				},
			},
		},
		"TestInitializedSliceOfSliceField": {
			args: args{
				desiredObject: &nestedStruct5{
					F1: [][]string{
						{
							testStringDesiredField,
						},
					},
				},
				observedObject: &nestedStruct5{
					F1: [][]string{
						{
							testStringObservedField,
						},
					},
				},
			},
			wantModified: false,
			wantCRObject: &nestedStruct5{
				F1: [][]string{
					{
						testStringDesiredField,
					},
				},
			},
		},
		"TestInitializedMapOfStringField": {
			args: args{
				desiredObject: &nestedStruct7{
					F1: map[string]string{
						testKeyDesiredField: testStringDesiredField,
					},
				},
				observedObject: &nestedStruct7{
					F1: map[string]string{
						testKeyObservedField: testStringObservedField,
					},
				},
			},
			wantModified: false,
			wantCRObject: &nestedStruct7{
				F1: map[string]string{
					testKeyDesiredField: testStringDesiredField,
				},
			},
		},
		"TestUninitializedMapOfStringField": {
			args: args{
				desiredObject: &nestedStruct7{},
				observedObject: &nestedStruct7{
					F1: map[string]string{
						testKeyObservedField: testStringObservedField,
					},
				},
			},
			wantModified: true,
			wantCRObject: &nestedStruct7{
				F1: map[string]string{
					testKeyObservedField: testStringObservedField,
				},
			},
		},
		"TestInitializedMapOfPointerStringField": {
			args: args{
				desiredObject: &nestedStruct8{
					F1: map[string]*string{
						testKeyDesiredField: &testStringDesiredField,
					},
				},
				observedObject: &nestedStruct8{
					F1: map[string]*string{
						testKeyObservedField: &testStringObservedField,
					},
				},
			},
			wantModified: false,
			wantCRObject: &nestedStruct8{
				F1: map[string]*string{
					testKeyDesiredField: &testStringDesiredField,
				},
			},
		},
		"TestUninitializedMapOfPointerStringField": {
			args: args{
				desiredObject: &nestedStruct8{},
				observedObject: &nestedStruct8{
					F1: map[string]*string{
						testKeyObservedField: &testStringObservedField,
					},
				},
			},
			wantModified: true,
			wantCRObject: &nestedStruct8{
				F1: map[string]*string{
					testKeyObservedField: &testStringObservedField,
				},
			},
		},
		"TestInitializedMapOfStringSliceField": {
			args: args{
				desiredObject: &nestedStruct9{
					F1: map[string][]string{
						testKeyDesiredField: {testStringDesiredField},
					},
				},
				observedObject: &nestedStruct9{
					F1: map[string][]string{
						testKeyObservedField: {testStringObservedField},
					},
				},
			},
			wantModified: false,
			wantCRObject: &nestedStruct9{
				F1: map[string][]string{
					testKeyDesiredField: {testStringDesiredField},
				},
			},
		},
		"TestUninitializedMapOfStringSliceField": {
			args: args{
				desiredObject: &nestedStruct9{},
				observedObject: &nestedStruct9{
					F1: map[string][]string{
						testKeyObservedField: {testStringObservedField},
					},
				},
			},
			wantModified: true,
			wantCRObject: &nestedStruct9{
				F1: map[string][]string{
					testKeyObservedField: {testStringObservedField},
				},
			},
		},
		"TestInitializeWithZeroValues": {
			args: args{
				desiredObject: &nestedStruct4{},
				observedObject: &nestedStruct4{
					F1: &testStringEmpty,
				},
			},
			wantModified: true,
			wantCRObject: &nestedStruct4{
				F1: &testStringEmpty,
			},
		},
		"TestSkipZeroElem": {
			args: args{
				desiredObject: &nestedStruct4{},
				observedObject: &nestedStruct4{
					F1: &testStringEmpty,
				},
				opts: []GenericLateInitializerOption{WithZeroElemPtrFilter("F1")},
			},
			wantModified: false,
			wantCRObject: &nestedStruct4{},
		},
		"TestSkipOmitemptyTaggedPtrElem": {
			args: args{
				desiredObject: &nestedStruct4{},
				observedObject: &nestedStruct4{
					F1: &testStringEmpty,
				},
				opts: []GenericLateInitializerOption{WithZeroValueJSONOmitEmptyFilter(CNameWildcard)},
			},
			wantModified: false,
			wantCRObject: &nestedStruct4{},
		},
		"TestSkipOmitemptyTaggedSliceElem": {
			args: args{
				desiredObject: &nestedStruct6{},
				observedObject: &nestedStruct6{
					F1: []string{},
				},
				opts: []GenericLateInitializerOption{WithZeroValueJSONOmitEmptyFilter("F1")},
			},
			wantModified: false,
			wantCRObject: &nestedStruct6{},
		},
		"TestSkipOmitemptyTaggedMapElem": {
			args: args{
				desiredObject: &nestedStruct9{},
				observedObject: &nestedStruct9{
					F1: map[string][]string{},
				},
				opts: []GenericLateInitializerOption{WithZeroValueJSONOmitEmptyFilter("F1")},
			},
			wantModified: false,
			wantCRObject: &nestedStruct9{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			li := NewGenericLateInitializer(tt.args.opts...)
			got, err := li.LateInitialize(tt.args.desiredObject, tt.args.observedObject)

			if (err != nil) != tt.wantErr {
				t.Errorf("lateInitializeFromResponse() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.wantErr {
				return
			}

			if got != tt.wantModified {
				t.Errorf("lateInitializeFromResponse() got = %v, want %v", got, tt.wantModified)
			}

			if diff := cmp.Diff(tt.wantCRObject, tt.args.desiredObject); diff != "" {
				t.Errorf("lateInitializeFromResponse(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConvertFieldPathToSnake(t *testing.T) {
	cases := map[string]struct {
		fieldPath string
		want      string
	}{
		"NestedPath": {
			fieldPath: "ScalingConfig.MaxSize",
			want:      "scaling_config.max_size",
		},
		"SingleSegment": {
			fieldPath: "ScalingConfig",
			want:      "scaling_config",
		},
		"IndexedPath": {
			fieldPath: "FooBar[0].BazQux",
			want:      "foo_bar[0].baz_qux",
		},
		"WildcardPath": {
			fieldPath: "FooBar[*].BazQux",
			want:      "foo_bar[*].baz_qux",
		},
		"AlreadySnakeWithDigit": {
			fieldPath: "ipv6_addresses",
			want:      "ipv6_addresses",
		},
		"AlreadySnakeNested": {
			fieldPath: "block_device_mappings.ebs",
			want:      "block_device_mappings.ebs",
		},
		"EmptyPath": {
			fieldPath: "",
			want:      "",
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			got, err := convertFieldPathToSnake(tc.fieldPath)
			if err != nil {
				t.Fatalf("convertFieldPathToSnake(%q): unexpected error: %v", tc.fieldPath, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("convertFieldPathToSnake(%q): -want, +got:\n%s", tc.fieldPath, diff)
			}
		})
	}
}

func TestConditionalFilter(t *testing.T) {
	cases := map[string]struct {
		reason       string
		cName        string
		initProvider map[string]any
		cn           string
		want         bool
	}{
		"TopLevelFieldSetInInitProvider": {
			reason:       "A top-level canonical name set in spec.initProvider should be filtered.",
			cName:        "ScalingConfig",
			initProvider: map[string]any{"scaling_config": map[string]any{"max_size": float64(3)}},
			cn:           "ScalingConfig",
			want:         true,
		},
		"TopLevelFieldNotSetInInitProvider": {
			reason:       "A top-level canonical name absent from spec.initProvider should not be filtered.",
			cName:        "ScalingConfig",
			initProvider: map[string]any{},
			cn:           "ScalingConfig",
			want:         false,
		},
		"NestedFieldSetInInitProvider": {
			reason:       "A nested canonical name set in spec.initProvider should be filtered.",
			cName:        "ScalingConfig.MaxSize",
			initProvider: map[string]any{"scaling_config": map[string]any{"max_size": float64(3)}},
			cn:           "ScalingConfig.MaxSize",
			want:         true,
		},
		"NestedFieldNotSetInInitProvider": {
			reason:       "A nested canonical name absent from spec.initProvider should not be filtered.",
			cName:        "ScalingConfig.MinSize",
			initProvider: map[string]any{"scaling_config": map[string]any{"max_size": float64(3)}},
			cn:           "ScalingConfig.MinSize",
			want:         false,
		},
		"DeeplyNestedFieldSetInInitProvider": {
			reason: "A canonical name nested more than two levels deep should be filtered.",
			cName:  "LaunchTemplate.BlockDeviceMappings.VolumeSize",
			initProvider: map[string]any{"launch_template": map[string]any{
				"block_device_mappings": map[string]any{"volume_size": float64(10)},
			}},
			cn:   "LaunchTemplate.BlockDeviceMappings.VolumeSize",
			want: true,
		},
		"DifferentCanonicalName": {
			reason:       "A canonical name other than the configured one should not be filtered.",
			cName:        "ScalingConfig.MaxSize",
			initProvider: map[string]any{"scaling_config": map[string]any{"max_size": float64(3)}},
			cn:           "ScalingConfig.MinSize",
			want:         false,
		},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			got := conditionalFilter(tc.cName, tc.initProvider)(tc.cn)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nconditionalFilter(%q, ...)(%q): -want, +got:\n%s", tc.reason, tc.cName, tc.cn, diff)
			}
		})
	}
}
