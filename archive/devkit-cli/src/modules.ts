import { existsSync, readdirSync } from "node:fs";
import { join } from "node:path";

export type CliMod = {
  name: string;
  root: string;
  justfile: string;
};

/**
 * 根任务通过 justfile 约定发现 CLI 子项目, 因为各模块技术栈不完全一致.
 */
export function discoverMods(root: string): CliMod[] {
  return readdirSync(root, {
    withFileTypes: true,
  })
    .filter((item) => item.isDirectory())
    .map((item) => {
      const modRoot = join(root, item.name);
      const lower = join(modRoot, "justfile");
      const upper = join(modRoot, "Justfile");
      return {
        name: item.name,
        root: modRoot,
        justfile: existsSync(lower) ? lower : upper,
      };
    })
    .filter((mod) => existsSync(mod.justfile))
    .sort((left, right) => left.name.localeCompare(right.name));
}

export function selectMods(
  mods: CliMod[],
  names: string[],
): CliMod[] {
  if (names.length === 0) {
    return mods;
  }

  const byName = new Map(mods.map((mod) => [mod.name, mod]));
  return names.map((name) => {
    const mod = byName.get(name);
    if (mod === undefined) {
      throw new Error(`module not found: ${name}`);
    }
    return mod;
  });
}
