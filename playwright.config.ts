import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
    testDir: "./browser-tests",
    fullyParallel: false,
    workers: 1,
    timeout: 30000,
    use: { baseURL: "http://127.0.0.1:8787", trace: "retain-on-failure", screenshot: "only-on-failure" },
    projects: [
        { name: "desktop", use: { ...devices["Desktop Chrome"] } },
        { name: "phone", use: { ...devices["Pixel 7"] } },
    ],
    webServer: {
        command: "go test ./handlers -run '^TestBrowserFixtureServer$' -count=1 -timeout=10m -v",
        env: { BROWSER_TEST_SERVER: "1", APP_ENV: "test", SECURE_COOKIES: "0", DATABASE_URL: "" },
        url: "http://127.0.0.1:8787/customer/login",
        timeout: 120000,
        reuseExistingServer: false,
    },
});
