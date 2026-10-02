package deployment

import (
	"bytes"
	"testing"

	"github.com/arttor/helmify/pkg/metadata"

	"github.com/arttor/helmify/internal"
	"github.com/stretchr/testify/assert"
)

const (
	strDepl = `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: my-operator-controller-manager
  namespace: my-operator-system
spec:
  revisionHistoryLimit: 5
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      containers:
      - args:
        - --secure-listen-address=0.0.0.0:8443
        - --upstream=http://127.0.0.1:8080/
        - --logtostderr=true
        - --v=10
        image: gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0
        name: kube-rbac-proxy
        ports:
        - containerPort: 8443
          name: https
      - args:
        - --health-probe-bind-address=:8081
        - --metrics-bind-address=127.0.0.1:8080
        - --leader-elect
        command:
        - /manager
        volumeMounts:
        - mountPath: /controller_manager_config.yaml
          name: manager-config
          subPath: controller_manager_config.yaml
        - name: secret-volume
          mountPath: /my.ca
        - name: sample-pv-storage
          mountPath: "/usr/share/nginx/html"
        env:
        - name: VAR1
          valueFrom:
            secretKeyRef:
              name: my-operator-secret-vars
              key: VAR1
        - name: VAR2
          valueFrom:
            configMapKeyRef:
              name: my-operator-configmap-vars
              key: VAR2
        - name: VAR3
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: VAR4
          valueFrom:
            resourceFieldRef:
              resource: limits.cpu
        - name: VAR5
          value: "123"
        - name: VAR6
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['app.kubernetes.io/something']
        image: controller:latest
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        name: manager
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 100m
            memory: 30Mi
          requests:
            cpu: 100m
            memory: 20Mi
        securityContext:
          allowPrivilegeEscalation: false
      securityContext:
        runAsNonRoot: true
      serviceAccountName: my-operator-controller-manager
      terminationGracePeriodSeconds: 10
      volumes:
      - configMap:
          name: my-operator-manager-config
        name: manager-config
      - name: secret-volume
        secret:
          secretName: my-operator-secret-ca
      - name: sample-pv-storage
        persistentVolumeClaim:
          claimName: my-sample-pv-claim
`
)

func Test_deployment_Process(t *testing.T) {
	var testInstance deployment

	t.Run("processed", func(t *testing.T) {
		obj := internal.GenerateObj(strDepl)
		processed, _, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, true, processed)
	})
	t.Run("skipped", func(t *testing.T) {
		obj := internal.TestNs
		processed, _, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, false, processed)
	})

	t.Run("processed with duplicate app.kubernetes.io/name in selector and pod labels", func(t *testing.T) {
		const deplWithDuplicateYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: platform-operator
  labels:
    app.kubernetes.io/name: platform-operator
    control-plane: controller-manager
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: platform-operator
      control-plane: controller-manager
  template:
    metadata:
      labels:
        app.kubernetes.io/name: platform-operator
        control-plane: controller-manager
    spec:
      containers:
      - name: manager
        image: controller:latest`
		obj := internal.GenerateObj(deplWithDuplicateYaml)
		processed, template, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.True(t, processed)
		var buf bytes.Buffer
		err = template.Write(&buf)
		assert.NoError(t, err)
		out := buf.String()
		assert.NotContains(t, out, "app.kubernetes.io/name: platform-operator")
		assert.Contains(t, out, "control-plane: controller-manager")
		assert.Contains(t, out, `{{- include ".selectorLabels" . | nindent 6 }}`)
		assert.Contains(t, out, `{{- include ".selectorLabels" . | nindent 8 }}`)
	})

	t.Run("processed with only app.kubernetes.io/name in selector and pod labels", func(t *testing.T) {
		const deplOnlyNameYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: platform-operator
  labels:
    app.kubernetes.io/name: platform-operator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: platform-operator
  template:
    metadata:
      labels:
        app.kubernetes.io/name: platform-operator
    spec:
      containers:
      - name: manager
        image: controller:latest`
		obj := internal.GenerateObj(deplOnlyNameYaml)
		processed, template, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.True(t, processed)
		var buf bytes.Buffer
		err = template.Write(&buf)
		assert.NoError(t, err)
		out := buf.String()
		assert.NotContains(t, out, "app.kubernetes.io/name: platform-operator")
		assert.NotContains(t, out, "matchLabels: {}")
		assert.Contains(t, out, "matchLabels:\n    {{- include \".selectorLabels\" . | nindent 6 }}")
		assert.Contains(t, out, "labels:\n      {{- include \".selectorLabels\" . | nindent 8 }}")
	})
}

var singleQuotesTest = []struct {
	input    string
	expected string
}{
	{
		"{{ .Values.x }}",
		"{{ .Values.x }}",
	},
	{
		"'{{ .Values.x }}'",
		"{{ .Values.x }}",
	},
	{
		"'{{ .Values.x }}:{{ .Values.y }}'",
		"{{ .Values.x }}:{{ .Values.y }}",
	},
	{
		"'{{ .Values.x }}:{{ .Values.y \n\t| default .Chart.AppVersion}}'",
		"{{ .Values.x }}:{{ .Values.y \n\t| default .Chart.AppVersion}}",
	},
	{
		"echo 'x'",
		"echo 'x'",
	},
	{
		"abcd: x.y['x/y']",
		"abcd: x.y['x/y']",
	},
	{
		"abcd: x.y[\"'{{}}'\"]",
		"abcd: x.y[\"{{}}\"]",
	},
	{
		"image: '{{ .Values.x }}'",
		"image: {{ .Values.x }}",
	},
	{
		"'{{ .Values.x }} y'",
		"{{ .Values.x }} y",
	},
	{
		"\t\t- mountPath: './x.y'",
		"\t\t- mountPath: './x.y'",
	},
	{
		"'{{}}'",
		"{{}}",
	},
	{
		"'{{ {nested} }}'",
		"{{ {nested} }}",
	},
	{
		"'{{ '{{nested}}' }}'",
		"{{ '{{nested}}' }}",
	},
	{
		"'{{ unbalanced }'",
		"'{{ unbalanced }'",
	},
	{
		"'{{\nincomplete content'",
		"'{{\nincomplete content'",
	},
	{
		"'{{ @#$%^&*() }}'",
		"{{ @#$%^&*() }}",
	},
}

func Test_replaceSingleQuotes(t *testing.T) {
	for _, tt := range singleQuotesTest {
		t.Run(tt.input, func(t *testing.T) {
			s := replaceSingleQuotes(tt.input)
			if s != tt.expected {
				t.Errorf("got %q, want %q", s, tt.expected)
			}
		})
	}
}
