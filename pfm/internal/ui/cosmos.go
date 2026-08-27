package ui

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"hostops/pfm/internal/compose"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/shared"
)

type star struct {
	px, py int
	phase  float64
	speed  float64
}

type cosmosPoint struct{ x, y float64 }

type cosmosSampleMsg struct {
	generation uint64
	events     []shared.CommsEvent
	err        error
}

type cosmosSampleTickMsg struct{ generation uint64 }
type cosmosTickMsg struct {
	generation uint64
	nowNS      int64
}

func (model *Model) startCosmosSample() tea.Cmd {
	if model.cosmosSampler == nil || model.cosmosLoading {
		return nil
	}
	model.statsGeneration++
	generation := model.statsGeneration
	model.cosmosLoading = true
	sampler := model.cosmosSampler
	sinceNS := model.nowNS - int64(compose.CosmosWindow)
	return func() tea.Msg {
		events, err := sampler.Sample(context.Background(), sinceNS)
		return cosmosSampleMsg{generation: generation, events: events, err: err}
	}
}

func cosmosWaitCmd(generation uint64) tea.Cmd {
	return tea.Tick(cosmosRefreshInterval, func(time.Time) tea.Msg {
		return cosmosSampleTickMsg{generation: generation}
	})
}

// cosmosTickInterval is the animation clock: ~15fps, fast enough that comets,
// wind, and twinkle read as motion rather than a slideshow. Safe mode drops
// back to 8fps because heavy braille truecolor churn corrupts SIBLING
// terminals through VS Code's shared WebGL glyph atlas
// (microsoft/vscode#332859, xtermjs/xterm.js#6043). Honesty note: no
// app-side output shaping is PROVEN to prevent that corruption — the atlas
// is a shared, cumulative GPU resource one app cannot bound — so safe mode
// only slows the pressure (fewer frames, coarser colours via Canvas.Quant)
// until the upstream atlas-eviction fix (xterm.js 6.1.0) ships in stable
// VS Code; the reliable user-side cure is
// terminal.integrated.gpuAcceleration "off". The mode announces itself in
// the panel title, so the degraded state is never silent.
func (model Model) cosmosTickInterval() time.Duration {
	if model.cosmosSafe {
		return 125 * time.Millisecond
	}
	return 66 * time.Millisecond
}

func cosmosTickCmd(generation uint64, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(now time.Time) tea.Msg {
		return cosmosTickMsg{generation: generation, nowNS: now.UnixNano()}
	})
}

// cosmosPhase derives a stable personal phase from a node or edge key so
// ambient motion never synchronizes across the universe — every seat, wind
// particle, and breath runs on its own clock, and a pinned test clock still
// renders the same frame every time.
func cosmosPhase(key string) float64 {
	var hash uint32 = 2166136261
	for _, b := range []byte(key) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	return float64(hash%4096) / 4096 * 2 * math.Pi
}

// cosmosOrbits reads the spawn lineage out of the visible graph: every chat
// spawned by another visible chat is that parent's moon. Each parent keeps a
// stable, key-sorted list of its moons so a moon holds the same orbital slot
// no matter how the edge slice reorders on new activity.
func cosmosOrbits(edges []compose.CosmosEdge, nodes map[string]compose.CosmosNode) (map[string]string, map[string][]string) {
	parents := make(map[string]string)
	children := make(map[string][]string)
	for _, edge := range edges {
		if edge.Kind != shared.KindSpawn || edge.From == edge.To {
			continue
		}
		parent, parentSeen := nodes[edge.From]
		child, childSeen := nodes[edge.To]
		if !parentSeen || !childSeen || parent.Group || child.Group {
			continue
		}
		if parent.Home != child.Home {
			// Residence wins: a chat spawned into another star's system is a
			// planet of the star it lives under, never a moon dragged along
			// by its parent — the spawn lineage stays fully visible as the
			// dashed edge arcing between the two systems.
			continue
		}
		if _, taken := parents[edge.To]; taken {
			continue
		}
		parents[edge.To] = edge.From
		children[edge.From] = append(children[edge.From], edge.To)
	}
	for _, moons := range children {
		sort.Strings(moons)
	}
	return parents, children
}

func wrapDelta(delta float64) float64 {
	for delta > math.Pi {
		delta -= 2 * math.Pi
	}
	for delta < -math.Pi {
		delta += 2 * math.Pi
	}
	return delta
}

func (model *Model) advanceCosmos(nowNS int64) {
	dt := float64(nowNS-model.cosmosNowNS) / float64(time.Second)
	if dt < 0 {
		dt = 0
	}
	for _, seat := range model.cosmosSeats {
		seat.Angle += wrapDelta(seat.Target-seat.Angle) * math.Min(1, dt*5)
	}
	model.cosmosNowNS = nowNS
	model.layoutCosmosSeats()
}

