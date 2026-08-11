#!/usr/bin/env bun
import { helpText, parseCliArgs } from "./args";
import { findProfile, loadConfig, parseJdbcUrl } from "./config";
import { profileCompletion } from "./fish";
import { buildMysqlInvocation, resolveMysqlPath } from "./mysql";
import { createTabStreamFormatter } from "./output";
import { createMysqlStderrFilter } from "./stderr";

async function main(argv: string[]): Promise<number> {
  try {
    const command = parseCliArgs(argv);

    if (command.kind === "help") {
      console.log(helpText());
      return 0;
    }

    const config = loadConfig();

    if (command.kind === "completion") {
      process.stdout.write(profileCompletion(config));
      return 0;
    }

    const profile = findProfile(config, command.profile);
    const connection = parseJdbcUrl(profile.name, profile.jdbcUrl);
    const mysqlPath = resolveMysqlPath();
    const invocation = buildMysqlInvocation(mysqlPath, connection, {
      execute: command.execute,
      output: command.output,
    });

    if (!invocation.captureOutput) {
      const child = Bun.spawn([invocation.command, ...invocation.args], {
        stdin: "inherit",
        stdout: "inherit",
        stderr: "pipe",
      });
      const stderrDone = writeFilteredStderr(child.stderr);
      const exitCode = await child.exited;
      await stderrDone;
      return exitCode;
    }

    const child = Bun.spawn([invocation.command, ...invocation.args], {
      stdout: "pipe",
      stderr: "pipe",
    });
    const stderrDone = writeFilteredStderr(child.stderr);
    await writeFormattedOutput(child.stdout, command.output);
    const exitCode = await child.exited;
    await stderrDone;
    return exitCode;
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    return 1;
  }
}

async function writeFilteredStderr(stream: ReadableStream<Uint8Array>): Promise<void> {
  const filter = createMysqlStderrFilter();
  const reader = stream.pipeThrough(new TextDecoderStream()).getReader();

  while (true) {
    const result = await reader.read();
    if (result.done) {
      break;
    }
    process.stderr.write(filter.write(result.value));
  }

  process.stderr.write(filter.end());
}

async function writeFormattedOutput(
  stream: ReadableStream<Uint8Array>,
  output: "json" | "csv" | undefined,
): Promise<void> {
  if (!output) {
    return;
  }

  const formatter = createTabStreamFormatter(output);
  const reader = stream.pipeThrough(new TextDecoderStream()).getReader();

  while (true) {
    const result = await reader.read();
    if (result.done) {
      break;
    }
    process.stdout.write(formatter.write(result.value));
  }

  process.stdout.write(formatter.end());
}

const exitCode = await main(Bun.argv.slice(2));
process.exit(exitCode);
