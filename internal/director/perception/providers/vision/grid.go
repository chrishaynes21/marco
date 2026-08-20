package vision

import (
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Grid semantics.
//
//	Introduce a reusable observation type: SemanticGrid.
//	Grid semantics should not mention games. Games merely contribute interpreters.
//
// There is no game word in this file, and that is not restraint — it is the whole design.
// What a detector produces is a scatter of similar boxes. What a person sees is a grid: a
// regular arrangement with rows, columns and cells that keep their positions. That
// arrangement is a fact about GEOMETRY, and it is the same fact whether the cells hold
// inventory items, calendar days, colour swatches, spreadsheet values or seats in a
// theatre.
//
// So this finds grids, and what a grid MEANS is somebody else's question. A capability
// pack reads "a 6×6 grid whose cells are at these positions" out of the World State and
// says "that is the inventory". It never sees a pixel.
//
// # Why a grid is worth finding at all
//
//	Vision observations must contribute stable identity hints. Never coordinates alone.
//
// This is the answer to that. A detected box has no label, no automation id and nothing
// durable about it — its only property is where it is, and where it is changes when the
// window moves. A grid POSITION is durable: cell (3,4) of the grid is cell (3,4) in the
// next frame, in the next window position, at a different DPI. It is the closest thing to
// an identity that pixels can offer, and without it every vision element would be a new
// element every cycle.

// Grid is a regular arrangement of cells found in one frame.
type Grid struct {
	// ID identifies the grid within its frame.
	ID string
	// Bounds is the whole arrangement, in desktop coordinates.
	Bounds directorapi.Rect
	// Rows and Columns are its shape.
	Rows, Columns int
	// Cells are the members, in reading order: left to right, top to bottom.
	Cells []Cell
	// CellWidth and CellHeight are the typical cell size, for the diagnostic.
	CellWidth, CellHeight int
	// Confidence is how regular the arrangement is, 0..1 — see regularity.
	Confidence float64
}

// Cell is one member of a grid.
type Cell struct {
	// Row and Column are 1-based positions within the grid.
	Row, Column int
	// Index is the reading-order position, 1-based. What a request means by "the
	// fourth one".
	Index int
	// Bounds is where it is, in desktop coordinates.
	Bounds directorapi.Rect
	// Detection is the index of the accepted detection this cell came from, so its
	// observation can be annotated with the position.
	Detection int
}

// GridSummary is a grid as a diagnostic reports it.
//
// No cell list: a diagnostic saying "a 6×6 grid of 48×48 cells" is what a reader wants,
// and thirty-six rectangles is what they do not.
type GridSummary struct {
	ID         string           `json:"id"`
	Bounds     directorapi.Rect `json:"bounds"`
	Rows       int              `json:"rows"`
	Columns    int              `json:"columns"`
	Cells      int              `json:"cells"`
	CellSize   string           `json:"cell_size,omitempty"`
	Confidence float64          `json:"confidence"`
}

// Summarise renders a grid for a diagnostic.
func (g Grid) Summarise() GridSummary {
	return GridSummary{
		ID: g.ID, Bounds: g.Bounds, Rows: g.Rows, Columns: g.Columns,
		Cells:      len(g.Cells),
		CellSize:   fmt.Sprintf("%dx%d", g.CellWidth, g.CellHeight),
		Confidence: g.Confidence,
	}
}

// Grid-detection thresholds. Provisional, like everything else here.
const (
	// MinGridCells is the fewest boxes that can be a grid.
	//
	// Four, arranged in at least two rows and two columns. Three boxes in a line are a
	// row, and calling a row a grid would give every toolbar a spurious geometry.
	MinGridCells = 4
	// MinGridRows and MinGridColumns are the shape floor.
	MinGridRows    = 2
	MinGridColumns = 2
	// sizeTolerance is how much cell sizes may vary and still be one grid, as a
	// fraction. Cells of visibly different size are different things that happen to be
	// near each other.
	sizeTolerance = 0.25
	// alignTolerance is how far a cell may sit from its row or column line, as a
	// fraction of the cell size. Rendering and rounding move things by a pixel or two.
	alignTolerance = 0.4
	// MinGridConfidence is how regular an arrangement must be to be reported.
	MinGridConfidence = 0.6
	// pitchTolerance is how much the gap between successive rows or columns may vary
	// from the pitch the run has established, as a fraction.
	pitchTolerance = 0.25
	// maxPitchFactor is the largest gap between successive rows or columns that can
	// still be one arrangement, as a multiple of the cell size. Cells of a grid are
	// adjacent; a gap several cells wide is the space between two different things.
	maxPitchFactor = 3.0
)

// placed is one accepted detection, ready for the geometry pass.
type placed struct {
	detection Detection
	class     Class
	// box is where it is in DESKTOP coordinates; image is the same box in image-local
	// ones, kept because the diagnostic reports it.
	box   directorapi.Rect
	image image.Rectangle
}

// gridsOver finds the regular arrangements among the accepted detections.
//
// Returns the grids and, for each accepted detection that is a cell, the attributes to
// merge into its observation. The CALLER does the merging, before the observation exists —
// see the ordering note in Provider.observations for why a grid position must be a
// property of a cell rather than a second observation at the same place.
//
// # Every structural detection is a candidate, whatever it is called
//
// An earlier version considered only detections classified `slot`, and it was wrong in a
// way that no unit test could show: it made grid inference depend on a detector emitting
// one particular word. The first real model tried — Ultralytics YOLO11m via OmniParser —
// has exactly ONE class, `icon`, so `slots` was always empty and this function returned on
// its first line, on every screen, forever. A desktop with two tidy rows of icons produced
// no grid, and the diagnostic could only report that it had found none.
//
// "Slot" is an inventory word, and no general UI detector emits it. A grid is a fact about
// repeated geometry, so candidacy is geometric: any structural class may form one, and
// detectGrids keeps the kinds apart.
//
// It cannot fail. An arrangement that is not regular enough is simply not a grid, and its
// boxes remain the perfectly good observations they were going to be.
func (p *Provider) gridsOver(accepted []placed, slots []int, frame Frame) (
	[]Grid, map[int]map[string]any) {

	if len(slots) < MinGridCells {
		return nil, nil
	}
	// The geometry pass works over boxes; the index rides along so a cell can be traced
	// back to the detection it came from, and the class so kinds are not blended.
	cells := make([]indexed, 0, len(slots))
	for _, i := range slots {
		cells = append(cells, indexed{
			box: accepted[i].box, class: accepted[i].class, index: i,
		})
	}

	found := detectGrids(cells)
	if len(found) == 0 {
		return nil, nil
	}

	positions := map[int]map[string]any{}
	for i := range found {
		found[i].ID = fmt.Sprintf("%s:grid:%d", frame.ID, i+1)
		g := found[i]
		for _, c := range g.Cells {
			positions[c.Detection] = map[string]any{
				"grid_id":      g.ID,
				"grid_row":     c.Row,
				"grid_column":  c.Column,
				"grid_index":   c.Index,
				"grid_rows":    g.Rows,
				"grid_columns": g.Columns,
			}
		}
	}
	return found, positions
}

// gridElements is one element per grid — the arrangement itself.
//
// A grouping, not a control: it is a region containing things, and RoleGroup is what the
// Director's vocabulary calls one. Its bounds enclose every cell, so it is genuinely a
// second OBJECT rather than a second account of one — which is what makes it safe to emit
// from the same source as its cells.
func (p *Provider) gridElements(grids []Grid, img capture.Image,
	window directorapi.Window, frame Frame) []observation.Observation {

	out := make([]observation.Observation, 0, len(grids))
	for _, g := range grids {
		out = append(out, observation.NewElement(directorapi.Observation{
			ID:         mintID(),
			Kind:       directorapi.ObservationElement,
			Source:     directorapi.SourceVision,
			Timestamp:  img.CapturedAt,
			WindowID:   window.ID,
			Role:       directorapi.RoleGroup,
			Bounds:     g.Bounds,
			Confidence: g.Confidence,
			Attributes: map[string]any{
				"vision_class": "grid",
				"provider":     p.Name(),
				"frame":        string(frame.ID),
				"grid_id":      g.ID,
				"grid_rows":    g.Rows,
				"grid_columns": g.Columns,
				"grid_cells":   len(g.Cells),
			},
		}))
	}
	return out
}

// summarise renders grids for a diagnostic.
func summarise(grids []Grid) []GridSummary {
	if len(grids) == 0 {
		return nil
	}
	out := make([]GridSummary, 0, len(grids))
	for _, g := range grids {
		out = append(out, g.Summarise())
	}
	return out
}

// indexed is a box with the accepted-detection index it came from, and the class the
// detector gave it.
//
// The class rides along so arrangements are found WITHIN a kind. A toolbar of same-sized
// buttons sitting above a grid of same-sized icons is two things, and size alone cannot
// tell them apart.
type indexed struct {
	box   directorapi.Rect
	class Class
	index int
}

// detectGrids groups similar boxes into regular arrangements.
//
// Three passes, in this order for a reason:
//
//  1. Group by CLASS. A grid is an arrangement of things of one kind.
//  2. Group by SIZE. Cells of one grid are the same size as each other; a 48×48 inventory
//     square and a 200×24 list row are not members of one arrangement however neatly they
//     line up.
//  3. Within a size group, find ROWS and COLUMNS by clustering the box centres. A grid is
//     then the cross product, and how well the boxes fill it is the confidence.
//
// Deliberately not a general clustering algorithm. This finds regular arrangements and
// nothing else — an irregular scatter produces no grid, which is the correct answer.
func detectGrids(slots []indexed) []Grid {
	var out []Grid
	for _, kind := range byClass(slots) {
		for _, group := range bySize(kind) {
			if len(group) < MinGridCells {
				continue
			}
			out = append(out, arrangeAll(group)...)

		}
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Cells) > len(out[j].Cells) })
	return out
}

