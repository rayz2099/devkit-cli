type Table = {
  headers: string[];
  rows: string[][];
};

type StreamFormat = "json" | "csv";

// mysql batch 输出天然是表格流, 这里统一解析后再交给不同格式编码.
function parseTabOutput(content: string): Table {
  const lines = content.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  if (lines.length === 1 && lines[0] === "") {
    return { headers: [], rows: [] };
  }

  const headers = lines[0]?.split("\t") ?? [];
  const rows = lines.slice(1).filter((line) => line.length > 0).map((line) => line.split("\t"));

  return { headers, rows };
}

export function toJsonLines(content: string): string {
  const table = parseTabOutput(content);
  return table.rows.map((row) => encodeJsonLine(table.headers, row)).join("\n");
}

export function toCsv(content: string): string {
  const table = parseTabOutput(content);
  if (table.headers.length === 0) {
    return "";
  }

  return [table.headers, ...table.rows]
    .map((row) => row.map(escapeCsvCell).join(","))
    .join("\n");
}

function escapeCsvCell(value: string): string {
  if (!/[",\n\r]/.test(value)) {
    return value;
  }

  return `"${value.replaceAll('"', '""')}"`;
}

function encodeJsonLine(headers: string[], row: string[]): string {
  return JSON.stringify(Object.fromEntries(headers.map((header, index) => [header, row[index] ?? ""])));
}

function encodeCsvLine(row: string[]): string {
  return row.map(escapeCsvCell).join(",");
}

export function createTabStreamFormatter(format: StreamFormat) {
  let buffer = "";
  let headers: string[] | undefined;

  function encodeRow(row: string[]): string {
    if (format === "json") {
      return `${encodeJsonLine(headers ?? [], row)}\n`;
    }
    return `${encodeCsvLine(row)}\n`;
  }

  return {
    write(chunk: string): string {
      buffer += chunk.replace(/\r\n/g, "\n");
      const lines = buffer.split("\n");
      buffer = lines.pop() ?? "";

      let output = "";
      for (const line of lines) {
        if (headers === undefined) {
          headers = line.split("\t");
          if (format === "csv") {
            output += `${encodeCsvLine(headers)}\n`;
          }
          continue;
        }

        if (line.length > 0) {
          output += encodeRow(line.split("\t"));
        }
      }

      return output;
    },

    end(): string {
      if (buffer.length === 0) {
        return "";
      }

      if (headers === undefined) {
        headers = buffer.split("\t");
        buffer = "";
        return format === "csv" ? `${encodeCsvLine(headers)}\n` : "";
      }

      const output = encodeRow(buffer.split("\t"));
      buffer = "";
      return output;
    },
  };
}
