import { Kafka, logLevel, type Cluster } from "kafkajs";
import ClusterCtor from "kafkajs/src/cluster";
import RequestQueue from "kafkajs/src/network/requestQueue";
import createSocketFactory from "kafkajs/src/network/socketFactory";
import {
  KAFKA_MAX,
  parseKafkaStmt,
  type KafkaStmt,
} from "./kafka-stmt";
import { DsnErr, type QueryOut, type Timeouts } from "./types";

const socketFactory = createSocketFactory();
patchRequestQueueTimeout();

const REC_COLS = ["partition", "offset", "timestamp", "key", "value", "headers"];
const PART_MAX_BYTES = 262144;
const FETCH_MAX_BYTES = 1048576;

export type KafkaRec = {
  partition: number;
  offset: string;
  timestamp: number;
  key: unknown;
  value: unknown;
  headers: Record<string, unknown>;
};

type FetchMsg = {
  offset: string;
  timestamp: string;
  key: Buffer | null;
  value: Buffer | null;
  headers?: Record<string, Buffer | string | Buffer[] | string[] | undefined>;
  isControlRecord?: boolean;
};

type FetchRes = {
  errorCode?: number;
  responses: Array<{
    topicName?: string;
    partitions: Array<{
      partition: number;
      errorCode: number;
      messages?: FetchMsg[];
    }>;
  }>;
};

type TopicMeta = {
  topic: string;
  isInternal?: boolean;
  partitionMetadata: Array<{ partitionId: number; leader: number; replicas: number[] }>;
};

/** 为什么: Peek 禁止 JoinGroup, kafkajs Consumer 强制 groupId, 只能走 Cluster+Fetch. */
export async function queryKafka(
  url: string,
  stmt: string,
  timeouts: Timeouts,
): Promise<QueryOut> {
  if (stmt.trim() === "ping") {
    return await withCluster(url, timeouts, pingKafka);
  }
  const parsed = parseKafkaStmt(stmt);
  if (parsed.head === "topics") {
    return await withCluster(url, timeouts, listTopics);
  }
  if (parsed.head === "peek") {
    const recs = await withCluster(url, timeouts, (cluster) => peekKafka(cluster, parsed, timeouts));
    return recsOut(recs);
  }
  const recs = await withCluster(url, timeouts, (cluster) => listenKafka(cluster, parsed, timeouts));
  return recsOut(recs);
}

export function pickNewest(rows: KafkaRec[], n: number): KafkaRec[] {
  const sorted = [...rows].sort(cmpRec);
  return sorted.length <= n ? sorted : sorted.slice(sorted.length - n);
}

export function decodeBytes(buf: Buffer | null | undefined): unknown {
  if (buf === null || buf === undefined) {
    return null;
  }
  if (buf.length === 0) {
    return "";
  }
  const text = buf.toString("utf8");
  const round = Buffer.from(text, "utf8");
  if (round.length !== buf.length || !round.equals(buf)) {
    return buf.toString("base64");
  }
  const trim = text.trimStart();
  if (trim.startsWith("{") || trim.startsWith("[")) {
    try {
      return JSON.parse(text) as unknown;
    } catch {
      return text;
    }
  }
  return text;
}

async function withCluster<T>(
  url: string,
  timeouts: Timeouts,
  fn: (cluster: Cluster) => Promise<T>,
): Promise<T> {
  const cluster = openCluster(url, timeouts);
  try {
    await cluster.connect();
    return await fn(cluster);
  } finally {
    await cluster.disconnect();
  }
}

function openCluster(url: string, timeouts: Timeouts): Cluster {
  const spec = kafkaSpec(url);
  const kafka = new Kafka({
    clientId: "dsn-cli",
    brokers: [spec.broker],
    ssl: spec.ssl,
    sasl: spec.sasl,
    logLevel: logLevel.NOTHING,
    connectionTimeout: timeouts.connectMs,
    authenticationTimeout: timeouts.connectMs,
    requestTimeout: timeouts.execMs,
    enforceRequestTimeout: true,
    retry: { retries: 0 },
  });
  return new ClusterCtor({
    logger: kafka.logger(),
    socketFactory,
    brokers: [spec.broker],
    ssl: spec.ssl,
    sasl: spec.sasl,
    clientId: "dsn-cli",
    connectionTimeout: timeouts.connectMs,
    authenticationTimeout: timeouts.connectMs,
    requestTimeout: timeouts.execMs,
    enforceRequestTimeout: true,
    metadataMaxAge: 300000,
    retry: { retries: 0 },
    allowAutoTopicCreation: false,
  });
}

/** 为什么: kafkajs 未节流时 setTimeout(throttledUntil - now) 为负数, bun 会打 TimeoutNegativeWarning. */
function patchRequestQueueTimeout(): void {
  RequestQueue.prototype.scheduleCheckPendingRequests = function scheduleCheckPendingRequests() {
    if (this.throttleCheckTimeoutId) {
      return;
    }
    let wait = this.throttledUntil - Date.now();
    if (this.pending.length > 0) {
      wait = wait > 0 ? wait : 10;
    } else if (wait <= 0) {
      return;
    }
    this.throttleCheckTimeoutId = setTimeout(() => {
      this.throttleCheckTimeoutId = null;
      this.checkPendingRequests();
    }, wait);
  };
}

