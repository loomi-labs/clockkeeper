import { describe, it, expect } from "vitest";
import {
  isPublicPath,
  PUBLIC_PATH_PREFIXES,
  PUBLIC_EXACT_PATHS,
} from "./public-paths";

describe("isPublicPath", () => {
  const cases: [string, boolean][] = [
    // Token bag entry points.
    ["/join/abc", true],
    ["/join/", true],
    ["/join", true],
    ["/joinery", false],
    ["/joinery/abc", false],
    ["/device/x", true],
    ["/device", true],
    ["/devices", false],
    // Unchanged auth behavior.
    ["/login", true],
    ["/login/", true],
    ["/auth/discord/callback", true],
    ["/auth/", true],
    ["/auth", false],
    // Everything else needs a session.
    ["/", false],
    ["/games/1", false],
    ["/scripts", false],
    ["", false],
  ];

  for (const [pathname, expected] of cases) {
    it(`${pathname || "(empty)"} -> ${expected}`, () => {
      expect(isPublicPath(pathname)).toBe(expected);
    });
  }

  it("treats every listed prefix and exact path as public", () => {
    for (const prefix of PUBLIC_PATH_PREFIXES) {
      expect(isPublicPath(prefix)).toBe(true);
    }
    for (const path of PUBLIC_EXACT_PATHS) {
      expect(isPublicPath(path)).toBe(true);
    }
  });
});
