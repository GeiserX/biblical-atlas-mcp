// Package tools defines the MCP tools. Every tool is read only and answers
// from one atlas snapshot.
package tools

import (
	"context"
	"encoding/json"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Instructions is the orientation the server sends at initialisation.
const Instructions = `This server answers from biblical-atlas, a Spanish Bible atlas (https://biblical-atlas.geiser.cloud/).
Names, summaries and phrases come back in Spanish as the atlas has them. Quote them or translate them yourself; say so when you translate.
Records have a type and an id. Ids repeat across types, so always pass both. Find ids with search; read several records in one call with get_records.
Years are signed historical years: -607 is 607 a.e.c. (BCE), 33 is 33 e.c. (CE). There is no year 0.
A date with kind "narrativa" is placed only by the order of the story, not by a dated source. Say so when you report it.
status "pendiente" or "pending" means the atlas has not yet confirmed the claim against its source.
Every result includes sources with links. Cite them. Passage links open the jw.org study Bible at the cited verse.
A place without lat/lon has an unknown site; its candidates list the proposed sites.
Bible references and book filters take Spanish or English book names ("Hch 16:1" or "Acts 16:1").
dataset_info lists the values the kind and role filters accept.`

// Title, Description and WebsiteURL identify the server in server/discover
// and initialize.
const (
	Title       = "Biblical Atlas"
	Description = "A Spanish Bible atlas as read-only tools: people, places, events, passages and their sources."
	WebsiteURL  = "https://github.com/GeiserX/biblical-atlas-mcp"
)

// Icons are the atlas site's own icons.
var Icons = []mcp.Icon{
	{Src: SiteBase + "favicon.svg", MIMEType: "image/svg+xml", Sizes: []string{"any"}},
	{Src: SiteBase + "apple-touch-icon.png", MIMEType: "image/png", Sizes: []string{"180x180"}},
}

// MaxResultBytes caps one tool result, text and structured content
// together. The largest answers on the real atlas are under 100 KB, and
// get_records stops adding records at batchBudget.
const MaxResultBytes = 512 << 10

// listTTL is how long a client may reuse tools/list and server/discover:
// both are fixed for the life of the process.
const listTTL = 60 * 60 * 1000

// ServerOptions are the options every server of this program uses.
func ServerOptions() []server.ServerOption {
	return []server.ServerOption{
		server.WithToolCapabilities(false),
		server.WithRecovery(),
		server.WithInstructions(Instructions),
		server.WithTitle(Title),
		server.WithDescription(Description),
		server.WithWebsiteURL(WebsiteURL),
		server.WithIcons(Icons...),
		// The same for every caller, so shared caches may keep them.
		server.WithCacheHints(listTTL, mcp.CacheScopePublic),
		// Arguments are checked against the input schema before a handler
		// runs, and a name the schema does not list is refused.
		server.WithInputSchemaValidation(),
		server.WithStrictInputSchemaDefault(),
		server.WithToolResultSizeLimit(MaxResultBytes),
	}
}

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

// All returns every tool. tools/list sorts them by name, so this order only
// sets the order of registration.
func All() []Tool {
	return []Tool{
		searchTool(), getRecordTool(), getRecordsTool(), listRecordsTool(), listEventsTool(), peopleInYearTool(),
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
		// The same JSON twice: structuredContent for clients that read it,
		// a text block for those that do not.
		return mcp.NewToolResultStructured(json.RawMessage(b), string(b)), nil
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

// maxOffset is the largest offset any tool accepts.
const maxOffset = 1_000_000

// maxIDLen is the longest id any tool accepts.
const maxIDLen = 200

func pagingOptions(defLimit, maxLimit int) []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithInteger("limit", mcp.Description("Results per page."), mcp.DefaultNumber(defLimit), mcp.Min(1), mcp.Max(maxLimit)),
		offsetOption("offset", "Results to skip; use next_offset from the previous page."),
	}
}

func offsetOption(name, description string) mcp.ToolOption {
	return mcp.WithInteger(name, mcp.Description(description), mcp.DefaultNumber(0), mcp.Min(0), mcp.Max(maxOffset))
}

// idOption is a record id argument.
func idOption(name, description string, opts ...mcp.PropertyOption) mcp.ToolOption {
	return mcp.WithString(name, append([]mcp.PropertyOption{mcp.Description(description), mcp.MinLength(1), mcp.MaxLength(maxIDLen)}, opts...)...)
}