function kafkaSpec(url: string): {
  broker: string;
  ssl: boolean;
  sasl?: { mechanism: "plain"; username: string; password: string };
} {
  const parsed = new URL(url);
  const host = parsed.hostname;
  if (host === "") {
    throw new DsnErr("kafka url host is required", 1);
  }
  const port = parsed.port === "" ? "9092" : parsed.port;
  const user = decodeURIComponent(parsed.username);
  const pass = decodeURIComponent(parsed.password);
  const sasl =
    user === "" && pass === ""
      ? undefined
      : { mechanism: "plain" as const, username: user, password: pass };
  return {
    broker: `${host}:${port}`,
    ssl: parsed.protocol === "kafkas:",
    sasl,
  };
}

async function pingKafka(cluster: Cluster): Promise<QueryOut> {
  await cluster.metadata();
  return { columns: ["ok"], rows: [{ ok: 1 }] };
}

async function listTopics(cluster: Cluster): Promise<QueryOut> {
  const meta = await cluster.metadata();
  const topics = (meta.topicMetadata as TopicMeta[]).filter((item) => !internalTopic(item));
  return {
    columns: ["name", "partitions", "replicas"],
    rows: topics.map((item) => ({
      name: item.topic,
      partitions: item.partitionMetadata.length,
      replicas: item.partitionMetadata[0]?.replicas.length ?? 0,
    })),
  };
}

function internalTopic(item: TopicMeta): boolean {
  return item.isInternal === true || item.topic.startsWith("__");
}

async function peekKafka(
  cluster: Cluster,
  stmt: Extract<KafkaStmt, { head: "peek" }>,
  timeouts: Timeouts,
): Promise<KafkaRec[]> {
  const parts = await topicParts(cluster, stmt.topic, stmt.partition);
  const windows = await peekWindows(cluster, stmt.topic, parts, stmt.n);
  const recs = await fetchWindows(cluster, stmt.topic, windows, Math.min(1000, timeouts.execMs), stmt.n);
  return pickNewest(recs, stmt.n);
}

async function listenKafka(
  cluster: Cluster,
  stmt: Extract<KafkaStmt, { head: "listen" }>,
  timeouts: Timeouts,
): Promise<KafkaRec[]> {
  const parts = await topicParts(cluster, stmt.topic, stmt.partition);
  const highs = await offsetsOf(cluster, stmt.topic, parts, false);
  let cursors = parts.map((part) => {
    const high = highs.get(part) ?? 0n;
    return { partition: part, start: high, stop: high + 1_000_000_000n };
  });
  const recs: KafkaRec[] = [];
  let stop = false;
  const onStop = (): void => {
    stop = true;
  };
  process.on("SIGINT", onStop);
  try {
    const deadline = Date.now() + timeouts.execMs;
    while (!stop && Date.now() < deadline && recs.length < KAFKA_MAX) {
      const remain = deadline - Date.now();
      if (remain <= 0) {
        break;
      }
      const wait = Math.min(5000, remain);
      const batch = await fetchWindows(
        cluster,
        stmt.topic,
        cursors,
        wait,
        KAFKA_MAX - recs.length,
      );
      recs.push(...batch);
      cursors = advance(cursors, batch);
    }
  } finally {
    process.off("SIGINT", onStop);
  }
  return recs.slice(0, KAFKA_MAX);
}

async function topicParts(cluster: Cluster, topic: string, partition?: number): Promise<number[]> {
  await cluster.addTargetTopic(topic);
  await cluster.refreshMetadataIfNecessary();
  const meta = cluster.findTopicPartitionMetadata(topic);
  if (meta.length === 0) {
    throw new DsnErr(`unknown topic ${topic}`, 3);
  }
  if (partition === undefined) {
    return meta.map((item) => item.partitionId);
  }
  if (!meta.some((item) => item.partitionId === partition)) {
    throw new DsnErr(`unknown partition ${partition}`, 3);
  }
  return [partition];
}

async function peekWindows(
  cluster: Cluster,
  topic: string,
  parts: number[],
  n: number,
): Promise<Array<{ partition: number; start: bigint; stop: bigint }>> {
  const highs = await offsetsOf(cluster, topic, parts, false);
  const lows = await offsetsOf(cluster, topic, parts, true);
  const take = BigInt(n);
  const out: Array<{ partition: number; start: bigint; stop: bigint }> = [];
  for (const part of parts) {
    const high = highs.get(part) ?? 0n;
    const low = lows.get(part) ?? 0n;
    if (high <= low) {
      continue;
    }
    let start = high - take;
    if (start < low) {
      start = low;
    }
    out.push({ partition: part, start, stop: high });
  }
  return out;
}

