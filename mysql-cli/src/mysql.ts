import type { MysqlConnection, OutputFormat } from "./types";

export type MysqlInvocation = {
  command: string;
  args: string[];
  captureOutput: boolean;
};

export type InvocationOptions = {
  execute?: string;
  output?: OutputFormat;
};

// 密码以单个 -p<password> 参数传给 mysql, 避免触发交互式 password prompt.
export function buildMysqlInvocation(
  mysqlPath: string,
  connection: MysqlConnection,
  options: InvocationOptions,
): MysqlInvocation {
  const args: string[] = ["-h", connection.host];

  if (connection.port) {
    args.push("-P", connection.port);
  }
  if (connection.user) {
    args.push("-u", connection.user);
  }
  if (connection.password) {
    args.push(`-p${connection.password}`);
  }
  if (options.output) {
    args.push("--batch", "--raw");
  }
  if (options.execute) {
    args.push("-e", options.execute);
  }
  if (connection.database) {
    args.push(connection.database);
  }

  return {
    command: mysqlPath,
    args,
    captureOutput: Boolean(options.output),
  };
}

// which 的结果是用户当前 PATH 下的 mysql, 符合包装器定位规则.
export function resolveMysqlPath(): string {
  const result = Bun.spawnSync(["which", "mysql"], {
    stdout: "pipe",
    stderr: "pipe",
  });

  if (result.exitCode !== 0) {
    throw new Error("mysql client not found. Please install mysql and ensure it is available in PATH");
  }

  return new TextDecoder().decode(result.stdout).trim();
}
