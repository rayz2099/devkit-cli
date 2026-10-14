import { apiSend, taRoot, withQuery } from "./http";
import { encryptPwd } from "./rsa";
import { isObj, asText, originOf } from "./util";

export type Session = {
  host: string;
  token: string;
  bff: boolean;
};

/** 为什么: TE 4.4 没有设备码登录, 只能走页面同一条 loginForToken. */
export async function login(host: string, loginName: string, password: string): Promise<{
  accessToken: string;
  refreshToken: string;
  loginName: string;
}> {
  const form = new URLSearchParams();
  form.set("loginName", loginName);
  form.set("enPassword", encryptPwd(password));
  form.set("locale", "zh_CN");
  const url = `${originOf(host)}/v1/oauth/loginForToken`;
  const data = await apiSend({ url, method: "POST", token: "", form });
  return readToken(data);
}

/** 为什么: TE 4.4 的 /v1/ta/project/list 会 500, 页面项目列表来自登录校验返回的 projectInfoList. */
export async function listProjects(session: Session): Promise<unknown> {
  const url = `${originOf(session.host)}/v1/oauth/check`;
  const data = await apiSend({
    url,
    method: "POST",
    token: session.token,
    form: new URLSearchParams(),
  });
  if (!isObj(data) || !Array.isArray(data.projectInfoList)) {
    throw new Error("project list missing");
  }
  return data.projectInfoList;
}

export async function listEvents(session: Session, projectId: string): Promise<unknown> {
  const url = withQuery(
    `${taRoot(session.host, session.bff)}/event/catalog/listEvent`,
    { projectId },
    true,
  );
  return await apiSend({ url, method: "GET", token: session.token });
}

export async function listProps(session: Session, projectId: string, eventName: string | undefined): Promise<unknown> {
  if (eventName === undefined) {
    const url = withQuery(
      `${taRoot(session.host, session.bff)}/user/loadUserProps`,
      { projectId },
      true,
    );
    return await apiSend({ url, method: "GET", token: session.token });
  }
  const url = withQuery(
    `${taRoot(session.host, session.bff)}/event/model/meta/loadFiltProps`,
    { projectId },
    false,
  );
  const body = {
    data: {
      commonHeader: { projectId: Number(projectId) },
      eventModel: 0,
      events: [{ eventName, eventDesc: eventName, eventType: "event" }],
    },
  };
  return await apiSend({ url, method: "POST", token: session.token, json: body });
}

export async function listReports(session: Session, projectId: string): Promise<unknown> {
  const url = withQuery(
    `${taRoot(session.host, session.bff)}/event/reportlist`,
    { projectId },
    true,
  );
  return await apiSend({ url, method: "GET", token: session.token });
}

export async function getReport(session: Session, projectId: string, reportId: string): Promise<unknown> {
  const url = withQuery(
    `${taRoot(session.host, session.bff)}/event/reportsearch`,
    { projectId, reportId, searchSource: "model_search" },
    true,
  );
  return await apiSend({ url, method: "GET", token: session.token });
}

function readToken(data: unknown): { accessToken: string; refreshToken: string; loginName: string } {
  if (!isObj(data) || !isObj(data.tokenInfo)) {
    throw new Error("login response has no tokenInfo");
  }
  const accessToken = asText(data.tokenInfo.accessToken);
  if (accessToken === "") {
    throw new Error("login response has no accessToken");
  }
  const oauth = isObj(data.oAuthLoginInfo) ? data.oAuthLoginInfo : {};
  return {
    accessToken,
    refreshToken: asText(oauth.refreshToken),
    loginName: asText(data.tokenInfo.loginName),
  };
}