// applyCosmosGraph is the single place model.cosmos is ever replaced. It is
// what makes "when did it die" honest: compose.BuildCosmos is stateless and
// reads no clock, so DiedNS as IT reports is only ever last edge activity —
// close to useless for animating a death, since killing a chat writes no
// ledger event and a chat idle for even a few seconds before its kill would
// already read as long past any blink+fade window on the very first frame
// that notices it. This method is the caller with memory across renders
// that compose cannot be: it diffs the incoming graph against the model's
// OWN previous graph, and the render-time moment (model.cosmosNowNS) a node
// key is first seen transition from alive to Dead is what actually drives
// cosmosDeathVisual — not compose's DiedNS, which this method overwrites
// before the node is ever stored or rendered.
//
// A node that is Dead on the very FIRST graph this method ever sees for
// that key (no prior "was alive" observation — a fresh app launch, or a
// chat that was already dead before anything here ever watched it) gets
// no animation and is DROPPED, exactly as if its blink+fade window had
// already elapsed — because it has: this method never witnessed the
// transition, so there is no honest detection moment, and the drop is the
// only truthful outcome left. This used to be a middle branch that kept
// such a node undated, dimmed, and permanent, which was wrong for exactly
// this case — a chat killed before the picker ever opened stayed on
// screen for the entire 24h ledger window, the same "permanent debris"
// this whole feature exists to remove, arriving through a different door.
//
// That undated branch WAS right for one thing it was also asked to cover:
// a freshly spawned child whose row has not been gathered yet, which also
// arrives as Dead with no baseline and must not be hidden
// (TestCosmosParentlessSpawnIsNotEmptyInEitherLayout). Compose now answers
// that ambiguity upstream instead, with the one nowNS-dependent question
// it CAN answer honestly (see BuildCosmos's own doc comment): a no-row
// node is PENDING — Dead false, rendered like any other node — until a
// short grace window elapses with still no row, at which point compose
// itself calls it Dead. By the time a node reaches this method as Dead
// with no witnessed transition, compose has already given it every chance
// to be a newborn; dropping it here is correct in both cases, because they
// are no longer the same case.
//
// Once a node IS blinking or fading, this method is also what finally
// drops it — and it prunes any edge naming a dropped node in the same
// pass, so model.cosmos.Nodes and model.cosmos.Edges stay consistent by
// construction and the header's "N chats · N edges" always names a
// quantity the canvas can actually draw. A dropped node's cosmosDiedAtNS
// entry is deliberately NOT deleted at drop time — compose keeps emitting
// it every frame the ledger still carries it (compose no longer prunes by
// time at all), and an entry-less "known" would read exactly like a node
// this method had never seen before, resurrecting it into the undated
// branch above forever. The entry is retired in place instead, and the
// only sweep that removes it is keyed on compose ceasing to emit the node
// at all — bounded by the same 24h ledger window compose already enforces
// upstream, one entry per node it still knows about.
func (model *Model) applyCosmosGraph(next compose.CosmosGraph) {
	previousAlive := make(map[string]bool, len(model.cosmos.Nodes))
	for _, node := range model.cosmos.Nodes {
		if !node.Dead {
			previousAlive[node.Key] = true
		}
	}
	if model.cosmosDiedAtNS == nil {
		model.cosmosDiedAtNS = make(map[string]int64)
	}
	deathWindow := int64(compose.CosmosDeathBlink + compose.CosmosDeathFade)
	seenThisGraph := make(map[string]bool, len(next.Nodes))
	keep := make(map[string]bool, len(next.Nodes))
	nodes := make([]compose.CosmosNode, 0, len(next.Nodes))
	for _, node := range next.Nodes {
		seenThisGraph[node.Key] = true
		if !node.Dead {
			delete(model.cosmosDiedAtNS, node.Key) // alive again (or never dead): forget any stale detection
			nodes = append(nodes, node)
			keep[node.Key] = true
			continue
		}
		diedAt, known := model.cosmosDiedAtNS[node.Key]
		if !known && !previousAlive[node.Key] {
			// No witnessed transition — see the doc comment above for why
			// this is a drop, not a keep, now that compose has already
			// resolved the newborn-vs-vanished ambiguity upstream.
			continue
		}
		if !known {
			diedAt = model.cosmosNowNS
			model.cosmosDiedAtNS[node.Key] = diedAt
		}
		if model.cosmosNowNS-diedAt > deathWindow {
			// Retired, not forgotten: the entry MUST survive so `known`
			// stays true on the next frame. compose no longer prunes by
			// time, so it will keep emitting this Dead node every frame
			// the ledger still carries it — deleting the entry here made
			// the very next frame see known=false, previousAlive=false
			// (this key is no longer in model.cosmos.Nodes to be alive
			// in), and land in the undated branch above: a retired node
			// resurrected as permanent dim-static debris one frame after
			// it correctly dropped. The only place this entry is removed
			// is the seenThisGraph sweep below, which fires exactly when
			// compose stops emitting the node at all (it left the 24h
			// ledger window) — so the map stays bounded at one entry per
			// node compose still knows about, dead or not.
			continue // blinked, faded, and its time as debris is over
		}
		node.DiedNS = diedAt
		nodes = append(nodes, node)
		keep[node.Key] = true
	}
	for key := range model.cosmosDiedAtNS {
		if !seenThisGraph[key] {
			delete(model.cosmosDiedAtNS, key) // aged out of the ledger window entirely; nothing left to animate
		}
	}
	edges := make([]compose.CosmosEdge, 0, len(next.Edges))
	for _, edge := range next.Edges {
		if !keep[edge.From] || !keep[edge.To] {
			continue
		}
		edges = append(edges, edge)
	}
	next.Nodes = nodes
	next.Edges = edges
	model.cosmos = next
}

func (model *Model) mergeCosmosSeats() {
	previous := model.cosmosSeats
	if previous == nil {
		previous = make(map[string]*cosmosSeat)
	}
	hadSeats := len(previous) != 0
	next := make(map[string]*cosmosSeat, len(model.cosmos.Nodes))
	newKeys := make(map[string]bool, len(model.cosmos.Nodes))
	chats := make([]compose.CosmosNode, 0, len(model.cosmos.Nodes))
	groups := make([]compose.CosmosNode, 0, len(model.cosmos.Nodes))
	for _, node := range model.cosmos.Nodes {
		if node.Group {
			groups = append(groups, node)
		} else {
			chats = append(chats, node)
		}
	}
	assign := func(nodes []compose.CosmosNode, offset float64) {
		for index, node := range nodes {
			target := offset
			if len(nodes) != 0 {
				target += 2 * math.Pi * float64(index) / float64(len(nodes))
			}
			if old := previous[node.Key]; old != nil {
				copy := *old
				copy.Target = target
				next[node.Key] = &copy
				continue
			}
			next[node.Key] = &cosmosSeat{Angle: target, Target: target}
			newKeys[node.Key] = true
		}
	}
	nodeMap := cosmosNodeMap(model.cosmos.Nodes)
	parents, _ := cosmosOrbits(model.cosmos.Edges, nodeMap)
	roots := make([]compose.CosmosNode, 0, len(chats))
	moons := make([]compose.CosmosNode, 0, len(chats))
	for _, node := range chats {
		if parents[node.Key] != "" {
			moons = append(moons, node)
			continue
		}
		roots = append(roots, node)
	}
	// Roots ring their own star, not one global circle: each star's planets
	// are spaced evenly among THEMSELVES, so a three-chat project reads as a
	// three-planet system regardless of how crowded the next star over is.
	byHome := make(map[string][]compose.CosmosNode)
	for _, node := range roots {
		byHome[node.Home] = append(byHome[node.Home], node)
	}
	homes := make([]string, 0, len(byHome))
	for home := range byHome {
		homes = append(homes, home)
	}
	sort.Strings(homes)
	for _, home := range homes {
		assign(byHome[home], -math.Pi/2)
	}
	assign(groups, math.Pi/6)
	// A moon's seat is only the fallback for the day its parent vanishes:
	// while the parent renders, cosmosLayout overrides the moon's position
	// with the parent-anchored orbit, and the ring target parked here is
	// where the orphan eases once the parent is gone.
	for _, node := range moons {
		target := 0.0
		if parent := next[parents[node.Key]]; parent != nil {
			target = parent.Target
		}
		if old := previous[node.Key]; old != nil {
			copy := *old
			copy.Target = target
			next[node.Key] = &copy
			continue
		}
		next[node.Key] = &cosmosSeat{Angle: target, Target: target}
		newKeys[node.Key] = true
	}
	for _, edge := range model.cosmos.Edges {
		if !hadSeats || edge.Kind != shared.KindSpawn || !newKeys[edge.To] {
			continue
		}
		parent, child := next[edge.From], next[edge.To]
		if parent != nil && child != nil {
			child.Angle = parent.Angle
		}
	}
	model.cosmosSeats = next
	model.layoutCosmosSeats()
}

