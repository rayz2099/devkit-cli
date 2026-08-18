# Secret resolution prefers apiToken

A Profile stores both `password` and `apiToken`. REST Basic auth uses `apiToken` whenever it is non-empty, otherwise `password`. Both empty fails.

This is an explicit exception to the repo-wide no-fallback rule. The user added both fields and then chose apiToken as the preferred Secret so an empty token can still reuse the old `JENKINS_PASSWORD` value. Changing the order later will break whichever field people stop filling.
