package harvestpy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// FeatureStatus keeps supported PDF capabilities visible to installer and
// doctor callers. Their activation defaults remain exactly as in old Harvester.
type FeatureStatus struct {
	OCR    string `json:"ocr"`
	Layout string `json:"layout"`
	Models string `json:"models"`
}

// EnvironmentDigest is the machine-readable convergence/smoke record written
// only after an environment has passed its no-download import smoke.
type EnvironmentDigest struct {
	Schema          int            `json:"schema"`
	Target          string         `json:"target"`
	Python          string         `json:"python"`
	UV              string         `json:"uv"`
	PythonSHA256    string         `json:"python_sha256"`
	UVSHA256        string         `json:"uv_sha256"`
	LockSHA256      string         `json:"lock_sha256"`
	SourceSHA256    string         `json:"source_sha256"`
	InventorySHA256 string         `json:"inventory_sha256,omitempty"`
	InventoryCount  int            `json:"inventory_count,omitempty"`
	Environment     string         `json:"environment,omitempty"`
	Digest          string         `json:"digest,omitempty"`
	State           string         `json:"state,omitempty"`
	Features        FeatureStatus  `json:"features"`
	Sizes           SizeReport     `json:"sizes,omitempty"`
	Imports         map[string]any `json:"imports,omitempty"`
}

// SizeReport records measured bytes rather than estimates.  Download and
// install are separate because archive compression can kill the shipping cost.
type SizeReport struct {
	UVDownloadBytes     int64 `json:"uv_download_bytes,omitempty"`
	PythonDownloadBytes int64 `json:"python_download_bytes,omitempty"`
	EnvironmentBytes    int64 `json:"environment_bytes,omitempty"`
	EnvironmentFiles    int64 `json:"environment_files,omitempty"`
	InstallerBytes      int64 `json:"installer_bytes,omitempty"`
}

// ReadEnvironmentDigest parses a digest and refuses an unknown schema.
func ReadEnvironmentDigest(path string) (EnvironmentDigest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return EnvironmentDigest{}, fmt.Errorf("read harvestpy environment digest %s: %w", path, err)
	}
	var digest EnvironmentDigest
	if err := json.Unmarshal(body, &digest); err != nil {
		return EnvironmentDigest{}, fmt.Errorf("decode harvestpy environment digest %s: %w", path, err)
	}
	if digest.Schema != 1 {
		return EnvironmentDigest{}, fmt.Errorf("unsupported harvestpy environment digest schema %d", digest.Schema)
	}
	return digest, nil
}

func lockSHA256() string {
	sum := sha256.Sum256(LockMetadata())
	return hex.EncodeToString(sum[:])
}

func sourceSHA256() string {
	sum := sha256.Sum256(ConverterSource())
	return hex.EncodeToString(sum[:])
}

func digestID(value EnvironmentDigest) string {
	value.Digest = ""
	body, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal harvestpy digest: %v", err))
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