func (model *Model) layoutCosmosSeats() {
	width := maxInt(40, model.width)
	height := maxInt(12, model.height)
	bodyHeight := maxInt(4, height-6)
	cols, rows := maxInt(1, width-2), maxInt(1, bodyHeight-2)
	top := 4.0
	bottom := float64((rows - 4) * 4)
	cx := float64(cols)
	cy := (top + bottom) / 2
	rx := float64(cols) - 26
	if rx < 12 {
		rx = 12
	}
	ry := (bottom-top)/2 - 5
	if ry < 6 {
		ry = 6
	}
	nodes := make(map[string]compose.CosmosNode, len(model.cosmos.Nodes))
	for _, node := range model.cosmos.Nodes {
		nodes[node.Key] = node
	}
	for key, seat := range model.cosmosSeats {
		radius := 1.0
		if nodes[key].Group {
			radius = 0.34
		}
		seat.X = cx + rx*radius*math.Cos(seat.Angle)
		seat.Y = cy + ry*radius*math.Sin(seat.Angle)
	}
}

func (model Model) renderCosmosPanel(width, height int) string {
	innerWidth := maxInt(1, width-2)
	innerHeight := maxInt(1, height-2)
	if width < 20 || height < 8 {
		return model.renderCompactCosmos(width, innerWidth, innerHeight)
	}
	canvas := NewCanvas(innerWidth, innerHeight)
	title := " cosmos "
	if model.cosmosSafe {
		// Safe mode names itself — a viewer must never wonder why the sky
		// is calmer inside VS Code.
		canvas.Quant = 16
		title = " cosmos · safe "
	}
	now := time.Unix(0, model.cosmosNowNS)
	if model.skyEnabled {
		drawCosmosStars(canvas, now)
	}
	if len(model.cosmos.Nodes) != 0 {
		model.drawCosmosUniverse(canvas, now)
	}
	if model.cosmos.Err != "" {
		canvas.DimAll(0.30)
		model.drawCosmosBanner(canvas)
	} else if len(model.cosmos.Nodes) == 0 {
		centerCosmosText(canvas, canvas.Rows/2-1, "no chat traffic recorded yet — the ledger fills as chats talk", cosmosDimColor(), false)
		centerCosmosText(canvas, canvas.Rows/2+1, "cosmos maps conversation · pfm ls maps existence", scaleRGB(cosmosDimColor(), 0.7), false)
	} else {
		model.drawCosmosTicker(canvas)
	}
	if len(model.cosmos.Warnings) > 0 {
		warning := "⚠ " + model.cosmos.Warnings[0]
		if len(model.cosmos.Warnings) > 1 {
			warning += fmt.Sprintf(" (+%d)", len(model.cosmos.Warnings)-1)
		}
		canvas.Text(1, canvas.Rows-1, truncateRunes(warning, maxInt(0, canvas.Cols-2)), cosmosDimColor(), false)
	}
	return framePanel(title, strings.Split(canvas.render(), "\n"), width)
}

func (model Model) renderCompactCosmos(width, innerWidth, innerHeight int) string {
	lines := make([]string, 0, innerHeight)
	if model.cosmos.Err != "" {
		lines = append(lines, warnStyle.Render(fillLine(
			ansiTruncateRunes("⚠ comms ledger unreachable: "+model.cosmos.Err, innerWidth), innerWidth,
		)))
	} else if len(model.cosmos.Nodes) == 0 {
		lines = append(lines, dimStyle.Render(fillLine(
			ansiTruncateRunes("no chat traffic recorded yet — the ledger fills as chats talk", innerWidth), innerWidth,
		)))
	} else {
		nodes := cosmosNodeMap(model.cosmos.Nodes)
		for index, edge := range model.cosmos.Edges {
			if index >= 3 || len(lines) >= innerHeight {
				break
			}
			from, to := nodes[edge.From], nodes[edge.To]
			labelWidth := maxInt(1, (innerWidth-3)/2)
			identity := truncateRunes(from.Label.String(), labelWidth) + " → " + truncateRunes(to.Label.String(), labelWidth)
			lines = append(lines, fillLine(ansiTruncateRunes(identity, innerWidth), innerWidth))
			if edge.LastMessage != "" && len(lines) < innerHeight {
				lines = append(lines, dimStyle.Render(fillLine(
					ansiTruncateRunes(edge.LastMessage, innerWidth), innerWidth,
				)))
			}
		}
	}
	if len(model.cosmos.Warnings) > 0 && len(lines) < innerHeight {
		lines = append(lines, dimStyle.Render(fillLine(
			ansiTruncateRunes("⚠ "+model.cosmos.Warnings[0], innerWidth), innerWidth,
		)))
	}
	for len(lines) < innerHeight {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}
	return framePanel(" cosmos ", lines, width)
}

