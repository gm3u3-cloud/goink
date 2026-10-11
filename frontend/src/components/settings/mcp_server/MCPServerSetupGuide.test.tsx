import { createInstance } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import en from "@/i18n/locales/en.json";
import zhCN from "@/i18n/locales/zh-CN.json";
import MCPServerSetupGuide from "./MCPServerSetupGuide";

vi.unmock("react-i18next");
const runtime = vi.hoisted(() => ({
  ClipboardSetText: vi.fn(),
  BrowserOpenURL: vi.fn(),
}));
vi.mock("@/lib/wailsjs/runtime/runtime", () => runtime);

const examples = [
  {
    name: "Codex",
    hint: "codexConfigHint",
    path: "~/.codex/config.toml",
    format: "TOML",
    docs: "https://developers.openai.com/codex/mcp",
  },
  {
    name: "TRAE",
    hint: "traeConfigHint",
    path: ".trae/mcp.json",
    format: "JSON",
    docs: "https://docs.trae.cn/ide_add-mcp-servers",
  },
  {
    name: "WorkBuddy",
    hint: "workbuddyConfigHint",
    path: "mcp.json",
    format: "JSON",
    type: "http",
    docs: "https://open.workbuddy.cn/docs/expert",
  },
  {
    name: "VS Code",
    hint: "vscodeConfigHint",
    path: ".mcp.json",
    format: "JSON",
    type: "http",
    docs: "https://code.visualstudio.com/docs/agent-customization/mcp-servers",
  },
] as const;

async function setup(language: "zh-CN" | "en", token = "saved-token") {
  const i18n = createInstance();
  await i18n.use(initReactI18next).init({
    lng: language,
    resources: { "zh-CN": { translation: zhCN }, en: { translation: en } },
    interpolation: { escapeValue: false },
  });
  const { container } = render(
    <I18nextProvider i18n={i18n}>
      <MCPServerSetupGuide
        url="http://127.0.0.1:6208/mcp"
        token={token}
        disabled={false}
      />
    </I18nextProvider>,
  );
  return { i18n, container, user: userEvent.setup() };
}

describe.each(["zh-CN", "en"] as const)(
  "official MCP client examples (%s)",
  (language) => {
    beforeEach(() => {
      vi.resetAllMocks();
      runtime.ClipboardSetText.mockResolvedValue(true);
    });

    it.each(examples)(
      "shows translated instructions and copies $name's configuration format",
      async (example) => {
        const { i18n, container, user } = await setup(language);
        const choice = screen.getByRole("button", { name: example.name });
        await user.click(choice);
        expect(choice).toHaveAttribute("aria-pressed", "true");
        expect(
          screen.getByText(
            i18n.t(`settings.mcpServer.${example.hint}`, {
              path: example.path,
            }),
          ),
        ).toBeInTheDocument();
        expect(screen.getByText(example.format)).toBeInTheDocument();
        expect(
          screen.getByText(example.path, { exact: true }),
        ).toBeInTheDocument();
        const preview = container.querySelector("pre code")!;
        expect(preview).toHaveTextContent("YOUR_TOKEN");
        expect(preview).not.toHaveTextContent("saved-token");
        await user.click(
          screen.getByRole("button", {
            name: i18n.t("settings.mcpServer.copyConfig"),
          }),
        );
        await waitFor(() =>
          expect(runtime.ClipboardSetText).toHaveBeenCalledTimes(1),
        );
        const copied: string = runtime.ClipboardSetText.mock.lastCall![0];
        expect(copied).not.toContain("YOUR_TOKEN");
        if (example.format === "TOML") {
          expect(copied).toBe(
            '[mcp_servers.goink]\nurl = "http://127.0.0.1:6208/mcp"\nhttp_headers = { Authorization = "Bearer saved-token" }',
          );
          expect(preview).toHaveTextContent("http_headers");
        } else {
          const server = {
            url: "http://127.0.0.1:6208/mcp",
            headers: { Authorization: "Bearer saved-token" },
            ...("type" in example ? { type: example.type } : {}),
          };
          expect(JSON.parse(copied)).toEqual({ mcpServers: { goink: server } });
        }
        await user.click(
          screen.getByRole("button", {
            name: i18n.t("settings.mcpServer.clientDocs"),
          }),
        );
        expect(runtime.BrowserOpenURL).toHaveBeenCalledWith(example.docs);
      },
    );

    it("keeps copying disabled without a token for every new client", async () => {
      const { i18n, user } = await setup(language, "");
      for (const example of examples) {
        await user.click(screen.getByRole("button", { name: example.name }));
        expect(
          screen.getByRole("button", {
            name: i18n.t("settings.mcpServer.copyConfig"),
          }),
        ).toBeDisabled();
      }
      expect(runtime.ClipboardSetText).not.toHaveBeenCalled();
    });
  },
);
