package compose

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/naming"
	"hostops/pfm/internal/store"
)

const (
	claudeResumeCap = 30
	codexResumeCap  = 15
)

type composer struct {
	input Input

	transcriptByID   map[string]store.Transcript
	transcriptByPath map[string]store.Transcript
	rolloutByID      map[string]store.Rollout
	rolloutByPath    map[string]store.Rollout
	codexLineages    []store.CodexLineage
	lineageByRoot    map[string]store.CodexLineage
	lineageRootByID  map[string]string
	killedByID       map[string]store.Killed
	panesBySocket    map[string][]gather.Pane
	paneByTarget     map[string]gather.Pane
	claudeSockets    map[string]struct{}
	cacheSockets     map[string]struct{}
	liveTranscripts  map[string]struct{}
	liveRollouts     map[string]struct{}
	accountRoots     []AccountRoot
	canonicalPaths   map[string]string
	projectDirs      map[string]string
}

// Compose performs the complete side-effect-free row composition pass.
func Compose(input Input) Output {
	current := &composer{input: input}
	current.buildIndexes()

	liveClaude, splits := current.liveClaudeRows()
	liveCodex := current.liveCodexRows()
	liveRows := collapseLiveServers(append(liveClaude, liveCodex...))
	liveRows = append(liveRows, splits...)
	// Booting rows are already deduped by socket in gather (DetectCrumblessLive
	// skips any socket a crumb resolves for), so they bypass
	// collapseLiveServers — that function's multi-server winner selection
	// exists for LiveClaude/LiveCodex identity collapse, a different problem.
	liveRows = append(liveRows, current.bootingRows()...)
	agentRows := current.agentRows()

	output := Output{
		ProjectDirs:      cloneStringMap(current.projectDirs),
		includeNewClaude: input.Options.View != KilledView,
		includeNewCodex:  input.Options.View != KilledView && input.Options.CodexAvailable,
		primaryAccount:   input.Options.PrimaryAccount,
		fallbackDir:      input.Options.CurrentDir,
	}
	if !configuredAccount(input.AccountRoots, output.primaryAccount) {
		if len(input.AccountRoots) != 0 {
			output.primaryAccount = input.AccountRoots[0].Account
		} else {
			output.primaryAccount = 1
		}
	}

	for _, row := range append(liveRows, agentRows...) {
		row = current.applyKill(row, EngineForKind(row.Kind))
		countOmitted(row, &output.KilledCount, &output.SuppressedCount)
		if visibleInView(row, input.Options.View) {
			output.Rows = append(output.Rows, current.finalize(row))
		}
	}

	claudeResume := make([]Row, 0)
	claudeEligible := 0
	for _, transcript := range input.Transcripts {
		if _, live := current.liveTranscripts[transcript.UUID]; live {
			continue
		}
		row := current.transcriptRow(transcript, ResumeClaude)
		row = current.applyKill(row, "cc")
		countOmitted(row, &output.KilledCount, &output.SuppressedCount)
		if input.Options.View == DefaultView {
			if defaultEligible(row) {
				claudeEligible++
				claudeResume = insertTopRow(
					claudeResume,
					row,
					claudeResumeCap,
				)
			}
		} else {
			claudeResume = append(claudeResume, row)
		}
	}
	if input.Options.View == DefaultView {
		if claudeEligible > claudeResumeCap {
			output.SuppressedCount += claudeEligible - claudeResumeCap
		}
		for _, row := range claudeResume {
			output.Rows = append(output.Rows, current.finalize(row))
		}
	} else {
		output.Rows = append(
			output.Rows,
			current.selectResumeRows(
				claudeResume,
				claudeResumeCap,
				&output.SuppressedCount,
			)...,
		)
	}

	codexResume := make([]Row, 0)
	codexEligible := 0
	for _, lineage := range current.codexLineages {
		if _, live := current.liveRollouts[lineage.RootID]; live {
			continue
		}
		row := current.rolloutRow(lineage.Newest, ResumeCodex)
		row = current.applyKill(row, "cx")
		countOmitted(row, &output.KilledCount, &output.SuppressedCount)
		if input.Options.View == DefaultView {
			if defaultEligible(row) {
				codexEligible++
				codexResume = insertTopRow(
					codexResume,
					row,
					codexResumeCap,
				)
			}
		} else {
			codexResume = append(codexResume, row)
		}
	}
	if input.Options.View == DefaultView {
		if codexEligible > codexResumeCap {
			output.SuppressedCount += codexEligible - codexResumeCap
		}
		for _, row := range codexResume {
			output.Rows = append(output.Rows, current.finalize(row))
		}
	} else {
		output.Rows = append(
			output.Rows,
			current.selectResumeRows(
				codexResume,
				codexResumeCap,
				&output.SuppressedCount,
			)...,
		)
	}

	output.Rows, output.ProjectOrder = sortProjectRows(output.Rows)
	output = leadWithCurrentProject(output, input.Options.CurrentDir)
	output = withNewRows(output)
	return output
}