func drawCosmosStars(canvas *Canvas, now time.Time) {
	t := float64(now.UnixNano()) / 1e9
	starColor := rgbFromHex(configuredCosmosPalette.CosmosStar)
	// Far layer: dimmer, slower, sparser — depth behind the main twinkle.
	far := rand.New(rand.NewSource(56))
	for range canvas.Cols * canvas.Rows / 32 {
		px, py := far.Intn(canvas.PW()), far.Intn(canvas.PH())
		phase := far.Float64() * 2 * math.Pi
		speed := 0.2 + far.Float64()*0.5
		canvas.Dot(px, py, scaleRGB(starColor, 0.10+0.16*(0.5+0.5*math.Sin(t*speed+phase))))
	}
	rng := rand.New(rand.NewSource(55))
	count := canvas.Cols * canvas.Rows / 20
	for range count {
		star := star{
			px: rng.Intn(canvas.PW()), py: rng.Intn(canvas.PH()),
			phase: rng.Float64() * 2 * math.Pi,
			speed: 0.6 + rng.Float64()*1.8,
		}
		brightness := 0.18 + 0.42*(0.5+0.5*math.Sin(t*star.speed+star.phase))
		canvas.Dot(star.px, star.py, scaleRGB(starColor, brightness))
	}
	// A few bright stars that flare into a four-point sparkle at their peak.
	bright := rgbFromHex(configuredCosmosPalette.CosmosBright)
	poles := rand.New(rand.NewSource(57))
	for range 4 {
		px, py := poles.Intn(canvas.PW()), poles.Intn(canvas.PH())
		phase := poles.Float64() * 2 * math.Pi
		glow := 0.5 + 0.5*math.Sin(t*0.7+phase)
		canvas.Dot(px, py, scaleRGB(bright, 0.20+0.50*glow))
		if glow > 0.75 {
			cross := scaleRGB(bright, (glow-0.75)*2.2)
			canvas.Dot(px+1, py, cross)
			canvas.Dot(px-1, py, cross)
			canvas.Dot(px, py+1, cross)
			canvas.Dot(px, py-1, cross)
		}
	}
	drawCosmosMeteor(canvas, now)
}

// drawCosmosMeteor streaks one meteor across the sky roughly every half
// minute. Pure deterministic decoration: the time bucket alone decides
// whether, where, and along which diagonal it flies, so a pinned test clock
// always renders the same sky.
func drawCosmosMeteor(canvas *Canvas, now time.Time) {
	const bucketNS = int64(9 * time.Second)
	bucket := now.UnixNano() / bucketNS
	seed := uint64(bucket)*0x9E3779B97F4A7C15 + 0xD1B54A32D192ED03
	seed ^= seed >> 29
	if seed%3 != 0 {
		return
	}
	flight := float64(now.UnixNano()-bucket*bucketNS) / float64(bucketNS) / 0.16
	if flight >= 1 {
		return
	}
	headX := float64(seed >> 8 % uint64(canvas.PW()))
	headY := float64(seed >> 20 % uint64(maxInt(1, canvas.PH()/3)))
	directionX, directionY := 34.0, 13.0
	if seed%2 == 0 {
		directionX = -34
	}
	headX += directionX * flight
	headY += directionY * flight
	bright := rgbFromHex(configuredCosmosPalette.CosmosBright)
	starColor := rgbFromHex(configuredCosmosPalette.CosmosStar)
	for segment := 0; segment < 7; segment++ {
		trail := float64(segment) / 7
		color := scaleRGB(lerpRGB(bright, starColor, trail), (1-trail)*(1-0.5*flight))
		canvas.Dot(int(headX-directionX*trail*0.2), int(headY-directionY*trail*0.2), color)
	}
}

// cosmosCometDuration is the shared flight time for a comet along an edge —
// used both to animate the comet itself and to decide when the edge rail
// beneath it should read as "in flight". Spawn lineage flights run longer
// than a plain message so the two remain distinguishable at a glance.
func cosmosCometDuration(kind string) time.Duration {
	if kind == shared.KindSpawn {
		return 1800 * time.Millisecond
	}
	return 1500 * time.Millisecond
}

