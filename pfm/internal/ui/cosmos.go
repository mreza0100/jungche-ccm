package ui

import (
	"context"
	"fmt"
	"math"
	"math/rand"
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
type cosmosTickMsg struct{ nowNS int64 }

func (model *Model) startCosmosSample() tea.Cmd {
	if model.cosmosSampler == nil || model.cosmosLoading {
		return nil
	}
	model.statsGeneration++
	generation := model.statsGeneration
	model.cosmosLoading = true
	sampler := model.cosmosSampler
	sinceNS := model.nowNS - int64(cosmosWindow)
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

func cosmosTickCmd() tea.Cmd {
	return tea.Tick(125*time.Millisecond, func(now time.Time) tea.Msg {
		return cosmosTickMsg{nowNS: now.UnixNano()}
	})
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
	assign(chats, -math.Pi/2)
	assign(groups, math.Pi/6)
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
	return framePanel(" cosmos ", strings.Split(canvas.render(), "\n"), width)
}

func (model Model) renderCompactCosmos(width, innerWidth, innerHeight int) string {
	lines := make([]string, 0, innerHeight)
	if model.cosmos.Err != "" {
		lines = append(lines, warnStyle.Render(fillLine(
			ansiTruncateRunes("⚠ comms ledger unreachable: "+model.cosmos.Err, innerWidth), innerWidth,
		)))
	} else if len(model.cosmos.Edges) == 0 {
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
			identity := truncateRunes(from.Label, labelWidth) + " → " + truncateRunes(to.Label, labelWidth)
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
	rng := rand.New(rand.NewSource(55))
	count := canvas.Cols * canvas.Rows / 20
	t := float64(now.UnixNano()) / 1e9
	for range count {
		star := star{
			px: rng.Intn(canvas.PW()), py: rng.Intn(canvas.PH()),
			phase: rng.Float64() * 2 * math.Pi,
			speed: 0.6 + rng.Float64()*1.8,
		}
		brightness := 0.18 + 0.42*(0.5+0.5*math.Sin(t*star.speed+star.phase))
		canvas.Dot(star.px, star.py, scaleRGB(rgbFromHex(configuredCosmosPalette.CosmosStar), brightness))
	}
}

func (model Model) drawCosmosUniverse(canvas *Canvas, now time.Time) {
	nodes := cosmosNodeMap(model.cosmos.Nodes)
	points, cx, cy := cosmosLayout(canvas, model.cosmosSeats, nodes)

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
		x0, y0, cpx, cpy, x1, y1 := cosmosEdgeRail(fp, tp, cx, cy)
		canvas.Bezier(x0, y0, cpx, cpy, x1, y1, fromColor, toColor, fade, dashed)
	}

	if model.skyEnabled {
		white := rgbFromHex(configuredCosmosPalette.CosmosBright)
		for _, edge := range model.cosmos.Edges {
			duration := 900 * time.Millisecond
			if edge.Kind == shared.KindSpawn {
				duration = 1200 * time.Millisecond
			}
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
			for tail := 0; tail < 8; tail++ {
				tt := et - float64(tail)*0.030
				if tt < 0 {
					break
				}
				px, py := BezierPoint(x0, y0, cpx, cpy, x1, y1, tt)
				strength := 1 - float64(tail)/8
				color := scaleRGB(lerpRGB(base, white, 0.65*strength), 0.35+0.65*strength)
				canvas.Dot(int(px), int(py), color)
				if tail == 0 {
					canvas.Dot(int(px)+1, int(py), color)
					canvas.Dot(int(px)-1, int(py), color)
					canvas.Dot(int(px), int(py)+1, color)
					canvas.Dot(int(px), int(py)-1, color)
				}
			}
		}

		for _, edge := range model.cosmos.Edges {
			if edge.Kind != shared.KindSpawn {
				continue
			}
			age := now.Sub(time.Unix(0, edge.LastNS)).Seconds()
			if age < 0 || age >= 1.6 {
				continue
			}
			point, ok := points[edge.To]
			if !ok {
				continue
			}
			progress := age / 1.6
			radius := 2 + progress*9
			color := scaleRGB(lerpRGB(cosmosNodeColor(nodes[edge.To]), white, 0.5), 1-progress)
			for index := 0; index < 26; index++ {
				angle := 2 * math.Pi * float64(index) / 26
				canvas.Dot(int(point.x+radius*math.Cos(angle)*1.6), int(point.y+radius*math.Sin(angle)), color)
			}
		}
	}

	flash := newestInboundFlash(model.cosmos.Edges, model.cosmosNowNS)
	white := rgbFromHex(configuredCosmosPalette.CosmosBright)
	ordered := append(groupsOnly(model.cosmos.Nodes), chatsOnly(model.cosmos.Nodes)...)
	for _, node := range ordered {
		point, ok := points[node.Key]
		if !ok {
			continue
		}
		colX, colY := int(point.x)/2, int(point.y)/4
		base := cosmosNodeColor(node)
		color := scaleRGB(base, 0.85)
		arrival := flash[node.Key]
		if arrival > 0 {
			color = lerpRGB(base, white, arrival*0.7)
		}
		canvas.SetCell(colX, colY, cosmosNodeGlyph(node), color, true)
		label := node.Label
		if point.x >= cx {
			canvas.Text(colX+2, colY, label, color, arrival > 0.4)
		} else {
			canvas.Text(colX-1-len([]rune(label)), colY, label, color, arrival > 0.4)
		}
		if arrival > 0.55 {
			canvas.SetCell(colX, colY-1, '✦', scaleRGB(white, arrival), false)
		}
	}
}

func cosmosLayout(canvas *Canvas, seats map[string]*cosmosSeat, nodes map[string]compose.CosmosNode) (map[string]cosmosPoint, float64, float64) {
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
	points := make(map[string]cosmosPoint, len(seats))
	for key, seat := range seats {
		radius := 1.0
		if nodes[key].Group {
			radius = 0.34
		}
		points[key] = cosmosPoint{
			x: cx + rx*radius*math.Cos(seat.Angle),
			y: cy + ry*radius*math.Sin(seat.Angle),
		}
	}
	return points, cx, cy
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
		cosmosNodeGlyph(from), from.Label,
		cosmosNodeGlyph(to), to.Label,
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