func (current *composer) buildIndexes() {
	current.codexLineages, current.lineageRootByID =
		store.ResolveCodexLineages(current.input.Rollouts)
	current.lineageByRoot = make(
		map[string]store.CodexLineage,
		len(current.codexLineages),
	)
	for _, lineage := range current.codexLineages {
		current.lineageByRoot[lineage.RootID] = lineage
	}

	wantedTranscriptPaths := make(map[string]struct{}, len(current.input.Snapshot.Crumbs))
	wantedTranscriptIDs := make(map[string]struct{}, len(current.input.Snapshot.Crumbs))
	for _, crumb := range current.input.Snapshot.Crumbs {
		wantedTranscriptPaths[cleanPath(crumb.TranscriptPath)] = struct{}{}
		wantedTranscriptIDs[transcriptIDFromPath(crumb.TranscriptPath)] = struct{}{}
	}
	wantedAgentIDs := make(map[string]struct{}, len(current.input.Snapshot.Agents))
	for _, agent := range current.input.Snapshot.Agents {
		wantedAgentIDs[agent.SessionID] = struct{}{}
	}
	wantedRolloutPaths := make(map[string]struct{}, len(current.input.Snapshot.Codex))
	wantedRolloutIDs := make(map[string]struct{}, len(current.input.Snapshot.Codex))
	for _, process := range current.input.Snapshot.Codex {
		wantedRolloutPaths[cleanPath(process.RolloutPath)] = struct{}{}
		wantedRolloutIDs[liveCodexID(process)] = struct{}{}
	}
	current.transcriptByID = make(
		map[string]store.Transcript,
		len(wantedAgentIDs)+len(wantedTranscriptIDs),
	)
	current.transcriptByPath = make(
		map[string]store.Transcript,
		len(wantedTranscriptPaths),
	)
	directories := make(map[string]projectDirectory)
	if current.input.Options.CurrentDir != "" {
		project := projectName(current.input.Options.CurrentDir)
		directories[project] = projectDirectory{
			path:   cleanPath(current.input.Options.CurrentDir),
			seeded: true,
		}
	}
	for _, transcript := range current.input.Transcripts {
		_, wantedAgent := wantedAgentIDs[transcript.UUID]
		_, wantedLive := wantedTranscriptIDs[transcript.UUID]
		if wantedAgent || wantedLive {
			current.transcriptByID[transcript.UUID] = transcript
		}
		normalizedPath := cleanPath(transcript.Path)
		if _, wanted := wantedTranscriptPaths[normalizedPath]; wanted {
			current.transcriptByPath[normalizedPath] = transcript
		}
		rememberProjectDir(directories, transcript.CWD, transcript.MTimeNS)
	}
	current.rolloutByPath = make(map[string]store.Rollout, len(wantedRolloutPaths))
	current.rolloutByID = make(map[string]store.Rollout, len(wantedRolloutIDs))
	for _, rollout := range current.input.Rollouts {
		normalizedPath := cleanPath(rollout.Path)
		if _, wanted := wantedRolloutPaths[normalizedPath]; wanted {
			current.rolloutByPath[normalizedPath] = rollout
		}
		if _, wanted := wantedRolloutIDs[rollout.ID]; wanted {
			current.rolloutByID[rollout.ID] = rollout
		}
		if rollout.UserThread {
			rememberProjectDir(directories, rollout.CWD, rollout.MTimeNS)
		}
	}
	current.projectDirs = make(map[string]string, len(directories))
	for project, directory := range directories {
		current.projectDirs[project] = directory.path
	}
	current.killedByID = make(map[string]store.Killed, len(current.input.Killed))
	for _, killed := range current.input.Killed {
		current.killedByID[killed.ID] = killed
	}
	current.panesBySocket = make(map[string][]gather.Pane)
	current.paneByTarget = make(map[string]gather.Pane, len(current.input.Snapshot.Panes))
	for _, pane := range current.input.Snapshot.Panes {
		current.panesBySocket[pane.Socket] = append(
			current.panesBySocket[pane.Socket],
			pane,
		)
		current.paneByTarget[targetKey(pane.Socket, pane.PaneID)] = pane
	}
	for socket := range current.panesBySocket {
		sort.Slice(current.panesBySocket[socket], func(left, right int) bool {
			return current.panesBySocket[socket][left].PaneID <
				current.panesBySocket[socket][right].PaneID
		})
	}
	current.claudeSockets = make(map[string]struct{})
	for _, process := range current.input.Snapshot.ClaudeProcesses {
		current.claudeSockets[process.Socket] = struct{}{}
	}
	// Agent rows are themselves live Claude processes. Counting them here also
	// keeps hand-built snapshots backward-compatible with pre-WP5 fixtures.
	for _, agent := range current.input.Snapshot.Agents {
		if agent.Socket != "" {
			current.claudeSockets[agent.Socket] = struct{}{}
		}
	}
	current.cacheSockets = make(map[string]struct{}, len(current.input.Snapshot.Cache1HSockets))
	for _, socket := range current.input.Snapshot.Cache1HSockets {
		current.cacheSockets[socket] = struct{}{}
	}
	current.liveTranscripts = make(map[string]struct{})
	current.liveRollouts = make(map[string]struct{})
	current.accountRoots = make([]AccountRoot, 0, len(current.input.AccountRoots))
	for _, root := range current.input.AccountRoots {
		root.Path = canonicalPath(root.Path)
		current.accountRoots = append(current.accountRoots, root)
	}
	current.canonicalPaths = make(map[string]string, len(current.input.Transcripts)+len(current.input.Snapshot.Agents))
	for _, transcript := range current.input.Transcripts {
		current.canonicalPaths[transcript.Path] = canonicalPath(transcript.Path)
	}
	for _, agent := range current.input.Snapshot.Agents {
		current.canonicalPaths[agent.ConfigDir] = canonicalPath(agent.ConfigDir)
	}
}

