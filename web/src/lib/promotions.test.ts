import { describe, it, expect } from "vitest";
import { Team } from "~/lib/gen/clockkeeper/v1/clockkeeper_pb";
import { buildPromotionsByRole, type PromotionInput } from "./promotions";

type Char = { id: string; name: string; edition: string; team: Team };

function char(id: string, overrides: Partial<Char> = {}): Char {
  return { id, name: id, edition: "tb", team: Team.TOWNSFOLK, ...overrides };
}

function lookup(chars: Char[]): Map<string, Char> {
  return new Map(chars.map((c) => [c.id, c]));
}

describe("buildPromotionsByRole", () => {
  const chars = lookup([
    char("baron", { name: "Baron", team: Team.MINION }),
    char("scarletwoman", { name: "Scarlet Woman", team: Team.MINION }),
    char("imp", { name: "Imp", team: Team.DEMON }),
  ]);

  it('builds the "(ex …)" label from the original role name', () => {
    const promos: PromotionInput[] = [{ roleId: "baron", actsAsRoleId: "imp" }];
    const byRole = buildPromotionsByRole(promos, chars);
    expect(byRole.get("baron")?.label).toBe("Imp (ex Baron)");
  });

  it("resolves edition/team from the acts-as character, not the original", () => {
    const byRole = buildPromotionsByRole(
      [{ roleId: "baron", actsAsRoleId: "imp" }],
      chars,
    );
    expect(byRole.get("baron")).toEqual({
      actsAsId: "imp",
      actsAsName: "Imp",
      actsAsEdition: "tb",
      actsAsTeam: Team.DEMON,
      label: "Imp (ex Baron)",
    });
  });

  it("handles multiple entries independently", () => {
    const byRole = buildPromotionsByRole(
      [
        { roleId: "baron", actsAsRoleId: "imp" },
        { roleId: "scarletwoman", actsAsRoleId: "imp" },
      ],
      chars,
    );
    expect(byRole.size).toBe(2);
    expect(byRole.get("baron")?.label).toBe("Imp (ex Baron)");
    expect(byRole.get("scarletwoman")?.label).toBe("Imp (ex Scarlet Woman)");
  });

  it("supports a chained promotion (acts-as of one is the original of another)", () => {
    // baron acts as scarletwoman, scarletwoman acts as imp.
    const byRole = buildPromotionsByRole(
      [
        { roleId: "baron", actsAsRoleId: "scarletwoman" },
        { roleId: "scarletwoman", actsAsRoleId: "imp" },
      ],
      chars,
    );
    expect(byRole.get("baron")).toMatchObject({
      actsAsId: "scarletwoman",
      actsAsTeam: Team.MINION,
      label: "Scarlet Woman (ex Baron)",
    });
    expect(byRole.get("scarletwoman")).toMatchObject({
      actsAsId: "imp",
      actsAsTeam: Team.DEMON,
      label: "Imp (ex Scarlet Woman)",
    });
  });

  it("skips an entry whose acts-as character is unknown", () => {
    const byRole = buildPromotionsByRole(
      [{ roleId: "baron", actsAsRoleId: "ghost" }],
      chars,
    );
    expect(byRole.has("baron")).toBe(false);
    expect(byRole.size).toBe(0);
  });

  it("falls back to the role id when the original character is unknown", () => {
    const byRole = buildPromotionsByRole(
      [{ roleId: "mystery", actsAsRoleId: "imp" }],
      chars,
    );
    expect(byRole.get("mystery")?.label).toBe("Imp (ex mystery)");
  });
});
