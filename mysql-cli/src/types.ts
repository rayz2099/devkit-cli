export type OutputFormat = "json" | "csv";

export type ProfileConfig = {
  name: string;
  jdbcUrl: string;
};

export type MysqlCliConfig = {
  profiles: ProfileConfig[];
};

export type MysqlConnection = {
  host: string;
  port?: string;
  user?: string;
  password?: string;
  database?: string;
};

export type RunCommand = {
  kind: "run";
  profile: string;
  execute?: string;
  output?: OutputFormat;
};

export type HelpCommand = {
  kind: "help";
};

export type CompletionCommand = {
  kind: "completion";
};

export type CliCommand = RunCommand | HelpCommand | CompletionCommand;