func canonicalPath(path string) string {
	if path == "" {
		return ""
	}
	cleaned := cleanPath(path)
	if absolute, err := filepath.Abs(cleaned); err == nil {
		cleaned = cleanPath(absolute)
	}
	probe := cleaned
	suffix := make([]string, 0, 4)
	for {
		if resolved, err := filepath.EvalSymlinks(probe); err == nil {
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return cleanPath(resolved)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return cleaned
		}
		suffix = append(suffix, filepath.Base(probe))
		probe = parent
	}
}

func (current *composer) accountFor(path string) int {
	if canonical, ok := current.canonicalPaths[path]; ok {
		return accountForPath(canonical, current.accountRoots)
	}
	return accountForPath(path, current.accountRoots)
}

type crumbsForSocket struct {
	socket *gather.Crumb
	panes  map[string]gather.Crumb
}

func (current *composer) liveClaudeRows() ([]Row, []Row) {
	crumbs := make(map[string]*crumbsForSocket)
	for index := range current.input.Snapshot.Crumbs {
		crumb := current.input.Snapshot.Crumbs[index]
		if !strings.HasPrefix(crumb.Socket, "cc-") {
			continue
		}
		socketCrumbs := crumbs[crumb.Socket]
		if socketCrumbs == nil {
			socketCrumbs = &crumbsForSocket{panes: make(map[string]gather.Crumb)}
			crumbs[crumb.Socket] = socketCrumbs
		}
		if crumb.PaneID == "" {
			if socketCrumbs.socket == nil ||
				crumb.Filename < socketCrumbs.socket.Filename {
				copy := crumb
				socketCrumbs.socket = &copy
			}
			continue
		}
		if _, live := current.paneByTarget[targetKey(crumb.Socket, crumb.PaneID)]; !live {
			continue
		}
		incumbent, found := socketCrumbs.panes[crumb.PaneID]
		if !found || crumb.Filename < incumbent.Filename {
			socketCrumbs.panes[crumb.PaneID] = crumb
		}
	}

	sockets := make([]string, 0, len(crumbs))
	for socket := range crumbs {
		sockets = append(sockets, socket)
	}
	sort.Strings(sockets)

	rows := make([]Row, 0, len(sockets))
	splits := make([]Row, 0)
	for _, socket := range sockets {
		panes := current.panesBySocket[socket]
		if len(panes) == 0 {
			continue
		}
		socketCrumbs := crumbs[socket]
		paneIDs := make([]string, 0, len(socketCrumbs.panes))
		for paneID := range socketCrumbs.panes {
			paneIDs = append(paneIDs, paneID)
		}
		sort.Strings(paneIDs)
		if len(paneIDs) >= 2 {
			splits = append(splits, current.splitRow(socket, paneIDs, socketCrumbs.panes))
			continue
		}

		var crumb gather.Crumb
		var pane gather.Pane
		if len(paneIDs) == 1 {
			crumb = socketCrumbs.panes[paneIDs[0]]
			pane = current.paneByTarget[targetKey(socket, paneIDs[0])]
		} else {
			if socketCrumbs.socket == nil {
				continue
			}
			if _, running := current.claudeSockets[socket]; !running {
				continue
			}
			crumb = *socketCrumbs.socket
			pane = panes[0]
		}
		row, id := current.liveClaudeRow(socket, pane, crumb.TranscriptPath)
		if id != "" {
			current.liveTranscripts[id] = struct{}{}
		}
		rows = append(rows, row)
	}
	return rows, splits
}

