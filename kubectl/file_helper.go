package kubectl

import (
	"encoding/base64"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func deleteFile(path string) error {
	// delete file
	var err = os.Remove(path)
	if err != nil {
		return err
	}

	return nil
}

func createTempfile(base64Content string) (string, error) {

	content, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return "", err
	}

	tmpfile, err := ioutil.TempFile(os.TempDir(), "kubeconfig_")
	if err != nil {
		return "", err
	}

	tempFilePath, err := filepath.Abs(tmpfile.Name())
	if err != nil {
		return "", err
	}

	if _, err := tmpfile.Write(content); err != nil {
		return tempFilePath, err
	}

	if err := tmpfile.Close(); err != nil {
		return tempFilePath, err
	}

	return tempFilePath, nil
}

func ReadFile(path string) (string, error) {
	// re-open file
	var file, err = os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// read file, line by line
	var text = make([]byte, 1024)
	for {
		_, err = file.Read(text)

		// break if finally arrived at end of file
		if err == io.EOF {
			break
		}

		// break if error occured
		if err != nil && err != io.EOF {
			break
		}
	}
	return string(text), nil
}
