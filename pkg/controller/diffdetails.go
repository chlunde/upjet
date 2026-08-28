// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	tf "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/crossplane/upjet/v2/pkg/config"
	"github.com/crossplane/upjet/v2/pkg/schema/traverser"
)

// Keys under which the changed attribute set is reported in the
// managed.AdditionalDetails of an update, which the crossplane-runtime change
// logger forwards verbatim to the change log service.
const (
	// DetailsKeyChangedAttributes holds the comma separated, sorted set of
	// Terraform attribute paths that differed between the observed and the
	// desired state of the external resource. Paths only, never values: see
	// the package documentation of this file for the exact rule.
	DetailsKeyChangedAttributes = "changedAttributes"
	// DetailsKeyChangedAttributeCount holds the total number of changed
	// attribute paths, which is larger than the number of paths reported in
	// DetailsKeyChangedAttributes if the latter had to be truncated.
	DetailsKeyChangedAttributeCount = "changedAttributeCount"
)

const (
	// maxChangedAttributes is the maximum number of attribute paths rendered
	// into DetailsKeyChangedAttributes. A diff over a large resource can
	// contain hundreds of entries and the rendered value ends up in a change
	// log entry.
	maxChangedAttributes = 32
	// maxChangedAttributesBytes bounds the rendered value. It includes the
	// truncation notice, for which omissionReserve bytes are kept in reserve.
	maxChangedAttributesBytes = 2048
	omissionReserve           = 48

	markerRequiresReplace = "requiresReplace"
	markerSensitive       = "sensitive"

	// pathWildcard stands in for every path segment that is not an attribute
	// name declared by the Terraform schema, i.e. a list index, a set element
	// hash or a map key. It matches the wildcard upjet already uses in its own
	// sensitive Terraform field paths.
	pathWildcard = traverser.FieldPathWildcard

	// Terraform flatmap keys for the element count of a list or set, and for
	// the length of a map, respectively.
	flatmapCountKey  = "#"
	flatmapLengthKey = "%"
)

// changedAttribute is a single entry of the changed attribute set. It
// deliberately carries no value: neither the old nor the new value of an
// attribute is ever recorded, so no attribute value can reach a change log
// entry through this path.
type changedAttribute struct {
	// path is a Terraform attribute path whose segments are either attribute
	// names taken from the resource's Terraform schema, or pathWildcard.
	path string
	// requiresReplace reports that changing this attribute forces the
	// replacement of the external resource. Upjet refuses such updates, so
	// this marker names the attribute that blocks the update.
	requiresReplace bool
	// sensitive reports that the attribute is marked sensitive by the
	// Terraform schema, by the diff itself, or by upjet's own sensitive field
	// path configuration. It is informational only: no value is reported for
	// any attribute, sensitive or not.
	sensitive bool
}

func (c changedAttribute) String() string {
	switch {
	case c.requiresReplace && c.sensitive:
		return c.path + " (" + markerRequiresReplace + "," + markerSensitive + ")"
	case c.requiresReplace:
		return c.path + " (" + markerRequiresReplace + ")"
	case c.sensitive:
		return c.path + " (" + markerSensitive + ")"
	default:
		return c.path
	}
}

// changedAttributes accumulates the changed attribute set of a single diff.
// Distinct elements of a collection normalize to the same path, so entries are
// deduplicated and their markers merged.
type changedAttributes struct {
	byPath map[string]changedAttribute
}

func (c *changedAttributes) add(a changedAttribute) {
	if a.path == "" {
		return
	}
	if c.byPath == nil {
		c.byPath = make(map[string]changedAttribute)
	}
	if e, ok := c.byPath[a.path]; ok {
		a.requiresReplace = a.requiresReplace || e.requiresReplace
		a.sensitive = a.sensitive || e.sensitive
	}
	c.byPath[a.path] = a
}

func (c *changedAttributes) len() int {
	if c == nil {
		return 0
	}
	return len(c.byPath)
}

