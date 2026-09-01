package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hostops/pfm/internal/professor"
)

type projectStatus string

const (
	projectCurrent      projectStatus = "current"
	projectUpdated      projectStatus = "UPDATED"
	projectNew          projectStatus = "NEW"
	projectGoneUpstream projectStatus = "GONE-UPSTREAM"
	projectLocalDeleted projectStatus = "LOCAL-DELETED"
)

var projectStatusOrder = []projectStatus{
	projectCurrent,
	projectUpdated,
	projectNew,
	projectGoneUpstream,
	projectLocalDeleted,
}

type projectReportItem struct {
	Status   projectStatus     `json:"status"`
	Local    string            `json:"local,omitempty"`
	Template string            `json:"template"`
	Pin      professor.FilePin `json:"pin,omitempty"`
}

type projectReport struct {
	Root     string
	Store    professor.Store
	Baseline professor.Baseline
	Counts   map[projectStatus]int
	Items    []projectReportItem
}

func (report projectReport) reviewRequired() int {
	return report.Counts[projectUpdated] + report.Counts[projectNew] + report.Counts[projectGoneUpstream] + report.Counts[projectLocalDeleted]
}

func runProjectUpdate(action string, args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	switch action {
	case "check":
		flags := newFlagSet("update check", "usage: pfm update check [--root DIR] [--json]", stderr)
		rootFlag := flags.String("root", "", "project root")
		jsonOutput := flags.Bool("json", false, "write one JSON object")
		positional, code, ok := parseFlagsAnywhere(flags, args)
		if !ok {
			return code
		}
		if len(positional) != 0 {
			flags.Usage()
			return 2
		}
		root, found, err := resolveProjectRoot(*rootFlag)
		if err != nil {
			writeProjectFailure(stdout, *jsonOutput, err)
			return 1
		}
		if !found {
			writeProjectFailure(stdout, *jsonOutput, errors.New(".professor/baseline.json not found"))
			return 1
		}
		return renderProjectCheck(root, runtime.Paths.Home, *jsonOutput, stdout)
	case "pin":
		return runProjectPin(args, stdout, stderr, runtime)
	case "drop":
		return runProjectDrop(args, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "pfm update: unknown project action %q\n", action)
		return 2
	}
}

func renderProjectCheck(root, home string, jsonOutput bool, stdout io.Writer) int {
	report, err := buildProjectReport(root, home)
	if err != nil {
		writeProjectFailure(stdout, jsonOutput, err)
		return 1
	}
	if jsonOutput {
		if err := writeProjectJSON(stdout, report); err != nil {
			writeProjectFailure(stdout, false, err)
			return 1
		}
	} else {
		writeProjectHuman(stdout, report)
	}
	if report.reviewRequired() != 0 {
		return 3
	}
	return 0
}

