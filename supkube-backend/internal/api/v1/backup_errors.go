package v1

import (
	"sort"
	"strings"
)

// VeleroRestoreResults represents parsed restore results used by API responses.
// Keep backward-compatible fields so existing callers compiling against older
// names (Errors/Warnings, section fields) continue to work.
type VeleroRestoreResults struct {
	Errors   []string              `json:"errors,omitempty"`
	Warnings []string              `json:"warnings,omitempty"`
	Sections []VeleroResultSection `json:"sections,omitempty"`
}

// VeleroResultSection represents grouped restore result messages.
// The fields mirror the shape expected by backup_errors.go call sites.
type VeleroResultSection struct {
	Velero     []string                       `json:"velero,omitempty"`
	Cluster    []string                       `json:"cluster,omitempty"`
	Namespaces map[string][]string            `json:"namespaces,omitempty"`
	Other      map[string][]string            `json:"other,omitempty"`
}

// normalize ensures zero-value maps/slices are stable for downstream use.
func (r *VeleroRestoreResults) normalize() {
	if r == nil {
		return
	}
	for i := range r.Sections {
		if r.Sections[i].Namespaces == nil {
			r.Sections[i].Namespaces = map[string][]string{}
		}
		if r.Sections[i].Other == nil {
			r.Sections[i].Other = map[string][]string{}
		}
	}
}

// flattenMessages collects all result messages into deterministic slices.
func (r *VeleroRestoreResults) flattenMessages() {
	if r == nil {
		return
	}
	var errs []string
	var warns []string
	for _, sec := range r.Sections {
		errMsgs := append([]string{}, sec.Velero...)
		errMsgs = append(errMsgs, sec.Cluster...)
		for _, ns := range sortedKeys(sec.Namespaces) {
			errMsgs = append(errMsgs, sec.Namespaces[ns]...)
		}
		for _, k := range sortedKeys(sec.Other) {
			warns = append(warns, sec.Other[k]...)
		}
		errMsgs = compactNonEmpty(errMsgs)
		if len(errMsgs) > 0 {
			errs = append(errs, errMsgs...)
		}
	}
	r.Errors = compactNonEmpty(errs)
	r.Warnings = compactNonEmpty(warns)
}

func compactNonEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
