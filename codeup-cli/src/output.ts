import type { Audience, ChangeRequest, Repo, Webhook } from "./types";

/** 为什么: agent 只要稳定 JSON, human 才需要对齐扫读. */
export function renderJson(data: unknown): string {
  return `${JSON.stringify(data, null, 2)}\n`;
}

export function renderText(text: string): string {
  if (text === "") {
    return "";
  }
  return text.endsWith("\n") ? text : `${text}\n`;
}

export function renderTable(
  headers: string[],
  rows: string[][],
): string {
  if (rows.length === 0) {
    return "";
  }
  const table = [headers, ...rows];
  const widths = headers.map((_, col) =>
    Math.max(...table.map((row) => (row[col] ?? "").length)),
  );
  return `${table
    .map((row) =>
      row.map((cell, col) => (cell ?? "").padEnd(widths[col] ?? 0)).join("  "),
    )
    .join("\n")}\n`;
}

export function pickBody(
  audience: Audience,
  text: string,
  data: unknown,
): string {
  return audience === "agent" ? renderJson(data) : text;
}

export function reposText(repos: Repo[]): string {
  if (repos.length === 0) {
    return "No repositories found\n";
  }
  return renderTable(
    ["ID", "NAME", "PATH", "DEFAULT", "URL"],
    repos.map((repo) => [repo.id, repo.name, repo.pathNs, repo.defBranch, repo.webUrl]),
  );
}

export function crListText(repo: string, crs: ChangeRequest[]): string {
  if (crs.length === 0) {
    return `No change requests in ${repo}\n`;
  }
  return renderTable(
    ["#", "STATE", "TITLE", "SOURCE", "TARGET", "AUTHOR", "UPDATED"],
    crs.map((cr) => [
      cr.localId,
      cr.state,
      clip(cr.title, 50),
      cr.srcBranch,
      cr.tgtBranch,
      cr.author,
      cr.updatedAt,
    ]),
  );
}

export function crViewText(repo: string, cr: ChangeRequest): string {
  const lines = [
    `repo\t${repo}`,
    `#\t${cr.localId}`,
    `state\t${cr.state}`,
    `title\t${cr.title}`,
    `source\t${cr.srcBranch}`,
    `target\t${cr.tgtBranch}`,
    `author\t${cr.author}`,
    `created\t${cr.createdAt}`,
    `updated\t${cr.updatedAt}`,
    `url\t${cr.crUrl}`,
  ];
  if (cr.description !== "") {
    lines.push("", cr.description);
  }
  return `${lines.join("\n")}\n`;
}

export function webhookText(hooks: Webhook[]): string {
  if (hooks.length === 0) {
    return "No webhooks found\n";
  }
  return renderTable(
    ["ID", "URL", "PUSH", "MR", "TAG", "NOTE", "SECRET"],
    hooks.map((hook) => [
      hook.id,
      hook.url,
      yn(hook.pushEvents),
      yn(hook.mergeRequestsEvents),
      yn(hook.tagPushEvents),
      yn(hook.noteEvents),
      hook.secretToken,
    ]),
  );
}

/** 为什么: secretToken 进 agent 日志会泄漏, 默认打码是合同不是开关彩蛋. */
export function maskHooks(hooks: Webhook[], showSecrets: boolean): Webhook[] {
  if (showSecrets) {
    return hooks;
  }
  return hooks.map((hook) => ({
    ...hook,
    secretToken: hook.secretToken === "" ? "" : "***",
  }));
}

function yn(value: boolean): string {
  return value ? "Y" : "N";
}

function clip(text: string, max: number): string {
  return text.length <= max ? text : text.slice(0, max);
}
