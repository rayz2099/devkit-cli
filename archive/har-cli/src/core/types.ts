export type OutputFormat = "text" | "json";

export interface HarHeader {
  name: string;
  value: string;
}

export interface CliResult {
  stdout: string;
  stderr: string;
  exitCode: number;
}

export interface ScanOptions {
  files: string[];
  filter?: string;
  select: string;
  prefixDepth: number;
  format: OutputFormat;
  dedupe: boolean;
}

export interface NormalizedEntry {
  sourceFile: string;
  method: string;
  url: string;
  path: string;
  pathWithQuery: string;
  httpStatus: number;
  statusText?: string;
  mimeType?: string;
  body: unknown;
  requestHttpVersion?: string;
  responseHttpVersion?: string;
  requestHeaders: HarHeader[];
  responseHeaders: HarHeader[];
  requestBodyText?: string;
  responseBodyText?: string;
}
