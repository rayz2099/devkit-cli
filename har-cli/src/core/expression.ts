type ExpressionContext = Record<string, unknown>;

type Token =
  | { type: "identifier"; value: string }
  | { type: "string"; value: string }
  | { type: "number"; value: number }
  | { type: "boolean"; value: boolean }
  | { type: "null"; value: null }
  | { type: "operator"; value: string }
  | { type: "paren"; value: "(" | ")" };

/**
 * 将 where 表达式解析并求值，供 HAR entry 过滤使用。
 */
export function evaluateWhere(expression: string, context: ExpressionContext): boolean {
  const parser = new ExpressionParser(tokenize(expression), context);
  return Boolean(parser.parse());
}

/**
 * 将表达式拆成词法单元，便于后续递归下降解析。
 */
function tokenize(expression: string): Token[] {
  const tokens: Token[] = [];
  let cursor = 0;

  while (cursor < expression.length) {
    const current = expression[cursor];

    if (/\s/.test(current)) {
      cursor += 1;
      continue;
    }

    const operator = matchOperator(expression, cursor);
    if (operator) {
      if (operator === "(" || operator === ")") {
        tokens.push({ type: "paren", value: operator });
      } else {
        tokens.push({ type: "operator", value: operator });
      }
      cursor += operator.length;
      continue;
    }

    if (/[0-9]/.test(current)) {
      const matched = readWhile(expression, cursor, /[0-9.]/);
      tokens.push({ type: "number", value: Number(matched.value) });
      cursor = matched.nextCursor;
      continue;
    }

    if (/[A-Za-z_]/.test(current)) {
      const matched = readWhile(expression, cursor, /[A-Za-z0-9_.]/);

      if (matched.value === "and" || matched.value === "or" || matched.value === "not") {
        tokens.push({ type: "operator", value: matched.value });
      } else if (matched.value === "true" || matched.value === "false") {
        tokens.push({ type: "boolean", value: matched.value === "true" });
      } else if (matched.value === "null") {
        tokens.push({ type: "null", value: null });
      } else {
        tokens.push({ type: "identifier", value: matched.value });
      }

      cursor = matched.nextCursor;
      continue;
    }

    if (current === "'" || current === "\"") {
      const value = readString(expression, cursor);
      tokens.push({ type: "string", value });
      cursor += value.length + 2;
      continue;
    }

    throw new Error(`Unsupported token near: ${expression.slice(cursor, cursor + 10)}`);
  }

  return tokens;
}

/**
 * 负责表达式语法分析与安全取值，避免直接使用 eval。
 */
class ExpressionParser {
  private readonly tokens: Token[];
  private readonly context: ExpressionContext;
  private cursor = 0;

  constructor(tokens: Token[], context: ExpressionContext) {
    this.tokens = tokens;
    this.context = context;
  }

  parse(): unknown {
    const result = this.parseOr();
    this.ensureCompleted();
    return result;
  }

  private parseOr(): unknown {
    let left = this.parseAnd();

    while (this.matchOperator("or", "||")) {
      const right = this.parseAnd();
      left = Boolean(left) || Boolean(right);
    }

    return left;
  }

  private parseAnd(): unknown {
    let left = this.parseNot();

    while (this.matchOperator("and", "&&")) {
      const right = this.parseNot();
      left = Boolean(left) && Boolean(right);
    }

    return left;
  }

  private parseNot(): unknown {
    if (this.matchOperator("not", "!")) {
      return !Boolean(this.parseNot());
    }

    return this.parseComparison();
  }

  private parseComparison(): unknown {
    const left = this.parsePrimary();
    const operator = this.peek();

    if (operator?.type !== "operator" || !isComparisonOperator(operator.value)) {
      return left;
    }

    this.cursor += 1;
    const right = this.parsePrimary();

    switch (operator.value) {
      case "==":
        return left == right;
      case "!=":
        return left != right;
      case ">":
        return Number(left) > Number(right);
      case ">=":
        return Number(left) >= Number(right);
      case "<":
        return Number(left) < Number(right);
      case "<=":
        return Number(left) <= Number(right);
      default:
        throw new Error(`Unsupported operator: ${operator.value}`);
    }
  }

  private parsePrimary(): unknown {
    const token = this.peek();

    if (!token) {
      throw new Error("Unexpected end of expression");
    }

    if (token.type === "paren" && token.value === "(") {
      this.cursor += 1;
      const value = this.parseOr();
      this.expectParen(")");
      return value;
    }

    this.cursor += 1;

    switch (token.type) {
      case "identifier":
        return resolvePath(this.context, token.value);
      case "number":
      case "string":
      case "boolean":
      case "null":
        return token.value;
      default:
        throw new Error(`Unexpected token: ${token.value}`);
    }
  }

  private matchOperator(...operators: string[]): boolean {
    const token = this.peek();
    if (token?.type === "operator" && operators.includes(token.value)) {
      this.cursor += 1;
      return true;
    }
    return false;
  }

  private expectParen(value: ")" | "("): void {
    const token = this.peek();
    if (token?.type !== "paren" || token.value !== value) {
      throw new Error(`Expected ${value}`);
    }
    this.cursor += 1;
  }

  private ensureCompleted(): void {
    if (this.cursor !== this.tokens.length) {
      const token = this.tokens[this.cursor];
      throw new Error(`Unexpected token: ${token.value}`);
    }
  }

  private peek(): Token | undefined {
    return this.tokens[this.cursor];
  }
}

/**
 * 按 a.b.c 路径从上下文安全取值，路径不存在时返回 undefined。
 */
function resolvePath(context: ExpressionContext, path: string): unknown {
  return path.split(".").reduce<unknown>((value, segment) => {
    if (value === null || value === undefined || typeof value !== "object") {
      return undefined;
    }

    return (value as Record<string, unknown>)[segment];
  }, context);
}

function matchOperator(expression: string, cursor: number): string | undefined {
  const candidates = ["==", "!=", ">=", "<=", "&&", "||", ">", "<", "(", ")"];
  return candidates.find((candidate) => expression.slice(cursor, cursor + candidate.length) === candidate);
}

function readWhile(expression: string, cursor: number, matcher: RegExp): { value: string; nextCursor: number } {
  let nextCursor = cursor;

  while (nextCursor < expression.length && matcher.test(expression[nextCursor])) {
    nextCursor += 1;
  }

  return {
    value: expression.slice(cursor, nextCursor),
    nextCursor,
  };
}

function readString(expression: string, cursor: number): string {
  const quote = expression[cursor];
  let nextCursor = cursor + 1;

  while (nextCursor < expression.length && expression[nextCursor] !== quote) {
    nextCursor += 1;
  }

  if (expression[nextCursor] !== quote) {
    throw new Error("Unterminated string literal");
  }

  return expression.slice(cursor + 1, nextCursor);
}

function isComparisonOperator(operator: string): boolean {
  return ["==", "!=", ">", ">=", "<", "<="].includes(operator);
}
