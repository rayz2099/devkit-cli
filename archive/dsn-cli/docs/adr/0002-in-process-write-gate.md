# In-process write Gate

Except MySQL session `TRANSACTION READ ONLY`, the TypeScript drivers cannot refuse Writes on connect. Access defaults to `read`; the Gate intercepts Writes in our code before the Driver sends them.

A read-only server account is extra defense, not the contract. Forwarding unknown statements would let an agent mutate Redis/Mongo/ES through `-e`.
