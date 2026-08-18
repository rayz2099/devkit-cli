import { helpText } from "./args";
import { NacosClient } from "./client";
import { completeLine } from "./complete";
import { loadFileCfg, nsCands, resolveRuntime } from "./config";
import { fishScript, groupsByDataId, uniqueDataIds } from "./fish";
import { render, renderTable } from "./output";
import type { CliCmd, Runtime } from "./types";

/** 为什么: 入口只解析 argv, 这里集中把命令打到 OpenAPI, 方便单测注入 client. */
export async function runCmd(
  cmd: CliCmd,
  createClient: (runtime: Runtime) => NacosClient = (runtime) => new NacosClient(runtime),
): Promise<string> {
  if (cmd.kind === "help") {
    return `${helpText(cmd.topic)}\n`;
  }
  if (cmd.kind === "completion-fish") {
    return fishScript();
  }

  const fileCfg = await loadFileCfg();
  if (cmd.kind === "fish-namespaces") {
    return completeLines(nsCands(fileCfg), cmd.prefix);
  }
  if (cmd.kind === "complete") {
    try {
      const values = await completeLine(cmd.tokens, cmd.current, fileCfg, createClient);
      return cobraComp(values);
    } catch {
      return cobraComp([]);
    }
  }

  const runtime = resolveRuntime(cmd.global, fileCfg);
  const client = createClient(runtime);

  if (cmd.kind === "fish-data-ids") {
    return completeLines(uniqueDataIds(await client.listItemsCached(), cmd.prefix), cmd.prefix);
  }
  if (cmd.kind === "fish-groups") {
    return completeLines(
      groupsByDataId(await client.listItemsCached(), cmd.dataId, cmd.prefix),
      cmd.prefix,
    );
  }
  if (cmd.kind === "config-get") {
    const content = await client.getConfig(cmd.dataId, cmd.group);
    return render(runtime.output, content, {
      dataId: cmd.dataId,
      group: cmd.group,
      content,
    });
  }
  if (cmd.kind === "config-put") {
    await client.putConfig(cmd.dataId, cmd.group, cmd.content);
    return render(runtime.output, "ok", { success: true });
  }
  if (cmd.kind === "config-delete") {
    await client.deleteConfig(cmd.dataId, cmd.group);
    return render(runtime.output, "ok", { success: true });
  }
  if (cmd.kind === "config-list") {
    const page = await client.listConfigs({
      search: cmd.search,
      dataId: cmd.dataId,
      group: cmd.group,
      pageNo: cmd.pageNo,
      pageSize: cmd.pageSize,
    });
    if (runtime.output === "json") {
      return render(runtime.output, "", page);
    }
    const rows = page.pageItems.map((item) => [item.dataId, item.group]);
    return render(
      runtime.output,
      renderTable(
        `Total: ${page.totalCount}  Page: ${page.pageNumber}/${page.pagesAvailable}`,
        ["DATA_ID", "GROUP"],
        rows,
      ),
      undefined,
    );
  }
  if (cmd.kind === "naming-register") {
    await client.registerInst({
      service: cmd.service,
      ip: cmd.ip,
      port: cmd.port,
      group: cmd.group,
      cluster: cmd.cluster,
      weight: cmd.weight,
      ephemeral: cmd.ephemeral,
    });
    return render(runtime.output, "ok", { success: true });
  }
  if (cmd.kind === "naming-deregister") {
    await client.deregisterInst({
      service: cmd.service,
      ip: cmd.ip,
      port: cmd.port,
      group: cmd.group,
      cluster: cmd.cluster,
      ephemeral: cmd.ephemeral,
    });
    return render(runtime.output, "ok", { success: true });
  }

  const insts = await client.listInsts({
    service: cmd.service,
    group: cmd.group,
    clusters: cmd.clusters,
    healthyOnly: cmd.healthyOnly,
  });
  if (runtime.output === "json") {
    return render(runtime.output, "", insts);
  }
  const lines = [`count: ${insts.length}`, ...insts.map((item) => `${item.ip}:${item.port}`)];
  return render(runtime.output, lines.join("\n"), undefined);
}

function completeLines(values: string[], prefix: string): string {
  const matched = values.filter((item) => item.startsWith(prefix));
  return matched.length === 0 ? "" : `${matched.join("\n")}\n`;
}

/** 为什么: my-script 里 cobra fish 用最后一行 :directive, 缺了会把 COMMON 拿去 math. */
function cobraComp(values: string[]): string {
  if (values.length === 0) {
    return ":4\n";
  }
  return `${values.join("\n")}\n:4\n`;
}
