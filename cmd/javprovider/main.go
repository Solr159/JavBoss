package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"pornboss/internal/jav"
)

type methodOption struct {
	name   string
	prompt string
	call   func(jav.Provider, string) (any, error)
}

type providerOption struct {
	name     string
	provider jav.Provider
	methods  []methodOption
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	providers := []providerOption{
		{
			name:     "javbus",
			provider: jav.ProviderJavBus,
		},
		{
			name:     "javdatabase",
			provider: jav.ProviderJavDatabase,
		},
		{
			name:     "javdb",
			provider: jav.ProviderJavDB,
		},
		{
			name:     "avmoo",
			provider: jav.ProviderAvmoo,
		},
		{
			name:     "javmodel",
			provider: jav.ProviderJavModel,
		},
		{
			name:     "theporndb",
			provider: jav.ProviderThePornDB,
		},
	}
	methods := []methodOption{
		{
			name:   "LookupActressByCode",
			prompt: "请输入番号",
			call: func(provider jav.Provider, input string) (any, error) {
				return jav.LookupActressByCode(input, provider)
			},
		},
		{
			name:   "LookupActressByJapaneseName",
			prompt: "请输入女优日文名",
			call: func(provider jav.Provider, input string) (any, error) {
				return jav.LookupActressByJapaneseName(input, provider)
			},
		},
		{
			name:   "LookupCoverURLByCode",
			prompt: "请输入番号",
			call: func(provider jav.Provider, input string) (any, error) {
				return jav.LookupCoverURLByCode(input, provider)
			},
		},
		{
			name:   "LookupJavByCode",
			prompt: "请输入番号",
			call: func(provider jav.Provider, input string) (any, error) {
				return jav.LookupJavByCode(input, provider)
			},
		},
	}

	provider := providers[mustChoose(reader, "请选择 provider", providerNames(providers))]
	method := methods[mustChoose(reader, "请选择 method", methodNames(methods))]
	input := mustReadNonEmpty(reader, method.prompt)

	result, err := safeCall(method, provider.provider, input)
	if err != nil {
		if errors.Is(err, errMethodNotSupported) {
			fmt.Println("不支持")
			return
		}
		log.Fatalf("调用失败: %v", err)
	}
	if result == nil {
		fmt.Println("null")
		return
	}

	switch value := result.(type) {
	case string:
		fmt.Println(value)
	default:
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			log.Fatalf("序列化失败: %v", err)
		}
		fmt.Println(string(data))
	}
}

var errMethodNotSupported = errors.New("method not supported")

func providerNames(providers []providerOption) []string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.name)
	}
	return names
}

func methodNames(methods []methodOption) []string {
	names := make([]string, 0, len(methods))
	for _, method := range methods {
		names = append(names, method.name)
	}
	return names
}

func mustChoose(reader *bufio.Reader, title string, options []string) int {
	if len(options) == 0 {
		log.Fatalf("%s: 没有可选项", title)
	}
	fmt.Println(title + ":")
	for i, option := range options {
		fmt.Printf("%d. %s\n", i+1, option)
	}
	for {
		text := mustReadNonEmpty(reader, "请输入编号")
		index, err := strconv.Atoi(text)
		if err != nil || index < 1 || index > len(options) {
			fmt.Printf("无效编号，请输入 1-%d\n", len(options))
			continue
		}
		return index - 1
	}
}

func mustReadNonEmpty(reader *bufio.Reader, prompt string) string {
	for {
		fmt.Printf("%s: ", prompt)
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("读取输入失败: %v", err)
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text
		}
	}
}

func safeCall(method methodOption, provider jav.Provider, input string) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errMethodNotSupported
		}
	}()
	result, err = method.call(provider, input)
	if err == nil {
		return result, nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "not supported") || strings.Contains(lower, "unsupported") || strings.Contains(lower, "unimplemented") {
		return nil, errMethodNotSupported
	}
	return result, err
}
