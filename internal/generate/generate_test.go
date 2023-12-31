package generate

import (
	"bufio"
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/manifest"
)

// TestGenerateFromTemplates tests rendered templates contains two documents
func TestGenerateFromTemplates(t *testing.T) {
	buf := bytes.NewBuffer([]byte{})
	bufio.NewWriter(buf)
	err := Run(buf, manifest.UniqueIDFormatULID)
	assert.NilError(t, err, "error running generate action")
	assert.Check(t, buf.Len() > 0, "expected output to be greater than 0")
	t.Logf(buf.String())
	assert.Check(t, !bytes.Contains(buf.Bytes(), []byte(`{{ .ID }}`)), "output contains template string")
	reader := bufio.NewReader(buf)
	decoder := yaml.NewDecoder(reader)
	assert.NilError(t, err, "error decoding generated yaml")
	var decoded []interface{}
	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			break
		}
		t.Logf("decoded yaml: %v", node)
		decoded = append(decoded, node)
	}
	assert.Check(t, len(decoded) > 1, "expected decoded yaml documents to be greater than 1")
}