func (current *composer) liveClaudeRow(
	socket string,
	pane gather.Pane,
	path string,
) (Row, string) {
	transcript, found := current.transcriptByPath[cleanPath(path)]
	if !found {
		transcript, found = current.transcriptByID[transcriptIDFromPath(path)]
	}
	if !found {
		transcript = store.Transcript{
			UUID: transcriptIDFromPath(path),
			Path: path,
		}
	}
	row := current.transcriptRow(transcript, LiveClaude)
	row.Socket = socket
	row.PaneID = pane.PaneID
	row.PanePIDs = []int{pane.PID}
	row.SessionName = pane.SessionName
	row.ServerCount = 1
	row.Attached = pane.Attached
	row.Here = socket == current.input.Options.CurrentSocket
	_, row.C1H = current.cacheSockets[socket]
	if row.CWD == "" && pane.CurrentPath != "" {
		row.CWD = pane.CurrentPath
		row.Project = projectName(pane.CurrentPath)
	}
	indexed := naming.DisplayName(
		transcript.CustomTitle,
		transcript.AITitle,
		transcript.FirstPrompt,
	)
	row.Name = naming.LiveFallback(
		indexed,
		pane.PaneTitle,
		pane.SessionName,
		transcript.LastPrompt,
		true,
	)
	if row.ActivityNS == 0 {
		row.ActivityNS = socketEpochNS(socket)
	}
	return row, transcript.UUID
}

func (current *composer) splitRow(
	socket string,
	paneIDs []string,
	crumbs map[string]gather.Crumb,
) Row {
	row := Row{
		Kind:        LiveSplit,
		Socket:      socket,
		ServerCount: 1,
		SplitCount:  len(paneIDs),
		Here:        socket == current.input.Options.CurrentSocket,
	}
	_, row.C1H = current.cacheSockets[socket]
	names := make([]string, 0, len(paneIDs))
	accounts := make(map[int]struct{})
	for _, paneID := range paneIDs {
		pane := current.paneByTarget[targetKey(socket, paneID)]
		row.PanePIDs = append(row.PanePIDs, pane.PID)
		crumb := crumbs[paneID]
		transcript, found := current.transcriptByPath[cleanPath(crumb.TranscriptPath)]
		if !found {
			transcript, found = current.transcriptByID[transcriptIDFromPath(
				crumb.TranscriptPath,
			)]
		}
		if !found {
			transcript = store.Transcript{
				UUID: transcriptIDFromPath(crumb.TranscriptPath),
				Path: crumb.TranscriptPath,
			}
		}
		if transcript.UUID != "" {
			current.liveTranscripts[transcript.UUID] = struct{}{}
		}
		indexed := naming.DisplayName(
			transcript.CustomTitle,
			transcript.AITitle,
			transcript.FirstPrompt,
		)
		name := naming.LiveFallback(
			indexed,
			pane.PaneTitle,
			pane.SessionName,
			transcript.LastPrompt,
			true,
		)
		if name == "(unnamed)" {
			name = "?"
		}
		names = append(names, name)
		row.Size += transcript.Size
		row.PromptCount += transcript.PromptCount
		row.BG = row.BG || transcript.IsBG
		row.Attached = row.Attached || pane.Attached
		if transcript.MTimeNS >= row.ActivityNS {
			row.ActivityNS = transcript.MTimeNS
			row.CWD = transcript.CWD
			if row.CWD == "" {
				row.CWD = pane.CurrentPath
			}
			row.Path = transcript.Path
			row.LastPrompt = transcript.LastPrompt
		}
		if account := current.accountFor(transcript.Path); account != 0 {
			accounts[account] = struct{}{}
		}
	}
	row.Name = strings.Join(names, "+")
	if row.Name == "" {
		row.Name = "(split)"
	}
	if row.ActivityNS == 0 {
		row.ActivityNS = socketEpochNS(socket)
	}
	row.Project = projectName(row.CWD)
	row.Accounts = sortedIntKeys(accounts)
	return row
}

