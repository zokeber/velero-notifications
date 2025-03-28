{{/*
Create a default fullname using the release name and chart name.
*/}}
{{- define "velero-notifications.fullname" -}}
{{- default .Chart.Name .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Generate a hash from the values file.
*/}}
{{- define "velero-notifications.confighash" -}}
{{- toYaml .Values | sha256sum -}}
{{- end -}}