import { describe, it, expect, beforeEach } from "vitest";
import { getToken, setToken, clearToken, initAuth, auth } from "./auth.svelte";

describe("auth", () => {
  beforeEach(() => {
    localStorage.clear();
    clearToken();
  });

  it("getToken returns null when no token set", () => {
    expect(getToken()).toBeNull();
  });

  it("setToken stores and retrieves token", () => {
    setToken("my-jwt-token");
    expect(getToken()).toBe("my-jwt-token");
    expect(auth.isAuthenticated).toBe(true);
  });

  it("clearToken removes token", () => {
    setToken("my-jwt-token");
    clearToken();
    expect(getToken()).toBeNull();
    expect(auth.isAuthenticated).toBe(false);
  });

  it("initAuth sets authenticated when token exists", () => {
    localStorage.setItem("clockkeeper_token", "existing-token");
    initAuth();
    expect(auth.isAuthenticated).toBe(true);
  });

  it("initAuth sets unauthenticated when no token", () => {
    initAuth();
    expect(auth.isAuthenticated).toBe(false);
  });
});
