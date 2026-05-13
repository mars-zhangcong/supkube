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
