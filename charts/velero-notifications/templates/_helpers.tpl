{{/*
Create a default fullname using the release name and chart name.
*/}}
{{- define "velero-notifications.fullname" -}}
{{- default .Chart.Name .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Generate a hash from the config data.
*/}}
{{- define "velero-notifications.confighash" -}}
{{- $configData := .Values.config | toYaml -}}
{{- sha256sum $configData -}}
{{- end -}}