package api

// ExecRequest is sent by the GUI to run a named command.
type ExecRequest struct {
	Cmd      string `json:"cmd"`
	Password string `json:"password,omitempty"`
}

// StatusResponse describes the current VM + service state.
type StatusResponse struct {
	Exists  bool   `json:"exists"`
	Running bool   `json:"running"`
	VMState string `json:"vmState"`
	IP      string `json:"ip"`
	Busy    bool   `json:"busy"`
}

// SSEEvent is the payload of each server-sent event line.
type SSEEvent struct {
	Type  string `json:"type"`  // "output" | "done"
	Line  string `json:"line,omitempty"`
	Error string `json:"error,omitempty"`
}

// VersionResponse is returned by GET /api/version.
type VersionResponse struct {
	Version string `json:"version"`
}

// UpdateCheckResponse is returned by GET /api/update/check.
type UpdateCheckResponse struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	HasUpdate bool   `json:"hasUpdate"`
	SvcURL    string `json:"svcUrl,omitempty"`
	GUIURL    string `json:"guiUrl,omitempty"`
	Error     string `json:"error,omitempty"`
}
