import { readFileSync } from "node:fs";
import { getReport, listEvents, listProjects, listProps, listReports, login } from "./api";
import type { Session } from "./api";
import { helpText } from "./args";
import { cfgPath, loadFileCfg, pickProfile } from "./config";
import { fishScript } from "./fish";
import { completeLines } from "./complete";
import {
  emit,
  eventRows,
  jsonText,
  projectRows,
  propRows,
  queryText,
  renderTable,
  reportRows,
} from "./output";
import { assertReadModel, eventQp, modelPrefix, propertyQp, takeReport } from "./qp";
import { resolveSpan } from "./time";
import type { CliCmd, Profile, Scope } from "./types";
import { asInt, originOf } from "./util";
import { runQuery } from "./ws";

/** 为什么: 入口只编排只读命令, 写入接口不在这里出现. */
export async function runCmd(cmd: CliCmd): Promise<string> {
  if (cmd.kind === "completion-fish") {
    return fishScript();
  }
  if (cmd.kind === "complete") {
    return await completeLines(cmd.tokens, cmd.current);
  }
  if (cmd.kind === "help") {
    const text = helpText(cmd.topic);
    return emit(cmd.audience, { usage: text }, text);
  }
  if (cmd.kind === "auth-login") {
    return await doLogin(cmd);
  }
  if (cmd.kind === "auth-status") {
    return doStatus(cmd);
  }
  if (cmd.kind === "project-ls") {
    const session = await open(cmd);
    const data = await listProjects(session);
    return emit(cmd.audience, data, rowsOrJson(["ID", "NAME"], projectRows(data), data));
  }
  if (cmd.kind === "meta-events") {
    const session = await open(cmd);
    const data = await listEvents(session, cmd.project);
    return emit(cmd.audience, data, rowsOrJson(["EVENT", "DESC"], eventRows(data), data));
  }
  if (cmd.kind === "meta-props") {
    const session = await open(cmd);
    const data = await listProps(session, cmd.project, cmd.event);
    return emit(cmd.audience, data, rowsOrJson(["PROP", "DESC", "TABLE"], propRows(data), data));
  }
  if (cmd.kind === "report-ls") {
    const session = await open(cmd);
    const data = await listReports(session, cmd.project);
    return emit(cmd.audience, data, rowsOrJson(["ID", "NAME", "MODEL"], reportRows(data), data));
  }
  if (cmd.kind === "report-get") {
    const session = await open(cmd);
    const data = await getReport(session, cmd.project, cmd.reportId);
    return emit(cmd.audience, data, jsonText(data));
  }
  if (cmd.kind === "query-event" || cmd.kind === "query-property" || cmd.kind === "query-run" || cmd.kind === "query-report") {
    return await doQuery(cmd);
  }
  return missed(cmd);
}

async function doLogin(cmd: Extract<CliCmd, { kind: "auth-login" }>): Promise<string> {
  const profile = loadProfile(cmd);
  const session = await connect(profile);
  const view = { profile: profile.name, url: session.host, username: profile.username, ok: true };
  const text = `profile\t${view.profile}\nurl\t${view.url}\nusername\t${view.username}\nok\ttrue\n`;
  return emit(cmd.audience, view, text);
}

function doStatus(cmd: Extract<CliCmd, { kind: "auth-status" }>): string {
  const profile = loadProfile(cmd);
  const view = { profile: profile.name, url: profile.url, username: profile.username };
  const text = `profile\t${view.profile}\nurl\t${view.url}\nusername\t${view.username}\n`;
  return emit(cmd.audience, view, text);
}

async function doQuery(
  cmd: Extract<CliCmd, { kind: "query-event" | "query-property" | "query-run" | "query-report" }>,
): Promise<string> {
  const session = await open(cmd);
  const projectId = asInt(cmd.project, "--project");
  const built = await buildQuery(session, cmd);
  assertReadModel(built.model);
  const onProgress = cmd.audience === "human"
    ? (line: string) => {
        process.stderr.write(`${line}\n`);
      }
    : undefined;
  const result = await runQuery({
    host: session.host,
    token: session.token,
    bff: session.bff,
    prefix: modelPrefix(built.model),
    projectId,
    eventModel: built.model,
    qp: built.qp,
    timeoutMs: cmd.timeoutMs,
    onProgress,
  });
  return emit(cmd.audience, result, queryText(result));
}

async function buildQuery(
  session: Session,
  cmd: Extract<CliCmd, { kind: "query-event" | "query-property" | "query-run" | "query-report" }>,
): Promise<{ model: number; qp: unknown }> {
  if (cmd.kind === "query-run") {
    assertReadModel(cmd.model);
    return { model: cmd.model, qp: readQp(cmd.qp) };
  }
  if (cmd.kind === "query-report") {
    const data = await getReport(session, cmd.project, cmd.reportId);
    return takeReport(data, cmd.model);
  }
  if (cmd.qp !== undefined) {
    const model = cmd.kind === "query-event" ? 0 : 3;
    return { model, qp: readQp(cmd.qp) };
  }
  if (cmd.kind === "query-event") {
    const span = resolveSpan(cmd.recent, cmd.from, cmd.to, true, new Date());
    if (span === undefined) {
      throw new Error("event query needs --recent or --from/--to");
    }
    return {
      model: 0,
      qp: eventQp({
        eventName: cmd.eventName,
        analysis: cmd.analysis,
        span,
        particle: cmd.particle,
        group: cmd.group,
      }),
    };
  }
  return {
    model: 3,
    qp: propertyQp({
      analysis: cmd.analysis,
      quota: cmd.quota,
      group: cmd.group,
      span: resolveSpan(cmd.recent, cmd.from, cmd.to, false, new Date()),
    }),
  };
}

function readQp(path: string): unknown {
  const text = readFileSync(path, "utf8");
  return JSON.parse(text) as unknown;
}

async function open(cmd: Scope): Promise<Session> {
  return await connect(loadProfile(cmd));
}

function loadProfile(cmd: Scope): Profile {
  const path = cmd.config ?? cfgPath();
  return pickProfile(loadFileCfg(path), cmd.profile);
}

/** 为什么: 令牌只活在这一次进程里, 配置改完不用再单独 login. */
async function connect(profile: Profile): Promise<Session> {
  if (profile.username === "" || profile.password === "") {
    throw new Error(`profile ${profile.name} needs username and password`);
  }
  const host = originOf(profile.url);
  const token = await login(host, profile.username, profile.password);
  return { host, token: token.accessToken, bff: false };
}

function rowsOrJson(headers: string[], rows: string[][], data: unknown): string {
  if (rows.length === 0) {
    return jsonText(data);
  }
  return renderTable(headers, rows);
}

/** 为什么: 新增命令漏接时编译期就能失败, 运行期也不静默走到别的分支. */
function missed(value: never): never {
  throw new Error(`unknown command: ${JSON.stringify(value)}`);
}
