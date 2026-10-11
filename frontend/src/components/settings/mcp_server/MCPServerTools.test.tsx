import { useState } from "react";
import { createInstance } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import HelpDialog from "@/components/help/HelpDialog";
import { toolGroups } from "@/components/tools/catalog";
import en from "@/i18n/locales/en.json";
import zhCN from "@/i18n/locales/zh-CN.json";
import MCPServerTools from "./MCPServerTools";

vi.unmock("react-i18next");

async function translations(language: "zh-CN" | "en") {
  const i18n = createInstance();
  await i18n.use(initReactI18next).init({
    lng: language,
    resources: { "zh-CN": { translation: zhCN }, en: { translation: en } },
    interpolation: { escapeValue: false },
  });
  return i18n;
}

function Permissions({ available }: { available: string[] }) {
  const [selected, setSelected] = useState<string[]>([]);
  return (
    <MCPServerTools
      available={available}
      selected={selected}
      disabled={false}
      onChange={setSelected}
    />
  );
}

describe("shared tool reference and permissions", () => {
  it.each(["zh-CN", "en"] as const)(
    "uses the same translated names, descriptions and domains in both pages (%s)",
    async (language) => {
      const i18n = await translations(language);
      const available = toolGroups
        .flatMap((group) => group.tools)
        .filter((name) => name !== "run_subagent");
      render(
        <I18nextProvider i18n={i18n}>
          <section aria-label="help">
            <HelpDialog open onClose={vi.fn()} />
          </section>
          <section aria-label="permissions">
            <Permissions available={available} />
          </section>
        </I18nextProvider>,
      );
      const user = userEvent.setup();
      const help = within(screen.getByRole("region", { name: "help" }));
      const permissions = within(
        screen.getByRole("region", { name: "permissions" }),
      );
      await user.click(
        help.getByRole("button", { name: i18n.t("help.toolReference") }),
      );
      for (const name of available) {
        const labelKey = `tools.names.${name}`;
        const descriptionKey = `tools.descriptions.${name}`;
        expect(i18n.exists(labelKey)).toBe(true);
        expect(i18n.exists(descriptionKey)).toBe(true);
        expect(help.getByText(i18n.t(labelKey))).toBeInTheDocument();
        expect(help.getByText(i18n.t(descriptionKey))).toBeInTheDocument();
        expect(
          permissions.getByRole("checkbox", { name: i18n.t(labelKey) }),
        ).toBeInTheDocument();
        expect(
          permissions.getByText(i18n.t(descriptionKey)),
        ).toBeInTheDocument();
      }
      for (const group of toolGroups) {
        const title = i18n.t(`tools.groups.${group.id}`);
        expect(help.getByRole("heading", { name: title })).toBeInTheDocument();
        expect(
          permissions.getByRole("group", { name: title }),
        ).toBeInTheDocument();
      }
      const characters = within(
        permissions.getByRole("group", {
          name: i18n.t("tools.groups.characters"),
        }),
      );
      expect(
        characters.getByRole("checkbox", {
          name: i18n.t("tools.names.get_character_relations"),
        }),
      ).toBeInTheDocument();
      expect(
        characters.getByRole("checkbox", {
          name: i18n.t("tools.names.update_character_relationship"),
        }),
      ).toBeInTheDocument();
      expect(
        help.getByText(i18n.t("tools.names.run_subagent")),
      ).toBeInTheDocument();
      expect(permissions.queryByText("run_subagent")).not.toBeInTheDocument();
    },
  );

  it("keeps new server tools visible with an honest fallback and no duplicate selections", async () => {
    const i18n = await translations("en");
    render(
      <I18nextProvider i18n={i18n}>
        <Permissions available={["future_tool", "future_tool"]} />
      </I18nextProvider>,
    );
    const user = userEvent.setup();
    const group = within(
      screen.getByRole("group", { name: i18n.t("tools.groups.general") }),
    );
    expect(
      group.getByText(i18n.t("tools.unknownDescription")),
    ).toBeInTheDocument();
    const tool = group.getByRole("checkbox", { name: "future_tool" });
    await user.click(
      group.getByRole("checkbox", {
        name: i18n.t("settings.mcpServer.selectGroup", {
          group: i18n.t("tools.groups.general"),
        }),
      }),
    );
    expect(tool).toBeChecked();
    expect(
      screen.getByText(
        i18n.t("settings.mcpServer.selectedCount", { selected: 1, total: 1 }),
      ),
    ).toBeInTheDocument();
  });
});
