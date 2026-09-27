package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/urfave/cli/v3"

	"javboss/internal/jav"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := newCommand().run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "javprovider:", err)
		os.Exit(1)
	}
}

func (cmd command) run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	var provider providerOption
	method, err := cmd.findMethod("LookupJavByCode")
	if err != nil {
		return err
	}
	app := &cli.Command{
		Name:            "javprovider",
		Usage:           "查询 JAV 影片和演员资料；不带参数进入交互模式",
		HideHelpCommand: true,
		Reader:          in, Writer: errOut, ErrWriter: errOut,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "provider", Usage: "数据来源: " + strings.Join(providerNames(cmd.providers), ", "),
				Required: len(args) > 0,
				Action: func(_ context.Context, _ *cli.Command, value string) error {
					var err error
					provider, err = cmd.findProvider(value)
					return err
				},
			},
			&cli.StringFlag{
				Name: "method", Value: "LookupJavByCode", Usage: "查询方法: " + strings.Join(methodNames(cmd.methods), ", "),
				Action: func(_ context.Context, _ *cli.Command, value string) error {
					var err error
					method, err = cmd.findMethod(value)
					return err
				},
			},
			&cli.StringFlag{Name: "input", Usage: "查询内容（番号或女优名字）", Required: len(args) > 0, Validator: nonEmptyInput},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.Args().Present() {
				return errors.New("不支持位置参数，请使用 --provider、--method 和 --input")
			}
			input := strings.TrimSpace(c.String("input"))
			if len(args) == 0 {
				var err error
				provider, method, input, err = cmd.selectLookup(ctx, in, out)
				if err != nil {
					return err
				}
			}
			return executeLookup(ctx, out, provider, method, input)
		},
	}
	return app.Run(ctx, append([]string{"javprovider"}, args...))
}

func nonEmptyInput(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("查询内容不能为空")
	}
	return nil
}

func executeLookup(ctx context.Context, out io.Writer, provider providerOption, method methodOption, input string) error {
	result, err := method.call(ctx, provider.provider, input)
	if err != nil {
		if errors.Is(err, jav.ErrUnsupportedOperation) {
			return fmt.Errorf("%s 不支持 %s: %w", provider.name, method.name, err)
		}
		return fmt.Errorf("调用 %s/%s 失败: %w", provider.name, method.name, err)
	}
	if value, ok := result.(string); ok {
		_, err = fmt.Fprintln(out, value)
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("输出结果失败: %w", err)
	}
	return nil
}