func (current *composer) liveCodexRows() []Row {
	live := append([]gather.LiveCodex(nil), current.input.Snapshot.Codex...)
	sort.Slice(live, func(left, right int) bool {
		if live[left].Socket != live[right].Socket {
			return live[left].Socket < live[right].Socket
		}
		return live[left].PID < live[right].PID
	})
	rows := make([]Row, 0, len(live))
	for _, process := range live {
		pane, paneFound := current.paneByTarget[targetKey(
			process.Socket,
			process.PaneID,
		)]
		if !paneFound {
			// A responsive cx-* server is not a live Codex chat. The fd-walk
			// process must still resolve to a pane that exists in this same
			// gather snapshot; otherwise its indexed rollout remains resumable.
			continue
		}
		rollout, found := current.rolloutByPath[cleanPath(process.RolloutPath)]
		if !found {
			rollout, found = current.rolloutByID[liveCodexID(process)]
		}
		if !found {
			rollout = store.Rollout{
				ID:   liveCodexID(process),
				Path: process.RolloutPath,
			}
		}
		root := current.lineageRoot(rollout)
		if root != "" {
			current.liveRollouts[root] = struct{}{}
		}
		row := current.rolloutRow(rollout, LiveCodex)
		row.Socket = process.Socket
		row.PaneID = process.PaneID
		row.PanePIDs = []int{process.PanePID}
		row.ServerCount = 1
		row.Here = process.Socket == current.input.Options.CurrentSocket
		row.SessionName = pane.SessionName
		row.WindowName = pane.WindowName
		row.Attached = pane.Attached
		if row.CWD == "" && pane.CurrentPath != "" {
			row.CWD = pane.CurrentPath
			row.Project = projectName(pane.CurrentPath)
		}
		if row.Name == "" {
			row.Name = "Codex chat"
		}
		if row.ActivityNS == 0 {
			row.ActivityNS = socketEpochNS(process.Socket)
		}
		rows = append(rows, row)
	}
	return rows
}

// bootingRows synthesizes one row per crumbless-live entry gather found: a
// chat with a live pane and process but no SID crumb yet, because its
// statusline has not rendered a first time. There is no transcript identity
// to key on — the socket IS the identity, exactly as a fresh cc-new-* socket
// has no other name either.
func (current *composer) bootingRows() []Row {
	entries := current.input.Snapshot.CrumblessLive
	if len(entries) == 0 {
		return nil
	}
	rows := make([]Row, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.WindowName)
		if name == "" && entry.SessionName != entry.Socket {
			name = strings.TrimSpace(entry.SessionName)
		}
		if name == "" {
			name = "booting…"
		}
		row := Row{
			Kind:        Booting,
			ID:          entry.Socket,
			Socket:      entry.Socket,
			PaneID:      entry.PaneID,
			PanePIDs:    []int{entry.PID},
			SessionName: entry.SessionName,
			WindowName:  entry.WindowName,
			Name:        name,
			CWD:         entry.CWD,
			Project:     projectName(entry.CWD),
			ServerCount: 1,
			Here:        entry.Socket == current.input.Options.CurrentSocket,
			ActivityNS:  paneStartActivityNS(entry.PaneStartUnix),
		}
		_, row.C1H = current.cacheSockets[entry.Socket]
		if row.ActivityNS == 0 {
			row.ActivityNS = socketEpochNS(entry.Socket)
		}
		rows = append(rows, row)
	}
	return rows
}

func paneStartActivityNS(paneStartUnix int64) int64 {
	if paneStartUnix <= 0 {
		return 0
	}
	return paneStartUnix * 1_000_000_000
}

func (current *composer) agentRows() []Row {
	agents := make(map[string]gather.Agent)
	for _, agent := range current.input.Snapshot.Agents {
		if agent.SessionID == "" {
			continue
		}
		incumbent, found := agents[agent.SessionID]
		if !found || agentSortKey(agent) < agentSortKey(incumbent) {
			agents[agent.SessionID] = agent
		}
	}
	ids := make([]string, 0, len(agents))
	for id := range agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rows := make([]Row, 0, len(ids))
	for _, id := range ids {
		if _, live := current.liveTranscripts[id]; live {
			continue
		}
		agent := agents[id]
		transcript, found := current.transcriptByID[id]
		if !found {
			transcript = store.Transcript{UUID: id}
		}
		current.liveTranscripts[id] = struct{}{}
		row := current.transcriptRow(transcript, Agent)
		row.Socket = agent.Socket
		row.PaneID = agent.PaneID
		row.ConfigDir = agent.ConfigDir
		row.ServerCount = boolInt(agent.Socket != "")
		row.Here = agent.Socket == current.input.Options.CurrentSocket
		if pane, found := current.paneByTarget[targetKey(agent.Socket, agent.PaneID)]; found {
			row.PanePIDs = []int{pane.PID}
			row.SessionName = pane.SessionName
			row.WindowName = pane.WindowName
			row.Attached = pane.Attached
		}
		if row.Name == "" {
			row.Name = "(no prompt)"
		}
		if row.Account == 0 {
			row.Account = current.accountFor(agent.ConfigDir)
		}
		_, row.C1H = current.cacheSockets[agent.Socket]
		rows = append(rows, row)
	}
	return rows
}

