package tools

import (
	"errors"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

const maxSharedEvents = 10

func findConnectionTool() Tool {
	return Tool{
		Def: newTool("find_connection", "Find how two people are connected",
			`How two people are connected: the shortest chain of family ties, succession, discipleship and companionship between them, one step at a time with the atlas's Spanish phrase and the Bible reference behind each step, plus the events both take part in. In each step, word is the role of the person word_of names: "to" or "from". Use for "how is Rut related to David" or "did Timoteo know Pedro".`,
			idOption("from", `Person id from search, for example "rut".`, mcp.Required()),
			idOption("to", `Another person id from search, for example "david".`, mcp.Required()),
			mcp.WithInteger("max_steps", mcp.Description("Longest chain to look for, in relation steps."), mcp.DefaultNumber(8), mcp.Min(1), mcp.Max(12)),
			outputSchema(object(schema{
				"from":      refSchema,
				"to":        refSchema,
				"connected": boolS,
				"steps": arrayOf(object(schema{
					"from": refSchema, "to": refSchema, "phrase": strS, "type": strS, "word": strS,
					"word_of":   schema{"type": "string", "enum": []string{"from", "to"}},
					"reference": strS, "status": strS, "sources": arrayOf(sourceSchema),
				}, "from", "to")),
				"shared_events":       arrayOf(itemSchema(nil)),
				"shared_events_total": intS,
				"dataset":             strS,
			}, "from", "to", "connected", "shared_events_total")),
		),
		Handle: findConnection,
	}
}

func findConnection(c *Call) (*obj, error) {
	s := c.Snap
	from, _, err := recordID(s, c.Args, "from", atlas.TypePerson, true)
	if err != nil {
		return nil, err
	}
	to, _, err := recordID(s, c.Args, "to", atlas.TypePerson, true)
	if err != nil {
		return nil, err
	}
	if from == to {
		return nil, errors.New("from and to must be two different people")
	}
	maxSteps, _, err := wholeNumber(c.Args, "max_steps", 1, 12, 8)
	if err != nil {
		return nil, err
	}

	// Breadth first; neighbours in ascending relation key, so the same
	// snapshot always gives the same path.
	type prev struct {
		from string
		edge atlas.RelEdge
	}
	back := map[string]prev{from: {}}
	depth := map[string]int{from: 0}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == to || depth[cur] >= maxSteps {
			continue
		}
		for _, e := range s.Adjacent[cur] {
			if _, seen := back[e.Other]; seen {
				continue
			}
			back[e.Other] = prev{from: cur, edge: e}
			depth[e.Other] = depth[cur] + 1
			queue = append(queue, e.Other)
		}
	}
	out := newObj().keep("from", ref(s, atlas.TypePerson, from)).keep("to", ref(s, atlas.TypePerson, to))
	if _, ok := back[to]; ok {
		var steps []*obj
		for cur := to; cur != from; cur = back[cur].from {
			p := back[cur]
			step := newObj().keep("from", ref(s, atlas.TypePerson, p.from)).keep("to", ref(s, atlas.TypePerson, cur))
			// The edge is listed for p.from: word describes "to" when the
			// relation is stored on p.from, and "from" otherwise.
			steps = append([]*obj{relationObj(s, p.edge, step, "to", "from", 0)}, steps...)
		}
		out.keep("connected", true).keep("steps", steps)
	} else {
		out.keep("connected", false)
	}

	inTo := map[int]bool{}
	for _, i := range s.EventsByPerson[to] {
		inTo[i] = true
	}
	var shared []*obj
	total := 0
	for _, i := range s.EventsByPerson[from] {
		if !inTo[i] {
			continue
		}
		total++
		if len(shared) < maxSharedEvents {
			shared = append(shared, itemOf(s, atlas.TypeEvent, s.File.Events[i].ID))
		}
	}
	out.set("shared_events", shared).keep("shared_events_total", total)
	return out.set("dataset", s.Generated), nil
}