// byClass partitions boxes by the class the detector gave them, in a stable order.
func byClass(slots []indexed) [][]indexed {
	var order []Class
	groups := map[Class][]indexed{}
	for _, s := range slots {
		if _, seen := groups[s.class]; !seen {
			order = append(order, s.class)
		}
		groups[s.class] = append(groups[s.class], s)
	}
	out := make([][]indexed, 0, len(order))
	for _, c := range order {
		out = append(out, groups[c])
	}
	return out
}

// bySize groups boxes whose dimensions are within tolerance of each other.
// It sorts by area first, and that is not tidiness.
//
// Comparing every candidate against whichever box happened to come first is single-linkage
// clustering, and it CHAINS: with a 25% tolerance, a 40px box admits a 50px one, which
// admits a 62px one, and a group ends up spanning sizes no two of which are alike. On the
// first live frame that swallowed two rows of 45×47 desktop icons into a group seeded by a
// 58×50 toolbar icon, and the arrangement that came out was correctly refused as irregular.
//
// Sorting by area makes the seed the SMALLEST member, so a group spans at most one
// tolerance from its smallest box. The bound is real and the result no longer depends on
// the order the detector happened to report things in.
func bySize(slots []indexed) [][]indexed {
	ordered := make([]indexed, len(slots))
	copy(ordered, slots)
	sort.SliceStable(ordered, func(i, j int) bool {
		return area(ordered[i].box) < area(ordered[j].box)
	})

	var groups [][]indexed
	used := make([]bool, len(ordered))

	for i := range ordered {
		if used[i] {
			continue
		}
		group := []indexed{ordered[i]}
		used[i] = true
		for j := i + 1; j < len(ordered); j++ {
			if used[j] {
				continue
			}
			if similarSize(ordered[i].box, ordered[j].box) {
				group = append(group, ordered[j])
				used[j] = true
			}
		}
		groups = append(groups, group)
	}
	return groups
}

