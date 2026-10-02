package daemonset

import (
	"bytes"
	"testing"

	"github.com/arttor/helmify/pkg/metadata"

	"github.com/arttor/helmify/internal"
	"github.com/stretchr/testify/assert"
)

const (
	strDepl = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluentd-elasticsearch
  namespace: kube-system
  labels:
    k8s-app: fluentd-logging
spec:
  selector:
    matchLabels:
      name: fluentd-elasticsearch
  template:
    metadata:
      labels:
        name: fluentd-elasticsearch
    spec:
      tolerations:
      # this toleration is to have the daemonset runnable on master nodes
      # remove it if your masters can't run pods
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      containers:
      - name: fluentd-elasticsearch
        image: quay.io/fluentd_elasticsearch/fluentd:v2.5.2
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 200Mi
        volumeMounts:
        - name: varlog
          mountPath: /var/log
        - name: varlibdockercontainers
          mountPath: /var/lib/docker/containers
          readOnly: true
      terminationGracePeriodSeconds: 30
      volumes:
      - name: varlog
        hostPath:
          path: /var/log
      - name: varlibdockercontainers
        hostPath:
          path: /var/lib/docker/containers
`
)

func Test_daemonset_Process(t *testing.T) {
	var testInstance daemonset

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
		const daeWithDuplicateYaml = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: platform-operator-daemon
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
		obj := internal.GenerateObj(daeWithDuplicateYaml)
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
		const daeOnlyNameYaml = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: platform-operator-daemon
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
		obj := internal.GenerateObj(daeOnlyNameYaml)
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
