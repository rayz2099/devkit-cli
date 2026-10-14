# td-cli

ThinkingData / Thinking Engine 4.4 的只读 CLI。官方 `ae-cli` 只走设备码登录，这台 4.4 没有该接口。本工具用页面同一条会话登录：`POST /v1/oauth/loginForToken`，密码按前端 RSA 公钥做 PKCS1 加密。

只做查询和统计。保存报表、删除、导入、下载任务、SQL 查询都不在范围内。

## 功能

约定范围是只读：登录、看元数据、跑统计。下表按这个范围标。

| 功能 | 状态 | 说明 |
| --- | --- | --- |
| 账号密码登录 | 已实现 | `auth login`。`POST /v1/oauth/loginForToken`，密码按页面 RSA 公钥做 PKCS1。令牌只留在当次进程 |
| 查看当前 profile | 已实现 | `auth status`。只打印 profile、url、username |
| 项目列表 | 已实现 | `project ls`。走 `POST /v1/oauth/check` 的 `projectInfoList`。已在 `114.67.80.181:8993` 跑通 |
| 事件目录 | 已实现 | `meta events`。`GET /v1/ta/event/catalog/listEvent` |
| 用户属性 | 已实现 | `meta props`。`GET /v1/ta/user/loadUserProps` |
| 某事件的筛选属性 | 已实现 | `meta props --event`。`POST /v1/ta/event/model/meta/loadFiltProps` |
| 报表列表 | 已实现 | `report ls`。`GET /v1/ta/event/reportlist` |
| 读一张报表 | 已实现 | `report get`。`GET /v1/ta/event/reportsearch` |
| 事件分析 | 已实现 | `query event`。单事件、单个分组、无筛选。默认 `anyEvent`、总次数 `A100`、过去 7 天、粒度 `T1` |
| 属性分析 | 已实现 | `query property`。默认用户数 `A101`。不写时间时不补时间条件 |
| 按页面 QP 重跑 | 已实现 | `query run --model N --qp FILE`。模型 `0/1/2/3/4/7/12/14` |
| 按已存报表重跑 | 已实现 | `query report`。用报表自己的模型和 QP |
| agent / human 输出 | 已实现 | `agent` 固定一条 JSON。`human` 文本。fish 补全 `completion fish` |
| 留存、漏斗、路径、分布、间隔、归因的快捷参数 | 未做独立命令 | 模型号分别是 `1/2/4/7/12/14`。用 `query run --qp` 提交页面 QP |
| 多事件、多分组、筛选、时间对比 | 未做快捷参数 | 事件分析和属性分析的旗标拼不出这些。把页面 QP 交给 `--qp` 或 `query run` |
| 退出登录 | 未实现 | 配置里不存令牌，没有可退出的本地会话 |
| LDAP / 设备码登录 | 不做 | 这台 4.4 的设备码接口不存在。LDAP 不是这条 CLI 的登录方式 |
| SQL 查询 | 不做 | 模型 `10` 会直接拒绝 |
| 热力图、排行榜 | 不做 | 模型 `101`、`102` |
| 保存、删除、修改报表或目录 | 不做 | 命令里出现 save、delete、update、create、edit、import、upload、download、remove 会在解析阶段拒绝 |
| 异步下载 | 不做 | 不提交下载任务 |

除 `project ls` 外，其余已实现命令有代码和单测，还没有在这台服务器上逐条打过。

## 命令

```bash
td-cli [-c config.json] [-p profile] [agent|human] <command>

td-cli auth login
td-cli auth status

td-cli project ls
td-cli meta events --project ID
td-cli meta props --project ID
td-cli meta props --project ID --event EVENT

td-cli report ls --project ID
td-cli report get --project ID --id REPORT

td-cli query event --project ID [--event NAME] [--analysis A100] [--recent 1-7]
td-cli query property --project ID [--analysis A101] [--group user:PROP]
td-cli query run --project ID --model N --qp qp.json
td-cli query report --project ID --id REPORT
```

`agent` 输出稳定 JSON。`human` 输出对齐文本。事件分析不带时间时，按页面默认「过去 7 天」（`1-7`）。属性分析不带时间时，不补时间条件。

统计模型：`0` 事件，`1` 留存，`2` 漏斗，`3` 属性，`4` 路径，`7` 分布，`12` 间隔，`14` 归因。SQL 模型不开放。

`--qp` 是页面 QP JSON。带了它就不能再写 `--event` 等条件。

## 配置

`~/.config/td-cli/config.json`，示例见 `config.example.json`。命令自己登录，不把令牌写回文件。

```json
{
  "defaultProfile": "local",
  "profiles": [
    {
      "name": "local",
      "url": "http://114.67.80.181:8993",
      "username": "",
      "password": ""
    }
  ]
}
```

```bash
just install
```
