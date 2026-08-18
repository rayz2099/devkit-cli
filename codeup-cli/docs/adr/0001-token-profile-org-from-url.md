# Token Profile, Organization from URL

Internal Codeup SDK uses RAM AK/SK plus a hardcoded organization id. This CLI stores only a Yunxiao personal token and parses Organization from `https://codeup.aliyun.com/<organizationId>`.

Webhook list officially accepts `accessToken`, so one secret covers CR writes and webhook reads. Swapping later rewrites every profile and caller.

## Considered Options

- RAM AK/SK like the Kotlin SDK: rejected. User specified `profile[url, token]`.
- Separate `orgId` field plus OpenAPI base URL: rejected. The web org URL already carries the id.
