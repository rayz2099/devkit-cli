import { describe, expect, test } from "bun:test";
import { parseCliArgs } from "../src/args";
import { parseConfigJson, parseJdbcUrl } from "../src/config";
import { buildMysqlInvocation } from "../src/mysql";
import { createTabStreamFormatter, toCsv, toJsonLines } from "../src/output";
import { profileCompletion } from "../src/fish";
import { filterMysqlStderr } from "../src/stderr";

const jdbcUrl =
  "jdbc:mysql://127.0.0.1:3306/app_db?user=adm&password=secret&useUnicode=true&serverTimezone=Asia/Shanghai";

describe("config", () => {
  test("parses jdbc url and ignores unrelated jdbc parameters", () => {
    expect(parseJdbcUrl("testdb", jdbcUrl)).toEqual({
      host: "127.0.0.1",
      port: "3306",
      user: "adm",
      password: "secret",
      database: "app_db",
    });
  });

  test("omits database when jdbc path is empty", () => {
    const parsed = parseJdbcUrl(
      "testdb",
      "jdbc:mysql://127.0.0.1:3306/?user=adm&password=x",
    );

    expect(parsed.database).toBeUndefined();
  });

  test("parses config profiles", () => {
    const config = parseConfigJson(
      JSON.stringify({
        profiles: [{ name: "testdb", jdbcUrl }],
      }),
    );

    expect(config.profiles.map((profile) => profile.name)).toEqual(["testdb"]);
  });
});

describe("cli args", () => {
  test("parses interactive profile command", () => {
    expect(parseCliArgs(["-p", "testdb"])).toEqual({
      kind: "run",
      profile: "testdb",
    });
  });

  test("parses execute command with output", () => {
    expect(parseCliArgs(["-p", "testdb", "-e", "select 1", "--output", "json"])).toEqual({
      kind: "run",
      profile: "testdb",
      execute: "select 1",
      output: "json",
    });
  });

  test("rejects output without execute", () => {
    expect(() => parseCliArgs(["-p", "testdb", "--output", "json"])).toThrow(
      "--output requires -e",
    );
  });
});

describe("mysql invocation", () => {
  test("builds interactive mysql args from profile", () => {
    expect(
      buildMysqlInvocation("/usr/local/bin/mysql", parseJdbcUrl("testdb", jdbcUrl), {}),
    ).toEqual({
      command: "/usr/local/bin/mysql",
      args: ["-h", "127.0.0.1", "-P", "3306", "-u", "adm", "-psecret", "app_db"],
      captureOutput: false,
    });
  });

  test("builds batch mysql args for formatted output", () => {
    expect(
      buildMysqlInvocation("/usr/local/bin/mysql", parseJdbcUrl("testdb", jdbcUrl), {
        execute: "select 1",
        output: "csv",
      }),
    ).toEqual({
      command: "/usr/local/bin/mysql",
      args: [
        "-h",
        "127.0.0.1",
        "-P",
        "3306",
        "-u",
        "adm",
        "-psecret",
        "--batch",
        "--raw",
        "-e",
        "select 1",
        "app_db",
      ],
      captureOutput: true,
    });
  });
});

describe("output", () => {
  test("converts mysql tab output to one json object per line", () => {
    expect(toJsonLines("id\tname\n1\tfoo\n2\tbar\n")).toBe(
      '{"id":"1","name":"foo"}\n{"id":"2","name":"bar"}',
    );
  });

  test("streams json lines after reading only the header and current row", () => {
    const formatter = createTabStreamFormatter("json");

    expect(formatter.write("id\tname\n1\tfoo\n2")).toBe('{"id":"1","name":"foo"}\n');
    expect(formatter.write("\tbar\n")).toBe('{"id":"2","name":"bar"}\n');
    expect(formatter.end()).toBe("");
  });

  test("converts mysql tab output to csv with escaping", () => {
    expect(toCsv('id\tname\n1\tfoo,bar\n2\t"baz"\n')).toBe(
      'id,name\n1,"foo,bar"\n2,"""baz"""',
    );
  });
});

describe("fish completion", () => {
  test("prints profile names one per line", () => {
    expect(
      profileCompletion({
        profiles: [
          { name: "testdb", jdbcUrl },
          { name: "prod", jdbcUrl },
        ],
      }),
    ).toBe("testdb\nprod\n");
  });
});

describe("stderr", () => {
  test("filters mysql password command line warning and keeps real errors", () => {
    expect(
      filterMysqlStderr(
        "mysql: [Warning] Using a password on the command line interface can be insecure.\nERROR 1146 (42S02): Table missing\n",
      ),
    ).toBe("ERROR 1146 (42S02): Table missing\n");
  });
});
