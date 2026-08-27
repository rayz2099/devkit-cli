import { expect, test } from "bun:test";
import { runDriver } from "../src/drivers";

test("es 在 bun 下请求成功后 close 不炸", async () => {
  const server = Bun.serve({
    port: 0,
    fetch() {
      return new Response(JSON.stringify({ cluster_name: "local" }), {
        headers: {
          "content-type": "application/json",
          "x-elastic-product": "Elasticsearch",
        },
      });
    },
  });
  try {
    const url = `http://127.0.0.1:${server.port}`;
    const out = await runDriver("elasticsearch", url, "GET /", {
      connectMs: 1000,
      execMs: 1000,
    });
    expect(out.rows[0]?.cluster_name).toBe("local");
  } finally {
    await server.stop(true);
  }
});
