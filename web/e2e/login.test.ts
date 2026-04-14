import { test, expect } from "@playwright/test";

test("login page shows auth options", async ({ page }) => {
  await page.goto("/login");

  await expect(page.locator("h1")).toContainText("Clock Keeper");
  await expect(
    page.locator("button", { hasText: "Continue without account" }),
  ).toBeVisible();
});
