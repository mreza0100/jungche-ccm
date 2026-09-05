package ui

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
)

func TestInteractiveColorProfileIgnoresNoninteractiveColorOptOuts(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("TTY_FORCE", "0")
	t.Setenv("COLORTERM", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR", "0")

	if got := interactiveColorProfile(io.Discard); got != colorprofile.ANSI {
		t.Fatalf("interactiveColorProfile() = %s, want minimum ANSI", got)
	}
}

func TestInteractiveColorProfileKeepsAdvertisedCapabilities(t *testing.T) {
	t.Setenv("TTY_FORCE", "1")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")

	for _, testCase := range []struct {
		name      string
		term      string
		colorTerm string
		want      colorprofile.Profile
	}{
		{name: "256 color", term: "xterm-256color", want: colorprofile.ANSI256},
		{name: "true color", term: "xterm-256color", colorTerm: "truecolor", want: colorprofile.TrueColor},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("TERM", testCase.term)
			t.Setenv("COLORTERM", testCase.colorTerm)
			if got := interactiveColorProfile(io.Discard); got != testCase.want {
				t.Fatalf("interactiveColorProfile() = %s, want %s", got, testCase.want)
			}
		})
	}
}

func TestBubblePickerRendererStaysColoredWhenParentDisablesColor(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("TTY_FORCE", "0")
	t.Setenv("COLORTERM", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR", "0")

	terminal := &pickerTestTerminal{input: bytes.NewReader([]byte{'\x1b'})}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	outcome, err := (BubblePicker{
		OpenTTY: func() (ReadWriteCloser, error) { return terminal, nil },
	}).Pick(ctx, fixtureSnapshot(80))
	if err != nil {
		t.Fatalf("BubblePicker.Pick: %v\noutput=%q", err, terminal.output.String())
	}
	if outcome.Kind != OutcomeCancelled {
		t.Fatalf("BubblePicker outcome = %v, want cancelled", outcome.Kind)
	}
	if output := terminal.output.String(); !hasANSIColor(output) {
		t.Fatalf("Bubble Tea renderer emitted no ANSI color SGR with NO_COLOR=1 CLICOLOR=0: %q", output)
	}
}

func TestPlainAndTSVPickersRemainUncolored(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TTY_FORCE", "1")
	t.Setenv("COLORTERM", "truecolor")
	snapshot := fixtureSnapshot(80)
	plainOutput := &bytes.Buffer{}
	tsvOutput := &bytes.Buffer{}

	for _, testCase := range []struct {
		name string
		pick Picker
		out  *bytes.Buffer
	}{
		{name: "plain", pick: PlainPicker{Writer: plainOutput}, out: plainOutput},
		{name: "TSV", pick: TSVPicker{Writer: tsvOutput}, out: tsvOutput},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := testCase.pick.Pick(context.Background(), snapshot); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(testCase.out.String(), "\x1b[") {
				t.Fatalf("%s picker emitted ANSI: %q", testCase.name, testCase.out.String())
			}
		})
	}
}

type pickerTestTerminal struct {
	input  *bytes.Reader
	output bytes.Buffer
}

func (terminal *pickerTestTerminal) Read(value []byte) (int, error) {
	return terminal.input.Read(value)
}

func (terminal *pickerTestTerminal) Write(value []byte) (int, error) {
	return terminal.output.Write(value)
}

func (*pickerTestTerminal) Close() error { return nil }

var ansiColorSGR = regexp.MustCompile(`\x1b\[[0-9;]*(?:3[0-7]|4[0-7]|9[0-7]|10[0-7]|38|48)[0-9;]*m`)

func hasANSIColor(value string) bool { return ansiColorSGR.MatchString(value) }
