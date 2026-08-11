import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { isMap, parseDocument, YAMLMap, type Document, type Node } from "yaml";
import type { ImageDb, ImageRecord, SyncStatus } from "./types";

const aliasPattern = /^[a-z0-9]+(?:[._-][a-z0-9]+)*$/;
const versionPattern = /^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/;

export function validateAlias(alias: string): void {
  if (!aliasPattern.test(alias)) {
    throw new Error(`invalid alias: ${alias}; expected a single lowercase repository name`);
  }
}

export function validateVersion(version: string): void {
  if (!versionPattern.test(version)) {
    throw new Error(`invalid Docker tag: ${version}`);
  }
}

export function splitSource(raw: string): { source: string; version?: string } {
  const value = raw.trim();
  if (value.length === 0 || value.includes("@")) {
    throw new Error("source must be a non-empty tagged image name, digest references are not supported");
  }
  const slash = value.lastIndexOf("/");
  const colon = value.lastIndexOf(":");
  if (colon > slash) {
    return {
      source: value.slice(0, colon),
      version: value.slice(colon + 1),
    };
  }
  return { source: value };
}

export class ImagesStore {
  readonly doc: Document;

  constructor(readonly path: string) {
    const text = existsSync(path) ? readFileSync(path, "utf8") : "images: {}\n";
    this.doc = parseDocument(text, {
      keepSourceTokens: true,
    });
    if (this.doc.errors.length > 0) {
      throw new Error(`invalid images.yaml: ${this.doc.errors[0]?.message}`);
    }
    const data = this.doc.toJS() as Partial<ImageDb> | null;
    if (data === null || typeof data !== "object" || typeof data.images !== "object") {
      throw new Error("images.yaml must contain an images map");
    }
  }

  data(): ImageDb {
    return this.doc.toJS() as ImageDb;
  }

  get(alias: string): ImageRecord | undefined {
    return this.data().images[alias];
  }

  aliases(): string[] {
    return Object.keys(this.data().images);
  }

  versions(alias: string): string[] {
    return Object.keys(this.require(alias).versions ?? {});
  }

  require(alias: string): ImageRecord {
    const image = this.get(alias);
    if (image === undefined) {
      throw new Error(`image alias not found: ${alias}`);
    }
    return image;
  }

  addMirror(
    source: string,
    alias: string,
    version: string,
  ): void {
    validateAlias(alias);
    validateVersion(version);
    const current = this.get(alias);
    if (current !== undefined && (current.type !== "mirror" || current.source !== source)) {
      throw new Error(`alias ${alias} already maps to a different image source`);
    }
    if (current === undefined) {
      this.imagesMap().add(this.doc.createPair(alias, {
          type: "mirror",
          source,
          versions: {},
      }));
    }
    this.setStatus(alias, version, "init", true);
  }

  addDockerfile(
    script: string,
    alias: string,
    version: string,
  ): void {
    validateAlias(alias);
    validateVersion(version);
    if (script.startsWith("/") || script.split("/").includes("..")) {
      throw new Error("Dockerfile script must be a repository-relative path without '..'");
    }
    const current = this.get(alias);
    if (current !== undefined && (current.type !== "dockerfile" || current.script !== script)) {
      throw new Error(`alias ${alias} already maps to a different image source`);
    }
    if (current === undefined) {
      this.imagesMap().add(this.doc.createPair(alias, {
          type: "dockerfile",
          script,
          versions: {},
      }));
    }
    this.setStatus(alias, version, "init", true);
  }

  setStatus(
    alias: string,
    version: string,
    status: SyncStatus,
    moveFirst = false,
  ): void {
    validateVersion(version);
    const versions = this.versionsMap(alias);
    if (moveFirst && versions.has(version)) {
      versions.delete(version);
    }
    const pair = this.doc.createPair(version, { status });
    if (moveFirst) {
      versions.items.unshift(pair);
    } else {
      versions.set(version, { status });
    }
  }

  save(): void {
    writeFileSync(this.path, this.doc.toString());
  }

  private imagesMap(): YAMLMap {
    const node = this.doc.get("images", true) as Node | undefined;
    if (!isMap(node)) {
      throw new Error("images.yaml images must be a map");
    }
    return node;
  }

  private versionsMap(alias: string): YAMLMap {
    const image = this.imagesMap().get(alias, true);
    if (!isMap(image)) {
      throw new Error(`invalid image record: ${alias}`);
    }
    let versions = image.get("versions", true);
    if (versions === undefined) {
      image.set("versions", new YAMLMap());
      versions = image.get("versions", true);
    }
    if (!isMap(versions)) {
      throw new Error(`versions must be a map: ${alias}`);
    }
    return versions;
  }
}