func buildProjectReport(root, home string) (projectReport, error) {
	baseline, err := professor.Load(root)
	if err != nil {
		return projectReport{}, err
	}
	store, err := professor.ResolveStore(root, home)
	if err != nil {
		return projectReport{}, err
	}
	report := projectReport{
		Root:     root,
		Store:    store,
		Baseline: baseline,
		Counts:   make(map[projectStatus]int, len(projectStatusOrder)),
	}
	for _, status := range projectStatusOrder {
		report.Counts[status] = 0
	}
	pinnedTemplates := make(map[string]bool, len(baseline.Files))
	locals := make([]string, 0, len(baseline.Files))
	for local := range baseline.Files {
		locals = append(locals, local)
	}
	sort.Strings(locals)
	for _, local := range locals {
		pin := baseline.Files[local]
		local, err := safeProjectRelative(local)
		if err != nil {
			return projectReport{}, fmt.Errorf("baseline local %q: %w", local, err)
		}
		template, err := safeTemplateRelative(pin.Template)
		if err != nil {
			return projectReport{}, fmt.Errorf("baseline template %q: %w", pin.Template, err)
		}
		pinnedTemplates[template] = true
		templatePath := filepath.Join(store.Templates, filepath.FromSlash(template))
		status := projectCurrent
		if _, err := os.Stat(templatePath); errors.Is(err, fs.ErrNotExist) {
			status = projectGoneUpstream
		} else if err != nil {
			return projectReport{}, fmt.Errorf("UNREADABLE %s: %w", templatePath, err)
		} else {
			localPath := filepath.Join(root, filepath.FromSlash(local))
			if _, err := os.Stat(localPath); errors.Is(err, fs.ErrNotExist) {
				status = projectLocalDeleted
			} else if err != nil {
				return projectReport{}, fmt.Errorf("UNREADABLE %s: %w", localPath, err)
			} else {
				hash, err := professor.HashTemplate(templatePath)
				if err != nil {
					return projectReport{}, err
				}
				if hash != pin.TemplateHash {
					status = projectUpdated
				}
			}
		}
		report.Counts[status]++
		if status != projectCurrent {
			report.Items = append(report.Items, projectReportItem{Status: status, Local: local, Template: template, Pin: pin})
		}
	}
	projectTemplates := filepath.Join(store.Templates, "project")
	if err := filepath.WalkDir(projectTemplates, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("UNREADABLE %s: %w", path, walkErr)
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("UNREADABLE %s: symlink templates are unsupported", path)
		}
		relative, err := filepath.Rel(store.Templates, path)
		if err != nil {
			return err
		}
		template := filepath.ToSlash(relative)
		if pinnedTemplates[template] {
			return nil
		}
		if _, err := professor.HashTemplate(path); err != nil {
			return err
		}
		report.Counts[projectNew]++
		report.Items = append(report.Items, projectReportItem{Status: projectNew, Template: template})
		return nil
	}); err != nil {
		return projectReport{}, err
	}
	sort.Slice(report.Items, func(i, j int) bool {
		if report.Items[i].Status != report.Items[j].Status {
			return statusIndex(report.Items[i].Status) < statusIndex(report.Items[j].Status)
		}
		if report.Items[i].Local != report.Items[j].Local {
			return report.Items[i].Local < report.Items[j].Local
		}
		return report.Items[i].Template < report.Items[j].Template
	})
	return report, nil
}

func statusIndex(status projectStatus) int {
	for index, candidate := range projectStatusOrder {
		if status == candidate {
			return index
		}
	}
	return len(projectStatusOrder)
}

func writeProjectHuman(stdout io.Writer, report projectReport) {
	fmt.Fprintf(stdout, "professor: %s  blueprint %s → %s\n", report.Root, report.Baseline.Blueprint.SHA, report.Store.SHA)
	for _, status := range projectStatusOrder {
		fmt.Fprintf(stdout, "  %-13s %d\n", status, report.Counts[status])
		for _, item := range report.Items {
			if item.Status != status {
				continue
			}
			switch status {
			case projectUpdated:
				fmt.Fprintf(stdout, "    %s   %s  pinned @%s\n", item.Local, item.Template, item.Pin.PinnedSHA)
				fmt.Fprintf(stdout, "      review: git -C %s diff %s..%s -- templates/%s\n", report.Store.Root, item.Pin.PinnedSHA, report.Store.SHA, item.Template)
				fmt.Fprintf(stdout, "      then apply by hand and: pfm update pin %s\n", item.Local)
			case projectNew:
				fmt.Fprintf(stdout, "    %s — adopt: copy/adapt it locally, then pfm update pin --template %s <local> — or ignore\n", item.Template, item.Template)
			case projectGoneUpstream:
				fmt.Fprintf(stdout, "    %s   %s — local file is YOURS now — keep it and pfm update drop %s, or delete both\n", item.Local, item.Template, item.Local)
			case projectLocalDeleted:
				fmt.Fprintf(stdout, "    %s   %s — pfm update drop %s to forget, or restore the file\n", item.Local, item.Template, item.Local)
			}
		}
	}
	if count := report.reviewRequired(); count != 0 {
		fmt.Fprintf(stdout, "REVIEW REQUIRED — %d items; nothing was written.\n", count)
		return
	}
	fmt.Fprintln(stdout, "clean")
}

