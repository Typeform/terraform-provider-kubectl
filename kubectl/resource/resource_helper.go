package resource

import (
	"bytes"
	yamlReader "github.com/kubernetes/apimachinery/pkg/util/yaml"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"strings"
)

func hasKind(resource yaml.MapSlice) bool {

	for _, item := range resource {
		key, ok := item.Key.(string)
		if !ok {
			continue
		}
		if key == "kind" {
			return true
		}
	}
	return false
}

func SplitYAMLDocument(multiResourceDoc string) ([]string, error) {

	yamlReaderCloser := ioutil.NopCloser(strings.NewReader(multiResourceDoc))
	dec := yamlReader.NewDocumentDecoder(yamlReaderCloser)

	docs := make([]string, 0)
	buffer := bytes.NewBuffer(make([]byte, 0))
	part := make([]byte, 4092)

	for {
		count, err := dec.Read(part)

		if err == io.EOF {
			break
		}
		if err == io.ErrShortBuffer {
			buffer.Write(part[:count])
			continue
		}

		buffer.Write(part[:count])
		res := yaml.MapSlice{}
		yaml.Unmarshal(buffer.Bytes(), &res)

		if !hasKind(res) {
			buffer = bytes.NewBuffer(make([]byte, 0))
			continue
		}
		out, err := yaml.Marshal(&res)
		if err != nil {
			return nil, err
		}

		docs = append(docs, string(out))
		buffer = bytes.NewBuffer(make([]byte, 0))
	}

	return docs, nil
}
