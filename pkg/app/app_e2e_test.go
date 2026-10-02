package app

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/arttor/helmify/pkg/config"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/action"
)

const (
	operatorChartName = "test-operator"
	appChartName      = "test-app"
)

func TestOperator(t *testing.T) {
	file, err := os.Open("../../test_data/k8s-operator-kustomize.output")
	assert.NoError(t, err)

	objects := bufio.NewReader(file)
	err = Start(objects, config.Config{ChartName: operatorChartName})
	assert.NoError(t, err)

	t.Cleanup(func() {
		err = os.RemoveAll(operatorChartName)
		assert.NoError(t, err)
	})

	helmLint := action.NewLint()
	helmLint.Strict = true
	helmLint.Namespace = "test-ns"
	result := helmLint.Run([]string{operatorChartName}, nil)
	for _, err = range result.Errors {
		assert.NoError(t, err)
	}
}

func TestApp(t *testing.T) {
	file, err := os.Open("../../test_data/sample-app.yaml")
	assert.NoError(t, err)

	objects := bufio.NewReader(file)
	err = Start(objects, config.Config{ChartName: appChartName})
	assert.NoError(t, err)

	t.Cleanup(func() {
		err = os.RemoveAll(appChartName)
		assert.NoError(t, err)
	})

	helmLint := action.NewLint()
	helmLint.Strict = true
	helmLint.Namespace = "test-ns"
	result := helmLint.Run([]string{appChartName}, nil)
	for _, err = range result.Errors {
		assert.NoError(t, err)
	}
}

func TestDuplicateSelectorLabelKustomize(t *testing.T) {
	const chartName = "test-dup-selector"
	input := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: platform-operator
  namespace: default
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
        image: controller:latest
---
apiVersion: v1
kind: Service
metadata:
  name: platform-operator-service
  labels:
    app.kubernetes.io/name: platform-operator
spec:
  selector:
    app.kubernetes.io/name: platform-operator
    control-plane: controller-manager
  ports:
  - port: 8080
`
	reader := bufio.NewReader(strings.NewReader(input))
	err := Start(reader, config.Config{ChartName: chartName})
	assert.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(chartName)
	})

	helmLint := action.NewLint()
	helmLint.Strict = true
	helmLint.Namespace = "test-ns"
	result := helmLint.Run([]string{chartName}, nil)
	for _, err = range result.Errors {
		assert.NoError(t, err)
	}
	assert.Empty(t, result.Errors)

	deplContent, err := os.ReadFile(chartName + "/templates/deployment.yaml")
	assert.NoError(t, err)
	assert.NotContains(t, string(deplContent), "app.kubernetes.io/name: platform-operator")

	svcContent, err := os.ReadFile(chartName + "/templates/service.yaml")
	assert.NoError(t, err)
	assert.NotContains(t, string(svcContent), "app.kubernetes.io/name: platform-operator")
}
