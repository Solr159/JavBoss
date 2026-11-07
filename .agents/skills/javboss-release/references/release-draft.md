# 日志格式与命令参考

## 日志模板

沿用 [v2.1.2](https://github.com/Solr159/JavBoss/releases/tag/v2.1.2) 的风格：固定一级标题“更新日志”，按新功能、优化改进、问题修复排列；新功能用编号三级标题按用户能力归组，优化和修复使用无序列表。无内容的分类省略，不追加作者名单、commit 列表或英文自动日志。

```markdown
# 更新日志

## ✨ 新功能

### 1. 功能名称

- 描述升级后可用的最终能力、入口和行为。

## 🚀 优化改进

- 描述相对于基线的体验改进。

## 🐛 问题修复

- 描述基线中存在、目标版本已解决的问题。
```

例如：某功能本周期依次经历“新增选项 → 修复默认值 → 更名”，日志只写最终选项及行为，不再列“修复默认值”和中间名称；若修复的是上一正式版已有问题，才作为问题修复。

## 整体比较

以下命令假设 ref 已核验且对象已获取。先看两端快照的净差异，再按需追溯；提交列表用于核对范围和寻找证据，不作为日志大纲。

```bash
git diff --stat "$base_tag" "$target_sha"
git diff "$base_tag" "$target_sha"
# 按功能检查最终差异及两端源码
git diff "$base_tag" "$target_sha" -- path/to/feature
git show "$base_tag:path/to/file"
git show "$target_sha:path/to/file"
# 补充变更意图，包含没有 PR 的提交
git log --format='%H%n%B' "$base_tag..$target_sha"
```

## 构建资产

执行时以目标 ref 的 workflow 为准：

- 主程序包：`javboss-VERSION-PLATFORM.zip`；平台为 `windows-x86_64`、`linux-x86_64`、`macos-x86_64`、`macos-arm64`。
- 扩展包：`javboss-browser-extension-vEXTENSION_VERSION.zip`；版本取构建 SHA 下 `browser-extension/manifest.json`，不要套用主程序版本。
- 最后的 `publish` job 通过 `gh release create --draft --generate-notes` 创建草稿；自动生成的正文随后替换为核实过的中文日志。

## GitHub CLI

以下是分阶段示例，先完成 SKILL.md 中的前置检查并给变量赋值。文件正文用文件工具或带引号的 heredoc 写入，不插值到 shell 命令。

```bash
release_repo=Solr159/JavBoss

# 分页查 Release；草稿按 tag_name 精确匹配并确认唯一，再按 ID 读取。
# jq -s 兼容没有 --slurp 的旧版 gh。
gh api --paginate "repos/$release_repo/releases?per_page=100" | jq -s 'add'
gh api "repos/$release_repo/releases/$release_id"

# dispatch 前后读取同一条件的运行列表，并记录请求时间。
gh run list --repo "$release_repo" --workflow release-draft.yml \
  --event workflow_dispatch --branch "$release_ref" --limit 50 \
  --json databaseId,headSha,createdAt,status,conclusion,url,displayTitle

# include_extension 必须来自用户明确选择，false 也显式传入。
gh workflow run release-draft.yml --repo "$release_repo" --ref "$release_ref" \
  -f "release=$release_version" -f "publish_browser_extension=$include_extension"

gh run view "$release_run_id" --repo "$release_repo" \
  --json databaseId,headSha,status,conclusion,jobs,url

# release_prewrite_file 为刚核验的草稿 JSON；保留元数据及草稿状态。
jq --rawfile body "$release_notes_file" \
  '{tag_name, target_commitish, name, prerelease, draft: true, body: $body}' \
  "$release_prewrite_file" > "$release_payload_file"
gh api --method PATCH "repos/$release_repo/releases/$release_id" \
  --input "$release_payload_file"

# 回读验证；最终链接使用这次返回的 html_url。
gh api "repos/$release_repo/releases/$release_id"
```

`gh workflow run` 可能不返回 ID，必要时通过日志中的 `RELEASE`、扩展选项和 Release 信息确认归属；不能把 dispatch 成功当作构建成功。超时后先查询是否已生效再决定下一步。
