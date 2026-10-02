package poddisruptionbudget

import (
	"bytes"
	"os"
	"testing"

	"github.com/arttor/helmify/pkg/metadata"

	"github.com/arttor/helmify/internal"
	"github.com/stretchr/testify/assert"
)

const pdbYaml = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  labels:
    control-plane: controller-manager
  name: my-operator-controller-manager-pdb
  namespace: my-operator-system
spec:
  minAvailable: 2
  selector:
    matchLabels:
      control-plane: controller-manager`

func Test_pdb_Process(t *testing.T) {
	var testInstance pdb

	t.Run("processed", func(t *testing.T) {
		obj := internal.GenerateObj(pdbYaml)
		processed, tt, err := testInstance.Process(&metadata.Service{}, obj)
		_ = tt.Write(os.Stdout)
		assert.NoError(t, err)
		assert.Equal(t, true, processed)
	})
	t.Run("skipped", func(t *testing.T) {
		obj := internal.TestNs
		processed, _, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.Equal(t, false, processed)
	})

	t.Run("processed with duplicate app.kubernetes.io/name in selector", func(t *testing.T) {
		const pdbWithDuplicateYaml = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: platform-operator
      control-plane: controller-manager`
		obj := internal.GenerateObj(pdbWithDuplicateYaml)
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
	})

	t.Run("processed with only app.kubernetes.io/name in selector", func(t *testing.T) {
		const pdbOnlyNameYaml = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: platform-operator`
		obj := internal.GenerateObj(pdbOnlyNameYaml)
		processed, template, err := testInstance.Process(&metadata.Service{}, obj)
		assert.NoError(t, err)
		assert.True(t, processed)
		var buf bytes.Buffer
		err = template.Write(&buf)
		assert.NoError(t, err)
		out := buf.String()
		assert.NotContains(t, out, "app.kubernetes.io/name: platform-operator")
		assert.NotContains(t, out, "{}")
		assert.Contains(t, out, "matchLabels:\n    {{- include \".selectorLabels\" . | nindent 6 }}")
	})
}
