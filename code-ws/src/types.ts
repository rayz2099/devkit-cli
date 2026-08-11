export type RepoCfg = {
  name: string;
  path: string;
  group: string;
  branch: string;
  checkoutBranch?: string;
  description?: string;
};

export type RawProfileCfg = {
  name: string;
  agentsTemplate: string;
  repos: string[];
};

export type ProfileCfg = {
  name: string;
  agentsTemplate: string;
  repos: RepoCfg[];
};

export type RawCodeWsCfg = {
  workspaceRoot: string;
  baseBranch: string;
  remote: string;
  initAgentsTemplate: string;
  profiles: Record<string, RawProfileCfg>;
};

export type CodeWsCfg = {
  workspaceRoot: string;
  baseBranch: string;
  remote: string;
  initAgentsTemplate: string;
  projects: RepoCfg[];
  profiles: Record<string, ProfileCfg>;
};

export type Cmd = {
  repo: string;
  branch: string;
  step: string;
  cwd: string;
  args: string[];
  ignoredErrors?: string[];
};
