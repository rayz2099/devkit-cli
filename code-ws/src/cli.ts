import { join } from "node:path";
import type { ServeArgs } from "./serve";

export type InitProfileArgs = {
  cmd: "init";
  branch: string;
  profile: string;
  config?: string;
  verbose: boolean;
};

export type InitProjectArgs = {
  cmd: "init";
  branch: string;
  project: string;
  config?: string;
  verbose: boolean;
};

export type InitArgs = InitProfileArgs | InitProjectArgs;

export type ListArgs = {
  cmd: "list";
  config?: string;
};

export type ProjectsArgs = {
  cmd: "projects";
  config?: string;
};

export type CfgCheckArgs = {
  cmd: "config-check";
  config?: string;
};

export type CompletionFishArgs = {
  cmd: "completion-fish";
};

export type AddProjectArgs = {
  cmd: "add-project";
  repo: string;
  branch?: string;
  config?: string;
  verbose: boolean;
};

export type RemoveProjectArgs = {
  cmd: "remove-project";
  repo: string;
  config?: string;
  verbose: boolean;
};

export type SyncMasterArgs = {
  cmd: "sync-master";
  config?: string;
  verbose: boolean;
};

export type DestroyWorkspaceArgs = {
  cmd: "destroy-workspace";
  config?: string;
  verbose: boolean;
};

export type ForkWorkspaceArgs = {
  cmd: "fork-workspace";
  branch: string;
  config?: string;
  verbose: boolean;
};

export type HelpArgs = {
  cmd: "help";
};

export type CliArgs =
  | InitArgs
  | ListArgs
  | ProjectsArgs
  | CfgCheckArgs
  | CompletionFishArgs
  | AddProjectArgs
  | RemoveProjectArgs
  | SyncMasterArgs
  | DestroyWorkspaceArgs
  | ForkWorkspaceArgs
  | ServeArgs
  | HelpArgs;

/**
 * Bun compile 会把 import.meta.url 指到 /$bunfs, 所以二进制运行时要改用 execPath。
 */
export function resolveDefaultConfig(
  metaUrl: string,
  execPath: string,
  env: NodeJS.ProcessEnv = process.env,
): string {
  const xdg = env.XDG_CONFIG_HOME;
  if (xdg !== undefined && xdg.length > 0) {
    return join(xdg, "code-ws", "config.json");
  }

  const home = env.HOME;
  if (home === undefined || home.length === 0) {
    throw new Error("HOME must be set");
  }

  return join(home, ".config", "code-ws", "config.json");
}

export const defaultCfg = resolveDefaultConfig(import.meta.url, process.execPath);

function valueAfter(
  args: string[],
  idx: number,
  flag: string,
): string {
  const val = args[idx + 1];
  if (val === undefined || val.startsWith("-")) {
    throw new Error(`${flag} requires a value`);
  }
  return val;
}

function parseOpts(args: string[]): {
  rest: string[];
  profile?: string;
  branch?: string;
  config?: string;
  help: boolean;
  verbose: boolean;
  lan: boolean;
  watch: boolean;
  port?: number;
} {
  const rest: string[] = [];
  let profile: string | undefined;
  let branch: string | undefined;
  let config: string | undefined;
  let help = false;
  let verbose = false;
  // serve 默认暴露局域网, 跨设备预览更常见; --local 才收回到 loopback.
  let lan = true;
  let watch = true;
  let port: number | undefined;

  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (arg === "-t" || arg === "--template") {
      profile = valueAfter(args, i, arg);
      i += 1;
    } else if (arg === "-b" || arg === "--branch") {
      branch = valueAfter(args, i, arg);
      i += 1;
    } else if (arg === "--config") {
      config = valueAfter(args, i, arg);
      i += 1;
    } else if (arg === "--port") {
      const raw = valueAfter(args, i, arg);
      const parsedPort = Number(raw);
      if (!Number.isInteger(parsedPort)) {
        throw new Error("--port requires an integer");
      }
      port = parsedPort;
      i += 1;
    } else if (arg === "--lan") {
      lan = true;
    } else if (arg === "--local") {
      lan = false;
    } else if (arg === "--no-watch") {
      watch = false;
    } else if (arg === "-h" || arg === "--help") {
      help = true;
    } else if (arg === "-v" || arg === "--verbose") {
      verbose = true;
    } else if (arg !== undefined) {
      rest.push(arg);
    }
  }

  return {
    rest,
    profile,
    branch,
    config,
    help,
    verbose,
    lan,
    watch,
    port,
  };
}

