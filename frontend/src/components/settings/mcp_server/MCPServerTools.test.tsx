import { useState } from "react";
import { createInstance } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
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

function Permissions({
  available,
  initialSelected = [],
}: {
  available: string[];
  initialSelected?: string[];
}) {
  const [selected, setSelected] = useState<string[]>(initialSelected);
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

describe.each(["zh-CN", "en"] as const)(
  "essential tool confirmation (%s)",
  (language) => {
    const basicTools = ["get_chapter_list", "read", "edit"];
    const available = [...basicTools, "delete_record"];

    async function setup(selected = available) {
      const i18n = await translations(language);
      render(
        <I18nextProvider i18n={i18n}>
          <Permissions available={available} initialSelected={selected} />
        </I18nextProvider>,
      );
      return { i18n, user: userEvent.setup() };
    }

    it.each(basicTools)(
      "confirms disabling %s and explains only that tool's lost capability",
      async (name) => {
        const { i18n, user } = await setup();
        const checkbox = screen.getByRole("checkbox", {
          name: i18n.t(`tools.names.${name}`),
        });
        await user.click(checkbox);
        const dialog = within(await screen.findByRole("alertdialog"));
        expect(checkbox).toBeChecked();
        expect(
          dialog.getByText(
            i18n.t(`settings.mcpServer.coreToolEffects.${name}`),
          ),
        ).toBeInTheDocument();
        for (const other of basicTools.filter((tool) => tool !== name)) {
          expect(
            dialog.queryByText(
              i18n.t(`settings.mcpServer.coreToolEffects.${other}`),
            ),
          ).not.toBeInTheDocument();
        }
        const keep = dialog.getByRole("button", {
          name: i18n.t("settings.mcpServer.keepCoreTools"),
        });
        expect(keep).toHaveFocus();
        await user.click(keep);
        expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
        expect(checkbox).toBeChecked();
        await user.click(checkbox);
        await user.click(
          screen.getByRole("button", {
            name: i18n.t("settings.mcpServer.disableCoreTools"),
          }),
        );
        expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
        expect(checkbox).not.toBeChecked();
        for (const other of available.filter((tool) => tool !== name)) {
          expect(
            screen.getByRole("checkbox", {
              name: i18n.t(`tools.names.${other}`),
            }),
          ).toBeChecked();
        }
      },
    );

    it("keeps tools enabled on Enter or Escape and allows keyboard confirmation", async () => {
      const { i18n, user } = await setup();
      const read = screen.getByRole("checkbox", {
        name: i18n.t("tools.names.read"),
      });
      await user.click(read);
      expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
      await user.keyboard("{Enter}");
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      expect(read).toBeChecked();
      await waitFor(() => expect(read).toHaveFocus());
      await user.click(read);
      await screen.findByRole("alertdialog");
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      expect(read).toBeChecked();
      await user.click(read);
      await screen.findByRole("alertdialog");
      await user.tab();
      expect(
        screen.getByRole("button", {
          name: i18n.t("settings.mcpServer.disableCoreTools"),
        }),
      ).toHaveFocus();
      await user.keyboard("{Enter}");
      expect(read).not.toBeChecked();
    });

    it("confirms a whole group once and preserves selections in other domains", async () => {
      const { i18n, user } = await setup();
      const files = within(
        screen.getByRole("group", {
          name: i18n.t("tools.groups.files"),
        }),
      );
      const group = files.getByRole("checkbox", {
        name: i18n.t("settings.mcpServer.selectGroup", {
          group: i18n.t("tools.groups.files"),
        }),
      });
      await user.click(group);
      expect(screen.getAllByRole("alertdialog")).toHaveLength(1);
      const dialog = within(screen.getByRole("alertdialog"));
      for (const name of ["read", "edit"]) {
        expect(
          dialog.getByText(
            i18n.t(`settings.mcpServer.coreToolEffects.${name}`),
          ),
        ).toBeInTheDocument();
      }
      expect(
        dialog.queryByText(
          i18n.t("settings.mcpServer.coreToolEffects.get_chapter_list"),
        ),
      ).not.toBeInTheDocument();
      expect(group).toBeChecked();
      await user.click(
        dialog.getByRole("button", {
          name: i18n.t("settings.mcpServer.disableCoreTools"),
        }),
      );
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      expect(group).not.toBeChecked();
      for (const name of ["get_chapter_list", "delete_record"]) {
        expect(
          screen.getByRole("checkbox", {
            name: i18n.t(`tools.names.${name}`),
          }),
        ).toBeChecked();
      }
    });

    it("does not prompt for existing disabled tools, enabling tools, or disabling other tools", async () => {
      const { i18n, user } = await setup(["delete_record"]);
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      for (const name of basicTools) {
        const checkbox = screen.getByRole("checkbox", {
          name: i18n.t(`tools.names.${name}`),
        });
        await user.click(checkbox);
        expect(checkbox).toBeChecked();
        expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      }
      const deletion = screen.getByRole("checkbox", {
        name: i18n.t("tools.names.delete_record"),
      });
      await user.click(deletion);
      expect(deletion).not.toBeChecked();
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  },
);