async function offsetsOf(
  cluster: Cluster,
  topic: string,
  parts: number[],
  fromBeginning: boolean,
): Promise<Map<number, bigint>> {
  const listed = await cluster.fetchTopicsOffset([
    {
      topic,
      fromBeginning,
      partitions: parts.map((partition) => ({ partition })),
    },
  ]);
  const first = listed[0];
  const map = new Map<number, bigint>();
  if (first === undefined) {
    return map;
  }
  for (const item of first.partitions) {
    map.set(item.partition, BigInt(item.offset));
  }
  return map;
}

async function fetchWindows(
  cluster: Cluster,
  topic: string,
  windows: Array<{ partition: number; start: bigint; stop: bigint }>,
  waitMs: number,
  remain: number,
): Promise<KafkaRec[]> {
  if (windows.length === 0 || remain <= 0) {
    return [];
  }
  const ids = windows.map((item) => item.partition);
  const byLeader = cluster.findLeaderForPartitions(topic, ids);
  const recs: KafkaRec[] = [];
  for (const [nodeId, parts] of Object.entries(byLeader)) {
    if (recs.length >= remain) {
      break;
    }
    const want = windows.filter((item) => parts.includes(item.partition));
    if (want.length === 0) {
      continue;
    }
    const broker = await cluster.findBroker({ nodeId });
    const res = (await broker.fetch({
      replicaId: -1,
      isolationLevel: 0,
      maxWaitTime: waitMs,
      minBytes: 1,
      maxBytes: FETCH_MAX_BYTES,
      topics: [
        {
          topic,
          partitions: want.map((item) => ({
            partition: item.partition,
            fetchOffset: item.start.toString(),
            maxBytes: PART_MAX_BYTES,
          })),
        },
      ],
    })) as FetchRes;
    recs.push(...readFetch(res, want, remain - recs.length));
  }
  return recs;
}

function readFetch(
  res: FetchRes,
  windows: Array<{ partition: number; start: bigint; stop: bigint }>,
  remain: number,
): KafkaRec[] {
  if (res.errorCode !== undefined && res.errorCode !== 0) {
    throw new DsnErr(`kafka fetch error ${res.errorCode}`, 3);
  }
  const stopOf = new Map(windows.map((item) => [item.partition, item.stop]));
  const recs: KafkaRec[] = [];
  for (const topicRes of res.responses) {
    for (const part of topicRes.partitions) {
      if (part.errorCode !== 0) {
        throw new DsnErr(`kafka partition ${part.partition} error ${part.errorCode}`, 3);
      }
      const stop = stopOf.get(part.partition);
      for (const msg of part.messages ?? []) {
        if (recs.length >= remain) {
          return recs;
        }
        if (msg.isControlRecord === true) {
          continue;
        }
        const offset = BigInt(msg.offset);
        if (stop !== undefined && offset >= stop) {
          continue;
        }
        recs.push(toRec(part.partition, msg));
      }
    }
  }
  return recs;
}

function toRec(partition: number, msg: FetchMsg): KafkaRec {
  return {
    partition,
    offset: msg.offset,
    timestamp: Number(msg.timestamp),
    key: decodeBytes(asBuf(msg.key)),
    value: decodeBytes(asBuf(msg.value)),
    headers: decodeHeaders(msg.headers),
  };
}

function asBuf(value: Buffer | null): Buffer | null {
  if (value === null) {
    return null;
  }
  return Buffer.isBuffer(value) ? value : Buffer.from(value);
}

function decodeHeaders(
  headers: FetchMsg["headers"],
): Record<string, unknown> {
  if (headers === undefined) {
    return {};
  }
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(headers)) {
    if (Array.isArray(value)) {
      out[key] = value.map((item) => decodeBytes(asBuf(item as Buffer)));
      continue;
    }
    if (typeof value === "string") {
      out[key] = value;
      continue;
    }
    out[key] = decodeBytes(asBuf(value ?? null));
  }
  return out;
}

function advance(
  cursors: Array<{ partition: number; start: bigint; stop: bigint }>,
  batch: KafkaRec[],
): Array<{ partition: number; start: bigint; stop: bigint }> {
  const last = new Map<number, bigint>();
  for (const rec of batch) {
    last.set(rec.partition, BigInt(rec.offset) + 1n);
  }
  return cursors.map((cur) => {
    const next = last.get(cur.partition);
    return next === undefined ? cur : { ...cur, start: next };
  });
}

function recsOut(recs: KafkaRec[]): QueryOut {
  return {
    columns: REC_COLS,
    rows: recs.map((rec) => ({
      partition: rec.partition,
      offset: rec.offset,
      timestamp: Number.isFinite(rec.timestamp) ? new Date(rec.timestamp).toISOString() : "",
      key: rec.key,
      value: rec.value,
      headers: rec.headers,
    })),
  };
}

function cmpRec(a: KafkaRec, b: KafkaRec): number {
  if (a.timestamp !== b.timestamp) {
    return a.timestamp - b.timestamp;
  }
  if (a.partition !== b.partition) {
    return a.partition - b.partition;
  }
  const ao = BigInt(a.offset);
  const bo = BigInt(b.offset);
  if (ao < bo) {
    return -1;
  }
  if (ao > bo) {
    return 1;
  }
  return 0;
}
