import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { mcpserver, servercfg } from "@/lib/wailsjs/go/models";
import MCPServerConfigTab from "./MCPServerConfigTab";

const api = vi.hoisted(() => ({
  GetMCPServerSettings: vi.fn(),
  SaveMCPServerSettings: vi.fn(),
  RotateMCPServerCredential: vi.fn(),
  RevokeMCPServerCredential: vi.fn(),
}));
const runtime = vi.hoisted(() => ({
  ClipboardSetText: vi.fn(),
  BrowserOpenURL: vi.fn(),
}));
vi.mock("@/lib/wailsjs/go/app/App", () => api);
vi.mock("@/lib/wailsjs/runtime/runtime", () => runtime);

function view(config: Partial<servercfg.Local> = {}, token = "saved-token") {
  return new mcpserver.LocalView({
    config: { enabled: false, port: 5108, allowed_tools: [], ...config },
    available_tools: ["read", "edit"],
    token,
    status: { running: config.enabled ?? false, stopping: false, url: "" },
    last_error: "",
  });
}

let stored: mcpserver.LocalView;

async function setup() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={qc}>
      <MCPServerConfigTab />
    </QueryClientProvider>,
  );
  await screen.findByRole("switch", { name: "settings.mcpServer.enabled" });
  return userEvent.setup();
}

function tokenInput() {
  return screen.getByLabelText("settings.mcpServer.token");
}

