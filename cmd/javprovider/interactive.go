package main

import (
	"context"
	"errors"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"javboss/internal/jav"
)

func (cmd command) selectLookup(ctx context.Context, in io.Reader, out io.Writer) (providerOption, methodOption, string, error) {
	// Forms require a terminal; scripts use the CLI's flags instead of simulating keystrokes.
	tty, ok := in.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(int(tty.Fd())) {
		return providerOption{}, methodOption{}, "", errors.New("交互模式需要终端；请使用 --provider、--method 和 --input 参数")
	}
	methodIndex := 0
	var providerID jav.Provider
	var input string
	options := make([]huh.Option[int], 0, len(cmd.methods))
	for i, method := range cmd.methods {
		options = append(options, huh.NewOption(method.name, i))
	}
	groups := []*huh.Group{
		huh.NewGroup(huh.NewSelect[int]().Title("请选择 method").Options(options...).Value(&methodIndex)),
	}
	// Static lists retain their row positions; huh's dynamic options reset the
	// scroll offset on every update. Show only the selected method's group.
	for i, method := range cmd.methods {
		options := make([]huh.Option[jav.Provider], 0)
		for _, provider := range cmd.supportedProviders(method) {
			options = append(options, huh.NewOption(provider.name, provider.provider))
		}
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[jav.Provider]().Title("请选择 provider").Options(options...).Value(&providerID),
		).WithHideFunc(func() bool { return methodIndex != i }))
	}
	groups = append(groups,
		huh.NewGroup(huh.NewInput().TitleFunc(func() string { return cmd.methods[methodIndex].prompt }, &methodIndex).
			Validate(nonEmptyInput).Value(&input)),
	)
	form := huh.NewForm(groups...).
		// main owns signal handling and cancels the form's context.
		WithProgramOptions(tea.WithoutSignalHandler()).WithInput(in).WithOutput(out)
	if err := form.RunWithContext(ctx); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if errors.Is(err, huh.ErrUserAborted) {
			err = context.Canceled
		}
		return providerOption{}, methodOption{}, "", err
	}
	provider, err := cmd.findProvider(providerID.String())
	return provider, cmd.methods[methodIndex], strings.TrimSpace(input), err
}
