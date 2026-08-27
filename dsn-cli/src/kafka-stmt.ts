import { splitArgs } from "./split";
import { DsnErr } from "./types";

export const PEEK_DEFAULT = 50;
export const KAFKA_MAX = 500;
export const KAFKA_HEADS = ["topics", "peek", "listen"] as const;

export type KafkaStmt =
  | { head: "topics" }
  | { head: "peek"; topic: string; n: number; partition?: number }
  | { head: "listen"; topic: string; partition?: number };

/** 为什么: Gate 和 Driver 必须认同一套小写语句, 否则补全的 token 和拦截会对不上. */
export function parseKafkaStmt(stmt: string): KafkaStmt {
  const args = splitArgs(stmt);
  const head = args[0];
  if (head === undefined) {
    throw new DsnErr("empty statement", 2);
  }
  if (head === "topics") {
    if (args.length > 1) {
      throw new DsnErr("topics takes no arguments", 2);
    }
    return { head: "topics" };
  }
  if (head === "peek") {
    return parsePeek(args.slice(1));
  }
  if (head === "listen") {
    return parseListen(args.slice(1));
  }
  throw new DsnErr(`blocked head: ${head}`, 2);
}

export function isKafkaListen(stmt: string): boolean {
  return splitArgs(stmt)[0] === "listen";
}

export function isKafkaTopics(stmt: string): boolean {
  return splitArgs(stmt)[0] === "topics";
}

export function kafkaComplete(rest: string[], topics: string[]): string[] {
  if (rest.length === 0) {
    return [...KAFKA_HEADS];
  }
  if (rest[0] === "topics") {
    return [];
  }
  if (rest[0] === "peek" || rest[0] === "listen") {
    return rest.length === 1 ? topics : [];
  }
  return [...KAFKA_HEADS];
}

function parsePeek(args: string[]): Extract<KafkaStmt, { head: "peek" }> {
  const topic = args[0];
  if (topic === undefined || topic === "") {
    throw new DsnErr("peek requires a topic", 2);
  }
  let n = PEEK_DEFAULT;
  let index = 1;
  let partition: number | undefined;
  const maybeN = args[index];
  if (maybeN !== undefined && maybeN !== "partition") {
    n = readCount(maybeN);
    index += 1;
  }
  if (args[index] === "partition") {
    partition = readPart(args[index + 1]);
    index += 2;
  }
  if (index !== args.length) {
    throw new DsnErr("invalid peek statement", 2);
  }
  return { head: "peek", topic, n, partition };
}

function parseListen(args: string[]): Extract<KafkaStmt, { head: "listen" }> {
  const topic = args[0];
  if (topic === undefined || topic === "") {
    throw new DsnErr("listen requires a topic", 2);
  }
  let index = 1;
  let partition: number | undefined;
  if (args[index] === "partition") {
    partition = readPart(args[index + 1]);
    index += 2;
  }
  if (index !== args.length) {
    throw new DsnErr("invalid listen statement", 2);
  }
  return { head: "listen", topic, partition };
}

function readCount(raw: string): number {
  if (!/^[1-9][0-9]*$/.test(raw)) {
    throw new DsnErr(`peek n must be 1..${KAFKA_MAX}`, 2);
  }
  const n = Number.parseInt(raw, 10);
  if (n > KAFKA_MAX) {
    throw new DsnErr(`peek n must be 1..${KAFKA_MAX}`, 2);
  }
  return n;
}

function readPart(raw: string | undefined): number {
  if (raw === undefined || !/^[0-9]+$/.test(raw)) {
    throw new DsnErr("partition id required", 2);
  }
  return Number.parseInt(raw, 10);
}
