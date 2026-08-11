package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"git.internal.linran.top/linran/tl/internal/cli"
	"git.internal.linran.top/linran/tl/internal/infra/config"
	"git.internal.linran.top/linran/tl/internal/infra/llm/openai"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, newAppFactory(os.Stdin, os.Stdout, os.Stderr)); err != nil {
		exitWithError(err)
	}
}

type appFactory func() (*cli.App, error)

func run(ctx context.Context, args []string, stdout io.Writer, _ io.Writer, factory appFactory) error {
	if cli.IsHelpRequest(args) {
		_, err := io.WriteString(stdout, cli.HelpText(args))
		return err
	}
	if cli.IsVersionRequest(args) {
		_, err := io.WriteString(stdout, cli.VersionText())
		return err
	}
	if cli.IsCompletionRequest(args) {
		script, err := cli.CompletionScript(args)
		if err != nil {
			return err
		}
		_, err = io.WriteString(stdout, script)
		return err
	}

	app, err := factory()
	if err != nil {
		return err
	}
	return app.Run(ctx, args)
}

func newAppFactory(stdin io.Reader, stdout io.Writer, stderr *os.File) appFactory {
	return func() (*cli.App, error) {
		cfg, err := config.Loader{}.Load()
		if err != nil {
			return nil, err
		}

		openAIConfig := cfg.Providers.OpenAI
		if err := validateOpenAIConfig(openAIConfig); err != nil {
			return nil, err
		}

		return cli.New(cli.AppDependencies{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
			Translator: openai.NewClient(openai.ClientConfig{
				BaseURL:      openAIConfig.BaseURL,
				Token:        openAIConfig.Token,
				Model:        openAIConfig.Model,
				CustomPrompt: openAIConfig.CustomPrompt,
			}),
			Concurrency:     openAIConfig.Concurrency,
			ProgressEnabled: isInteractive(stderr),
		}), nil
	}
}

func validateOpenAIConfig(cfg config.OpenAIConfig) error {
	switch {
	case cfg.BaseURL == "":
		return errors.New("openai base_url is required")
	case cfg.Token == "":
		return errors.New("openai token is required")
	case cfg.Model == "":
		return errors.New("openai model is required")
	default:
		return nil
	}
}

func isInteractive(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func exitWithError(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