func (current *composer) transcriptRow(
	transcript store.Transcript,
	kind Kind,
) Row {
	return Row{
		Kind: kind,
		ID:   transcript.UUID,
		Path: transcript.Path,
		Name: naming.DisplayName(
			transcript.CustomTitle,
			transcript.AITitle,
			transcript.FirstPrompt,
		),
		LastPrompt:  transcript.LastPrompt,
		Project:     projectName(transcript.CWD),
		CWD:         transcript.CWD,
		Size:        transcript.Size,
		PromptCount: transcript.PromptCount,
		ActivityNS:  transcript.MTimeNS,
		Account:     current.accountFor(transcript.Path),
		BG:          transcript.IsBG,
	}
}

func (current *composer) rolloutRow(rollout store.Rollout, kind Kind) Row {
	root := current.lineageRoot(rollout)
	newest := rollout
	if lineage, found := current.lineageByRoot[root]; found {
		newest = lineage.Newest
		newest.PromptCount = lineage.PromptCount
	}
	name := naming.CodexRowName(
		newest.ID,
		newest.SessionID,
		newest.ParentThread,
		root,
		newest.FirstPrompt,
		current.input.CxNames,
	)
	return Row{
		Kind:        kind,
		ID:          root,
		Path:        newest.Path,
		Name:        name,
		Project:     projectName(newest.CWD),
		CWD:         newest.CWD,
		Size:        newest.Size,
		PromptCount: newest.PromptCount,
		ActivityNS:  newest.MTimeNS,
		BG:          newest.IsBG,
	}
}

func (current *composer) lineageRoot(rollout store.Rollout) string {
	if root := current.lineageRootByID[rollout.ID]; root != "" {
		return root
	}
	if rollout.LineageRoot != "" {
		return rollout.LineageRoot
	}
	if rollout.SessionID != "" {
		return rollout.SessionID
	}
	if rollout.ParentThread != "" {
		return rollout.ParentThread
	}
	return rollout.ID
}

func (current *composer) applyKill(row Row, engine string) Row {
	// A "_KILL…" label is a kill the user writes by renaming the chat, so it
	// needs no id, no store row and no picker keystroke — which is why it is
	// tested BEFORE every id-shaped guard below and applies to a live chat the
	// index has never seen. A split row is the one exclusion: its Name is a
	// join of its panes' names, not a label anyone set on one chat.
	if row.Kind != LiveSplit && naming.LabelKilled(row.Name) {
		row.Killed = true
		row.NameKilled = true
		return row
	}
	// A Booting row's ID is the crumbless socket name, not a chat identity —
	// unlike LiveSplit's empty-ID case, it WOULD pass the id check below, so
	// it needs its own guard. The socket is reused by the picker's own
	// kill-eligibility test (ui/model.go's toggleKilled): neither side may let
	// a kill land on an identity that stops meaning anything the moment the
	// crumb appears and the row becomes an ordinary live one.
	if row.Kind == LiveSplit || row.Kind == Booting || row.ID == "" {
		return row
	}
	// Explicit kills carry no baseline and stay permanent. A /clear kill is a
	// prompt-count ratchet: once this row grows past its stored baseline it is
	// visible again, matching the store's persisted auto-unkill pass.
	row.Killed = current.killedMatch(row.ID, engine, row.PromptCount)
	return row
}

// killedMatch reports whether id — or, for a Codex row, ANY id in its resume
// lineage — carries a kill whose engine agrees.
//
// A live Codex process exposes the raw id of its current rollout, which can be
// the resumed CHILD's id
// on a multi-file lineage, not the ROOT this row is keyed on (rolloutRow,
// liveCodexRows) — a kill written that way lands on a key nothing else here
// reads unless every member id is checked too. A kill the `kill` manager
// itself writes is already normalized onto the root
// (internal/kill/manager.go), so this lineage walk only ever WIDENS what
// matches, never narrows it.
//
// See also codexLineageKilled (store/queries.go), the cached first frame's
// copy of this same question — the two must never disagree about what the
// user sees.
func (current *composer) killedMatch(id, engine string, promptCount int64) bool {
	if killMatchesID(current.killedByID, id, engine, promptCount) {
		return true
	}
	if engine != "cx" {
		return false
	}
	lineage, found := current.lineageByRoot[id]
	if !found {
		return false
	}
	for _, member := range lineage.MemberIDs {
		if killMatchesID(current.killedByID, member, engine, promptCount) {
			return true
		}
	}
	return false
}

func killMatchesID(
	killedByID map[string]store.Killed,
	id, engine string,
	promptCount int64,
) bool {
	killed, found := killedByID[id]
	if !found {
		return false
	}
	if killed.Engine != "" && killed.Engine != engine {
		return false
	}
	return killed.BaselinePrompts == nil || promptCount <= *killed.BaselinePrompts
}