// additionalDetails renders the changed attribute set for
// managed.ExternalUpdate.AdditionalDetails. It returns nil for an empty set so
// that a no-op reconcile adds nothing to the change log entry.
func (c *changedAttributes) additionalDetails() managed.AdditionalDetails {
	if c.len() == 0 {
		return nil
	}
	paths := make([]string, 0, len(c.byPath))
	for p := range c.byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var sb strings.Builder
	rendered := 0
	for _, p := range paths {
		e := c.byPath[p].String()
		if rendered == maxChangedAttributes || sb.Len()+len(e)+2 > maxChangedAttributesBytes-omissionReserve {
			break
		}
		if rendered > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(e)
		rendered++
	}
	if rendered < len(paths) {
		sb.WriteString(", ... (")
		sb.WriteString(strconv.Itoa(len(paths) - rendered))
		sb.WriteString(" more omitted)")
	}
	return managed.AdditionalDetails{
		DetailsKeyChangedAttributes:     sb.String(),
		DetailsKeyChangedAttributeCount: strconv.Itoa(len(paths)),
	}
}

// sanitizePathSegment is the last line of defence of the path allowlist. Every
// segment that reaches the rendered output has already been matched against
// the resource's Terraform schema, so it is an identifier; anything that is not
// is replaced by pathWildcard rather than emitted.
func sanitizePathSegment(s string) string {
	if s == pathWildcard {
		return s
	}
	if s == "" {
		return pathWildcard
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return pathWildcard
		}
	}
	return s
}

func joinPath(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	out := make([]string, len(segments))
	for i, s := range segments {
		out[i] = sanitizePathSegment(s)
	}
	return strings.Join(out, ".")
}

// isSensitiveFieldPath reports whether path is covered by upjet's own sensitive
// Terraform field path model, which is independent of the sensitivity the
// Terraform schema declares. Both the sensitive path and the changed attribute
// path use "." separated segments with pathWildcard for collection elements, so
// they are comparable as prefixes of one another: a change below a sensitive
// path, and a change to a container of a sensitive path, both count.
func isSensitiveFieldPath(path string, sensitivePaths map[string]string) bool {
	for s := range sensitivePaths {
		if s == path || strings.HasPrefix(path, s+".") || strings.HasPrefix(s, path+".") {
			return true
		}
	}
	return false
}

// normalizeFlatmapPath converts a Terraform flatmap attribute key, as found in
// terraform.InstanceDiff.Attributes, into a reportable attribute path.
//
// The key is walked against the resource's Terraform schema. A segment is
// emitted verbatim only where the schema says an attribute name belongs, and
// only if the schema actually declares it; every other segment is user data - a
// list index, a set element hash or a map key - and is replaced by
// pathWildcard. A key that does not match the schema is dropped entirely. This
// is an allowlist: nothing that is not a compiled-in Terraform schema
// identifier can appear in the result.
//
// The second return value reports whether the schema marks the attribute, or
// one of its ancestors, sensitive.
func normalizeFlatmapPath(key string, root map[string]*schema.Schema) (string, bool, bool) { //nolint:gocyclo // a single flat state machine is easier to follow than the split alternative
	fields := root
	var elemOf *schema.Schema
	out := make([]string, 0, 4)
	sensitive := false

	for _, seg := range strings.Split(key, ".") {
		switch {
		case fields != nil:
			// An attribute name is expected here.
			s, ok := fields[seg]
			if !ok || s == nil {
				// Not declared by the schema. Refuse the whole key rather
				// than guessing what the segment is.
				return "", false, false
			}
			out = append(out, seg)
			if s.Sensitive {
				sensitive = true
			}
			fields, elemOf = nil, nil
			if s.Type == schema.TypeList || s.Type == schema.TypeSet || s.Type == schema.TypeMap {
				elemOf = s
			}
		case elemOf != nil:
			// A list index, a set element hash or a map key is expected here.
			if seg == flatmapCountKey || seg == flatmapLengthKey {
				// The diff is on the size of the collection itself.
				return joinPath(out), sensitive, len(out) > 0
			}
			out = append(out, pathWildcard)
			switch e := elemOf.Elem.(type) {
			case *schema.Resource:
				fields, elemOf = e.Schema, nil
			case *schema.Schema:
				fields, elemOf = nil, nil
				if e.Sensitive {
					sensitive = true
				}
				if e.Type == schema.TypeList || e.Type == schema.TypeSet || e.Type == schema.TypeMap {
					elemOf = e
				}
			default:
				fields, elemOf = nil, nil
			}
		default:
			// Below the deepest level the schema describes. Report the
			// deepest known path and stop; nothing derived from the
			// remainder of the key is emitted.
			out = append(out, pathWildcard)
			return joinPath(out), sensitive, true
		}
	}
	return joinPath(out), sensitive, len(out) > 0
}