describe("MCP server credentials and client setup", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    stored = view();
    api.GetMCPServerSettings.mockImplementation(() => Promise.resolve(stored));
    api.SaveMCPServerSettings.mockImplementation((config: servercfg.Local) => {
      stored = view(config, stored.token || "generated-token");
      return Promise.resolve(stored);
    });
    api.RotateMCPServerCredential.mockImplementation(() => {
      stored = view(stored.config, "rotated-token");
      return Promise.resolve(stored);
    });
    api.RevokeMCPServerCredential.mockImplementation(() => {
      stored = view({ ...stored.config, enabled: false }, "");
      return Promise.resolve();
    });
    runtime.ClipboardSetText.mockResolvedValue(true);
  });

  it("masks a saved token, supports reveal and copies it again after refresh", async () => {
    const user = await setup();
    expect(tokenInput()).toHaveAttribute("type", "password");
    expect(tokenInput()).toHaveValue("saved-token");
    expect(screen.getByText(/"mcpServers"/)).not.toHaveTextContent(
      "saved-token",
    );
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.showToken" }),
    );
    expect(tokenInput()).toHaveAttribute("type", "text");
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.hideToken" }),
    );
    expect(tokenInput()).toHaveAttribute("type", "password");
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.copyToken" }),
    );
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenCalledWith("saved-token"),
    );
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.refresh" }),
    );
    await waitFor(() =>
      expect(api.GetMCPServerSettings).toHaveBeenCalledTimes(2),
    );
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.copyToken" }),
    );
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenCalledTimes(2),
    );
    expect(runtime.ClipboardSetText).toHaveBeenLastCalledWith("saved-token");
  });

  it("copies client-specific JSON using the saved port and updates it only after saving", async () => {
    const user = await setup();
    const port = screen.getByRole("spinbutton", {
      name: "settings.mcpServer.port",
    });
    await user.clear(port);
    await user.type(port, "6208");
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.copyURL" }),
    );
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenLastCalledWith(
        "http://127.0.0.1:5108/mcp",
      ),
    );
    const copyConfig = screen.getByRole("button", {
      name: "settings.mcpServer.copyConfig",
    });
    await user.click(copyConfig);
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenCalledTimes(2),
    );
    expect(JSON.parse(runtime.ClipboardSetText.mock.lastCall![0])).toEqual({
      mcpServers: {
        goink: {
          url: "http://127.0.0.1:5108/mcp",
          headers: { Authorization: "Bearer saved-token" },
        },
      },
    });
    await user.click(screen.getByRole("button", { name: "Claude Code" }));
    await user.click(copyConfig);
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenCalledTimes(3),
    );
    expect(
      JSON.parse(runtime.ClipboardSetText.mock.lastCall![0]).mcpServers.goink
        .type,
    ).toBe("http");
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.clientDocs" }),
    );
    expect(runtime.BrowserOpenURL).toHaveBeenCalledWith(
      "https://code.claude.com/docs/en/mcp",
    );
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.save" }),
    );
    await screen.findByText("settings.mcpServer.saved");
    await user.click(copyConfig);
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenCalledTimes(4),
    );
    expect(
      JSON.parse(runtime.ClipboardSetText.mock.lastCall![0]).mcpServers.goink
        .url,
    ).toBe("http://127.0.0.1:6208/mcp");
  });

  it.each([false, "rejection"])(
    "reports clipboard failure (%s) and allows retry",
    async (failure) => {
      if (failure === false)
        runtime.ClipboardSetText.mockResolvedValueOnce(false);
      else
        runtime.ClipboardSetText.mockRejectedValueOnce(
          new Error("unavailable"),
        );
      const user = await setup();
      const copy = screen.getByRole("button", {
        name: "settings.mcpServer.copyToken",
      });
      await user.click(copy);
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "settings.mcpServer.copyFailed",
      );
      await user.click(copy);
      await waitFor(() =>
        expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
      );
      expect(runtime.ClipboardSetText).toHaveBeenCalledTimes(2);
    },
  );

  it("disables credential copies without a token and can generate one before enabling", async () => {
    stored = view({}, "");
    const user = await setup();
    for (const key of ["copyToken", "copyConfig", "revokeToken", "showToken"]) {
      expect(
        screen.getByRole("button", { name: `settings.mcpServer.${key}` }),
      ).toBeDisabled();
    }
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.generateToken" }),
    );
    await screen.findByText("settings.mcpServer.tokenRotated");
    expect(tokenInput()).toHaveValue("rotated-token");
    expect(
      screen.getByRole("switch", { name: "settings.mcpServer.enabled" }),
    ).not.toBeChecked();
    expect(
      screen.getByRole("button", { name: "settings.mcpServer.copyConfig" }),
    ).toBeEnabled();
  });

  it("requires confirmation before rotation and keeps unsaved fields after applying it", async () => {
    stored = view({ enabled: true });
    const user = await setup();
    const port = screen.getByRole("spinbutton", {
      name: "settings.mcpServer.port",
    });
    await user.clear(port);
    await user.type(port, "6208");
    await user.click(
      screen.getByRole("checkbox", { name: "tools.names.read" }),
    );
    const rotate = screen.getByRole("button", {
      name: "settings.mcpServer.rotateToken",
    });
    await user.click(rotate);
    expect(api.RotateMCPServerCredential).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "common.cancel" }));
    expect(api.RotateMCPServerCredential).not.toHaveBeenCalled();
    await user.click(rotate);
    await user.click(screen.getByRole("button", { name: "common.confirm" }));
    await screen.findByText("settings.mcpServer.tokenRotated");
    expect(tokenInput()).toHaveValue("rotated-token");
    expect(port).toHaveValue(6208);
    expect(
      screen.getByRole("checkbox", { name: "tools.names.read" }),
    ).toBeChecked();
    expect(api.SaveMCPServerSettings).not.toHaveBeenCalled();
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.copyToken" }),
    );
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenLastCalledWith(
        "rotated-token",
      ),
    );
  });

  it("reloads a persisted token after rotation fails to restart the server", async () => {
    stored = view({ enabled: true });
    api.RotateMCPServerCredential.mockImplementationOnce(() => {
      stored = view({ enabled: true }, "persisted-new-token");
      stored.status.running = false;
      stored.last_error = "port occupied";
      return Promise.reject("port occupied");
    });
    const user = await setup();
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.rotateToken" }),
    );
    await user.click(screen.getByRole("button", { name: "common.confirm" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("port occupied"),
    );
    expect(tokenInput()).toHaveValue("persisted-new-token");
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.copyToken" }),
    );
    await waitFor(() =>
      expect(runtime.ClipboardSetText).toHaveBeenLastCalledWith(
        "persisted-new-token",
      ),
    );
  });

  it("revokes only after confirmation, clears copies and reflects disabled state", async () => {
    stored = view({ enabled: true });
    const user = await setup();
    const revoke = screen.getByRole("button", {
      name: "settings.mcpServer.revokeToken",
    });
    await user.click(revoke);
    expect(api.RevokeMCPServerCredential).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "common.cancel" }));
    expect(api.RevokeMCPServerCredential).not.toHaveBeenCalled();
    await user.click(revoke);
    await user.click(screen.getByRole("button", { name: "common.confirm" }));
    await screen.findByText("settings.mcpServer.tokenRevoked");
    expect(tokenInput()).toHaveValue("");
    expect(
      screen.getByRole("switch", { name: "settings.mcpServer.enabled" }),
    ).not.toBeChecked();
    expect(
      screen.getByRole("button", { name: "settings.mcpServer.copyToken" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "settings.mcpServer.copyConfig" }),
    ).toBeDisabled();
  });

  it("reloads cleared credentials even if revocation returns an error", async () => {
    stored = view({ enabled: true });
    api.RevokeMCPServerCredential.mockImplementationOnce(() => {
      stored = view({}, "");
      stored.last_error = "shutdown timed out";
      return Promise.reject("shutdown timed out");
    });
    const user = await setup();
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.revokeToken" }),
    );
    await user.click(screen.getByRole("button", { name: "common.confirm" }));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent("shutdown timed out"),
    );
    expect(tokenInput()).toHaveValue("");
    expect(
      screen.getByRole("button", { name: "settings.mcpServer.copyConfig" }),
    ).toBeDisabled();
  });

  it("blocks conflicting operations until credential mutation and refresh complete", async () => {
    let finish: (next: mcpserver.LocalView) => void;
    api.RotateMCPServerCredential.mockImplementationOnce(
      () =>
        new Promise<mcpserver.LocalView>((resolve) => {
          finish = resolve;
        }),
    );
    const user = await setup();
    await user.click(
      screen.getByRole("button", { name: "settings.mcpServer.rotateToken" }),
    );
    await user.click(screen.getByRole("button", { name: "common.confirm" }));
    for (const key of [
      "rotateToken",
      "revokeToken",
      "copyToken",
      "copyConfig",
      "refresh",
    ]) {
      expect(
        screen.getByRole("button", { name: `settings.mcpServer.${key}` }),
      ).toBeDisabled();
    }
    expect(
      screen.getByRole("button", { name: "common.saving" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("switch", { name: "settings.mcpServer.enabled" }),
    ).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(
      screen.getByRole("heading", { name: "settings.mcpServer.rotateTitle" }),
    ).toBeInTheDocument();
    stored = view({}, "new-token");
    finish!(stored);
    await screen.findByText("settings.mcpServer.tokenRotated");
    expect(
      screen.getByRole("button", { name: "settings.mcpServer.copyToken" }),
    ).toBeEnabled();
  });
});