func (model Model) drawCosmosUniverse(canvas *Canvas, now time.Time) {
	nodes := cosmosNodeMap(model.cosmos.Nodes)
	points, starOrder, starPoints, cx, cy := cosmosLayout(canvas, model.cosmosSeats, nodes, model.cosmos.Edges, now, model.skyEnabled)
	clock := float64(now.UnixNano()) / 1e9

	for _, edge := range model.cosmos.Edges {
		from, to := nodes[edge.From], nodes[edge.To]
		fp, fok := points[edge.From]
		tp, tok := points[edge.To]
		if !fok || !tok {
			continue
		}
		age := now.Sub(time.Unix(0, edge.LastNS)).Seconds()
		if age < 0 {
			age = 0
		}
		fade := 0.20 + 0.80*math.Exp(-age/40)
		dashed := false
		fromColor, toColor := cosmosNodeColor(from), cosmosNodeColor(to)
		if edge.Kind == shared.KindSpawn {
			dashed = true
			fade = math.Max(0.35, fade)
			fromColor = rgbFromHex(configuredCosmosPalette.CosmosLineage)
		}
		if model.skyEnabled {
			if flightAge := now.Sub(time.Unix(0, edge.LastNS)); flightAge >= 0 && flightAge < cosmosCometDuration(edge.Kind) {
				fade = 1.0
			}
		}
		x0, y0, cpx, cpy, x1, y1 := cosmosEdgeRail(fp, tp, cx, cy)
		canvas.Bezier(x0, y0, cpx, cpy, x1, y1, fromColor, toColor, fade, dashed)
	}

	if model.skyEnabled {
		// Ambient wind: every edge carries a slow drift of dim particles from
		// sender to receiver, always — so a quiet universe breathes between
		// real messages. Brightness follows the edge's own memory (a fresh
		// edge blows brighter for ~15 minutes) and the drift stays tail-less
		// and dim so it can never be misread as a comet, which is a real
		// message in flight.
		for _, edge := range model.cosmos.Edges {
			fp, fok := points[edge.From]
			tp, tok := points[edge.To]
			if !fok || !tok {
				continue
			}
			age := now.Sub(time.Unix(0, edge.LastNS)).Seconds()
			if age < 0 {
				age = 0
			}
			memory := 0.22 + 0.45*math.Exp(-age/900)
			phase := cosmosPhase(edge.From + "\x00" + edge.To + "\x00" + edge.Kind)
			period := 5 + 2.5*math.Sin(phase)
			fromColor, toColor := cosmosNodeColor(nodes[edge.From]), cosmosNodeColor(nodes[edge.To])
			particles := 2
			if edge.Kind == shared.KindSpawn {
				fromColor = rgbFromHex(configuredCosmosPalette.CosmosLineage)
				particles = 1
			}
			x0, y0, cpx, cpy, x1, y1 := cosmosEdgeRail(fp, tp, cx, cy)
			for index := 0; index < particles; index++ {
				progress := math.Mod(clock/period+phase/(2*math.Pi)+float64(index)*0.5, 1)
				px, py := BezierPoint(x0, y0, cpx, cpy, x1, y1, progress)
				shimmer := 0.7 + 0.3*math.Sin(clock*2.4+phase+float64(index)*math.Pi)
				canvas.Dot(int(px), int(py), scaleRGB(lerpRGB(fromColor, toColor, progress), memory*shimmer))
			}
		}

		white := rgbFromHex(configuredCosmosPalette.CosmosBright)
		const tailSegments = 18
		for _, edge := range model.cosmos.Edges {
			duration := cosmosCometDuration(edge.Kind)
			age := now.Sub(time.Unix(0, edge.LastNS))
			if age < 0 || age >= duration {
				continue
			}
			from, to := nodes[edge.From], nodes[edge.To]
			fp, fok := points[edge.From]
			tp, tok := points[edge.To]
			if !fok || !tok {
				continue
			}
			x0, y0, cpx, cpy, x1, y1 := cosmosEdgeRail(fp, tp, cx, cy)
			t := float64(age) / float64(duration)
			base := lerpRGB(cosmosNodeColor(from), cosmosNodeColor(to), t)
			if edge.Kind == shared.KindSpawn {
				base = lerpRGB(rgbFromHex(configuredCosmosPalette.CosmosLineage), white, 0.4)
			}
			et := ease(t)
			for tail := 0; tail < tailSegments; tail++ {
				tt := et - float64(tail)*0.018
				if tt < 0 {
					break
				}
				px, py := BezierPoint(x0, y0, cpx, cpy, x1, y1, tt)
				strength := 1 - float64(tail)/tailSegments
				color := scaleRGB(lerpRGB(base, white, 0.85*strength), 0.55+0.45*strength)
				canvas.Dot(int(px), int(py), color)
				if tail == 0 {
					// The head is a burst, not a dim streak segment: a
					// full-white core plus a round ring of light — round
					// because braille sub-cells are half as wide as they
					// are tall, so the x offset carries the same 1.6
					// correction the shockwave rings below use.
					canvas.Dot(int(px), int(py), white)
					const burstDots = 10
					const burstRadius = 1.6
					for index := 0; index < burstDots; index++ {
						angle := 2 * math.Pi * float64(index) / burstDots
						bx := px + burstRadius*math.Cos(angle)*1.6
						by := py + burstRadius*math.Sin(angle)
						canvas.Dot(int(bx), int(by), color)
					}
				}
			}
		}

		for _, edge := range model.cosmos.Edges {
			var duration, startRadius, endRadius, blend float64
			var dotCount int
			if edge.Kind == shared.KindSpawn {
				duration, startRadius, endRadius, blend, dotCount = 1.6, 2, 11, 0.5, 26
			} else {
				// Inject rings stay tighter, brighter, and shorter-lived
				// than a spawn ring so the two events keep reading as
				// different things happening, not the same ring twice.
				duration, startRadius, endRadius, blend, dotCount = 1.0, 1.5, 7, 0.8, 18
			}
			age := now.Sub(time.Unix(0, edge.LastNS)).Seconds()
			if age < 0 || age >= duration {
				continue
			}
			point, ok := points[edge.To]
			if !ok {
				continue
			}
			progress := age / duration
			radius := startRadius + progress*(endRadius-startRadius)
			color := scaleRGB(lerpRGB(cosmosNodeColor(nodes[edge.To]), white, blend), 1-progress)
			for index := 0; index < dotCount; index++ {
				angle := 2 * math.Pi * float64(index) / float64(dotCount)
				canvas.Dot(int(point.x+radius*math.Cos(angle)*1.6), int(point.y+radius*math.Sin(angle)), color)
			}
		}
	}

	// Arrival/departure flashes are sky-only, like every other animated
	// glow in this function: --no-sky renders a static frame, full stop.
	var inboundFlash, outboundFlash map[string]float64
	if model.skyEnabled {
		inboundFlash = newestInboundFlash(model.cosmos.Edges, model.cosmosNowNS)
		outboundFlash = newestOutboundFlash(model.cosmos.Edges, model.cosmosNowNS)
	}
	white := rgbFromHex(configuredCosmosPalette.CosmosBright)
	moonParents, _ := cosmosOrbits(model.cosmos.Edges, nodes)
	orbitDepth := func(key string) int {
		depth := 0
		for parent := moonParents[key]; parent != "" && depth <= 8; parent = moonParents[parent] {
			depth++
		}
		return depth
	}
	// Deepest moons draw first so that where a system crowds, the parent's
	// glyph and label overwrite the moon's — the anchor of a system stays
	// readable, the way the bigger body wins an eclipse.
	ordered := append(groupsOnly(model.cosmos.Nodes), chatsOnly(model.cosmos.Nodes)...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return orbitDepth(ordered[i].Key) > orbitDepth(ordered[j].Key)
	})
	var glyphStamps, starStamps []struct {
		x, y  int
		glyph rune
		color RGB
	}
	// The stars: one sun per project, drawn before the planets so a planet
	// label may run over a star LABEL, while the star glyphs replay LAST in
	// the re-stamp below — a sun can never be erased by its planets. A sun's
	// radiance is its population: the more chats orbit it, the hotter it
	// burns (see drawCosmosSun).
	population := make(map[string]int)
	for _, node := range model.cosmos.Nodes {
		if !node.Group {
			population[node.Home]++
		}
	}
	for _, home := range starOrder {
		point := starPoints[home]
		color := drawCosmosSun(canvas, point, home, population[home], clock, model.skyEnabled)
		colX, colY := int(point.x)/2, int(point.y)/4
		canvas.SetCell(colX, colY, '✹', color, true)
		label := home
		if label == "" {
			label = "unknown"
		}
		label = truncateRunes(label, maxInt(0, canvas.Cols-2))
		canvas.Text(colX-len([]rune(label))/2, colY+1, label, cosmosDimColor(), false)
		starStamps = append(starStamps, struct {
			x, y  int
			glyph rune
			color RGB
		}{colX, colY, '✹', color})
	}
	for _, node := range ordered {
		point, ok := points[node.Key]
		if !ok {
			continue
		}
		colX, colY := int(point.x)/2, int(point.y)/4
		base := cosmosNodeColor(node)
		brightness := 0.85
		if model.skyEnabled && !node.Dead {
			// Ambient breath: each node glows on its own slow clock so a
			// quiet universe still reads as alive, not frozen.
			brightness = 0.80 + 0.10*math.Sin(clock*1.3+cosmosPhase(node.Key))
		}
		color := scaleRGB(base, brightness)
		arrival := math.Max(inboundFlash[node.Key], outboundFlash[node.Key])
		if arrival > 0 {
			color = lerpRGB(base, white, arrival*0.95)
		}
		bold := arrival > 0.4
		if node.Dead {
			if model.skyEnabled {
				// DiedNS here is model.applyCosmosGraph's own detection
				// timestamp, not compose's last-edge-activity value — the
				// honest answer to "when did the model first notice this
				// one was dead." Every Dead node reaching this render
				// already went through a witnessed alive-to-dead
				// transition (applyCosmosGraph drops anything that did
				// not), so DiedNS is always set here.
				color, bold = cosmosDeathVisual(base, now.Sub(time.Unix(0, node.DiedNS)))
			} else {
				// --no-sky: dimmed, never animated, the same frame every
				// render.
				color, bold = scaleRGB(base, 0.35), false
			}
		}
		if model.skyEnabled && node.Group && !node.Dead {
			// A hub is a little station: three faint satellites on a slow
			// orbit. Text cells win over braille, so the glyph and label
			// stay untouched above them.
			for index := 0; index < 3; index++ {
				orbit := clock*0.8 + cosmosPhase(node.Key) + float64(index)*2*math.Pi/3
				canvas.Dot(int(point.x+3*math.Cos(orbit)*1.6), int(point.y+3*math.Sin(orbit)), scaleRGB(base, 0.5))
			}
		}
		canvas.SetCell(colX, colY, cosmosNodeGlyph(node), color, true)
		glyphStamps = append(glyphStamps, struct {
			x, y  int
			glyph rune
			color RGB
		}{colX, colY, cosmosNodeGlyph(node), color})
		rightward := point.x >= cx
		if parentKey := moonParents[node.Key]; parentKey != "" {
			if anchor, ok := points[parentKey]; ok {
				// A moon's label grows away from its PARENT, not away from
				// the galactic centre — the parent's own label is the text
				// most likely to sit beside it.
				rightward = point.x >= anchor.x
			}
		}
		label := clipCosmosLabel(node.Label.String(), rightward, colX, canvas.Cols)
		if rightward {
			canvas.Text(colX+2, colY, label, color, bold)
		} else {
			canvas.Text(colX-1-len([]rune(label)), colY, label, color, bold)
		}
		if arrival > 0.55 {
			canvas.SetCell(colX, colY-1, '✦', scaleRGB(white, arrival), false)
		}
	}
	// Glyphs win over labels: a later node's text may have paved over an
	// earlier node's marker (a moon sitting inside its parent's long label),
	// and a chat that exists must never render invisible — so every node's
	// glyph is re-stamped after all text has landed, parents last, suns
	// after everything.
	for _, stamp := range glyphStamps {
		canvas.SetCell(stamp.x, stamp.y, stamp.glyph, stamp.color, true)
	}
	for _, stamp := range starStamps {
		canvas.SetCell(stamp.x, stamp.y, stamp.glyph, stamp.color, true)
	}
}

