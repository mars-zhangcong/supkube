{{- define "supkube.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "supkube.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "supkube.labels" -}}
helm.sh/chart: {{ include "supkube.name" . }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: supkube
{{ include "supkube.selectorLabels" . }}
{{- end }}

{{- define "supkube.selectorLabels" -}}
app.kubernetes.io/name: {{ include "supkube.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
v0.8.14 LV8 — supkube.image: render a full image reference honoring
global.airgapped.repository for airgapped customer installs.

Usage from any template:
  image: {{ include "supkube.image" (dict "root" $ "repository" .Values.backend.image.repository "tag" .Values.backend.image.tag) }}

Behavior:
  - global.airgapped.repository unset (default) → "<repository>:<tag>" unchanged
  - global.airgapped.repository set        → "<mirror>/<last-path-component>:<tag>"

E.g. "supkube.azurecr.io/backend:0.9.1.2-alpha" with
mirror="harbor.internal.corp/supkube-mirror" becomes
"harbor.internal.corp/supkube-mirror/backend:0.9.1.2-alpha".

The "last-path-component" rewrite (vs. Kasten's literal prefix substitution)
matches how customer-internal registries are typically organized — one flat
namespace with image names as siblings, not Azure-style deep hierarchy.
Customer mirrors 5 images:
  supkube.azurecr.io/backend       → <mirror>/backend
  supkube.azurecr.io/frontend      → <mirror>/frontend
  dexidp/dex                       → <mirror>/dex
  quay.io/minio/minio              → <mirror>/minio
  quay.io/minio/mc                 → <mirror>/mc
... and no extra path math is needed at install time.

Velero subchart images do NOT pass through this helper — Helm subcharts get
their own values namespace and don't inherit global.* by convention. Airgap
install docs (USER_MANUAL §24) explain the --set velero.image.repository
override pattern for those.
*/}}
{{- define "supkube.image" -}}
{{- $mirror := "" -}}
{{- if .root.Values.global -}}
  {{- if .root.Values.global.airgapped -}}
    {{- $mirror = default "" .root.Values.global.airgapped.repository -}}
  {{- end -}}
{{- end -}}
{{- if $mirror -}}
  {{- $parts := splitList "/" .repository -}}
  {{- $name := last $parts -}}
  {{- printf "%s/%s:%s" (trimSuffix "/" $mirror) $name .tag -}}
{{- else -}}
  {{- printf "%s:%s" .repository .tag -}}
{{- end -}}
{{- end -}}
