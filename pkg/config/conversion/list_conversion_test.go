// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package conversion

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

func TestConvert(t *testing.T) {
	type args struct {
		params map[string]any
		paths  []string
		mode   ListConversionMode
		opts   *ConvertOptions
	}
	type want struct {
		err    error
		params map[string]any
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilParamsAndPaths": {
			reason: "Conversion on an nil map should not fail.",
			args:   args{},
		},
		"EmptyPaths": {
			reason: "Empty conversion on a map should be an identity function.",
			args: args{
				params: map[string]any{"a": "b"},
			},
			want: want{
				params: map[string]any{"a": "b"},
			},
		},
		"SingletonListToEmbeddedObject": {
			reason: "Should successfully convert a singleton list at the root level to an embedded object.",
			args: args{
				params: map[string]any{
					"l": []map[string]any{
						{
							"k": "v",
						},
					},
				},
				paths: []string{"l"},
				mode:  ToEmbeddedObject,
			},
			want: want{
				params: map[string]any{
					"l": map[string]any{
						"k": "v",
					},
				},
			},
		},
		"NestedSingletonListsToEmbeddedObjectsPathsInLexicalOrder": {
			reason: "Should successfully convert the parent & nested singleton lists to embedded objects. Paths specified in lexical order.",
			args: args{
				params: map[string]any{
					"parent": []map[string]any{
						{
							"child": []map[string]any{
								{
									"k": "v",
								},
							},
						},
					},
				},
				paths: []string{"parent", "parent[*].child"},
				mode:  ToEmbeddedObject,
			},
			want: want{
				params: map[string]any{
					"parent": map[string]any{
						"child": map[string]any{
							"k": "v",
						},
					},
				},
			},
		},
		"NestedSingletonListsToEmbeddedObjectsPathsInReverseLexicalOrder": {
			reason: "Should successfully convert the parent & nested singleton lists to embedded objects. Paths specified in reverse-lexical order.",
			args: args{
				params: map[string]any{
					"parent": []map[string]any{
						{
							"child": []map[string]any{
								{
									"k": "v",
								},
							},
						},
					},
				},
				paths: []string{"parent[*].child", "parent"},
				mode:  ToEmbeddedObject,
			},
			want: want{
				params: map[string]any{
					"parent": map[string]any{
						"child": map[string]any{
							"k": "v",
						},
					},
				},
			},
		},
		"EmbeddedObjectToSingletonList": {
			reason: "Should successfully convert an embedded object at the root level to a singleton list.",
			args: args{
				params: map[string]any{
					"l": map[string]any{
						"k": "v",
					},
				},
				paths: []string{"l"},
				mode:  ToSingletonList,
			},
			want: want{
				params: map[string]any{
					"l": []map[string]any{
						{
							"k": "v",
						},
					},
				},
			},
		},
		"NestedEmbeddedObjectsToSingletonListInLexicalOrder": {
			reason: "Should successfully convert the parent & nested embedded objects to singleton lists. Paths are specified in lexical order.",
			args: args{
				params: map[string]any{
					"parent": map[string]any{
						"child": map[string]any{
							"k": "v",
						},
					},
				},
				paths: []string{"parent", "parent[*].child"},
				mode:  ToSingletonList,
			},
			want: want{
				params: map[string]any{
					"parent": []map[string]any{
						{
							"child": []map[string]any{
								{
									"k": "v",
								},
							},
						},
					},
				},
			},
		},
		"NestedEmbeddedObjectsToSingletonListInReverseLexicalOrder": {
			reason: "Should successfully convert the parent & nested embedded objects to singleton lists. Paths are specified in reverse-lexical order.",
			args: args{
				params: map[string]any{
					"parent": map[string]any{
						"child": map[string]any{
							"k": "v",
						},
					},
				},
				paths: []string{"parent[*].child", "parent"},
				mode:  ToSingletonList,
			},
			want: want{
				params: map[string]any{
					"parent": []map[string]any{
						{
							"child": []map[string]any{
								{
									"k": "v",
								},
							},
						},
					},
				},
			},
		},
		"FailConversionOfAMultiItemList": {
			reason: `Conversion of a multi-item list in mode "ToEmbeddedObject" should fail.`,
			args: args{
				params: map[string]any{
					"l": []map[string]any{
						{
							"k1": "v1",
						},
						{
							"k2": "v2",
						},
					},
				},
				paths: []string{"l"},
				mode:  ToEmbeddedObject,
			},
			want: want{
				err: errors.Errorf(errFmtMultiItemList, "l", 2),
			},
		},
		"FailConversionOfNonSlice": {
			reason: `Conversion of a non-slice value in mode "ToEmbeddedObject" should fail.`,
			args: args{
				params: map[string]any{
					"l": map[string]any{
						"k": "v",
					},
				},
				paths: []string{"l"},
				mode:  ToEmbeddedObject,
			},
			want: want{
				err: errors.Errorf(errFmtNonSlice, "l", reflect.TypeOf(map[string]any{})),
			},
		},
		"ToSingletonListWithNonExistentPath": {
			reason: `"ToSingletonList" mode conversions specifying only non-existent paths should be identity functions.`,
			args: args{
				params: map[string]any{
					"l": map[string]any{
						"k": "v",
					},
				},
				paths: []string{"nonexistent"},
				mode:  ToSingletonList,
			},
			want: want{
				params: map[string]any{
					"l": map[string]any{
						"k": "v",
					},
				},
			},
		},
		"ToEmbeddedObjectWithNonExistentPath": {
			reason: `"ToEmbeddedObject" mode conversions specifying only non-existent paths should be identity functions.`,
			args: args{
				params: map[string]any{
					"l": []map[string]any{
						{
							"k": "v",
						},
					},
				},
				paths: []string{"nonexistent"},
				mode:  ToEmbeddedObject,
			},
			want: want{
				params: map[string]any{
					"l": []map[string]any{
						{
							"k": "v",
						},
					},
				},
			},
		},
		"WithInjectedKeySingletonListToEmbeddedObject": {
			reason: "Should successfully convert a singleton list at the root level to an embedded object.",
			args: args{
				params: map[string]any{
					"l": []map[string]any{
						{
							"k":     "v",
							"index": "0",
						},
					},
				},
				paths: []string{"l"},
				mode:  ToEmbeddedObject,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"l": {
							Key:   "index",
							Value: "0",
						},
					},
				}},
			want: want{
				params: map[string]any{
					"l": map[string]any{
						"k": "v",
					},
				},
			},
		},
		"WithInjectedKeyEmbeddedObjectToSingletonList": {
			reason: "Should successfully convert an embedded object at the root level to a singleton list.",
			args: args{
				params: map[string]any{
					"l": map[string]any{
						"k": "v",
					},
				},
				paths: []string{"l"},
				mode:  ToSingletonList,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"l": {
							Key:   "index",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: map[string]any{
					"l": []map[string]any{
						{
							"k":     "v",
							"index": "0",
						},
					},
				},
			},
		},
		"WithInjectedKeyNestedEmbeddedObjectsToSingletonListInLexicalOrder": {
			reason: "Should successfully convert the parent & nested embedded objects to singleton lists. Paths are specified in lexical order.",
			args: args{
				params: map[string]any{
					"parent": map[string]any{
						"child": map[string]any{
							"k": "v",
						},
					},
				},
				paths: []string{"parent", "parent[*].child"},
				mode:  ToSingletonList,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"parent": {
							Key:   "index",
							Value: "0",
						},
						"parent[*].child": {
							Key:   "another",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: map[string]any{
					"parent": []map[string]any{
						{
							"index": "0",
							"child": []map[string]any{
								{
									"k":       "v",
									"another": "0",
								},
							},
						},
					},
				},
			},
		},
		"WithInjectedKeyNestedSingletonListsToEmbeddedObjectsPathsInLexicalOrder": {
			reason: "Should successfully convert the parent & nested singleton lists to embedded objects. Paths specified in lexical order.",
			args: args{
				params: map[string]any{
					"parent": []map[string]any{
						{
							"index": "0",
							"child": []map[string]any{
								{
									"k":       "v",
									"another": "0",
								},
							},
						},
					},
				},
				paths: []string{"parent", "parent[*].child"},
				mode:  ToEmbeddedObject,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"parent": {
							Key:   "index",
							Value: "0",
						},
						"parent[*].child": {
							Key:   "another",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: map[string]any{
					"parent": map[string]any{
						"child": map[string]any{
							"k": "v",
						},
					},
				},
			},
		},
		"WithInjectedKeyNonZeroIndexEmbeddedObjectsToSingletonLists": {
			reason: "Should inject the key into every element of the parent list, not only the one at index 0.",
			args: args{
				params: parentWithChildren(2, false),
				paths:  []string{"parent[*].child"},
				mode:   ToSingletonList,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"parent[*].child": {
							Key:   "index",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: parentWithChildren(2, true),
			},
		},
		"WithInjectedKeyNonZeroIndexSingletonListsToEmbeddedObjects": {
			reason: "Should remove the injected key from every element of the parent list, not only the one at index 0.",
			args: args{
				params: parentWithChildren(2, true),
				paths:  []string{"parent[*].child"},
				mode:   ToEmbeddedObject,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"parent[*].child": {
							Key:   "index",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: parentWithChildren(2, false),
			},
		},
		"WithInjectedKeyIndexGreaterThanNine": {
			reason: "Should inject the key at an index whose decimal representation contains a 0, e.g. 10.",
			args: args{
				params: parentWithChildren(11, false),
				paths:  []string{"parent[*].child"},
				mode:   ToSingletonList,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"parent[*].child": {
							Key:   "index",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: parentWithChildren(11, true),
			},
		},
		"WithInjectedKeyFieldNameContainingZero": {
			reason: "Should inject the key at a path whose field names contain the digit 0.",
			args: args{
				params: map[string]any{
					"x509_config": map[string]any{
						"k": "v",
					},
				},
				paths: []string{"x509_config"},
				mode:  ToSingletonList,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"x509_config": {
							Key:   "index",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: map[string]any{
					"x509_config": []map[string]any{
						{
							"k":     "v",
							"index": "0",
						},
					},
				},
			},
		},
		"WithInjectedKeyFieldNameContainingZeroSingletonListToEmbeddedObject": {
			reason: "Should remove the injected key at a path whose field names contain the digit 0.",
			args: args{
				params: map[string]any{
					"x509_config": []map[string]any{
						{
							"k":     "v",
							"index": "0",
						},
					},
				},
				paths: []string{"x509_config"},
				mode:  ToEmbeddedObject,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"x509_config": {
							Key:   "index",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: map[string]any{
					"x509_config": map[string]any{
						"k": "v",
					},
				},
			},
		},
		"WithInjectedKeyMultipleIndicesInPath": {
			reason: "Should inject the key at a nested path carrying more than one array index.",
			args: args{
				params: map[string]any{
					"a": []any{
						map[string]any{"b": []any{
							map[string]any{"c": map[string]any{"k": "v00"}},
							map[string]any{"c": map[string]any{"k": "v01"}},
						}},
						map[string]any{"b": []any{
							map[string]any{"c": map[string]any{"k": "v10"}},
							map[string]any{"c": map[string]any{"k": "v11"}},
						}},
					},
				},
				paths: []string{"a[*].b[*].c"},
				mode:  ToSingletonList,
				opts: &ConvertOptions{
					ListInjectKeys: map[string]SingletonListInjectKey{
						"a[*].b[*].c": {
							Key:   "index",
							Value: "0",
						},
					},
				},
			},
			want: want{
				params: map[string]any{
					"a": []any{
						map[string]any{"b": []any{
							map[string]any{"c": []any{map[string]any{"k": "v00", "index": "0"}}},
							map[string]any{"c": []any{map[string]any{"k": "v01", "index": "0"}}},
						}},
						map[string]any{"b": []any{
							map[string]any{"c": []any{map[string]any{"k": "v10", "index": "0"}}},
							map[string]any{"c": []any{map[string]any{"k": "v11", "index": "0"}}},
						}},
					},
				},
			},
		},
	}

	for n, tt := range tests {
		t.Run(n, func(t *testing.T) {
			params, err := roundTrip(tt.args.params)
			if err != nil {
				t.Fatalf("Failed to preprocess tt.args.params: %v", err)
			}
			wantParams, err := roundTrip(tt.want.params)
			if err != nil {
				t.Fatalf("Failed to preprocess tt.want.params: %v", err)
			}
			got, err := Convert(params, tt.args.paths, tt.args.mode, tt.args.opts)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\n%s\nConvert(tt.args.params, tt.args.paths): -wantErr, +gotErr:\n%s", tt.reason, diff)
			}
			if diff := cmp.Diff(wantParams, got); diff != "" {
				t.Errorf("\n%s\nConvert(tt.args.params, tt.args.paths): -wantConverted, +gotConverted:\n%s", tt.reason, diff)
			}
		})
	}
}

func TestModeString(t *testing.T) {
	tests := map[string]struct {
		m    ListConversionMode
		want string
	}{
		"ToSingletonList": {
			m:    ToSingletonList,
			want: "toSingletonList",
		},
		"ToEmbeddedObject": {
			m:    ToEmbeddedObject,
			want: "toEmbeddedObject",
		},
		"Unknown": {
			m:    ToSingletonList + 1,
			want: "unknown",
		},
	}
	for n, tt := range tests {
		t.Run(n, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, tt.m.String()); diff != "" {
				t.Errorf("String(): -want, +got:\n%s", diff)
			}
		})
	}
}

func roundTrip(m map[string]any) (map[string]any, error) {
	if len(m) == 0 {
		return m, nil
	}
	buff, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(m)
	if err != nil {
		return nil, err
	}
	var r map[string]any
	return r, jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(buff, &r)
}

// parentWithChildren builds a "parent" list of n elements, each holding a
// "child" embedded object. If asSingletonList is true, each child is instead
// a singleton list carrying the injected "index" key, i.e. the expected
// result of a ToSingletonList conversion of the same input.
func parentWithChildren(n int, asSingletonList bool) map[string]any {
	parent := make([]any, 0, n)
	for i := 0; i < n; i++ {
		child := map[string]any{"k": fmt.Sprintf("v%d", i)}
		var c any = child
		if asSingletonList {
			child["index"] = "0"
			c = []any{child}
		}
		parent = append(parent, map[string]any{"child": c})
	}
	return map[string]any{"parent": parent}
}

func TestWildcardIndexedPath(t *testing.T) {
	tests := map[string]struct {
		reason  string
		path    string
		want    string
		wantErr bool
	}{
		"Empty": {
			reason: "An empty path should be returned as is.",
			path:   "",
			want:   "",
		},
		"NoIndex": {
			reason: "A path without any index segments should be returned as is.",
			path:   "vpcConfig",
			want:   "vpcConfig",
		},
		"ZeroIndex": {
			reason: "The zero index segment should be replaced with the wildcard.",
			path:   "parent[0].child",
			want:   "parent[*].child",
		},
		"NonZeroIndex": {
			reason: "A non-zero index segment should be replaced with the wildcard.",
			path:   "parent[1].child",
			want:   "parent[*].child",
		},
		"IndexGreaterThanNine": {
			reason: "An index whose decimal representation contains a 0 should be replaced as a whole.",
			path:   "rule[10].filter",
			want:   "rule[*].filter",
		},
		"FieldNameContainingZero": {
			reason: "A field name containing the digit 0 should not be touched.",
			path:   "x509_config",
			want:   "x509_config",
		},
		"FieldNameContainingZeroWithIndex": {
			reason: "Only the index segments of a path with digit-bearing field names should be replaced.",
			path:   "x509_config[10].sha1_digest",
			want:   "x509_config[*].sha1_digest",
		},
		"MultipleIndices": {
			reason: "Every index segment of the path should be replaced.",
			path:   "a[0].b[10].c",
			want:   "a[*].b[*].c",
		},
		"AlreadyWildcard": {
			reason: "The conversion should be idempotent on an already wildcarded path.",
			path:   "parent[*].child",
			want:   "parent[*].child",
		},
		"InvalidPath": {
			reason:  "An unparsable field path should result in an error.",
			path:    "parent..child",
			wantErr: true,
		},
	}
	for n, tt := range tests {
		t.Run(n, func(t *testing.T) {
			got, err := wildcardIndexedPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("\n%s\nwildcardIndexedPath(%q): want error, got %q", tt.reason, tt.path, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("\n%s\nwildcardIndexedPath(%q): unexpected error: %v", tt.reason, tt.path, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("\n%s\nwildcardIndexedPath(%q): -want, +got:\n%s", tt.reason, tt.path, diff)
			}
		})
	}
}
