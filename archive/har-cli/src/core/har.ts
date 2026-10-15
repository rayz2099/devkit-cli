import { buildUriView } from "./uri";
import type { HarHeader, NormalizedEntry } from "./types";

interface HarFile {
  log?: {
    entries?: HarEntry[];
  };
}

interface HarEntry {
  request?: {
    method?: string;
    url?: string;
    httpVersion?: string;
    headers?: HarHeader[];
    postData?: {
      mimeType?: string;
      text?: string;
      encoding?: string;
    };
  };
  response?: {
    status?: number;
    statusText?: string;
    httpVersion?: string;
    headers?: HarHeader[];
    content?: {
      mimeType?: string;
      text?: string;
      encoding?: string;
    };
  };
}

/**
 * 读取并归一化多个 HAR 文件，供过滤与输出阶段复用。
 */
export async function readHarEntries(files: string[]): Promise<NormalizedEntry[]> {
  const results: NormalizedEntry[] = [];

  for (const file of files) {
    const parsed = JSON.parse(await Bun.file(file).text()) as HarFile;
    const entries = parsed.log?.entries ?? [];

    for (const entry of entries) {
      const requestUrl = entry.request?.url;
      const requestMethod = entry.request?.method;
      const responseStatus = entry.response?.status;

      if (!requestUrl || !requestMethod || responseStatus === undefined) {
        continue;
      }

      const uri = buildUriView(requestUrl, 1);
      results.push({
        sourceFile: file,
        method: requestMethod,
        url: uri.url,
        path: uri.path,
        pathWithQuery: buildPathWithQuery(requestUrl),
        httpStatus: responseStatus,
        statusText: entry.response?.statusText,
        mimeType: entry.response?.content?.mimeType,
        body: parseBody(entry.response?.content?.text, entry.response?.content?.encoding),
        requestHttpVersion: entry.request?.httpVersion,
        responseHttpVersion: entry.response?.httpVersion,
        requestHeaders: entry.request?.headers ?? [],
        responseHeaders: entry.response?.headers ?? [],
        requestBodyText: decodeBody(entry.request?.postData?.text, entry.request?.postData?.encoding),
        responseBodyText: decodeBody(entry.response?.content?.text, entry.response?.content?.encoding),
      });
    }
  }

  return results;
}

/**
 * 仅在响应体是合法 JSON 时返回对象，否则返回 undefined。
 */
function parseBody(contentText?: string, encoding?: string): unknown {
  const decoded = decodeBody(contentText, encoding);
  if (decoded === undefined) {
    return undefined;
  }

  try {
    return JSON.parse(decoded);
  } catch {
    return undefined;
  }
}

function decodeBody(contentText?: string, encoding?: string): string | undefined {
  if (typeof contentText !== "string") {
    return undefined;
  }

  return encoding === "base64"
    ? Buffer.from(contentText, "base64").toString("utf8")
    : contentText;
}

function buildPathWithQuery(rawUrl: string): string {
  const parsed = new URL(rawUrl);
  return `${parsed.pathname}${parsed.search}`;
}
