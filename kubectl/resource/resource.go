package resource

import ()

type KubectlResponse struct {
	Items []KubectlItem `json:"items"`
	Kind  string        `json:"kind"`
}

type KubectlItem struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Selflink string `json:"selflink"`
		UID      string `json:"uid"`
	} `json:"metadata"`
}