// drawCosmosSun paints one project star and returns the colour its glyph
// should wear. Radiance scales with population — the count of chats living
// under this star: a lone chat gets a modest ember; a crowded project burns
// white-hot with a wider corona, diffraction spikes from three planets up,
// and periodic solar flares from five. Every element derives from (clock,
// home, population) alone, so a pinned test clock renders the same sun.
func drawCosmosSun(canvas *Canvas, point cosmosPoint, home string, population int, clock float64, sky bool) RGB {
	sunColor := rgbFromHex(configuredCosmosPalette.CosmosSun)
	bright := rgbFromHex(configuredCosmosPalette.CosmosBright)
	heat := math.Min(1, float64(population)/8)
	base := lerpRGB(sunColor, bright, 0.45*heat)
	if !sky {
		return scaleRGB(base, 0.75+0.25*heat)
	}
	phase := cosmosPhase(home)
	glow := (0.78 + 0.10*heat) + (0.14+0.08*heat)*math.Sin(clock*(0.8+0.2*heat)+phase)

	// Corona: dots multiply and widen with the family.
	coronaRadius := 2.4 + math.Min(2.6, 0.25*float64(population))
	coronaDots := minInt(14, 6+population)
	for index := 0; index < coronaDots; index++ {
		angle := clock*0.5 + phase + 2*math.Pi*float64(index)/float64(coronaDots)
		shimmer := 0.30 + 0.25*heat + 0.20*math.Sin(clock*2.1+phase+float64(index))
		canvas.Dot(int(point.x+coronaRadius*math.Cos(angle)*1.6), int(point.y+coronaRadius*math.Sin(angle)), scaleRGB(base, shimmer))
	}

	// Diffraction spikes from three planets up: four arms of light that
	// twinkle independently, longer on hotter stars.
	if population >= 3 {
		armLength := 2 + minInt(4, population-2)
		for arm := 0; arm < 4; arm++ {
			angle := float64(arm) * math.Pi / 2
			twinkle := 0.5 + 0.5*math.Sin(clock*1.7+phase+float64(arm)*1.9)
			for step := 2; step <= armLength; step++ {
				strength := (1 - float64(step)/float64(armLength+1)) * (0.35 + 0.45*twinkle)
				canvas.Dot(int(point.x+float64(step)*math.Cos(angle)*1.6), int(point.y+float64(step)*math.Sin(angle)), scaleRGB(lerpRGB(base, bright, 0.4), strength))
			}
		}
	}

	// Solar flares from five planets up: every few seconds an arc erupts
	// off the corona and fades — which bucket of time it is decides when
	// and where, so the eruption is pure clockwork.
	if population >= 5 {
		const flarePeriod = 7.0
		bucket := int64(clock / flarePeriod)
		seed := uint64(bucket)*0x9E3779B97F4A7C15 + uint64(len(home))*0xBF58476D1CE4E5B9
		seed ^= seed >> 29
		progress := math.Mod(clock, flarePeriod) / 2.2
		if seed%2 == 0 && progress < 1 {
			eruptAt := phase + float64(seed%628)/100
			reach := coronaRadius + 1.5 + 4*progress
			for index := 0; index < 5; index++ {
				angle := eruptAt + (float64(index)-2)*0.16
				strength := (1 - progress) * (1 - math.Abs(float64(index)-2)/3) * (0.4 + 0.4*heat)
				canvas.Dot(int(point.x+reach*math.Cos(angle)*1.6), int(point.y+reach*math.Sin(angle)), scaleRGB(lerpRGB(base, bright, 0.6), strength))
			}
		}
	}
	return scaleRGB(base, glow)
}

