package transfer

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func getLayersFromTarball(tarPath string) ([]string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tr := tar.NewReader(file)
	var manifests []struct {
		Config string   `json:"Config"`
		Layers []string `json:"Layers"`
	}
	var configName string

	// Find manifest.json and the main image config
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == "manifest.json" {
			if err := json.NewDecoder(tr).Decode(&manifests); err != nil {
				return nil, err
			}
			// Find the main image manifest
			max := 0
			for _, m := range manifests {
				if len(m.Layers) > max {
					max = len(m.Layers)
					configName = m.Config
				}
			}
			break
		}
	}

	if configName == "" {
		return nil, fmt.Errorf("could not find main config in manifest")
	}

	//Rewind and parse the Config JSON to get the actual layer digests
	file.Seek(0, 0)
	tr = tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == configName {
			var config struct {
				RootFS struct {
					DiffIDs []string `json:"diff_ids"`
				} `json:"rootfs"`
			}
			if err := json.NewDecoder(tr).Decode(&config); err != nil {
				return nil, err
			}
			return config.RootFS.DiffIDs, nil
		}
	}

	return nil, fmt.Errorf("config file not found inside tarball")
}
