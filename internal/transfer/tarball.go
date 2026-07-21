package transfer

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func getLayersFromTarball(tarPath string) ([]string, error) {
	item, err := GetMainManifestItem(tarPath)
	if err != nil {
		return nil, err
	}
	configName := item.Config

	file, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tr := tar.NewReader(file)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == configName {
			var cfg struct {
				RootFS struct {
					DiffIDs []string `json:"diff_ids"`
				} `json:"rootfs"`
			}
			if err := json.NewDecoder(tr).Decode(&cfg); err != nil {
				return nil, err
			}
			return cfg.RootFS.DiffIDs, nil
		}
	}

	return nil, fmt.Errorf("config file not found inside tarball")
}
