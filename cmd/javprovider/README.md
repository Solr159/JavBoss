# javprovider

不带参数启动时进入交互模式，依次选择查询方法、支持该方法的数据来源，再填写输入内容：

```sh
go run ./cmd/javprovider
```

交互表单由 [huh](https://github.com/charmbracelet/huh) 提供：上下方向键选择，Enter 确认，Shift+Tab 返回上一项；文本输入支持左右方向键、行内编辑和中文。
在选择方法、选择来源或填写查询内容时，Ctrl+C 都会立即退出并恢复终端状态。
交互表单需要终端。脚本、管道或重定向 stdin 时，请使用下面的命令行参数模式。

命令行参数和帮助由 [urfave/cli](https://github.com/urfave/cli) 管理。传入参数时使用命令行模式；`--provider` 和 `--input` 必填，`--method` 默认是
`LookupJavByCode`。参数不完整会报错退出，不会等待交互输入。

```sh
# 查询影片信息
go run ./cmd/javprovider --provider javdb-api --input ABC-001

# 根据番号查询演员资料
go run ./cmd/javprovider --provider javdatabase --method LookupActressByCode --input ABC-001

# 根据名字查询演员资料
go run ./cmd/javprovider --provider minnanoav --method LookupActressByJapaneseName --input '女优名字'
go run ./cmd/javprovider --provider avwiki --method LookupActressByJapaneseName --input '森沢かな'

# 查看所有来源和查询方法
go run ./cmd/javprovider --help

# 将结果保存为 JSON，日志和错误仍输出到 stderr
go run ./cmd/javprovider --provider javbus --input ABC-001 > result.json
```

来源和方法名称不区分大小写。各站点支持的查询能力不同，不支持的组合会明确报错。
查询成功返回退出码 0，参数错误或查询失败返回非零退出码。
Ctrl+C 也可取消正在进行的查询；直接运行编译后的程序时，中断退出码为 130。

原有单横线参数（如 `-provider`、`-input`）仍可使用。
