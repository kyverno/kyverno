package cmd

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/internal/lsp"
	"golang.org/x/tools/internal/lsp/cache"
	"golang.org/x/tools/internal/lsp/protocol"
	"golang.org/x/tools/internal/span"
	errors "golang.org/x/xerrors"
)

// TestCapabilities does some minimal validation of the server's adherence to the LSP.
// The checks in the test are added as changes are made and errors noticed.
func TestCapabilities(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fake")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile := filepath.Join(tmpDir, "fake.go")
	if err := ioutil.WriteFile(tmpFile, []byte(""), 0775); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(`module fake`), 0775); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	app := New("gopls-test", tmpDir, os.Environ(), nil)
	c := newConnection(app)
	ctx := context.Background()
	defer c.terminate(ctx)

	params := &protocol.ParamInitialize{}
	params.RootURI = string(span.FileURI(c.Client.app.wd))
	params.Capabilities.Workspace.Configuration = true

	// Send an initialize request to the server.
	ctx, c.Server = lsp.NewClientServer(ctx, cache.New(app.options), c.Client)
	result, err := c.Server.Initialize(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	// Validate initialization result.
	if err := validateCapabilities(result); err != nil {
		t.Error(err)
	}
	// Complete initialization of server.
	if err := c.Server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatal(err)
	}

	// Open the file on the server side.
	uri := protocol.NewURI(span.FileURI(tmpFile))
	if err := c.Server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri,
			LanguageID: "go",
			Version:    1,
			Text:       `package main; func main() {};`,
		},
	}); err != nil {
		t.Fatal(err)
	}

	// If we are sending a full text change, the change.Range must be nil.
	// It is not enough for the Change to be empty, as that is ambiguous.
	if err := c.Server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: uri,
			},
			Version: 2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Range: nil,
				Text:  `package main; func main() { fmt.Println("") }`,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Send a code action request to validate expected types.
	actions, err := c.Server.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: uri,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range actions {
		// Validate that an empty command is sent along with import organization responses.
		if action.Kind == protocol.SourceOrganizeImports && action.Command != nil {
			t.Errorf("unexpected command for import organization")
		}
	}

	if err := c.Server.DidSave(ctx, &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			Version: 2,
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: uri,
			},
		},
		// LSP specifies that a file can be saved with optional text, so this field must be nil.
		Text: nil,
	}); err != nil {
		t.Fatal(err)
	}
}

func validateCapabilities(result *protocol.InitializeResult) error {
	// If the client sends "false" for RenameProvider.PrepareSupport,
	// the server must respond with a boolean.
	if v, ok := result.Capabilities.RenameProvider.(bool); !ok {
		return errors.Errorf("RenameProvider must be a boolean if PrepareSupport is false (got %T)", v)
	}
	// The same goes for CodeActionKind.ValueSet.
	if v, ok := result.Capabilities.CodeActionProvider.(bool); !ok {
		return errors.Errorf("CodeActionSupport must be a boolean if CodeActionKind.ValueSet has length 0 (got %T)", v)
	}
	return nil
}