// cosmosDeathVisual is what a dead chat's node looks like deathAge after it
// died: a fast, hard on/off alarm at the node's OWN engine hue (never a
// fixed alarm tint, which would erase engine identity mid-blink), then a
// short fade to nothing. Both windows are short on purpose — the chat
// should read as gone quickly and calmly, not strobe the whole tab.
func cosmosDeathVisual(base RGB, deathAge time.Duration) (RGB, bool) {
	const blinkInterval = 200 * time.Millisecond
	if deathAge < compose.CosmosDeathBlink {
		if (deathAge/blinkInterval)%2 == 0 {
			return scaleRGB(base, 1.0), true
		}
		return scaleRGB(base, 0.12), false
	}
	fadeT := float64(deathAge-compose.CosmosDeathBlink) / float64(compose.CosmosDeathFade)
	if fadeT > 1 {
		fadeT = 1
	}
	return scaleRGB(base, 0.85*(1-fadeT)), false
}

// clipCosmosLabel bounds a node's label to the canvas space actually
// available in the direction it draws, so a long label (a long chat name, or
// the bracketed diagnostic an unresolved identity renders as) can be clipped
// with a visible ellipsis instead of running off-canvas or smearing across
// whatever sits past the edge.
func clipCosmosLabel(label string, rightward bool, colX, cols int) string {
	available := cols - (colX + 2)
	if !rightward {
		available = colX - 1
	}
	if available < 0 {
		available = 0
	}
	return truncateRunes(label, available)
}

// cosmosStarPoints seats every star — one per distinct Home among the visible
// chats, the unknown "" home included — on the galactic ring, or alone at the
// centre when the whole fleet lives in one place. Sorted order keeps both the
// seating and the draw order deterministic.
func cosmosStarPoints(nodes map[string]compose.CosmosNode, cx, cy, rx, ry float64) ([]string, map[string]cosmosPoint) {
	seen := make(map[string]bool)
	for _, node := range nodes {
		if !node.Group {
			seen[node.Home] = true
		}
	}
	order := make([]string, 0, len(seen))
	for home := range seen {
		order = append(order, home)
	}
	sort.Strings(order)
	points := make(map[string]cosmosPoint, len(order))
	if len(order) == 1 {
		points[order[0]] = cosmosPoint{x: cx, y: cy}
		return order, points
	}
	for index, home := range order {
		// The ring starts at π — the LEFT of the canvas — so two stars land
		// left and right, not stacked top and bottom: a terminal is wide
		// before it is tall, and the wide axis is where systems fit.
		angle := math.Pi + 2*math.Pi*float64(index)/float64(len(order))
		points[home] = cosmosPoint{x: cx + rx*0.55*math.Cos(angle), y: cy + ry*0.5*math.Sin(angle)}
	}
	return order, points
}

func cosmosLayout(canvas *Canvas, seats map[string]*cosmosSeat, nodes map[string]compose.CosmosNode, edges []compose.CosmosEdge, now time.Time, sky bool) (map[string]cosmosPoint, []string, map[string]cosmosPoint, float64, float64) {
	top := 4.0
	bottom := float64((canvas.Rows - 4) * 4)
	cx := float64(canvas.PW()) / 2
	cy := (top + bottom) / 2
	rx := float64(canvas.PW())/2 - 26
	if rx < 12 {
		rx = 12
	}
	ry := (bottom-top)/2 - 5
	if ry < 6 {
		ry = 6
	}
	clock := float64(now.UnixNano()) / 1e9
	// The star tier: every chat orbits ITS OWN star — the project it lives
	// in. One star sits at the galactic centre with the full canvas as its
	// system; several stars share the ring, each with a tighter system.
	starOrder, starPoints := cosmosStarPoints(nodes, cx, cy, rx, ry)
	systemRx, systemRy := rx, ry
	if len(starOrder) > 1 {
		systemRx, systemRy = rx*0.32, ry*0.38
	}
	points := make(map[string]cosmosPoint, len(seats))
	for key, seat := range seats {
		angle, breath := seat.Angle, 1.0
		if sky {
			// Orbital sway: every seat drifts a couple of degrees around its
			// anchor on its own slow clock, so nodes float instead of
			// standing pinned. --no-sky reduces to the seat's exact values.
			phase := cosmosPhase(key)
			angle += 0.035 * math.Sin(clock*0.45+phase)
			breath = 1 + 0.015*math.Sin(clock*0.31+phase*1.7)
		}
		if nodes[key].Group {
			radius := 0.34 * breath
			points[key] = cosmosPoint{
				x: cx + rx*radius*math.Cos(angle),
				y: cy + ry*radius*math.Sin(angle),
			}
			continue
		}
		anchor, anchored := starPoints[nodes[key].Home]
		if !anchored {
			anchor = cosmosPoint{x: cx, y: cy}
		}
		if sky {
			// The planets actually revolve: the whole ring turns rigidly
			// around its sun — one revolution every ~3.5 minutes, slow
			// enough to feel orbital, fast enough to see — while even
			// spacing between siblings is preserved by construction. Moons
			// orbit their planet faster (~70s) one tier down.
			angle += clock * 0.03
		}
		points[key] = cosmosPoint{
			x: anchor.x + systemRx*breath*math.Cos(angle),
			y: anchor.y + systemRy*breath*math.Sin(angle),
		}
	}
	// Planetary hierarchy: a spawned chat leaves the main ring and becomes
	// its parent's moon — the ring point computed above stays behind only as
	// the seat fallback. All moons of one parent share ONE orbit: the same
	// distance from the parent, evenly spaced from each other, the ring
	// widening with the brood so a bigger family never crowds. Like the
	// earth: the moon circles its planet while the planet keeps circling
	// the sun — and a moon's own children circle IT, one level down.
	parents, children := cosmosOrbits(edges, nodes)
	if len(parents) != 0 {
		var place func(key string, depth int) cosmosPoint
		place = func(key string, depth int) cosmosPoint {
			parentKey := parents[key]
			if parentKey == "" || depth > 8 {
				return points[key]
			}
			anchor := place(parentKey, depth+1)
			moons := children[parentKey]
			slot := 0
			for index, moon := range moons {
				if moon == key {
					slot = index
					break
				}
			}
			orbit := 5 + 2.5*float64(len(moons))
			angle := 2*math.Pi*float64(slot)/float64(len(moons)) + cosmosPhase(parentKey)
			if sky {
				angle += clock * 0.09 // one slow revolution per ~70s
			}
			// No 1.6 aspect widening here on purpose: the orbit renders
			// vertical-major, so moons spend most of their revolution above
			// or below the parent's label row instead of ploughing through
			// the text beside it.
			return cosmosPoint{
				x: anchor.x + orbit*math.Cos(angle),
				y: anchor.y + orbit*math.Sin(angle),
			}
		}
		for key := range parents {
			points[key] = place(key, 0)
		}
	}
	return points, starOrder, starPoints, cx, cy
}

