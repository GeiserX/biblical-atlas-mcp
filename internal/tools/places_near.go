package tools

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

const earthRadiusKm = 6371.0

func placesNearTool() Tool {
	opts := []mcp.ToolOption{
		mcp.WithString("place", mcp.Description("Place id from search. Give this or lat and lon, not both.")),
		mcp.WithNumber("lat", mcp.Description("Latitude of the origin, -90 to 90."), mcp.Min(-90), mcp.Max(90)),
		mcp.WithNumber("lon", mcp.Description("Longitude of the origin, -180 to 180."), mcp.Min(-180), mcp.Max(180)),
		mcp.WithNumber("radius_km", mcp.Description("Search radius in km, above 0 and at most 2000."), mcp.DefaultNumber(50), mcp.Max(2000)),
		mcp.WithString("kind", mcp.Description(`Only places of this kind, for example "ciudad", "monte", "rio".`)),
	}
	return Tool{
		Def: newTool("places_near", "Places near a place or point",
			`Places in the atlas within a distance of a place or a coordinate, nearest first. Use for "what is near Capernaum" or "which cities lie within 30 km of Jerusalén". Only places with a known point are returned; the count of places without one is reported.`,
			append(opts, pagingOptions(20, maxLimit)...)...),
		Handle: placesNear,
	}
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(h)))
}

func placesNear(c *Call) (*obj, error) {
	s := c.Snap
	place, hasPlace, err := recordID(s, c.Args, "place", atlas.TypePlace, false)
	if err != nil {
		return nil, err
	}
	lat, hasLat, err := number(c.Args, "lat", -90, 90, false)
	if err != nil {
		return nil, err
	}
	lon, hasLon, err := number(c.Args, "lon", -180, 180, false)
	if err != nil {
		return nil, err
	}
	radius, hasRadius, err := number(c.Args, "radius_km", 0, 2000, true)
	if err != nil {
		return nil, err
	}
	if !hasRadius {
		radius = 50
	}
	kind, hasKind, err := str(c.Args, "kind", 60)
	if err != nil {
		return nil, err
	}
	if hasKind {
		if err := oneOf("kind", kind, s.PlaceKinds); err != nil {
			return nil, err
		}
	}
	switch {
	case hasPlace && (hasLat || hasLon):
		return nil, errors.New("give place or lat and lon, not both")
	case !hasPlace && (hasLat != hasLon):
		return nil, errors.New("lat and lon go together: give both")
	case !hasPlace && !hasLat:
		return nil, errors.New("give place, or lat and lon")
	}
	limit, offset, err := paging(c.Args, 20, maxLimit)
	if err != nil {
		return nil, err
	}
	origin := newObj()
	if hasPlace {
		p := s.File.Places[place]
		if !p.HasPoint() {
			return nil, noPointError(p)
		}
		lat, lon = *p.Lat, *p.Lon
		origin.keep("lat", lat).keep("lon", lon).keep("place", ref(s, atlas.TypePlace, place))
	} else {
		origin.keep("lat", lat).keep("lon", lon)
	}

	type near struct {
		rec *atlas.Rec
		p   *atlas.Place
		d   float64
	}
	var found []near
	without := 0
	for _, r := range s.Records(atlas.TypePlace) {
		p := r.Value.(*atlas.Place)
		if !p.HasPoint() {
			without++
			continue
		}
		if r.ID == place || (hasKind && p.Kind != kind) {
			continue
		}
		d := math.Round(haversine(lat, lon, *p.Lat, *p.Lon)*10) / 10
		if d <= radius {
			found = append(found, near{r, p, d})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].d != found[j].d {
			return found[i].d < found[j].d
		}
		return found[i].rec.ID < found[j].rec.ID
	})
	lo, hi := window(len(found), offset, limit)
	var results []*obj
	for _, n := range found[lo:hi] {
		results = append(results, item(s, n.rec).set("kind", n.p.Kind).floatp("lat", n.p.Lat).floatp("lon", n.p.Lon).
			set("precision", n.p.Precision).keep("distance_km", n.d))
	}
	return page(s, len(found), offset, limit, results).keep("origin", origin).keep("radius_km", radius).keep("without_coordinates", without), nil
}

func noPointError(p *atlas.Place) error {
	var cands []string
	for _, c := range p.Candidates {
		if g := c.Geometry; g != nil && g.Lat != nil && g.Lon != nil {
			cands = append(cands, fmt.Sprintf("%s (lat %s, lon %s)", c.Name, fmtNum(*g.Lat), fmtNum(*g.Lon)))
		}
	}
	msg := fmt.Sprintf("place %q has no known point", p.ID)
	if len(cands) > 0 {
		msg += ". Candidate sites: " + strings.Join(cands, "; ") + ". Retry with lat and lon of one of them"
	}
	return errors.New(msg)
}
