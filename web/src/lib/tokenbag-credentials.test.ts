import { describe, it, expect, beforeEach } from "vitest";
import {
  loadCredential,
  saveCredential,
  clearCredential,
  isDismissed,
  setDismissed,
} from "./tokenbag-credentials";

const CODE = "abc123";

describe("tokenbag credentials", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("returns null when nothing is stored", () => {
    expect(loadCredential(CODE)).toBeNull();
  });

  it("round-trips a credential", () => {
    saveCredential(CODE, {
      registrationId: "42",
      secret: "s3cret",
      name: "Alice",
    });
    expect(loadCredential(CODE)).toEqual({
      registrationId: "42",
      secret: "s3cret",
      name: "Alice",
    });
  });

  it("stores a bigint registration id as a string", () => {
    saveCredential(CODE, { registrationId: 42n, secret: "s", name: "Alice" });
    const cred = loadCredential(CODE);
    expect(cred?.registrationId).toBe("42");
    expect(typeof cred?.registrationId).toBe("string");
  });

  it("keeps credentials isolated per code", () => {
    saveCredential("one", { registrationId: "1", secret: "a", name: "Alice" });
    saveCredential("two", { registrationId: "2", secret: "b", name: "Bob" });
    expect(loadCredential("one")?.name).toBe("Alice");
    expect(loadCredential("two")?.name).toBe("Bob");
    clearCredential("one");
    expect(loadCredential("one")).toBeNull();
    expect(loadCredential("two")?.name).toBe("Bob");
  });

  it("uses a code-scoped key", () => {
    saveCredential(CODE, { registrationId: "1", secret: "a", name: "Alice" });
    expect(localStorage.getItem(`clockkeeper_player:${CODE}`)).not.toBeNull();
  });

  it("tolerates corrupt JSON", () => {
    localStorage.setItem(`clockkeeper_player:${CODE}`, "{not json");
    expect(loadCredential(CODE)).toBeNull();
  });

  it("rejects a credential without a usable secret or id", () => {
    localStorage.setItem(`clockkeeper_player:${CODE}`, JSON.stringify({}));
    expect(loadCredential(CODE)).toBeNull();
    localStorage.setItem(
      `clockkeeper_player:${CODE}`,
      JSON.stringify({ registrationId: "1", secret: "" }),
    );
    expect(loadCredential(CODE)).toBeNull();
    localStorage.setItem(
      `clockkeeper_player:${CODE}`,
      JSON.stringify({ secret: "a" }),
    );
    expect(loadCredential(CODE)).toBeNull();
  });

  it("defaults a missing name to an empty string", () => {
    localStorage.setItem(
      `clockkeeper_player:${CODE}`,
      JSON.stringify({ registrationId: 7, secret: "a" }),
    );
    expect(loadCredential(CODE)).toEqual({
      registrationId: "7",
      secret: "a",
      name: "",
    });
  });

  it("round-trips the dismissed flag per code", () => {
    expect(isDismissed(CODE)).toBe(false);
    setDismissed(CODE, true);
    expect(isDismissed(CODE)).toBe(true);
    expect(isDismissed("other")).toBe(false);
    setDismissed(CODE, false);
    expect(isDismissed(CODE)).toBe(false);
  });

  it("clearing a credential clears the dismissed flag", () => {
    saveCredential(CODE, { registrationId: "1", secret: "a", name: "Alice" });
    setDismissed(CODE, true);
    clearCredential(CODE);
    expect(loadCredential(CODE)).toBeNull();
    expect(isDismissed(CODE)).toBe(false);
  });
});