// changedAttributesFromInstanceDiff collects the changed attribute set of a
// terraform plugin SDK diff.
func changedAttributesFromInstanceDiff(d *tf.InstanceDiff, cfg *config.Resource) *changedAttributes {
	c := &changedAttributes{}
	if d == nil || d.Empty() || cfg == nil || cfg.TerraformResource == nil {
		return c
	}
	sensitivePaths := cfg.Sensitive.GetFieldPaths()
	for k, ad := range d.Attributes {
		if ad == nil {
			continue
		}
		path, schemaSensitive, ok := normalizeFlatmapPath(k, cfg.TerraformResource.Schema)
		if !ok {
			continue
		}
		c.add(changedAttribute{
			path:            path,
			requiresReplace: ad.RequiresNew,
			// The union of the two sensitivity models: what the diff
			// reports, what the Terraform schema declares, and what upjet
			// itself considers a sensitive field path.
			sensitive: ad.Sensitive || schemaSensitive || isSensitiveFieldPath(path, sensitivePaths),
		})
	}
	return c
}

// normalizeAttributePath converts a terraform-plugin-go attribute path into a
// reportable attribute path. Only AttributeName steps carry schema declared
// identifiers; element keys are list indices, map keys or - for sets - whole
// element values, and are replaced by pathWildcard.
func normalizeAttributePath(p *tftypes.AttributePath) (string, bool) {
	if p == nil || len(p.Steps()) == 0 {
		return "", false
	}
	out := make([]string, 0, len(p.Steps()))
	for _, st := range p.Steps() {
		if n, ok := st.(tftypes.AttributeName); ok {
			out = append(out, string(n))
			continue
		}
		out = append(out, pathWildcard)
	}
	return joinPath(out), true
}

// changedAttributesFromValueDiffs collects the changed attribute set of a
// terraform plugin framework plan, from the already filtered diff between the
// prior state and the planned state.
func changedAttributesFromValueDiffs(ctx context.Context, diffs []tftypes.ValueDiff, s rschema.Schema, requiresReplace []*tftypes.AttributePath, cfg *config.Resource) *changedAttributes {
	c := &changedAttributes{}
	replace := make(map[string]struct{}, len(requiresReplace))
	for _, p := range requiresReplace {
		if path, ok := normalizeAttributePath(p); ok {
			replace[path] = struct{}{}
		}
	}
	var sensitivePaths map[string]string
	if cfg != nil {
		sensitivePaths = cfg.Sensitive.GetFieldPaths()
	}
	for _, d := range diffs {
		path, ok := normalizeAttributePath(d.Path)
		if !ok {
			continue
		}
		_, requiresNew := replace[path]
		c.add(changedAttribute{
			path:            path,
			requiresReplace: requiresNew,
			sensitive:       isSensitiveSchemaPath(ctx, s, d.Path) || isSensitiveFieldPath(path, sensitivePaths),
		})
	}
	return c
}

// isSensitiveSchemaPath reports whether the plugin framework schema marks the
// attribute at the given path, or one of its ancestors, sensitive.
func isSensitiveSchemaPath(ctx context.Context, s rschema.Schema, p *tftypes.AttributePath) bool {
	for cur := p; cur != nil && len(cur.Steps()) > 0; cur = cur.WithoutLastStep() {
		attr, err := s.AttributeAtTerraformPath(ctx, cur)
		if err != nil || attr == nil {
			// Not an attribute, a block or an attribute without a schema,
			// etc. Continue with its parent.
			continue
		}
		if attr.IsSensitive() {
			return true
		}
	}
	return false
}