func writeProjectJSON(stdout io.Writer, report projectReport) error {
	type blueprintJSON struct {
		Pinned  string `json:"pinned"`
		Current string `json:"current"`
	}
	payload := struct {
		Professor      string                `json:"professor"`
		Blueprint      blueprintJSON         `json:"blueprint"`
		Counts         map[projectStatus]int `json:"counts"`
		Items          []projectReportItem   `json:"items"`
		ReviewRequired int                   `json:"reviewRequired"`
		Terminal       string                `json:"terminal"`
	}{
		Professor:      report.Root,
		Blueprint:      blueprintJSON{Pinned: report.Baseline.Blueprint.SHA, Current: report.Store.SHA},
		Counts:         report.Counts,
		Items:          report.Items,
		ReviewRequired: report.reviewRequired(),
	}
	if payload.ReviewRequired == 0 {
		payload.Terminal = "clean"
	} else {
		payload.Terminal = fmt.Sprintf("REVIEW REQUIRED — %d items", payload.ReviewRequired)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode project report: %w", err)
	}
	return nil
}

func writeProjectFailure(stdout io.Writer, jsonOutput bool, err error) {
	terminal := "FAILED — " + err.Error()
	if jsonOutput {
		if encodeErr := json.NewEncoder(stdout).Encode(map[string]any{"error": err.Error(), "terminal": terminal}); encodeErr != nil {
			fmt.Fprintf(stdout, "FAILED — encode project error: %v\n", encodeErr)
		}
		return
	}
	fmt.Fprintln(stdout, terminal)
}

