package transfer

// send JSON metadata before the actual file bytes
type TransferRequest struct {
	ImageName string   `json:"image"`
	Size      int64    `json:"size"`
	Hash      string   `json:"hash"`
	Author    string   `json:"author"`
	ImageArch string   `json:"image_arch"`
	Layers    []string `json:"layers"`
	IsControl bool     `json:"is_control,omitempty"`
	Action    string   `json:"action,omitempty"`
	Initiator string   `json:"initiator,omitempty"`
}

type TransferResponse struct {
	Approved      bool     `json:"approved"`
	MissingLayers []string `json:"missing_layers"`
}

type StreamHeader struct {
	PrunedHash string `json:"pruned_hash"`
	PrunedSize int64  `json:"pruned_size"`
}

// passes the metadata and provides a channel
type ApprovalRequest struct {
	Req      TransferRequest
	Response chan bool
}
