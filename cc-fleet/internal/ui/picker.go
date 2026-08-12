package ui

import (
	"context"
	"fmt"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
)

func (picker BubblePicker) Pick(
	ctx context.Context,
	snapshot Snapshot,
) (Outcome, error) {
	openTTY := picker.OpenTTY
	if openTTY == nil {
		openTTY = func() (ReadWriteCloser, error) {
			return os.OpenFile("/dev/tty", os.O_RDWR, 0)
		}
	}
	terminal, err := openTTY()
	if err != nil {
		return Outcome{}, fmt.Errorf("open picker /dev/tty: %w", err)
	}
	defer terminal.Close()

	model := NewModel(snapshot)
	program := tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(terminal),
		tea.WithOutput(terminal),
		tea.WithWindowSize(model.width, model.height),
	)
	done := make(chan struct{})
	var updates sync.WaitGroup
	if picker.Updates != nil {
		updates.Add(1)
		go func() {
			defer updates.Done()
			for {
				select {
				case refresh, ok := <-picker.Updates:
					if !ok {
						return
					}
					program.Send(RefreshMsg{Snapshot: refresh})
				case <-done:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	final, err := program.Run()
	close(done)
	updates.Wait()
	if err != nil {
		return Outcome{}, fmt.Errorf("run picker: %w", err)
	}
	result, ok := final.(Model)
	if !ok {
		return Outcome{}, fmt.Errorf("picker returned model %T", final)
	}
	return result.Result(), nil
}

var _ Picker = BubblePicker{}
