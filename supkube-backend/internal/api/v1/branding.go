// Package v1: branding.go — white-label / OEM rebranding (v0.8.11).
//
// Stored in the shared supkube-settings ConfigMap so EVERY user sees
// the same brand the second the admin saves. Keys:
//
//   branding.productName  — string shown in sidebar header + window title
//   branding.logoDataUrl  — data: URL for the sidebar shield icon
//   branding.faviconDataUrl — data: URL for the browser tab favicon
//
// Why data URLs (not a separate object-store upload)
// ──────────────────────────────────────────────────
// ConfigMap holds up to 1 MiB; a small SVG/PNG embeds easily in base64.
// One round-trip on app boot, no extra storage system, no extra CORS
// surface, no extra RBAC perm to manage. The 100 KB per-asset limit
// we enforce keeps us well clear of the CM ceiling even with future
// fields (terms-of-service link, support email, etc.).
//
// Default values (returned when the CM has no branding.* keys)
// ──────────────────────────────────────────────────────────────
//   productName: "SupKube"
//   logoDataUrl: "" → frontend falls back to its bundled built-in SVG
//   faviconDataUrl: "" → frontend falls back to /favicon.ico
//
// RBAC: GET is viewer-or-above (everyone needs branding to render
// the header), PUT is admin-only (rebrand is a privileged action).
package v1

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/supkube/supkube-backend/internal/k8s"
)

const (
	brandingCMName    = "supkube-settings" // shared with gc + policypair
	brandingCMNS      = "supkube"
	keyProductName    = "branding.productName"
	keyLogoDataUrl    = "branding.logoDataUrl"
	keyFaviconDataUrl = "branding.faviconDataUrl"
	keyPrimaryColor   = "branding.primaryColor"

	// Hard ceiling per asset. Below the 1 MiB CM limit by 10x, leaves
	// headroom for future fields, and a 100 KB logo PNG / SVG is plenty
	// for a 24×24-up-to-128×128 icon. Caller gets 400 + explanation.
	maxAssetBytes = 100 * 1024

	defaultProductName = "SupKube"
)

// BrandingResponse is what GET returns. All fields are best-effort; an
// empty string means "use built-in default" so the frontend never
// renders a broken icon during the brief window when only one asset is
// set.
type BrandingResponse struct {
	ProductName    string `json:"productName"`
	LogoDataUrl    string `json:"logoDataUrl"`
	FaviconDataUrl string `json:"faviconDataUrl"`
	// v0.8.11.1: primary color (hex string, e.g. "#4f46e5"). Empty = default.
	// Frontend applies it as --sk-primary override at runtime.
	PrimaryColor string `json:"primaryColor"`
}

// GetBranding returns the current branding. Public to any authenticated
// user because the sidebar + page title need it on every load — gating
// it admin-only would force the SPA to special-case viewer/editor
// boot flows.
func GetBranding(c *gin.Context) {
	k8sCli, err := k8s.GetClient()
	if err != nil {
		// Fall back to defaults rather than erroring; the SPA should
		// never be blocked from rendering by a transient cluster issue.
		c.JSON(http.StatusOK, BrandingResponse{ProductName: defaultProductName})
		return
	}
	cm, err := k8sCli.CoreV1().ConfigMaps(brandingCMNS).Get(c.Request.Context(), brandingCMName, metav1.GetOptions{})
	if err != nil {
		// CM missing → defaults. This is normal on fresh installs.
		c.JSON(http.StatusOK, BrandingResponse{ProductName: defaultProductName})
		return
	}
	resp := BrandingResponse{
		ProductName:    cm.Data[keyProductName],
		LogoDataUrl:    cm.Data[keyLogoDataUrl],
		FaviconDataUrl: cm.Data[keyFaviconDataUrl],
		PrimaryColor:   cm.Data[keyPrimaryColor],
	}
	if resp.ProductName == "" {
		resp.ProductName = defaultProductName
	}
	c.JSON(http.StatusOK, resp)
}

// hexColorRe matches a 3 or 6 digit hex colour with leading #. Anchored
// so an admin can't paste arbitrary CSS (e.g. "red; background:url(...)")
// into a value that goes straight into a style="--sk-primary: …" attr.
var hexColorRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// dataUrlRe validates the data: URL prefix. We accept SVG / PNG / JPEG /
// ICO / WebP. Disallowing arbitrary MIME types is defence-in-depth
// against an admin pasting a `data:text/html` URL that some browser
// version might try to render.
var dataUrlRe = regexp.MustCompile(`^data:image/(svg\+xml|png|jpeg|jpg|x-icon|vnd\.microsoft\.icon|webp);base64,([A-Za-z0-9+/=]+)$`)

