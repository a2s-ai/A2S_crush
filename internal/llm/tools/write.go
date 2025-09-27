package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/fantasy/ai"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
)

//go:embed write.md
var writeDescription []byte

type WriteParams struct {
	FilePath string `json:"file_path" description:"The path to the file to write"`
	Content  string `json:"content" description:"The content to write to the file"`
}

type WritePermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type writeTool struct {
	lspClients  *csync.Map[string, *lsp.Client]
	permissions permission.Service
	files       history.Service
	workingDir  string
}

type WriteResponseMetadata struct {
	Diff      string `json:"diff"`
	Additions int    `json:"additions"`
	Removals  int    `json:"removals"`
}

const WriteToolName = "write"

func NewWriteTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, files history.Service, workingDir string) ai.AgentTool {
	return ai.NewAgentTool(
		WriteToolName,
		string(writeDescription),
		func(ctx context.Context, params WriteParams, call ai.ToolCall) (ai.ToolResponse, error) {
			if params.FilePath == "" {
				return ai.NewTextErrorResponse("file_path is required"), nil
			}

			if params.Content == "" {
				return ai.NewTextErrorResponse("content is required"), nil
			}

			filePath := params.FilePath
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(workingDir, filePath)
			}

			fileInfo, err := os.Stat(filePath)
			if err == nil {
				if fileInfo.IsDir() {
					return ai.NewTextErrorResponse(fmt.Sprintf("Path is a directory, not a file: %s", filePath)), nil
				}

				modTime := fileInfo.ModTime()
				lastRead := getLastReadTime(filePath)
				if modTime.After(lastRead) {
					return ai.NewTextErrorResponse(fmt.Sprintf("File %s has been modified since it was last read.\nLast modification: %s\nLast read: %s\n\nPlease read the file again before modifying it.",
						filePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339))), nil
				}

				oldContent, readErr := os.ReadFile(filePath)
				if readErr == nil && string(oldContent) == params.Content {
					return ai.NewTextErrorResponse(fmt.Sprintf("File %s already contains the exact content. No changes made.", filePath)), nil
				}
			} else if !os.IsNotExist(err) {
				return ai.ToolResponse{}, fmt.Errorf("error checking file: %w", err)
			}

			dir := filepath.Dir(filePath)
			if err = os.MkdirAll(dir, 0o755); err != nil {
				return ai.ToolResponse{}, fmt.Errorf("error creating directory: %w", err)
			}

			oldContent := ""
			if fileInfo != nil && !fileInfo.IsDir() {
				oldBytes, readErr := os.ReadFile(filePath)
				if readErr == nil {
					oldContent = string(oldBytes)
				}
			}

			sessionID, messageID := GetContextValues(ctx)
			if sessionID == "" || messageID == "" {
				return ai.ToolResponse{}, fmt.Errorf("session_id and message_id are required")
			}

			diff, additions, removals := diff.GenerateDiff(
				oldContent,
				params.Content,
				strings.TrimPrefix(filePath, workingDir),
			)

			p := permissions.Request(
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        fsext.PathOrPrefix(filePath, workingDir),
					ToolCallID:  call.ID,
					ToolName:    WriteToolName,
					Action:      "write",
					Description: fmt.Sprintf("Create file %s", filePath),
					Params: WritePermissionsParams{
						FilePath:   filePath,
						OldContent: oldContent,
						NewContent: params.Content,
					},
				},
			)
			if !p {
				return ai.ToolResponse{}, permission.ErrorPermissionDenied
			}

			err = os.WriteFile(filePath, []byte(params.Content), 0o644)
			if err != nil {
				return ai.ToolResponse{}, fmt.Errorf("error writing file: %w", err)
			}

			// Check if file exists in history
			file, err := files.GetByPathAndSession(ctx, filePath, sessionID)
			if err != nil {
				_, err = files.Create(ctx, sessionID, filePath, oldContent)
				if err != nil {
					// Log error but don't fail the operation
					return ai.ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
				}
			}
			if file.Content != oldContent {
				// User Manually changed the content store an intermediate version
				_, err = files.CreateVersion(ctx, sessionID, filePath, oldContent)
				if err != nil {
					slog.Debug("Error creating file history version", "error", err)
				}
			}
			// Store the new version
			_, err = files.CreateVersion(ctx, sessionID, filePath, params.Content)
			if err != nil {
				slog.Debug("Error creating file history version", "error", err)
			}

			recordFileWrite(filePath)
			recordFileRead(filePath)

			notifyLSPs(ctx, lspClients, params.FilePath)

			result := fmt.Sprintf("File successfully written: %s", filePath)
			result = fmt.Sprintf("<result>\n%s\n</result>", result)
			result += getDiagnostics(filePath, lspClients)
			return ai.WithResponseMetadata(ai.NewTextResponse(result),
				WriteResponseMetadata{
					Diff:      diff,
					Additions: additions,
					Removals:  removals,
				},
			), nil
		})
}
