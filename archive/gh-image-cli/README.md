# gh-image-cli

通过当前 GitHub 仓库的 `workflow_dispatch` 同步上游镜像或构建 Dockerfile, 并以仓库根目录的 `images.yaml` 作为本地镜像台账。

## 安装

在 `devkit-cli` 根目录执行:

```bash
just build gh-image-cli
just install gh-image-cli
just clean gh-image-cli
```

安装目标为 `~/.local/bin/gh-image-cli`。CLI 依赖本机已经完成认证的 `gh`。

## 配置

配置文件可选, 路径固定为 `~/.config/gh-image-cli/config.json`, 需要覆盖默认值时手工创建:

```json
{
  "registry": "registry.cn-shanghai.aliyuncs.com",
  "namespace": "linran-pub",
  "timeoutSeconds": 600
}
```

缺少配置文件时使用上述默认值。仓库内不读取项目级 `config.json`。

## 使用

```bash
# 登记上游镜像, version 默认 latest
gh-image-cli add eceasy/cli-proxy-api cpa v7.2.130
gh-image-cli add postgres postgres

# source 也可以自带 tag
gh-image-cli add ghcr.io/sagernet/sing-box:latest sing-box

# 登记仓库内 Dockerfile 构建脚本
gh-image-cli add-dockerfile dockerfile/sail/build.sh sail 0.6.4

# 触发 Action; version 省略时取 images.yaml 中第一个版本
gh-image-cli build cpa v7.2.130
gh-image-cli build sail

# 镜像级视图
gh-image-cli list
gh-image-cli list cpa
```

每次 `build` 都会重新触发 Action。状态机为 `init -> done | failed | timeout`, 默认等待 10 分钟。CLI 不执行 Git commit 或 push。

## images.yaml

```yaml
images:
  cpa:
    type: mirror
    source: eceasy/cli-proxy-api
    versions:
      v7.2.130:
        status: done
      latest:
        status: init
  sail:
    type: dockerfile
    script: dockerfile/sail/build.sh
    versions:
      0.6.4:
        status: init
```

新登记的版本写到 `versions` 首位。`mirror` 使用 `sync-image.yml`, `dockerfile` 使用 `build-dockerfile.yml`; 两个 workflow 均只允许手动触发。

## Fish completion

生成补全定义:

```fish
gh-image-cli completion fish
```

补全会在当前 Git 仓库动态读取 `images.yaml`, 支持 `build/list` alias 以及 `build` version。非 Git 仓库或缺少 `images.yaml` 时静默返回空候选。
