import { expect, test } from "bun:test";
import { mongoDbName } from "../src/mongo-url";

test("mongoDbName 从 seed list 取 path, 没有 path 则空", () => {
  expect(
    mongoDbName(
      "mongodb://biz:secret@10.0.16.60:27017,10.0.16.61:27017,10.0.16.62:27017?readPreference=secondary",
    ),
  ).toBe("");
  expect(
    mongoDbName("mongodb://u:p@10.0.16.70:27017,10.0.16.71:27017/app?authSource=admin"),
  ).toBe("app");
  expect(mongoDbName("mongodb://127.0.0.1:27017/app")).toBe("app");
  expect(mongoDbName("mongodb+srv://app:secret@cluster.mongodb.net/app")).toBe("app");
});
