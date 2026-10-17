{{- define "yuva.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "yuva.fullname" -}}
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

{{- define "yuva.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "yuva.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yuva.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "yuva.labels" -}}
helm.sh/chart: {{ include "yuva.chart" . }}
{{ include "yuva.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "yuva.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "yuva.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "yuva.image" -}}
{{- if .Values.image.digest }}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest }}
{{- else }}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}
{{- end }}

{{- define "yuva.secretName" -}}
{{- default (include "yuva.fullname" .) .Values.secrets.existingSecret }}
{{- end }}

{{- define "yuva.secretVars" -}}
YUVA_DATABASE_URL: true
YUVA_MASTER_KEY: true
YUVA_INGRESS_SECRET: false
YUVA_SMTP_USERNAME: false
YUVA_SMTP_PASSWORD: false
YUVA_S3_ACCESS_KEY_ID: false
YUVA_S3_SECRET_ACCESS_KEY: false
YUVA_VAPID_PUBLIC_KEY: false
YUVA_VAPID_PRIVATE_KEY: false
YUVA_MICROSOFT_CLIENT_ID: false
YUVA_MICROSOFT_CLIENT_SECRET: false
YUVA_SES_ACCESS_KEY_ID: false
YUVA_SES_SECRET_ACCESS_KEY: false
{{- end }}

{{- define "yuva.publicHost" -}}
{{- $u := urlParse (required "publicUrl is required: the URL people and apps use, e.g. https://support.example.com" .Values.publicUrl) }}
{{- if not (has $u.scheme (list "http" "https")) }}
{{- fail (printf "publicUrl must be an absolute http(s) URL, got %q" .Values.publicUrl) }}
{{- end }}
{{- if not $u.host }}
{{- fail (printf "publicUrl must be an absolute http(s) URL, got %q" .Values.publicUrl) }}
{{- end }}
{{- regexReplaceAll ":[0-9]+$" $u.host "" }}
{{- end }}

{{- define "yuva.validate" -}}
{{- $_ := include "yuva.publicHost" . }}
{{- if not (has .Values.storage.driver (list "local" "s3")) }}
{{- fail (printf "storage.driver must be local or s3, got %q" .Values.storage.driver) }}
{{- end }}
{{- if and (eq .Values.storage.driver "local") (gt (int .Values.replicaCount) 1) }}
{{- fail "storage.driver local keeps attachments on one volume and needs replicaCount 1; use s3 for several replicas" }}
{{- end }}
{{- range $k, $_ := .Values.env }}
{{- if hasKey (include "yuva.secretVars" $ | fromYaml) $k }}
{{- fail (printf "%s is a secret: set it in secrets.existingSecret or secrets.values, not in env" $k) }}
{{- end }}
{{- if has $k (list "YUVA_PUBLIC_URL" "YUVA_STORAGE" "YUVA_STORAGE_DIR" "YUVA_LISTEN_ADDR" "YUVA_METRICS_ADDR") }}
{{- fail (printf "%s is set by the chart (publicUrl, storage.driver, metrics.enabled); remove it from env" $k) }}
{{- end }}
{{- end }}
{{- if not .Values.secrets.existingSecret }}
{{- range $k, $req := include "yuva.secretVars" . | fromYaml }}
{{- if and $req (not (index $.Values.secrets.values $k)) }}
{{- fail (printf "%s is required: set secrets.existingSecret, or secrets.values.%s" $k $k) }}
{{- end }}
{{- end }}
{{- end }}
{{- if and .Values.metrics.serviceMonitor.enabled (not .Values.metrics.enabled) }}
{{- fail "metrics.serviceMonitor.enabled needs metrics.enabled" }}
{{- end }}
{{- end }}

{{- define "yuva.memoryBytes" -}}
{{- $q := toString . }}
{{- $units := dict "Ki" 1024 "Mi" 1048576 "Gi" 1073741824 "Ti" 1099511627776 "k" 1000 "K" 1000 "M" 1000000 "G" 1000000000 "T" 1000000000000 "" 1 }}
{{- $num := regexFind "^[0-9.]+" $q }}
{{- $unit := trimPrefix $num $q }}
{{- if or (not $num) (not (hasKey $units $unit)) }}
{{- fail (printf "resources.limits.memory %q is not a quantity this chart understands (use Mi or Gi)" $q) }}
{{- end }}
{{- mulf (float64 $num) (index $units $unit) | int64 }}
{{- end }}

{{- define "yuva.env" -}}
- name: YUVA_PUBLIC_URL
  value: {{ .Values.publicUrl | quote }}
- name: YUVA_STORAGE
  value: {{ .Values.storage.driver | quote }}
{{- if eq .Values.storage.driver "local" }}
- name: YUVA_STORAGE_DIR
  value: /data/attachments
{{- end }}
{{- if not (hasKey .Values.env "YUVA_GEOIP_DIR") }}
- name: YUVA_GEOIP_DIR
  value: /data/geoip
{{- end }}
- name: YUVA_METRICS_ADDR
  value: {{ ternary ":9090" "off" .Values.metrics.enabled | quote }}
{{- $limit := dig "limits" "memory" "" .Values.resources }}
{{- if and $limit (not (hasKey .Values.env "GOMEMLIMIT")) }}
- name: GOMEMLIMIT
  value: {{ mulf (include "yuva.memoryBytes" $limit | float64) .Values.goMemLimitRatio | int64 | toString | quote }}
{{- end }}
{{- range $k, $v := .Values.env }}
- name: {{ $k }}
  value: {{ $v | toString | quote }}
{{- end }}
{{- range $k, $req := include "yuva.secretVars" . | fromYaml }}
{{- if or $.Values.secrets.existingSecret (index $.Values.secrets.values $k) }}
- name: {{ $k }}
  valueFrom:
    secretKeyRef:
      name: {{ include "yuva.secretName" $ }}
      key: {{ index $.Values.secrets.keys $k | default $k }}
      {{- if not $req }}
      optional: true
      {{- end }}
{{- end }}
{{- end }}
{{- if not .Values.secrets.existingSecret }}
{{- $known := include "yuva.secretVars" . | fromYaml }}
{{- range $k, $_ := .Values.secrets.values }}
{{- if not (hasKey $known $k) }}
- name: {{ $k }}
  valueFrom:
    secretKeyRef:
      name: {{ include "yuva.secretName" $ }}
      key: {{ $k }}
{{- end }}
{{- end }}
{{- end }}
{{- with .Values.extraEnv }}
{{ toYaml . }}
{{- end }}
{{- end }}