/**
 * 参数解析保持无副作用, 因为 CLI 行为需要能独立测试。
 */
export function parseCliArgs(args: string[]): CliArgs {
  const parsed = parseOpts(args);
  const [cmd, sub, ...tail] = parsed.rest;

  if (parsed.help) {
    return {
      cmd: "help",
    };
  }

  if (cmd === "init") {
    if (sub === undefined || tail.length > 1) {
      throw new Error(
        "usage: code-ws init <branch> <project> or code-ws init <branch> -t <profile>",
      );
    }
    const [project] = tail;
    if (project !== undefined && parsed.profile !== undefined) {
      throw new Error("usage: code-ws init <branch> <project>");
    }
    if (project !== undefined) {
      return {
        cmd: "init",
        branch: sub,
        project,
        config: parsed.config,
        verbose: parsed.verbose,
      };
    }
    if (parsed.profile === undefined) {
      throw new Error("profile is required: -t <profile>");
    }
    return {
      cmd: "init",
      branch: sub,
      profile: parsed.profile,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "projects") {
    if (sub !== undefined) {
      throw new Error("usage: code-ws projects");
    }
    return {
      cmd: "projects",
      config: parsed.config,
    };
  }

  if (cmd === "completion" && sub === "fish" && tail.length === 0) {
    return {
      cmd: "completion-fish",
    };
  }

  if (cmd === "list") {
    if (sub !== undefined) {
      throw new Error("usage: code-ws list");
    }
    return {
      cmd: "list",
      config: parsed.config,
    };
  }

  if (cmd === "add" && sub === "project") {
    const [repo, ...extra] = tail;
    if (repo === undefined || extra.length > 0) {
      throw new Error(
        "usage: code-ws add project <repo> [-b|--branch <branch>] [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "add-project",
      repo,
      branch: parsed.branch,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "remove" && sub === "project") {
    const [repo, ...extra] = tail;
    if (repo === undefined || extra.length > 0) {
      throw new Error(
        "usage: code-ws remove project <repo> [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "remove-project",
      repo,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "sync" && sub === "master") {
    if (tail.length > 0) {
      throw new Error(
        "usage: code-ws sync master [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "sync-master",
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "destroy") {
    if (sub !== undefined) {
      throw new Error(
        "usage: code-ws destroy [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "destroy-workspace",
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "fork") {
    if (sub === undefined || tail.length > 0) {
      throw new Error(
        "usage: code-ws fork <branch> [-v|--verbose] [--config <path>]",
      );
    }
    return {
      cmd: "fork-workspace",
      branch: sub,
      config: parsed.config,
      verbose: parsed.verbose,
    };
  }

  if (cmd === "serve") {
    if (tail.length > 0) {
      throw new Error(
        "usage: code-ws serve [path] [--lan|--local] [--port <n>] [--no-watch]",
      );
    }
    return {
      cmd: "serve",
      path: sub,
      lan: parsed.lan,
      watch: parsed.watch,
      port: parsed.port,
    };
  }

  if (cmd === "config" && sub === "check" && tail.length === 0) {
    return {
      cmd: "config-check",
      config: parsed.config,
    };
  }

  if (cmd === undefined || cmd === "help" || cmd === "--help" || cmd === "-h") {
    return {
      cmd: "help",
    };
  }

  throw new Error(`unknown command: ${parsed.rest.join(" ")}`);
}
