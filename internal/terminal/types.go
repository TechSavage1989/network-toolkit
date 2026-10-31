// Package terminal manages explicitly requested interactive device sessions.
// It never persists terminal output, passwords, private keys, or prompt answers.
package terminal

import (
	"context"
	"time"
)

type ConnectRequest struct {
	// DisplayName is resolved by the authenticated application from saved metadata.
	DisplayName         string `json:"-"`
	DisplayNameExplicit bool   `json:"-"`
	Type                string `json:"type"`
	WorkingDirectory    string `json:"workingDirectory,omitempty"`
	RunAsAdministrator  bool   `json:"runAsAdministrator,omitempty"`
	ProfileID           string `json:"profileId,omitempty"`
	CredentialID        string `json:"credentialId,omitempty"`
	LineEnding          string `json:"lineEnding,omitempty"`
	SSHCompatibility    string `json:"sshCompatibility,omitempty"`
	Host                string `json:"host,omitempty"`
	Port                int    `json:"port,omitempty"`
	Username            string `json:"username,omitempty"`
	AuthMethod          string `json:"authMethod,omitempty"`
	Password            string `json:"password,omitempty"`
	PrivateKey          string `json:"privateKey,omitempty"`
	Passphrase          string `json:"passphrase,omitempty"`
	Cols                int    `json:"cols,omitempty"`
	Rows                int    `json:"rows,omitempty"`
	Terminal            string `json:"terminal,omitempty"`
	TimeoutMs           int    `json:"timeoutMs,omitempty"`
	SerialPort          string `json:"serialPort,omitempty"`
	BaudRate            int    `json:"baudRate,omitempty"`
	DataBits            int    `json:"dataBits,omitempty"`
	StopBits            string `json:"stopBits,omitempty"`
	Parity              string `json:"parity,omitempty"`
	FlowControl         string `json:"flowControl,omitempty"`
	SerialDTR           *bool  `json:"dtr,omitempty"`
	SerialRTS           *bool  `json:"rts,omitempty"`

	// ExpectedHostFingerprint is an authenticated backend trust decision, never
	// a client-supplied pin. Saved credentials use it before user authentication.
	ExpectedHostFingerprint string `json:"-"`
}

type SessionInfo struct {
	ID               string  `json:"id"`
	WorkingDirectory string  `json:"workingDirectory,omitempty"`
	Elevated         bool    `json:"elevated,omitempty"`
	ProfileID        string  `json:"profileId,omitempty"`
	LineEnding       string  `json:"lineEnding,omitempty"`
	Type             string  `json:"type"`
	Label            string  `json:"label"`
	ManualName       string  `json:"manualName,omitempty"`
	NameRevision     uint64  `json:"nameRevision"`
	NameExplicit     bool    `json:"nameExplicit"`
	LabelSource      string  `json:"labelSource,omitempty"`
	Host             string  `json:"host,omitempty"`
	Port             int     `json:"port,omitempty"`
	Username         string  `json:"username,omitempty"`
	SSHCompatibility string  `json:"sshCompatibility,omitempty"`
	SerialPort       string  `json:"serialPort,omitempty"`
	State            string  `json:"state"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	Cols             int     `json:"cols"`
	Rows             int     `json:"rows"`
	BytesReceived    uint64  `json:"bytesReceived"`
	BytesSent        uint64  `json:"bytesSent"`
	Error            string  `json:"error,omitempty"`
	ErrorCode        string  `json:"errorCode,omitempty"`
	Prompt           *Prompt `json:"prompt,omitempty"`
}

type PromptField struct {
	Label string `json:"label"`
	Echo  bool   `json:"echo"`
}

type Prompt struct {
	ID                   string        `json:"id"`
	Kind                 string        `json:"kind"`
	Title                string        `json:"title"`
	Message              string        `json:"message"`
	Host                 string        `json:"host,omitempty"`
	Port                 int           `json:"port,omitempty"`
	Algorithm            string        `json:"algorithm,omitempty"`
	Fingerprint          string        `json:"fingerprint,omitempty"`
	PreviousFingerprints []string      `json:"previousFingerprints,omitempty"`
	Changed              bool          `json:"changed,omitempty"`
	Fields               []PromptField `json:"fields,omitempty"`
	Actions              []string      `json:"actions"`
}

type PromptReply struct {
	PromptID    string   `json:"promptId"`
	Action      string   `json:"action"`
	Answers     []string `json:"answers,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
}

type Event struct {
	Seq       uint64       `json:"seq"`
	SessionID string       `json:"sessionId"`
	Type      string       `json:"type"`
	Time      string       `json:"time"`
	Data      []byte       `json:"data,omitempty"`
	Message   string       `json:"message,omitempty"`
	Session   *SessionInfo `json:"session,omitempty"`
	Prompt    *Prompt      `json:"prompt,omitempty"`
}

type Batch struct {
	Events     []Event       `json:"events"`
	Next       uint64        `json:"next"`
	Oldest     uint64        `json:"oldest"`
	Latest     uint64        `json:"latest"`
	Lost       bool          `json:"lost"`
	LostEvents uint64        `json:"lostEvents"`
	Reset      bool          `json:"reset"`
	Sessions   []SessionInfo `json:"sessions"`
}

type SerialPort struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	HardwareID  string `json:"hardwareId,omitempty"`
}

type ControlRequest struct {
	DTR   *bool `json:"dtr,omitempty"`
	RTS   *bool `json:"rts,omitempty"`
	Break *bool `json:"break,omitempty"`
}

type transport interface {
	Write([]byte) (int, error)
	Resize(cols, rows int) error
	Close() error
	// Done sends one terminal exit result, then closes. Close must unblock I/O.
	Done() <-chan error
}

type serialControl interface {
	SetSignals(dtr, rts *bool) error
	SetBreak(enabled bool) error
}

type transportCallbacks struct {
	// Output may be called concurrently. The callback copies bytes before return.
	Output func([]byte)
	Prompt func(context.Context, Prompt) (PromptReply, error)
}

type transportOpener func(context.Context, ConnectRequest, transportCallbacks) (transport, error)

// CredentialResolver resolves a saved credential for a fresh authentication.
// The input contains only connection settings and a credential ID, never a
// previously used secret. Implementations must authorize the exact destination
// and return the same endpoint, username and credential ID.
type CredentialResolver func(context.Context, ConnectRequest) (ConnectRequest, error)

const (
	SSHCompatibilityModern = "modern"
	SSHCompatibilityLegacy = "legacy"

	maxLiveSessions   = 8
	maxSessionHistory = 64
	maxEventBytes     = 4 * 1024 * 1024
	maxEvents         = 8192
	maxOutputChunk    = 16 * 1024
	maxInputChunk     = 64 * 1024
	maxQueuedInput    = 256 * 1024
	maxBatchBytes     = 256 * 1024
	maxBatchEvents    = 256
	promptTimeout     = 2 * time.Minute
)
