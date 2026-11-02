# 日志格式与命令参考

## 日志模板

参考：[JavBoss v2.1.2](https://github.com/Solr159/JavBoss/releases/tag/v2.1.2)。保持其层级、编号、新功能／优化改进／问题修复三个分类及面向用户的写法；下面是结构模板，不是可直接发布的内容。

```markdown
# 更新日志

## ✨ 新功能

### 1. 功能名称

- 描述用户现在能完成的操作，必要时说明实际入口。
- 描述有证据支持的相关能力或使用方式。

### 2. 另一个功能

- 描述具体行为。

## 🚀 优化改进

- 描述用户可感知的体验改进。

## 🐛 问题修复

- 描述已确认修复的问题。
```

无对应变更时省略该分类。所有条目都应来自本次变更，不复制旧版本功能。正文默认不追加作者名单、commit 列表或额外的英文自动日志。

## 当前仓库约定

这些约定来自创建 skill 时的仓库，执行时仍须读取目标 ref 的 workflow：

- `.github/workflows/release-draft.yml`：手动触发，必填 `release`，workflow 中布尔值 `publish_browser_extension` 默认 `true`。**本 skill 要求用户显式提供该选项，不使用 workflow 默认值**，每次 dispatch 都传入用户选择的 `true` 或 `false`。
- 构建平台：`windows-x86_64`、`linux-x86_64`、`macos-x86_64`、`macos-arm64`。
- 主程序资产：`javboss-VERSION-PLATFORM.zip`。
- 扩展版本取构建 SHA 下 `browser-extension/manifest.json` 的 `version`，资产为 `javboss-browser-extension-vEXTENSION_VERSION.zip`，不要套用主程序版本号。
- 最后的 `publish` job 名称容易误解：这个 job 仅通过 `gh release create --draft` 创建草稿，现有 `--generate-notes` 生成的正文随后由本 skill 更新。
- `release-publish.yml` 是另一条正式发布 workflow，本 skill 不调用它。

## GitHub CLI 操作参考

以下都是分阶段命令示例，**不是可一键执行的脚本**。先按 SKILL.md 完成对应前置检查，并为变量提供已验证的值。始终显式指定 `--repo`，不要依赖当前目录推断仓库。

```bash
release_repo=Solr159/JavBoss

# 读取已发布版本候选；处理所有分页，再筛选主程序、正式版本及祖先关系。
gh api --paginate --slurp "repos/$release_repo/releases?per_page=100"

# 读取目标 Release 原始元数据，包括 target_commitish 和资产状态。
gh api "repos/$release_repo/releases/tags/$release_version"

# 保存 dispatch 前的运行列表，并记录请求时间；之后用同样的条件查新运行。
gh run list --repo "$release_repo" --workflow release-draft.yml \
  --event workflow_dispatch --branch "$release_ref" --limit 50 \
  --json databaseId,headSha,createdAt,status,conclusion,url,displayTitle

# include_extension 必须来自用户本次明确的选择，不可设置默认值。
# 仅在不存在可复用草稿或运行时，触发一次草稿构建。
case "${include_extension-}" in
  true|false) ;;
  *) echo "必须先提供是否包含浏览器扩展（true/false）" >&2; exit 1 ;;
esac
gh workflow run release-draft.yml --repo "$release_repo" --ref "$release_ref" \
  -f "release=$release_version" -f "publish_browser_extension=$include_extension"

# 跟踪已确定归属的运行；不要依赖列表位置选取运行。
gh run view "$release_run_id" --repo "$release_repo" \
  --json databaseId,headSha,status,conclusion,jobs,url

# 仅在仍为草稿、构建与资产已核验、正文无未处理冲突后写入。
gh release edit "$release_version" --repo "$release_repo" --notes-file "$release_notes_file"

# 回读核对正文和草稿状态。
gh release view "$release_version" --repo "$release_repo" \
  --json tagName,isDraft,isPrerelease,body,assets,url,targetCommitish
```

`gh workflow run` 可能不返回运行 ID。用 dispatch 前后 ID 差集、workflow、event、ref、时间及 SHA 识别运行；必要时查看日志中的 `RELEASE` 和最终 Release 信息确认版本。并发导致无法唯一确认时不要猜测。网络超时后先查询是否已经派发。

写日志文件时使用文件工具或带引号的 heredoc，保留真实换行；不要将日志插值到 shell 命令中。发布正文不需要另外创建 ChatGPT Page。

CLI 文档：[workflow run](https://cli.github.com/manual/gh_workflow_run)、[run list](https://cli.github.com/manual/gh_run_list)、[release edit](https://cli.github.com/manual/gh_release_edit)。