func cosmosEdgeRail(from, to cosmosPoint, cx, cy float64) (x0, y0, cpx, cpy, x1, y1 float64) {
	x0, y0, x1, y1 = from.x, from.y, to.x, to.y
	mx, my := (x0+x1)/2, (y0+y1)/2
	cpx = mx + (cx-mx)*0.30
	cpy = my + (cy-my)*0.30
	return
}

func ease(t float64) float64 { return t * t * (3 - 2*t) }

func (model Model) drawCosmosTicker(canvas *Canvas) {
	nodes := cosmosNodeMap(model.cosmos.Nodes)
	limit := minInt(2, len(model.cosmos.Edges))
	for index := 0; index < limit; index++ {
		row := canvas.Rows - 2 - index
		color := cosmosDimColor()
		if index == 0 {
			color = rgbFromHex(configuredCosmosPalette.CosmosBright)
		}
		line := cosmosTickerLine(model.cosmos.Edges[index], nodes)
		canvas.Text(1, row, truncateRunes(line, maxInt(0, canvas.Cols-2)), color, false)
	}
}

func cosmosTickerLine(edge compose.CosmosEdge, nodes map[string]compose.CosmosNode) string {
	from, to := nodes[edge.From], nodes[edge.To]
	return fmt.Sprintf("%s %c %s → %c %s · %s",
		time.Unix(0, edge.LastNS).Format("15:04:05"),
		cosmosNodeGlyph(from), from.Label.String(),
		cosmosNodeGlyph(to), to.Label.String(),
		edge.LastMessage,
	)
}

func (model Model) drawCosmosBanner(canvas *Canvas) {
	lines := []string{
		"⚠ comms ledger unreachable: " + model.cosmos.Err,
		"the sky below is the last recorded frame — not a live view",
	}
	width := 0
	for _, line := range lines {
		if n := len([]rune(line)); n > width {
			width = n
		}
	}
	width += 4
	if width > canvas.Cols-2 {
		width = canvas.Cols - 2
	}
	x := (canvas.Cols - width) / 2
	y := canvas.Rows/2 - 2
	alarm := rgbFromHex(configuredCosmosPalette.Warn)
	canvas.Text(x, y, "┌"+repeat('─', width-2)+"┐", alarm, false)
	for index, line := range lines {
		line = truncateRunes(line, maxInt(0, width-2))
		color := rgbFromHex(configuredCosmosPalette.CosmosBright)
		if index > 0 {
			color = cosmosDimColor()
		}
		padding := width - 2 - len([]rune(line))
		left := padding / 2
		canvas.Text(x, y+1+index, "│"+repeat(' ', left)+line+repeat(' ', padding-left)+"│", color, index == 0)
		canvas.SetCell(x, y+1+index, '│', alarm, false)
		canvas.SetCell(x+width-1, y+1+index, '│', alarm, false)
	}
	canvas.Text(x, y+1+len(lines), "└"+repeat('─', width-2)+"┘", alarm, false)
}

func centerCosmosText(canvas *Canvas, row int, text string, color RGB, bold bool) {
	text = truncateRunes(text, canvas.Cols)
	x := (canvas.Cols - len([]rune(text))) / 2
	if x < 0 {
		x = 0
	}
	canvas.Text(x, row, text, color, bold)
}

func repeat(char rune, count int) string {
	if count < 0 {
		count = 0
	}
	result := make([]rune, count)
	for index := range result {
		result[index] = char
	}
	return string(result)
}

func truncateRunes(value string, maximum int) string {
	if maximum <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	if maximum == 1 {
		return "…"
	}
	return string(runes[:maximum-1]) + "…"
}

func ansiTruncateRunes(value string, maximum int) string { return truncateRunes(value, maximum) }

func cosmosNodeMap(nodes []compose.CosmosNode) map[string]compose.CosmosNode {
	result := make(map[string]compose.CosmosNode, len(nodes))
	for _, node := range nodes {
		result[node.Key] = node
	}
	return result
}

func chatsOnly(nodes []compose.CosmosNode) []compose.CosmosNode {
	result := make([]compose.CosmosNode, 0, len(nodes))
	for _, node := range nodes {
		if !node.Group {
			result = append(result, node)
		}
	}
	return result
}

func groupsOnly(nodes []compose.CosmosNode) []compose.CosmosNode {
	result := make([]compose.CosmosNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Group {
			result = append(result, node)
		}
	}
	return result
}

func newestInboundFlash(edges []compose.CosmosEdge, nowNS int64) map[string]float64 {
	latest := make(map[string]int64)
	for _, edge := range edges {
		if edge.LastNS > latest[edge.To] {
			latest[edge.To] = edge.LastNS
		}
	}
	result := make(map[string]float64, len(latest))
	for key, atNS := range latest {
		age := float64(nowNS-atNS) / float64(time.Second)
		if age >= 0 && age < 1.5 {
			result[key] = math.Max(0, 1-age/1.5)
		}
	}
	return result
}

func newestOutboundFlash(edges []compose.CosmosEdge, nowNS int64) map[string]float64 {
	latest := make(map[string]int64)
	for _, edge := range edges {
		if edge.LastNS > latest[edge.From] {
			latest[edge.From] = edge.LastNS
		}
	}
	result := make(map[string]float64, len(latest))
	for key, atNS := range latest {
		age := float64(nowNS-atNS) / float64(time.Second)
		if age >= 0 && age < 1.5 {
			result[key] = math.Max(0, 1-age/1.5)
		}
	}
	return result
}

func cosmosNodeColor(node compose.CosmosNode) RGB {
	if node.Group {
		return rgbFromHex(configuredCosmosPalette.CosmosHub)
	}
	if id, err := pfmengine.Parse(node.Engine); err == nil {
		return rgbFromHex(configuredCosmosPalette.StatsEngine[id])
	}
	return rgbFromHex(configuredCosmosPalette.Muted)
}

func cosmosNodeGlyph(node compose.CosmosNode) rune {
	if node.Group {
		return '⬡'
	}
	switch pfmengine.ID(node.Engine) {
	case pfmengine.Codex:
		return '▲'
	case pfmengine.Opencode:
		return '◆'
	case pfmengine.Claude:
		return '✳'
	default:
		return '·'
	}
}

func cosmosDimColor() RGB { return rgbFromHex(configuredCosmosPalette.Dim) }