func (current *composer) selectResumeRows(
	rows []Row,
	capacity int,
	suppressedCount *int,
) []Row {
	switch current.input.Options.View {
	case AllView:
		selected := make([]Row, 0, len(rows))
		for _, row := range rows {
			selected = append(selected, current.finalize(row))
		}
		defaultRows := defaultEligibleCount(rows)
		if defaultRows > capacity {
			*suppressedCount += defaultRows - capacity
		}
		return selected
	case KilledView:
		selected := make([]Row, 0)
		for _, row := range rows {
			if row.Killed {
				selected = append(selected, current.finalize(row))
			}
		}
		defaultRows := defaultEligibleCount(rows)
		if defaultRows > capacity {
			*suppressedCount += defaultRows - capacity
		}
		return selected
	default:
		panic("default resume rows are selected while scanning")
	}
}

func countOmitted(row Row, killedCount, suppressedCount *int) {
	if row.Killed {
		*killedCount++
		return
	}
	if !defaultEligible(row) {
		*suppressedCount++
	}
}

func (current *composer) finalize(row Row) Row {
	if current.input.Options.NowNS > row.ActivityNS && row.ActivityNS > 0 {
		row.AgeNS = current.input.Options.NowNS - row.ActivityNS
	}
	return row
}

// defaultEligible answers whether a row belongs in the default listing.
//
// THE KILL IS TESTED FIRST, before any liveness short-circuit: killed is dead,
// live or not. A split row is the one row with no single id to kill — compose
// never marks it — so it short-circuits above the test rather than around it.
func defaultEligible(row Row) bool {
	if row.Killed {
		return false
	}
	if row.Kind == LiveSplit {
		return true
	}
	// A live agent is exempt from the emptiness tests below and from those
	// ALONE: it is rowed from its running process, so its transcript is often
	// still zero bytes with no prompt parsed out of it.
	if row.Kind == Agent {
		return true
	}
	// A booting row has no transcript at all — it exists BECAUSE the crumb
	// that would normally lead compose to one has not been written yet — so
	// the emptiness test below would suppress every one of them from the
	// default view and silently defeat this entire fix.
	if row.Kind == Booting {
		return true
	}
	// A live Codex row is exempt from the SIZE/PROMPT half of the test below
	// for the same reason: Codex >=0.146.1 keeps a paginated thread's content
	// in its own sqlite state store, so the rollout file gather and the index
	// parse from can carry prompt_count=0 and size=0 even while the chat is
	// genuinely running in tmux right now. The index enriches such a row from
	// the state store once it catches up (applyCodexThread); until then, a
	// row this self-evidently real must never be judged by a test built to
	// catch an abandoned zero-byte spawn.
	//
	// The BG half is NOT waived: a machine-spawned thread (codex exec, never
	// renamed) caught live in a pane is still background work, not a chat,
	// exactly like its resume-shape twin — the exemption is for genuinely
	// empty content, never for who started the conversation.
	if row.Kind == LiveCodex {
		return !row.BG
	}
	return !row.BG && row.Size > 0 && row.PromptCount > 0
}

func visibleInView(row Row, view View) bool {
	switch view {
	case AllView:
		return true
	case KilledView:
		return row.Killed
	default:
		return defaultEligible(row)
	}
}

func defaultEligibleCount(rows []Row) int {
	count := 0
	for _, row := range rows {
		if defaultEligible(row) {
			count++
		}
	}
	return count
}

func collapseLiveServers(rows []Row) []Row {
	type winner struct {
		row   Row
		count int
	}
	winners := make(map[string]winner)
	standalone := make([]Row, 0)
	for _, row := range rows {
		if row.ID == "" || row.Kind == LiveSplit {
			standalone = append(standalone, row)
			continue
		}
		key := EngineForKind(row.Kind) + "\x00" + row.ID
		incumbent, found := winners[key]
		if !found {
			winners[key] = winner{row: row, count: 1}
			continue
		}
		incumbent.count++
		if newerSocket(row.Socket, incumbent.row.Socket) {
			incumbent.row = row
		}
		winners[key] = incumbent
	}
	keys := make([]string, 0, len(winners))
	for key := range winners {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := winners[key]
		entry.row.ServerCount = entry.count
		standalone = append(standalone, entry.row)
	}
	return standalone
}

func newerSocket(challenger, incumbent string) bool {
	challengerEpoch := socketEpoch(challenger)
	incumbentEpoch := socketEpoch(incumbent)
	if challengerEpoch != incumbentEpoch {
		return challengerEpoch > incumbentEpoch
	}
	return challenger > incumbent
}

func socketEpoch(socket string) int64 {
	parts := strings.Split(socket, "-")
	if len(parts) != 4 || (parts[0] != "cc" && parts[0] != "cx") {
		return 0
	}
	epoch, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || epoch < 0 {
		return 0
	}
	return epoch
}

