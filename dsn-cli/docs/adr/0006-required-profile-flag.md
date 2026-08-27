# `-p` is required

`dsn-cli` addresses many clusters across Kinds. There is no `defaultProfile`. Omitting `-p` is an error for Query and Console. Doctor is the exception; see 0013.

Sister CLIs keep `defaultProfile` because they talk to one Jenkins or one Codeup. A silent default here would run a Query against the wrong backend. Humans already type `-p buy`.
