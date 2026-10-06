import { expect, test, Page } from "@playwright/test";

async function signIn(page: Page) {
    await page.goto("/customer/login");
    await page.locator('input[name="email"]').fill("browser@example.com");
    await page.locator('input[name="password"]').fill("Browser-pass-123");
    await page.getByRole("button", { name: "Sign In", exact: true }).click();
    await expect(page).toHaveURL(/\/customer\/dashboard$/);
    await page.goto("/customer/barbers?state=Lagos&area=Ikeja");
}

test("profile photo navigates and preserves locality", async ({ page }) => {
    await signIn(page);
    await page.locator(".barber-image img").click();
    await expect(page).toHaveURL(/\/customer\/barbers\/\d+\?state=Lagos&area=Ikeja/);
    await expect(page.getByRole("dialog")).toBeHidden();
    await expect(page.locator("[data-booking-summary]")).toContainText("Browser Cut · ₦1250.00 · 30 minutes");
});

test("keyboard modal isolates focus and Escape restores the opener", async ({ page }) => {
    await signIn(page);
    const opener = page.getByRole("button", { name: "Enlarge photo of Browser Shop" });
    await opener.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog", { name: "Full-size photo" });
    const close = page.getByRole("button", { name: "Close image" });
    await expect(dialog).toBeVisible();
    await expect(close).toBeFocused();
    await page.locator(".barber-image").evaluate((link: HTMLElement) => link.focus());
    await expect(close).toBeFocused();
    for (let i = 0; i < 3; i++) {
        await page.keyboard.press("Tab");
        await expect(opener).not.toBeFocused();
        await expect(page.locator(".barber-image")).not.toBeFocused();
    }
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(opener).toBeFocused();
    await expect(page.locator("body")).not.toHaveClass(/image-viewer-open/);
});

test("close and backdrop dismiss while image clicks keep the viewer open", async ({ page }) => {
    await signIn(page);
    const opener = page.getByRole("button", { name: "Enlarge photo of Browser Shop" });
    const dialog = page.getByRole("dialog", { name: "Full-size photo" });
    await opener.click();
    await dialog.locator("img").click();
    await expect(dialog).toBeVisible();
    await page.getByRole("button", { name: "Close image" }).click();
    await expect(dialog).toBeHidden();
    await expect(opener).toBeFocused();
    await opener.click();
    await dialog.click({ position: { x: 5, y: 5 } });
    await expect(dialog).toBeHidden();
    await expect(opener).toBeFocused();
});

test("service selection and enlargement are separate controls", async ({ page }) => {
    await signIn(page);
    await page.locator(".barber-image img").click();
    const radio = page.locator('input[name="hairstyle_id"]');
    await radio.evaluate((input: HTMLInputElement) => { input.checked = false; });
    await page.locator("label .style-image img").click();
    await expect(radio).toBeChecked();
    await expect(page.getByRole("dialog")).toBeHidden();
    await page.getByRole("button", { name: "Enlarge photo: Photo of Browser Cut" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(radio).toBeChecked();
});

test.describe("without JavaScript", () => {
    test.use({ javaScriptEnabled: false });
    test("navigation and booking summary work", async ({ page }) => {
        await signIn(page);
        await expect(page.getByRole("button", { name: "Enlarge photo of Browser Shop" })).toBeHidden();
        await page.locator(".barber-image img").click();
        await expect(page.locator("[data-booking-summary]")).toContainText("Browser Cut · ₦1250.00 · 30 minutes");
        await expect(page.locator('input[name="hairstyle_id"]')).toBeChecked();
    });
});