func socketEpochNS(socket string) int64 {
	epoch := socketEpoch(socket)
	if epoch <= 0 || epoch > (1<<63-1)/1_000_000_000 {
		return 0
	}
	return epoch * 1_000_000_000
}

func targetKey(socket, paneID string) string {
	return socket + "\x00" + paneID
}

func transcriptIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// liveCodexID names the conversation behind a live Codex process. The rollout
// filename carries it whenever the process holds a rollout file, and gather's
// own resolution is the only identity a session that writes no rollout file
// has — Codex 0.146.1 stopped writing one for a paginated thread.
//
// Deriving it from the path ALONE mints the empty id for such a process, and
// an empty id is a row that cannot be killed (applyKill and the picker both
// refuse one) and that never marks its conversation live — so the very same
// chat also comes back as a resume row underneath itself.
func liveCodexID(process gather.LiveCodex) string {
	if id := rolloutIDFromPath(process.RolloutPath); id != "" {
		return id
	}
	return process.ThreadID
}

func rolloutIDFromPath(path string) string {
	// The extension comes off the BASE, not off the whole path: for the empty
	// path Base is "." and Ext of the whole path is "", which left the stem as
	// "." — a bogus non-empty id that every pathless live process shared.
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	rest := strings.TrimPrefix(stem, "rollout-")
	if len(rest) > 20 &&
		rest[4] == '-' &&
		rest[7] == '-' &&
		rest[10] == 'T' &&
		rest[13] == '-' &&
		rest[16] == '-' &&
		rest[19] == '-' {
		return rest[20:]
	}
	return rest
}

func projectName(cwd string) string {
	if cwd == "" {
		return "?"
	}
	trimmed := strings.TrimRight(cwd, string(filepath.Separator))
	if trimmed == "" {
		return "?"
	}
	index := strings.LastIndexByte(trimmed, byte(filepath.Separator))
	project := trimmed[index+1:]
	if project == "." || project == ".." || project == "" {
		return "?"
	}
	return project
}

func accountForPath(path string, roots []AccountRoot) int {
	if path == "" {
		return 0
	}
	normalizedPath := cleanPath(path)
	account := 0
	longest := -1
	for _, root := range roots {
		if root.Account < 1 || root.Path == "" {
			continue
		}
		cleanRoot := root.Path
		if normalizedPath != cleanRoot &&
			!strings.HasPrefix(normalizedPath, cleanRoot+string(filepath.Separator)) {
			continue
		}
		if len(cleanRoot) > longest {
			longest = len(cleanRoot)
			account = root.Account
		}
	}
	return account
}

func configuredAccount(roots []AccountRoot, account int) bool {
	for _, root := range roots {
		if root.Account == account {
			return true
		}
	}
	return false
}

// EngineForKind reports the engine ("cc" or "cx") a row's kind belongs to —
// exported so a caller resolving an id against a compose pass (cmd/pfm's
// CLI kill) can vouch for the same engine the picker itself would.
func EngineForKind(kind Kind) string {
	switch kind {
	case LiveCodex, ResumeCodex, NewCodex:
		return "cx"
	default:
		return "cc"
	}
}

func agentSortKey(agent gather.Agent) string {
	return agent.Socket + "\x00" + agent.PaneID + "\x00" + strconv.Itoa(agent.PID)
}

func sortRowsByActivity(rows []Row) {
	sort.Slice(rows, func(left, right int) bool {
		return rowComesBefore(rows[left], rows[right])
	})
}

func rowComesBefore(left, right Row) bool {
	if left.ActivityNS != right.ActivityNS {
		return left.ActivityNS > right.ActivityNS
	}
	if left.ID != right.ID {
		return left.ID < right.ID
	}
	if left.Socket != right.Socket {
		return left.Socket < right.Socket
	}
	return left.Kind < right.Kind
}

func insertTopRow(rows []Row, row Row, capacity int) []Row {
	insert := sort.Search(len(rows), func(position int) bool {
		return rowComesBefore(row, rows[position])
	})
	if insert >= capacity {
		return rows
	}
	rows = append(rows, Row{})
	copy(rows[insert+1:], rows[insert:])
	rows[insert] = row
	if len(rows) > capacity {
		rows = rows[:capacity]
	}
	return rows
}

func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	if !strings.Contains(path, "//") &&
		!strings.Contains(path, "/./") &&
		!strings.Contains(path, "/../") &&
		!strings.HasSuffix(path, "/.") &&
		!strings.HasSuffix(path, "/..") {
		if path == string(filepath.Separator) {
			return path
		}
		return strings.TrimSuffix(path, string(filepath.Separator))
	}
	return filepath.Clean(path)
}

func sortedIntKeys(values map[int]struct{}) []int {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
