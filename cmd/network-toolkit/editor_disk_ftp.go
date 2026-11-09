package main

import (
	"context"
	"errors"
	"strings"

	"networktoolkit/internal/model"
)

// FTP connections are reconstructed by the existing client for each operation.
// Keep only the endpoint identity on the document. The password and any
// credential authorization are supplied afresh by the owning main renderer.
type editorFTPEndpoint struct {
	host, security, username, fingerprint, credentialID string
	port                                                int
}

func diskFTPConnection(details *remoteBrowserRequest) (remoteBrowserRequest, editorFTPEndpoint, error) {
	if details == nil || details.Protocol != "ftp" || details.Host == "" ||
		len(details.Host) > 253 || len(details.Password) > 4096 || len(details.Username) > 4096 ||
		len(details.Fingerprint) > 256 || len(details.CredentialID) > 128 ||
		details.SessionID != "" || details.ConnectionID != "" || details.KeyPath != "" ||
		details.PrivateKey != "" || details.Passphrase != "" || details.Content != "" {
		return remoteBrowserRequest{}, editorFTPEndpoint{}, errors.New("invalid FTP editor connection")
	}
	r := *details
	r.Host = strings.TrimSpace(r.Host)
	if r.Security == "" {
		r.Security = "explicit"
	}
	if r.Port == 0 {
		r.Port = 21
		if r.Security == "implicit" {
			r.Port = 990
		}
	}
	if r.Port < 1 || r.Port > 65535 || r.Security != "plain" && r.Security != "explicit" && r.Security != "implicit" {
		return remoteBrowserRequest{}, editorFTPEndpoint{}, errors.New("invalid FTP endpoint settings")
	}
	endpoint := editorFTPEndpoint{r.Host, r.Security, r.Username, r.Fingerprint, r.CredentialID, r.Port}
	return r, endpoint, nil
}

func (m *editorViewRegistry) diskRemoteOperation(ctx context.Context, g *editorDocumentGrant, d *editorDiskDocument, details *remoteBrowserRequest, operation remoteBrowserRequest) (model.Result, error) {
	if d.ftp == nil {
		if g.valid == nil || !g.valid() {
			return model.Result{}, errEditorLease
		}
		return g.run(ctx, operation)
	}
	r, endpoint, err := diskFTPConnection(details)
	if err != nil {
		return model.Result{}, err
	}
	if endpoint != *d.ftp {
		return model.Result{}, errors.New("the FTP connection changed; reopen this file before editing it")
	}
	r.Action, r.Path, r.LocalPath = operation.Action, operation.Path, operation.LocalPath
	r.Overwrite, r.StagedUpload = operation.Overwrite, false
	return m.app.runRemoteBrowserOperation(ctx, r)
}
