package action

import (
	"context"

	pfmconfig "hostops/pfm/internal/config"
)

func testMachineConfig(home string) pfmconfig.Config {
	return pfmconfig.Defaults(home, []string{
		pfmconfig.DefaultAccountProjectDir(home, 1),
		pfmconfig.DefaultAccountProjectDir(home, 2),
		pfmconfig.DefaultAccountProjectDir(home, 3),
	})
}

func synthesizeWithTestConfig(request Request) (Plan, error) {
	if len(request.Config.Accounts) == 0 {
		request.Config = testMachineConfig(request.Home)
	}
	return Synthesize(request)
}

func headlessWithTestConfig(request HeadlessRequest) (HeadlessPlan, error) {
	if len(request.Config.Accounts) == 0 {
		request.Config = testMachineConfig(request.Home)
	}
	return HeadlessRun(request)
}

func openWithTestConfig(executor *Executor, ctx context.Context, request Request) (string, error) {
	if len(request.Config.Accounts) == 0 {
		request.Config = testMachineConfig(request.Home)
	}
	return executor.Open(ctx, request)
}