// area is a box's pixel area, for ordering.
func area(r directorapi.Rect) int { return r.Width * r.Height }

// similarSize reports whether two boxes are the same size within tolerance.
func similarSize(a, b directorapi.Rect) bool {
	return within(a.Width, b.Width, sizeTolerance) && within(a.Height, b.Height, sizeTolerance)
}

// within reports whether two lengths are within a fraction of each other.
func within(a, b int, tolerance float64) bool {
	if a == 0 || b == 0 {
		return a == b
	}
	larger, smaller := a, b
	if smaller > larger {
		larger, smaller = smaller, larger
	}
	return float64(larger-smaller)/float64(larger) <= tolerance
}

// arrangeAll turns a size-consistent group into however many grids it holds.
//
// # Why one size group is not one grid
//
// The size family "about 45×47" on a real screen is not one arrangement. It was, on the
// first live frame, two rows of desktop icons AND a band of smaller icons 240px below AND a
// taskbar run below that. Treating them as a single candidate produced a sprawling
// rows×columns cross product that the boxes filled barely half of, and the fill test
// correctly refused it — reporting no grid on a screen that plainly had one.
//
// The missing evidence is SPACING. A grid's rows are evenly pitched and adjacent; a
// 240px jump between 47px cells is not a row pitch, it is a different part of the screen.
// So the lines are split into runs of consistent, plausible spacing, and each run is a
// candidate in its own right.
//
// Columns are clustered WITHIN a row run rather than across the whole group, because two
// unrelated arrangements at different heights have unrelated column positions, and mixing
// them invents columns that neither one has.
//
// Every box lands in exactly one row run and, within it, one column run — so the grids
// returned here partition the group and no detection is a cell of two of them.
func arrangeAll(group []indexed) []Grid {
	cellW, cellH := typicalSize(group)
	if cellW <= 0 || cellH <= 0 {
		return nil
	}

	rows := cluster(centresY(group), float64(cellH)*alignTolerance)
	if len(rows) < MinGridRows {
		return nil
	}

	var out []Grid
	for _, rowRun := range runsOf(rows, cellH) {
		if len(rowRun) < MinGridRows {
			continue
		}
		band := membersOf(group, rowRun, float64(cellH)*alignTolerance, centreY)
		if len(band) < MinGridCells {
			continue
		}
		cols := cluster(centresX(band), float64(cellW)*alignTolerance)
		for _, colRun := range runsOf(cols, cellW) {
			if len(colRun) < MinGridColumns {
				continue
			}
			cellsIn := membersOf(band, colRun, float64(cellW)*alignTolerance, centreX)
			if g, ok := arrange(cellsIn, rowRun, colRun, cellW, cellH); ok {
				out = append(out, g)
			}
		}
	}
	return out
}