// validateAsset checks size + format. Empty string is allowed (means
// "use default"). On invalid input returns a human-readable reason.
func validateAsset(field, value string) (ok bool, reason string) {
	if value == "" {
		return true, ""
	}
	if len(value) > maxAssetBytes {
		return false, field + " exceeds 100 KB limit (received " +
			humanKB(len(value)) + ")"
	}
	m := dataUrlRe.FindStringSubmatch(value)
	if m == nil {
		return false, field + " must be a data: URL with image MIME type " +
			"(image/svg+xml, image/png, image/jpeg, image/x-icon, or image/webp); " +
			"received prefix " + truncateForError(value, 40)
	}
	// Decode to make sure the base64 is well-formed; rejecting at this
	// layer beats serving a broken <img> later.
	if _, err := base64.StdEncoding.DecodeString(m[2]); err != nil {
		return false, field + ": base64 payload could not be decoded (" + err.Error() + ")"
	}
	return true, ""
}

func humanKB(n int) string {
	const (
		kb = 1024
	)
	v := float64(n) / float64(kb)
	if v < 1 {
		return "<1 KB"
	}
	// 1 decimal place is plenty for an error message.
	return formatFloat1(v) + " KB"
}

// formatFloat1 — minimal one-decimal formatter to avoid importing fmt
// just for this tiny helper. Inlining stays consistent with the rest
// of backup_meta.go's approach.
func formatFloat1(v float64) string {
	intPart := int(v)
	frac := int((v - float64(intPart)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return itoa10(intPart) + "." + itoa10(frac)
}

func itoa10(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func truncateForError(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// UpdateBranding writes the new values into the supkube-settings CM.
// Admin-only (enforced by the central RBAC table). Empty string for a
// field means "reset to default" — frontend will fall back to its
// bundled SVG.
//
// Idempotent: re-PUTing the same body is a no-op from the user's
// perspective; the CM gets updated each time but the rendered UI
// looks identical.
func UpdateBranding(c *gin.Context) {
	var req BrandingResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Trim whitespace + reject "all whitespace" as if empty so an
	// admin accidentally typing spaces doesn't blank the brand.
	req.ProductName = strings.TrimSpace(req.ProductName)
	if req.ProductName == "" {
		req.ProductName = defaultProductName
	}
	// Soft length cap on the name — keeps the sidebar/header tidy.
	if len(req.ProductName) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "productName must be 64 characters or fewer"})
		return
	}
	if ok, reason := validateAsset("logoDataUrl", req.LogoDataUrl); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": reason})
		return
	}
	if ok, reason := validateAsset("faviconDataUrl", req.FaviconDataUrl); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": reason})
		return
	}
	// v0.8.11.1: validate primary color. Empty = default. Otherwise must
	// be a #-prefixed hex (3 or 6 digit) — anything else gets refused so
	// the value can't break CSS at apply time.
	req.PrimaryColor = strings.TrimSpace(req.PrimaryColor)
	if req.PrimaryColor != "" && !hexColorRe.MatchString(req.PrimaryColor) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "primaryColor must be a # hex colour (e.g. #4f46e5)"})
		return
	}

	k8sCli, err := k8s.GetClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	cm, err := k8sCli.CoreV1().ConfigMaps(brandingCMNS).Get(ctx, brandingCMName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err != nil { // not found — create
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: brandingCMName, Namespace: brandingCMNS},
			Data: map[string]string{
				keyProductName:    req.ProductName,
				keyLogoDataUrl:    req.LogoDataUrl,
				keyFaviconDataUrl: req.FaviconDataUrl,
				keyPrimaryColor:   req.PrimaryColor,
			},
		}
		if _, err := k8sCli.CoreV1().ConfigMaps(brandingCMNS).Create(ctx, cm, metav1.CreateOptions{}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, req)
		return
	}
	// existing CM — patch the 3 keys in-place. We preserve other
	// settings (cleanup.enabled, policyPair.enabled, …) by not
	// touching them.
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[keyProductName] = req.ProductName
	cm.Data[keyLogoDataUrl] = req.LogoDataUrl
	cm.Data[keyFaviconDataUrl] = req.FaviconDataUrl
	cm.Data[keyPrimaryColor] = req.PrimaryColor
	if _, err := k8sCli.CoreV1().ConfigMaps(brandingCMNS).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// v0.8.12 TODO: emit a K8s Event tagged supkube.io/activity=true so
	// the rebrand shows up in Activity. Deferred to v0.8.12 because the
	// existing gc + policypair Event emitters expect typed clients we'd
	// have to plumb here too — out of scope for the urgent v0.8.11 ship.
	c.JSON(http.StatusOK, req)
}
