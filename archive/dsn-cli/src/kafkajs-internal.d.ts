declare module "kafkajs/src/cluster" {
  import type { Cluster } from "kafkajs";
  const ClusterCtor: new (opts: object) => Cluster;
  export default ClusterCtor;
}

declare module "kafkajs/src/network/socketFactory" {
  const createSocketFactory: () => object;
  export default createSocketFactory;
}

declare module "kafkajs/src/network/requestQueue" {
  class RequestQueue {
    throttledUntil: number;
    pending: unknown[];
    throttleCheckTimeoutId: ReturnType<typeof setTimeout> | null;
    scheduleCheckPendingRequests(): void;
    checkPendingRequests(): void;
  }
  export default RequestQueue;
}
