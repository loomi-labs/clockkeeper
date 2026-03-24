import { test, expect } from "@playwright/test";

test("login page shows form", async ({ page }) => {
  await page.goto("/login");

  await expect(page.locator("h1")).toContainText("Clock Keeper");
  await expect(page.locator("input#username")).toBeVisible();
  await expect(page.locator("input#password")).toBeVisible();
  await expect(page.locator('button[type="submit"]')).toContainText("Sign In");
});
