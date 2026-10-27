package tools

import (
	"fmt"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

// maxBatch is how many records one get_records call asks for.
const maxBatch = 20

// batchBudget caps the encoded size of the records one get_records answer
// carries. The answer goes out twice (text and structured content), so this
// stays well under MaxResultBytes. The first record always comes back.
var batchBudget = 160 << 10

func getRecordsTool() Tool {
	return Tool{
		Def: newTool("get_records", "Get several atlas records",
			`Get up to 20 atlas records in full in one call, each exactly as get_record returns it (first page of relations and sources). Use it after search, list_events, lookup_passage or find_connection to read the records their answers point at, instead of one get_record call per record. Records that fail (an unknown id, say) come back in errors with the reason; once the answer reaches its size budget, the remaining records come back in not_returned, in order, to ask for again.`,
			mcp.WithArray("records", mcp.Required(), mcp.MinItems(1), mcp.MaxItems(maxBatch),
				mcp.Description(`The records to read, 1 to 20, each a type and an id from another answer, for example [{"type":"person","id":"pablo"},{"type":"place","id":"jerusalen"}].`),
				mcp.Items(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type": map[string]any{"type": "string", "enum": typeNames(atlas.Types), "description": "Record type."},
						"id":   map[string]any{"type": "string", "minLength": 1, "maxLength": maxIDLen, "description": "Record id. Case is ignored."},
					},
					"required":             []string{"type", "id"},
					"additionalProperties": false,
				})),
			outputSchema(object(schema{
				"records": arrayOf(recordSchema),
				"errors": arrayOf(object(schema{
					"type": strS, "id": strS, "error": typed("string", "Why this record could not be read."),
				}, "type", "id", "error")),
				"not_returned": typed("array", "Records left out to keep the answer small; ask for them again."),
				"dataset":      strS,
			}, "records")),
		),
		Handle: getRecords,
	}
}

func getRecords(c *Call) (*obj, error) {
	raw, ok := c.Args["records"].([]any)
	if !ok || len(raw) == 0 || len(raw) > maxBatch {
		return nil, fmt.Errorf("records must be a list of 1 to %d {type, id} objects", maxBatch)
	}
	records := []*obj{}
	var errs, rest []*obj
	size := 0
	for _, x := range raw {
		args, isObj := x.(map[string]any)
		if !isObj {
			return nil, fmt.Errorf("records must hold {type, id} objects, got %s", fmtNum(x))
		}
		typ, _ := args["type"].(string)
		id, _ := args["id"].(string)
		if len(rest) > 0 {
			// Once the budget is spent, the rest keep their order.
			rest = append(rest, newObj().keep("type", typ).keep("id", id))
			continue
		}
		o, err := getRecord(&Call{Snap: c.Snap, Store: c.Store, Args: map[string]any{"type": args["type"], "id": args["id"]}})
		if err != nil {
			errs = append(errs, newObj().keep("type", typ).keep("id", id).keep("error", err.Error()))
			continue
		}
		b, err := marshal(o)
		if err != nil {
			return nil, err
		}
		if len(records) > 0 && size+len(b) > batchBudget {
			rest = append(rest, newObj().keep("type", typ).keep("id", id))
			continue
		}
		size += len(b)
		records = append(records, o)
	}
	return newObj().keep("records", records).set("errors", errs).set("not_returned", rest).set("dataset", c.Snap.Generated), nil
}
