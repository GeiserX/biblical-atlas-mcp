// Package tools defines the MCP tools. Every tool is read only and answers
// from one atlas snapshot.
package tools

import (
	"context"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Instructions is the orientation the server sends at initialisation.
const Instructions = `This server answers from biblical-atlas, a Spanish Bible atlas (https://biblical-atlas.geiser.cloud/).
Names, summaries and phrases come back in Spanish as the atlas has them. Quote them or translate them yourself; say so when you translate.
Records have a type and an id. Ids repeat across types, so always pass both. Find ids with search.
Years are signed historical years: -607 is 607 a.e.c. (BCE), 33 is 33 e.c. (CE). There is no year 0.
A date with kind "narrativa" is placed only by the order of the story, not by a dated source. Say so when you report it.
status "pendiente" or "pending" means the atlas has not yet confirmed the claim against its source.
Every result includes sources with links. Cite them. The atlas links to Bible chapters, never to single verses.
A place without lat/lon has an unknown site; its candidates list the proposed sites.`

// Call is what a handler gets: one snapshot, the store and the arguments.
type Call struct {
	Snap  *atlas.Snapshot
	Store *atlas.Store
	Args  map[string]any
}

// Tool is one registered tool.
type Tool struct {
	Def    mcp.Tool
	Handle func(c *Call) (*obj, error)
}

// All returns every tool, in the order clients see them.
func All() []Tool {
	return []Tool{
		searchTool(), getRecordTool(), listRecordsTool(), listEventsTool(), peopleInYearTool(),
		lookupPassageTool(), findConnectionTool(), placesNearTool(), datasetInfoTool(),
	}
}

// Register adds every tool to the server.
func Register(s *server.MCPServer, store *atlas.Store) {
	for _, t := range All() {
		s.AddTool(t.Def, handler(t, store))
	}
}

func handler(t Tool, store *atlas.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		snap, err := store.Snapshot(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := req.GetArguments()
		if args == nil {
			args = map[string]any{}
		}
		out, err := t.Handle(&Call{Snap: snap, Store: store, Args: args})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, err := marshal(out)
		if err != nil {
			return mcp.NewToolResultError("could not encode the result: " + err.Error()), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

func boolPtr(b bool) *bool { return &b }

// readOnly sets the title and the four hints every tool carries.
func readOnly(title string) []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithToolTitle(title),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:           title,
			ReadOnlyHint:    boolPtr(true),
			DestructiveHint: boolPtr(false),
			IdempotentHint:  boolPtr(true),
			OpenWorldHint:   boolPtr(false),
		}),
	}
}

func newTool(name, title, description string, opts ...mcp.ToolOption) mcp.Tool {
	all := append([]mcp.ToolOption{mcp.WithDescription(description)}, readOnly(title)...)
	return mcp.NewTool(name, append(all, opts...)...)
}

// maxLimit is the largest page the list tools return. Larger pages pass the
// tool-output limits of common MCP clients.
const maxLimit = 50

func pagingOptions(defLimit, maxLimit int) []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithInteger("limit", mcp.Description("Results per page."), mcp.DefaultNumber(defLimit), mcp.Min(1), mcp.Max(maxLimit)),
		mcp.WithInteger("offset", mcp.Description("Results to skip; use next_offset from the previous page."), mcp.DefaultNumber(0), mcp.Min(0)),
	}
}
