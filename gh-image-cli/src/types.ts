export type SyncStatus = "init" | "done" | "failed" | "timeout";

export type VersionRecord = {
  status: SyncStatus;
};

export type MirrorImage = {
  type: "mirror";
  source: string;
  versions: Record<string, VersionRecord>;
};

export type DockerfileImage = {
  type: "dockerfile";
  script: string;
  versions: Record<string, VersionRecord>;
};

export type ImageRecord = MirrorImage | DockerfileImage;

export type ImageDb = {
  images: Record<string, ImageRecord>;
};

export type AppConfig = {
  registry: string;
  namespace: string;
  timeoutSeconds: number;
};
