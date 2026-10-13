interface MCPClientExample {
  name: string;
  path: string;
  hintKey: string;
  docs: string;
  format: "JSON" | "TOML";
  httpType?: "http";
}

export const mcpClientExamples = {
  cursor: {
    name: "Cursor",
    path: "~/.cursor/mcp.json",
    hintKey: "cursorConfigHint",
    docs: "https://cursor.com/docs/mcp",
    format: "JSON",
  },
  claude: {
    name: "Claude Code",
    path: ".mcp.json",
    hintKey: "claudeConfigHint",
    docs: "https://code.claude.com/docs/en/mcp",
    format: "JSON",
    httpType: "http",
  },
  codex: {
    name: "Codex",
    path: "~/.codex/config.toml",
    hintKey: "codexConfigHint",
    docs: "https://developers.openai.com/codex/mcp",
    format: "TOML",
  },
  trae: {
    name: "TRAE",
    path: ".trae/mcp.json",
    hintKey: "traeConfigHint",
    docs: "https://docs.trae.cn/ide_add-mcp-servers",
    format: "JSON",
  },
  workbuddy: {
    name: "WorkBuddy",
    path: "mcp.json",
    hintKey: "workbuddyConfigHint",
    // The official local HTTP example uses type: "http". Connector publishing
    // has a different schema; this example is for a user's custom connection.
    // Setup: https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Connector
    docs: "https://open.workbuddy.cn/docs/expert",
    format: "JSON",
    httpType: "http",
  },
  vscode: {
    name: "VS Code",
    path: ".mcp.json",
    hintKey: "vscodeConfigHint",
    docs: "https://code.visualstudio.com/docs/agent-customization/mcp-servers",
    format: "JSON",
    httpType: "http",
  },
} satisfies Record<string, MCPClientExample>;

export type MCPExampleClient = keyof typeof mcpClientExamples;

export function createMCPClientConfig(
  client: MCPExampleClient,
  url: string,
  token: string,
): string {
  const example: MCPClientExample = mcpClientExamples[client];
  if (example.format === "TOML") {
    return [
      "[mcp_servers.goink]",
      `url = ${JSON.stringify(url)}`,
      `http_headers = { Authorization = ${JSON.stringify(`Bearer ${token}`)} }`,
    ].join("\n");
  }
  return JSON.stringify(
    {
      mcpServers: {
        goink: {
          ...(example.httpType ? { type: example.httpType } : {}),
          url,
          headers: { Authorization: `Bearer ${token}` },
        },
      },
    },
    null,
    2,
  );
}