func runProjectPin(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet("update pin", "usage: pfm update pin <local>... | --all [--template TEMPLATE] [--root DIR]", stderr)
	rootFlag := flags.String("root", "", "project root")
	pinAll := flags.Bool("all", false, "pin every UPDATED file")
	templateFlag := flags.String("template", "", "template path for a newly adopted local file")
	locals, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if (*pinAll && (len(locals) != 0 || *templateFlag != "")) || (*templateFlag != "" && len(locals) != 1) || (!*pinAll && *templateFlag == "" && len(locals) == 0) {
		flags.Usage()
		return 2
	}
	root, found, err := resolveProjectRoot(*rootFlag)
	if err != nil || !found {
		if err == nil {
			err = errors.New(".professor/baseline.json not found")
		}
		fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
		return 1
	}
	report, err := buildProjectReport(root, runtime.Paths.Home)
	if err != nil {
		fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
		return 1
	}
	selected := locals
	if *pinAll {
		selected = nil
		for _, item := range report.Items {
			if item.Status == projectUpdated {
				selected = append(selected, item.Local)
			}
		}
	}
	if *templateFlag != "" {
		template, err := safeTemplateRelative(*templateFlag)
		if err != nil {
			fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
			return 1
		}
		local, err := safeProjectRelative(selected[0])
		if err != nil {
			fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
			return 1
		}
		if _, exists := report.Baseline.Files[local]; exists {
			fmt.Fprintf(stderr, "pfm update pin: %s already has a pin\n", local)
			return 1
		}
		if err := requireReadableLocal(root, local); err != nil {
			fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
			return 1
		}
		hash, err := professor.HashTemplate(filepath.Join(report.Store.Templates, filepath.FromSlash(template)))
		if err != nil {
			fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
			return 1
		}
		report.Baseline.Files[local] = professor.FilePin{Template: template, TemplateHash: hash, PinnedSHA: report.Store.SHA, PinnedAt: time.Now().Format(time.DateOnly)}
		selected[0] = local
	} else {
		for index, value := range selected {
			local, err := safeProjectRelative(value)
			if err != nil {
				fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
				return 1
			}
			pin, exists := report.Baseline.Files[local]
			if !exists {
				fmt.Fprintf(stderr, "pfm update pin: %s has no pin; use --template for a new adoption\n", local)
				return 1
			}
			if err := requireReadableLocal(root, local); err != nil {
				fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
				return 1
			}
			hash, err := professor.HashTemplate(filepath.Join(report.Store.Templates, filepath.FromSlash(pin.Template)))
			if err != nil {
				fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
				return 1
			}
			pin.TemplateHash = hash
			pin.PinnedSHA = report.Store.SHA
			pin.PinnedAt = time.Now().Format(time.DateOnly)
			report.Baseline.Files[local] = pin
			selected[index] = local
		}
	}
	if len(selected) != 0 {
		report.Baseline.Blueprint = professor.BlueprintPin{Version: report.Store.Version, SHA: report.Store.SHA}
		if err := professor.Save(root, report.Baseline); err != nil {
			fmt.Fprintf(stderr, "pfm update pin: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "pinned %d file(s) at %s\n", len(selected), report.Store.SHA)
	return 0
}

func runProjectDrop(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("update drop", "usage: pfm update drop <local>... [--root DIR]", stderr)
	rootFlag := flags.String("root", "", "project root")
	locals, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(locals) == 0 {
		flags.Usage()
		return 2
	}
	root, found, err := resolveProjectRoot(*rootFlag)
	if err != nil || !found {
		if err == nil {
			err = errors.New(".professor/baseline.json not found")
		}
		fmt.Fprintf(stderr, "pfm update drop: %v\n", err)
		return 1
	}
	baseline, err := professor.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "pfm update drop: %v\n", err)
		return 1
	}
	for index, value := range locals {
		local, err := safeProjectRelative(value)
		if err != nil {
			fmt.Fprintf(stderr, "pfm update drop: %v\n", err)
			return 1
		}
		if _, exists := baseline.Files[local]; !exists {
			fmt.Fprintf(stderr, "pfm update drop: %s has no pin\n", local)
			return 1
		}
		delete(baseline.Files, local)
		locals[index] = local
	}
	if err := professor.Save(root, baseline); err != nil {
		fmt.Fprintf(stderr, "pfm update drop: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "dropped %d pin(s)\n", len(locals))
	return 0
}

func printProfessorDoctor(stdout io.Writer, start, home string) int {
	root, found, err := resolveProjectRoot(start)
	if err != nil {
		fmt.Fprintf(stdout, "professor: UNREADABLE %v\n", err)
		return 1
	}
	if !found {
		return 0
	}
	report, err := buildProjectReport(root, home)
	if err != nil {
		fmt.Fprintf(stdout, "professor: UNREADABLE %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "professor: current %d · review-required %d\n", report.Counts[projectCurrent], report.reviewRequired())
	return 0
}

func resolveProjectRoot(rootFlag string) (string, bool, error) {
	start := strings.TrimSpace(rootFlag)
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", false, fmt.Errorf("resolve current directory: %w", err)
		}
	}
	absolute, err := filepath.Abs(start)
	if err != nil {
		return "", false, fmt.Errorf("resolve project root %s: %w", start, err)
	}
	for {
		path := professor.BaselinePath(absolute)
		if _, err := os.Stat(path); err == nil {
			return absolute, true, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", false, fmt.Errorf("UNREADABLE %s: %w", path, err)
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			return "", false, nil
		}
		absolute = parent
	}
}

func safeProjectRelative(value string) (string, error) {
	return safeRelative(value, "local path", "")
}

func safeTemplateRelative(value string) (string, error) {
	return safeRelative(value, "template path", "project/")
}

func safeRelative(value, label, prefix string) (string, error) {
	value = filepath.ToSlash(strings.TrimSpace(value))
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if value == "" || clean == "." || filepath.IsAbs(filepath.FromSlash(value)) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid %s %q", label, value)
	}
	if prefix != "" && !strings.HasPrefix(clean, prefix) {
		return "", fmt.Errorf("invalid %s %q: expected %s**", label, value, prefix)
	}
	return clean, nil
}

func requireReadableLocal(root, local string) error {
	path := filepath.Join(root, filepath.FromSlash(local))
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("UNREADABLE %s: %w", path, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o444 == 0 {
		return fmt.Errorf("UNREADABLE %s: %w", path, fs.ErrPermission)
	}
	return nil
}
