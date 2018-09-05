package resource

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func SplitYAMLDocument(multiResourceDoc string) []string {
	emptyLINE := regexp.MustCompile("^\\s*$[\r\n]*")

	yamlSep := regexp.MustCompile(`^---\s*$`)
	docs := make([]string, 0)
	var buffer bytes.Buffer

	scanner := bufio.NewScanner(strings.NewReader(multiResourceDoc))

	for scanner.Scan() {
		line := scanner.Text()
		if emptyLINE.MatchString(line) {
			continue
		}

		if yamlSep.MatchString(line) {
			currentDoc := buffer.String()
			buffer.Reset()
			if currentDoc != "" {
				docs = append(docs, currentDoc)
			}
		}
		buffer.WriteString(fmt.Sprintf("%s\n", line))
	}

	lastDoc := buffer.String()
	if lastDoc != "" {
		docs = append(docs, lastDoc)
	}
	return docs
}
