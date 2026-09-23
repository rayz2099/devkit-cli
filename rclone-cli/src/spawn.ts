import type { ErrSink, SpawnFn, SpawnRet } from "./types";

/** 为什么: 进度在 stderr 上是流, 收完再一次性打印会让人类看不到传输还在跑. */
export const spawnProc: SpawnFn = async (bin, args, sink) => {
  const proc = Bun.spawn([bin, ...args], {
    stdout: "pipe",
    stderr: "pipe",
    stdin: "ignore",
  });
  const out: string[] = [];
  const err: string[] = [];
  await Promise.all([
    drain(proc.stdout, (chunk) => {
      out.push(chunk);
    }),
    drain(proc.stderr, (chunk) => {
      err.push(chunk);
      sink.writeErr(chunk);
    }),
    proc.exited,
  ]);
  return {
    code: proc.exitCode ?? 1,
    stdout: out.join(""),
    stderr: err.join(""),
  };
};

export function noopSink(): ErrSink {
  return { writeErr: () => {} };
}

export function stderrSink(): ErrSink {
  return {
    writeErr: (chunk) => {
      process.stderr.write(chunk);
    },
  };
}

async function drain(stream: ReadableStream<Uint8Array>, onChunk: (chunk: string) => void): Promise<void> {
  const reader = stream.getReader();
  const dec = new TextDecoder();
  while (true) {
    const step = await reader.read();
    if (step.done) {
      const rest = dec.decode();
      if (rest !== "") {
        onChunk(rest);
      }
      return;
    }
    onChunk(dec.decode(step.value, { stream: true }));
  }
}

export function obscureArgs(cfgFile: string, plain: string): string[] {
  return ["--config", cfgFile, "obscure", "--", plain];
}

/** 为什么: crypt 和 webdav 的口令字段都要求 obscure 之后的形式, 明文放进连接串 rclone 会当成另一种密钥. */
export async function obscureOne(
  bin: string,
  cfgFile: string,
  plain: string,
  spawn: SpawnFn,
): Promise<string> {
  const ret: SpawnRet = await spawn(bin, obscureArgs(cfgFile, plain), noopSink());
  if (ret.code !== 0) {
    throw new Error(`rclone obscure failed (${ret.code})`);
  }
  const line = ret.stdout.trim();
  if (line === "") {
    throw new Error("rclone obscure returned empty");
  }
  return line;
}
