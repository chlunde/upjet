// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/upjet/v2/pkg/config"
	"github.com/crossplane/upjet/v2/pkg/config/conversion"
)

func TestConvertFieldPathToSnake(t *testing.T) {
	cases := map[string]struct {
		fieldPath string
		want      string
	}{
		"NestedPath": {
			fieldPath: "fooBar.bazQux",
			want:      "foo_bar.baz_qux",
		},
		"IndexedPath": {
			fieldPath: "fooBar[0].bazQux",
			want:      "foo_bar[0].baz_qux",
		},
		"WildcardPath": {
			fieldPath: "fooBar[*].bazQux",
			want:      "foo_bar[*].baz_qux",
		},
		"QuotedSegment": {
			fieldPath: `fooBar["my.key"]`,
			want:      "foo_bar[my.key]",
		},
		"AlreadySnakeWithDigit": {
			fieldPath: "ipv6_addresses",
			want:      "ipv6_addresses",
		},
		"AlreadySnakeNested": {
			fieldPath: "foo_bar.baz_qux",
			want:      "foo_bar.baz_qux",
		},
		"SingleSegment": {
			fieldPath: "fooBar",
			want:      "foo_bar",
		},
		"EmptyPath": {
			fieldPath: "",
			want:      "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
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

func TestMergeAnnotationFieldsWithStatusNestedPath(t *testing.T) {
	c := &config.Resource{
		Version:                    "v1beta2",
		ControllerReconcileVersion: "v1beta1", //nolint:staticcheck // still handling deprecated field behavior
	}
	atProvider := map[string]any{}
	annotations := map[string]string{
		conversion.AnnotationKey: `{"status.atProvider.fooBar[0].bazQux": "stored"}`,
	}
	if err := mergeAnnotationFieldsWithStatus(atProvider, annotations, c); err != nil {
		t.Fatalf("mergeAnnotationFieldsWithStatus(...): unexpected error: %v", err)
	}
	want := map[string]any{
		"foo_bar": []any{
			map[string]any{
				"baz_qux": "stored",
			},
		},
	}
	if diff := cmp.Diff(want, atProvider); diff != "" {
		t.Errorf("mergeAnnotationFieldsWithStatus(...): -want, +got:\n%s", diff)
	}
}