// runsOf splits clustered line positions into maximal runs of regular spacing.
//
// A run breaks when the next gap is implausible for a cell of this size, or when it
// disagrees with the pitch the run has established. Both conditions are the same idea from
// two directions: cells of one grid are adjacent and evenly spaced.
func runsOf(lines []float64, cell int) [][]float64 {
	if len(lines) == 0 {
		return nil
	}
	maxGap := float64(cell) * maxPitchFactor

	var out [][]float64
	current := []float64{lines[0]}
	var gaps []float64

	for i := 1; i < len(lines); i++ {
		gap := lines[i] - lines[i-1]
		joins := gap <= maxGap
		if joins && len(gaps) > 0 {
			joins = withinF(gap, medianOf(gaps), pitchTolerance)
		}
		if joins {
			current = append(current, lines[i])
			gaps = append(gaps, gap)
			continue
		}
		out = append(out, current)
		current = []float64{lines[i]}
		gaps = nil
	}
	return append(out, current)
}

// membersOf keeps the boxes whose centre sits ON one of the given lines.
//
// Proximity, not nearest. `nearest` always answers with something — it has no notion of
// too far — so asking it which of a two-line run a box belongs to puts a box 500px below
// the run into it, and the run's band then contains the whole size group. Membership of a
// row is a question with a "no".
func membersOf(group []indexed, lines []float64, tolerance float64,
	centre func(directorapi.Rect) float64) []indexed {

	out := make([]indexed, 0, len(group))
	for _, s := range group {
		c := centre(s.box)
		for _, line := range lines {
			if math.Abs(c-line) <= tolerance {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// medianOf is the middle value of a non-empty slice.
func medianOf(xs []float64) float64 {
	s := make([]float64, len(xs))
	copy(s, xs)
	sort.Float64s(s)
	return s[len(s)/2]
}

// withinF reports whether two lengths are within a fraction of each other.
func withinF(a, b, tolerance float64) bool {
	if a <= 0 || b <= 0 {
		return a == b
	}
	larger, smaller := a, b
	if smaller > larger {
		larger, smaller = smaller, larger
	}
	return (larger-smaller)/larger <= tolerance
}

// arrange builds one grid from the boxes of a row run crossed with a column run.
func arrange(group []indexed, rows, cols []float64, cellW, cellH int) (Grid, bool) {
	if len(rows) < MinGridRows || len(cols) < MinGridColumns {
		return Grid{}, false
	}

	g := Grid{
		Rows: len(rows), Columns: len(cols),
		CellWidth: cellW, CellHeight: cellH,
	}
	for _, s := range group {
		cx, cy := centreX(s.box), centreY(s.box)
		col := nearest(cols, cx)
		row := nearest(rows, cy)
		if row < 0 || col < 0 {
			continue
		}
		g.Cells = append(g.Cells, Cell{
			Row: row + 1, Column: col + 1,
			Bounds: s.box, Detection: s.index,
		})
	}
	if len(g.Cells) < MinGridCells {
		return Grid{}, false
	}

	// Reading order, and the index that follows from it. Sorted rather than assumed,
	// because the detector's output order is its own business and a request that means
	// "the fourth slot" means the fourth in reading order.
	sort.SliceStable(g.Cells, func(i, j int) bool {
		if g.Cells[i].Row != g.Cells[j].Row {
			return g.Cells[i].Row < g.Cells[j].Row
		}
		return g.Cells[i].Column < g.Cells[j].Column
	})
	for i := range g.Cells {
		g.Cells[i].Index = i + 1
	}

	g.Bounds = enclosing(g.Cells)
	g.Confidence = regularity(g)
	if g.Confidence < MinGridConfidence {
		return Grid{}, false
	}
	return g, true
}

// regularity is how completely the cells fill the rows × columns they imply.
//
// A 6×6 arrangement with 36 cells is a grid; the same rows and columns with 9 cells
// scattered through them is a coincidence, and this is what tells them apart. It is also
// the CONFIDENCE the grid observation carries, so fusion weighs a ragged arrangement less
// than a full one.
func regularity(g Grid) float64 {
	slots := g.Rows * g.Columns
	if slots == 0 {
		return 0
	}
	filled := float64(len(g.Cells)) / float64(slots)
	if filled > 1 {
		// More cells than positions: two boxes landed in one cell, which means the
		// clustering was wrong about this arrangement.
		return 0
	}
	return filled
}

// typicalSize is the median cell size, which is robust to one oversized outlier in a way
// the mean is not.
func typicalSize(group []indexed) (int, int) {
	ws := make([]int, 0, len(group))
	hs := make([]int, 0, len(group))
	for _, s := range group {
		ws = append(ws, s.box.Width)
		hs = append(hs, s.box.Height)
	}
	sort.Ints(ws)
	sort.Ints(hs)
	return ws[len(ws)/2], hs[len(hs)/2]
}

// cluster groups values within tolerance of each other and returns the group centres, in
// ascending order.
func cluster(values []float64, tolerance float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	sort.Float64s(values)
	if tolerance <= 0 {
		tolerance = 1
	}

	var centres []float64
	start := 0
	for i := 1; i <= len(values); i++ {
		if i < len(values) && values[i]-values[i-1] <= tolerance {
			continue
		}
		sum := 0.0
		for _, v := range values[start:i] {
			sum += v
		}
		centres = append(centres, sum/float64(i-start))
		start = i
	}
	return centres
}

// nearest is the index of the closest centre, -1 when there is none.
func nearest(centres []float64, v float64) int {
	best, bestD := -1, 0.0
	for i, c := range centres {
		d := c - v
		if d < 0 {
			d = -d
		}
		if best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func centresX(group []indexed) []float64 {
	out := make([]float64, 0, len(group))
	for _, s := range group {
		out = append(out, centreX(s.box))
	}
	return out
}

func centresY(group []indexed) []float64 {
	out := make([]float64, 0, len(group))
	for _, s := range group {
		out = append(out, centreY(s.box))
	}
	return out
}

func centreX(r directorapi.Rect) float64 { return float64(r.X) + float64(r.Width)/2 }
func centreY(r directorapi.Rect) float64 { return float64(r.Y) + float64(r.Height)/2 }

// enclosing is the rectangle containing every cell.
func enclosing(cells []Cell) directorapi.Rect {
	if len(cells) == 0 {
		return directorapi.Rect{}
	}
	minX, minY := cells[0].Bounds.X, cells[0].Bounds.Y
	maxX := cells[0].Bounds.X + cells[0].Bounds.Width
	maxY := cells[0].Bounds.Y + cells[0].Bounds.Height
	for _, c := range cells[1:] {
		if c.Bounds.X < minX {
			minX = c.Bounds.X
		}
		if c.Bounds.Y < minY {
			minY = c.Bounds.Y
		}
		if r := c.Bounds.X + c.Bounds.Width; r > maxX {
			maxX = r
		}
		if b := c.Bounds.Y + c.Bounds.Height; b > maxY {
			maxY = b
		}
	}
	return directorapi.Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}
