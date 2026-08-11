const PASSWORD_WARNING =
  "mysql: [Warning] Using a password on the command line interface can be insecure.";

export function filterMysqlStderr(content: string): string {
  return content
    .split(/(\r?\n)/)
    .reduce(
      (state, part) => {
        if (part === "\n" || part === "\r\n") {
          const line = state.current;
          state.current = "";
          if (line !== PASSWORD_WARNING) {
            state.output += `${line}${part}`;
          }
          return state;
        }

        state.current += part;
        return state;
      },
      { current: "", output: "" },
    ).output;
}

// stderr 仍保持流式, 只过滤 mysql 固定密码 warning, 其他错误必须即时透出.
export function createMysqlStderrFilter() {
  let buffer = "";

  return {
    write(chunk: string): string {
      buffer += chunk;
      const lastNewline = Math.max(buffer.lastIndexOf("\n"), buffer.lastIndexOf("\r"));
      if (lastNewline === -1) {
        return "";
      }

      const complete = buffer.slice(0, lastNewline + 1);
      buffer = buffer.slice(lastNewline + 1);
      return filterMysqlStderr(complete);
    },

    end(): string {
      if (buffer.length === 0) {
        return "";
      }

      const output = buffer === PASSWORD_WARNING ? "" : buffer;
      buffer = "";
      return output;
    },
  };
}
